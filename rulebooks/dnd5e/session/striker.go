// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resolution"
)

// strikerSeam is this package's real encounter.Striker, bound to the live
// scope.enc a write verb is already operating on (rpg-project#254: "supply
// Striker from the strike machinery bound to the live scope.enc").
//
// A STRUCT HOLDING A *writeScope RATHER THAN A CLOSURE OVER ONE, so it can
// be constructed BEFORE scope.enc exists — encounter.LoadEncounter needs a
// Striker to construct the very *encounter.Encounter this seam's own Strike
// method later receives as a parameter, so scope is allocated with its data
// (and later enc) fields set in the order openForWrite needs, and this seam
// only ever reads scope AT CALL TIME, well after that ordering is settled.
//
// NO SECOND LOAD: attacker and target sheets come out of scope.data.NPCs
// and m.characters exactly as castFor already reads them for a player's own
// swing — this is the identical machinery, reached from the other
// direction.
type strikerSeam struct {
	m     *Manager
	scope *writeScope
}

// compile-time proof the seam satisfies what it is handed to.
var _ encounter.Striker = strikerSeam{}

// Strike resolves attacker's declared action against target and records the
// outcome itself via enc.Record — the same public verb Manager.Attack's own
// swing already uses, so a monster's blow and a player's land on the story
// through one path.
//
// NEVER ADOPTS A NEW *encounter.Encounter. enc is the SAME live object
// driveMonsterTurns is mid-loop over — swapping it out from inside a call
// enc itself made would orphan that loop. The world resolution.Resolve
// returns is read for its dirty sheets alone (out.DirtyCharacters,
// out.DirtyMonsters) and otherwise discarded; hit points and conditions
// live on SHEETS, not on the encounter's own story-and-position state, so
// nothing about them requires swapping enc.
//
// Attacker is always a monster: nothing outside this package's own
// TurnDriver-driven turns ever reaches this method, and only an unplayed
// member — always KindMonster today — has a TurnDriver in the first place.
func (s strikerSeam) Strike(
	ctx context.Context, enc *encounter.Encounter, attacker, target encounter.MemberID, action core.Ref,
) error {
	var attackerData *monster.Data
	for i := range s.scope.data.NPCs {
		if s.scope.data.NPCs[i].ID == string(attacker) {
			attackerData = &s.scope.data.NPCs[i]
			break
		}
	}
	if attackerData == nil {
		return fmt.Errorf("strike: attacker %q: %w", attacker, ErrNoSheet)
	}

	var definition combatActions.Definition
	var found bool
	for i := range attackerData.Actions {
		if attackerData.Actions[i].Ref == action {
			definition = attackerData.Actions[i].Clone()
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("strike: attacker %q: action %q: %w", attacker, action.String(), ErrBadAttack)
	}

	roster, err := enc.Members()
	if err != nil {
		return fmt.Errorf("strike: %w", translate(err))
	}

	// No readied sheet: a monster attacker has no action-economy readying
	// step the way a character's own swing does (Manager.priceSwing). If
	// persisted content declares a non-nil cost anyway, resolution sees it at
	// the same door and refuses the monster payer rather than silently
	// inventing monster economy here.
	cast, err := s.m.castFor(ctx, s.scope, roster, nil)
	if err != nil {
		return fmt.Errorf("strike: %w", err)
	}

	machine, err := resolution.NewAction(&resolution.ActionInput{
		Definition: definition,
		AttackerID: string(attacker),
		TargetID:   string(target),
		Roller:     &diceSeam{roller: s.m.dice},
	})
	if err != nil {
		return fmt.Errorf("strike: attacker %q: %w: %v", attacker, ErrBadAttack, err)
	}

	var cost *resolution.Cost
	if definition.Cost != nil {
		cost = &resolution.Cost{PayerID: string(attacker), Profile: definition.Cost}
	}

	// A pure view for resolution's Input.World — a mid-verb read, never the
	// storage boundary (encounter v0.43.0, #1385).
	world := enc.WorldView()
	out, err := resolution.Resolve(ctx, &resolution.Input{
		World:        world,
		Participants: cast,
		Initiative:   s.m.initiative,
		Standing:     s.scope.standing,
		Sight:        &sightSeam{members: append([]encounter.MemberData(nil), world.Members...)},
		TurnDriver:   s.m.turnDriver,
		// The concealment pair (rpg-toolkit#1378), bound to the same live
		// scope openForWrite and adopt bind — the one-seam consistency law:
		// a concealed world refuses to reconstruct without them, and
		// resolution carries them without consulting either, since no verb
		// runs inside an interaction.
		CheckResolver: checkSeam(s),
		Witness:       witnessSeam{scope: s.scope},
		Cost:          cost,
		Machine:       machine,
		Roller:        &diceSeam{roller: s.m.dice},
	})
	if err != nil {
		return fmt.Errorf("strike: %w", translateResolution(err))
	}

	struck, ok := out.Outcome.(resolution.StrikeOutcome)
	if !ok {
		return fmt.Errorf("strike: %w: strike produced %T", ErrInvalidWorld, out.Outcome)
	}

	if err := s.m.saveDirty(ctx, s.scope, out); err != nil {
		return fmt.Errorf("strike: %w", err)
	}

	in := &AttackInput{Attacker: string(attacker), Target: string(target)}
	if _, err := enc.Record(recordFor(in, struck, definition)); err != nil {
		return fmt.Errorf("strike: %w", translate(err))
	}
	return nil
}
