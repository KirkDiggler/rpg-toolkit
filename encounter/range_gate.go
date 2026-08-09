package encounter

// range_gate.go — the shared range/reach validation gate for rpg-toolkit#864.
//
// A 2026-07-30 QA walk found NO range/reach validation anywhere in the
// action-resolution path: Interact opened (and unlocked) doors from 10-16
// hexes away, and a 1-hex-reach melee weapon landed a hit from 3 hexes.
// Positions were already tracked (tools/spatial via encounter/core.Hex +
// hexDistance) — the action paths simply never consulted them.
//
// Kirk's spec (the issue body): one validation gate at action resolution,
// applying to ALL actors (players and monsters). Interact/door actions
// require adjacency (reach 1); melee attacks use weapon reach (default 1
// hex); ranged attacks use the weapon's range. Range is a parameter of the
// action, not an exception.
//
// This file is the single shared gate every call site below consults:
//   - OpenDoor / AttemptUnlock (encounter.go / prompts.go): checkInteractReach
//   - TakeActionPhased, the player attack path (combat_phased.go):
//     (*Encounter).checkAttackRange
//   - applyCapturedAttacks / npcActScripted, the monster attack paths
//     (npc.go): meleeReachForCombatant + checkReach directly
//
// Deliberately NOT touched: encounter/combat.go's SetMode/setMode/
// rollInitiative/checkCombatEntry/seedActorTurn and the Move path — those
// are rpg-toolkit#867's (in-flight, PR #867) initiative-ordering surface;
// this gate lives entirely in new call sites plus the door/attack verbs
// listed above, none of which #867 touches.

