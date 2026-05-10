package encounter

import (
	"errors"

	"github.com/KirkDiggler/rpg-toolkit/core"
	encountercore "github.com/KirkDiggler/rpg-toolkit/encounter/core"
)

// ErrNoCombatResolver is returned by combat verbs when the encounter was
// constructed without a CombatResolver. The orchestrator (rpg-api) wires
// one via WithCombatResolver; tests stub it. Without one, the encounter
// SDK has no way to evaluate attack mechanics — it does not embed
// rulebook logic itself.
var ErrNoCombatResolver = errors.New("no combat resolver wired")

// CombatResolver bridges the encounter SDK to a rulebook implementation
// of combat mechanics. The encounter SDK orchestrates state and flow
// (initiative, turn cycling, per-viewer event projection) but delegates
// "did this attack hit and how much damage" to the resolver.
//
// The orchestrator (rpg-api) implements this interface against a specific
// rulebook (today: dnd5e). The encounter SDK never imports rulebook
// packages — keeping the boundary clean across rulebooks.
//
// Wave 2.11a wires ResolveAttack for the player-attack path
// (combat.TakeAction). NPCAct continues to use the legacy stand-in path
// until Wave 2.11b adds monster rulebook adapters and OA / multiattack
// support; method additions on this interface in 2.11b are explicit
// breaking changes flagged in the encounter module's release notes.
type CombatResolver interface {
	// ResolveAttack evaluates one attack from attacker to target and
	// returns the outcome. The resolver is responsible for looking up
	// rich rulebook state (ability scores, weapon, AC chain) from
	// whatever store the orchestrator wires in; the encounter SDK only
	// passes IDs and the action ref.
	//
	// The resolver MUST NOT mutate encounter state directly. The
	// encounter SDK applies HP changes + publishes events from the
	// returned AttackOutcome.
	ResolveAttack(input AttackInput) (*AttackOutcome, error)
}

// AttackInput is the encounter-SDK-side request shape for ResolveAttack.
// The orchestrator's resolver implementation translates this to the
// rulebook's native AttackInput shape (e.g., dnd5e/combat.AttackInput).
type AttackInput struct {
	// AttackerID is the entity making the attack — could be a player
	// (combat.TakeAction path) or a monster (future NPCAct migration).
	AttackerID encountercore.EntityID

	// TargetID is the entity being attacked.
	TargetID encountercore.EntityID

	// ActionRef identifies the action being taken. For Wave 2.11a this
	// is always {Module:"dnd5e", Type:"action", ID:"attack"}; future
	// waves extend the action vocabulary.
	ActionRef core.Ref

	// AttackHand is the optional hand for two-weapon fighting. Empty or
	// "main" defaults to main-hand; "off" triggers off-hand validation.
	// The resolver implementation interprets this against the rulebook's
	// AttackHand vocabulary.
	AttackHand string

	// Combat-stat snapshots, populated by the encounter SDK from
	// PlayerData / MonsterData at attack time. Stand-in resolver
	// implementations use these directly to compute d20+bonus vs AC.
	// Real rulebook resolver implementations may ignore these and look
	// up richer state (ability scores, AC chain, equipped weapon) from
	// their own character/monster store.
	AttackerAttackBonus int
	AttackerDamageDice  string
	AttackerDamageType  string
	TargetAC            int
}

// AttackOutcome is the encounter-SDK-side result shape from ResolveAttack.
// The resolver implementation translates the rulebook's native result
// (e.g., dnd5e/combat.AttackResult) to this shape. The encounter SDK uses
// it to publish AttackResolvedEvent + (on hit) DamageDealtEvent and to
// mutate target HP.
type AttackOutcome struct {
	// Hit is true if the attack landed.
	Hit bool

	// Critical is true if the attack landed as a critical hit.
	Critical bool

	// AttackRoll is the final d20 attack roll (after advantage /
	// disadvantage resolution).
	AttackRoll int

	// AttackBonus is the total bonus added to the attack roll.
	AttackBonus int

	// TargetAC is the target's effective AC at the moment of the
	// attack (after the AC chain — Shield spell, cover, etc.).
	TargetAC int

	// Damage is the final damage dealt on a hit. Zero on miss.
	Damage int

	// DamageType identifies the kind of damage dealt (e.g.,
	// "slashing", "fire"). Empty on miss; encounter SDK falls back
	// to the entity's stored damage type if the resolver leaves it
	// empty on a hit.
	DamageType string
}

// toolkitRef converts the encounter SDK's local ActionRef shape (plain
// strings) to the toolkit-canonical core.Ref shape (typed Module/Type/ID).
// The resolver implementation receives the canonical shape so it can pass
// it directly to rulebook lookups without re-parsing.
func toolkitRef(r ActionRef) core.Ref {
	return core.Ref{
		Module: r.Module,
		Type:   r.Type,
		ID:     r.ID,
	}
}