import (
	"errors"
	"fmt"

	toolkitcore "github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/encounter/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

// ErrOutOfRange is returned by action verbs (Interact-class door verbs, and
// both the player and NPC attack paths) when the target is farther from the
// actor than the action's reach/range permits. rpg-toolkit#864: range is a
// parameter of the action, not an exception — every gated verb wraps this
// same sentinel so callers (and rpg-api's status-code mapping) can recognize
// the refusal uniformly regardless of which action produced it.
var ErrOutOfRange = errors.New("target out of range")

const (
	// interactReachHexes is the fixed adjacency reach for Interact-class
	// verbs (OpenDoor, AttemptUnlock). Doors have no weapon/reach concept of
	// their own — the issue's spec is explicit: "Interact requires
	// adjacency (reach 1)".
	interactReachHexes = 1

	// defaultMeleeReachHexes is the melee reach used when the attacker has
	// no real weapon to consult: a flat stat-snapshot combatant (today's
	// common case for UI-created characters — a known separate bug that
	// leaves them unarmed), a monster's natural weapon (bite/claw — no
	// catalog entry to resolve), or a weapon-resolution error. 1 hex (5ft)
	// is the D&D 5e default and the reach of the canonical unarmed strike.
	defaultMeleeReachHexes = 1

	// reachWeaponHexes is the melee reach granted by the Reach weapon
	// property (10ft — glaive, halberd, spear-with-reach-variant, pike).
	reachWeaponHexes = 2
)

// checkReach is the shared distance gate underlying every range-gated verb
// in this file: an error (wrapping ErrOutOfRange) when hexDistance(from, to)
// exceeds max, nil otherwise. label distinguishes "reach" (melee/interact,
// a fixed adjacency-style limit) from "range" (ranged weapons, a genuine
// distance limit) in the error text — mirrors the existing precondition
// style (e.g. ErrInsufficientMovement's callers in encounter.go).
func checkReach(from, to core.Hex, limit int, label string) error {
	if d := hexDistance(from, to); d > limit {
		return fmt.Errorf("%w: %d hexes, %s %d", ErrOutOfRange, d, label, limit)
	}
	return nil
}

func (e *Encounter) hasClearStructuralReach(from, to core.Hex) bool {
	if e.room == nil {
		return true
	}
	grid := e.room.GetGrid()
	fromPosition, toPosition := from.ToPosition(), to.ToPosition()
	if !grid.IsValidPosition(fromPosition) || !grid.IsValidPosition(toPosition) {
		return false
	}
	for _, position := range grid.GetLineOfSight(fromPosition, toPosition) {
		if !grid.IsValidPosition(position) {
			return false
		}
	}
	return true
}

// checkInteractReach gates legacy generated-door verbs on the actor being
// adjacent to the door's single blocked cell.
func checkInteractReach(actorPos, doorPos core.Hex) error {
	return checkReach(actorPos, doorPos, interactReachHexes, "reach")
}

// checkDoorInteractReach applies the correct reach semantics for either door
// representation. Generated connectors retain their legacy adjacency gate;
// an authored door is a boundary rather than a cell, so the actor must occupy
// one of its two incident endpoints to interact from that side.
func (e *Encounter) checkDoorInteractReach(actorPos core.Hex, door *DoorData) error {
	if edge, authored := e.authoredDoorEdge(door.ID); authored {
		if actorPos == edge.From || actorPos == edge.To {
			return nil
		}
		return fmt.Errorf("%w: authored door requires an incident endpoint", ErrOutOfRange)
	}
	return checkInteractReach(actorPos, door.Position)
}

// meleeReachForWeapon resolves the reach (in hexes) w grants: 2 for a Reach
// weapon, 1 otherwise — including w == nil, the "no weapon resolved" case
// (flat stat-snapshot combatant, natural weapon, or a lookup miss).
func meleeReachForWeapon(w *weapons.Weapon) int {
	if w != nil && w.HasProperty(weapons.PropertyReach) {
		return reachWeaponHexes
	}
	return defaultMeleeReachHexes
}

// meleeReachForCombatant resolves melee reach via combat.MeleeWeaponProvider
// — the same seam opportunity-attack resolution already uses
// (rpg-toolkit#722) to answer "which weapon would this combatant swing."
// Used by the monster attack path (applyCapturedAttacks, npcActScripted),
// which has no per-ActionRef weapon selection the way the player path's
// Character.WeaponForActionRef gives (see checkAttackRange). Falls back to
// defaultMeleeReachHexes when attacker doesn't implement the provider (a
// nil combat.Combatant included) or reports no weapon (natural attacks like
// bite/claw, or a flat stat-snapshot monster).
func meleeReachForCombatant(attacker combat.Combatant) int {
	provider, ok := attacker.(combat.MeleeWeaponProvider)
	if !ok {
		return defaultMeleeReachHexes
	}
	return meleeReachForWeapon(provider.MeleeWeapon())
}

// checkAttackRange gates a player attack (TakeActionPhased) on the target
// being within reach (melee) or range (ranged) of the weapon this specific
// ActionRef actually swings. Resolves the real weapon via
// Character.WeaponForActionRef (rpg-toolkit#712 — the same ref->weapon
// mapping the real combat resolver uses), so an off_hand_strike or
// unarmed_strike/flurry_strike gates on the weapon it will actually attack
// with rather than always the main-hand weapon. A player with no hydrated
// character (a flat stat-snapshot seat) has no weapon to consult and is
// gated as melee at the default reach — see defaultMeleeReachHexes.
//
// Ranged weapon range is checked against the weapon's LONG range (not
// Normal) — D&D 5e RAW imposes disadvantage beyond Normal range, not a hard
// block; only beyond Long range is the attack impossible. weapons.Weapon.Range
// is stored in feet; combat.FeetPerGridUnit converts to the encounter SDK's
// hex grid.
func (e *Encounter) checkAttackRange(player *PlayerData, targetPos core.Hex, ref *toolkitcore.Ref) error {
	attackerPos := player.View.Position

	var w *weapons.Weapon
	if char := e.heldCharacter(player.EntityID); char != nil {
		if sel, err := char.WeaponForActionRef(ref); err == nil && sel != nil {
			w = sel.Weapon
		}
	}

	// Gate-review note (rpg-toolkit#864 minor item): deliberately keyed on
	// w.IsRanged() (the weapon's Category), NOT "w.Range != nil". A thrown
	// weapon (dagger/handaxe/javelin: CategorySimpleMelee + PropertyThrown)
	// carries real Range data in the catalog, but the only ActionRef this
	// gate sees today is the standard melee "attack" (or a granted strike) —
	// there is no distinct "thrown_attack" ref yet, so WeaponForActionRef's
	// default case always resolves the ordinary melee swing for a dagger,
	// not a throw. Keying on Range!=nil instead of category would silently
	// gate a normal melee dagger swing on its 60ft THROWN range rather than
	// 5ft melee reach — wrong for the common case (any rogue swinging a
	// dagger) to fix a case that can't happen yet (no thrown ref exists to
	// route here). When a thrown-attack ref is added, IT should consult
	// w.Range explicitly for its own gating rather than this default case
	// being widened to guess at intent from the weapon's stats alone.
	if w != nil && w.IsRanged() && w.Range != nil {
		rangeHexes := w.Range.Long / int(combat.FeetPerGridUnit)
		return checkReach(attackerPos, targetPos, rangeHexes, "range")
	}
	return checkReach(attackerPos, targetPos, meleeReachForWeapon(w), "reach")
}
