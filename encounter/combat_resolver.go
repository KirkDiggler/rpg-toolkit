package encounter

import (
	"errors"

	"github.com/KirkDiggler/rpg-toolkit/core"
	encountercore "github.com/KirkDiggler/rpg-toolkit/encounter/core"
	dnd5events "github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
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
// Wave 2.11a wired ResolveAttack for the player-attack path (TakeAction).
// Wave 2.11b (encounter v0.7.0) folds NPCAct onto the same resolver — both
// player and monster attack paths share this single injection seam. The
// in-package resolveAttack helper has been removed; hosts that previously
// relied on it must wire a resolver via WithCombatResolver.
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

// PhasedCombatResolver extends CombatResolver with discrete RPC-phase verbs
// for two-phase attack resolution. The encounter SDK uses these verbs when
// an attack triggers reactions (Wave 2.11d).
//
// PhasedCombatResolver is an OPTIONAL interface — resolvers that implement
// it enable two-phase + reaction-prompt flows. Resolvers that only implement
// CombatResolver continue to work for single-phase attacks.
//
// The encounter SDK type-asserts CombatResolver to PhasedCombatResolver in
// TakeAction; absence of the phased interface falls back to the monolithic
// ResolveAttack call (today's behavior).
type PhasedCombatResolver interface {
	CombatResolver

	// ResolveAttackHit runs phase 1 of an attack: rolls the d20, evaluates
	// hit/miss against the target's original AC, and returns an opaque
	// PhasedAttackContext that the orchestrator stores between the two phases.
	//
	// Side effect: condition handlers may publish ReactionTriggerEvents on
	// the encounter bus during the chain. The encounter SDK drains those
	// events via a buffered subscriber installed before this call and
	// returns them to the caller alongside the context.
	ResolveAttackHit(input AttackInput) (*PhasedAttackContext, []ReactionTrigger, error)

	// ApplyAttackOutcome runs phase 2 of an attack: takes the PhasedAttackContext
	// from phase 1 plus any reaction modifiers chosen by reactors and produces
	// the final outcome (re-evaluating hit with modified AC, applying damage).
	ApplyAttackOutcome(ctx *PhasedAttackContext, modifiers []ReactionModifier) (*AttackOutcome, error)
}

// PhasedAttackContext is the opaque state carried between phase 1 and phase 2
// of a two-phase attack. The encounter SDK does NOT inspect its contents —
// it persists it in the encounter snapshot's pending-prompt state and passes
// it back to ApplyAttackOutcome when the reactor responds.
//
// Concretely the resolver implementation wraps a pointer to its rulebook's
// native attack-context type (e.g. *combat.AttackContext for dnd5e). The
// encounter SDK treats it as a black box.
type PhasedAttackContext struct {
	// Rulebook holds the resolver's native attack context. Opaque to the SDK.
	Rulebook any

	// AttackerID + TargetID are mirrored from phase-1 input so the SDK can
	// route reaction prompts and resolve final HP changes without reaching
	// into the opaque Rulebook payload.
	AttackerID encountercore.EntityID
	TargetID   encountercore.EntityID
}

// ReactionTrigger is the encounter-SDK-shaped projection of a rulebook
// ReactionTriggerEvent. The encounter SDK SDK partitions triggers by reactor
// (player vs NPC), surfaces player triggers to the orchestrator (rpg-api),
// and resolves NPC triggers inline by calling back into the resolver.
type ReactionTrigger struct {
	// ReactorID is the entity that can react — used by rpg-api to look up
	// the controlling player and route the InputRequired{reaction_prompt}
	// event onto the reactor's per-viewer stream.
	ReactorID encountercore.EntityID

	// ConditionRef identifies the reaction (e.g.
	// "dnd5e:conditions:opportunity_attack", "dnd5e:spells:shield"). Matches
	// the canonical core.Ref string format module:type:id.
	ConditionRef string

	// TriggerKind identifies which reaction window fired. Mirrors the
	// rulebook's TriggerKind enum (e.g. "post_hit", "movement_oa", "post_damage").
	TriggerKind string

	// SourceEntity is the entity that triggered this reaction opportunity
	// (the attacker for post-hit, the moving entity for OA, etc.).
	SourceEntity encountercore.EntityID

	// Payload carries window-specific context, opaque to the encounter SDK.
	// rpg-api passes it through to the reaction-modifier construction in
	// CompleteTakeAction.
	Payload any
}

// ReactionModifier is the encounter-SDK-shaped projection of a rulebook
// reaction modifier produced when a reactor takes (vs skips) a reaction.
// rpg-api builds these from take_reaction responses and passes them to
// CompleteTakeAction; the encounter SDK forwards them to ApplyAttackOutcome.
type ReactionModifier struct {
	// ConditionRef identifies the reaction (e.g. "dnd5e:spells:shield").
	ConditionRef string

	// ACBonus is the AC increase applied to the target for hit re-evaluation
	// in phase 2. Shield = +5; future reactions may set zero and modify other
	// fields once the modifier shape grows.
	ACBonus int
}

// TakeActionOutcome is the result returned by Encounter.TakeAction in the
// new two-phase model. The orchestrator branches on which fields are set:
//
//   - Resolved=true → phase 1 + phase 2 ran inline; events were published as
//     before. No further action required.
//   - len(Reactions) > 0 → phase 1 published ReactionTriggerEvents for one
//     or more player reactors. AttackContext is the in-flight phase-1 state
//     that must be persisted on the encounter snapshot until the orchestrator
//     calls CompleteTakeAction with the reactors' choices.
//
// Mutually exclusive: exactly one branch is populated per call.
type TakeActionOutcome struct {
	// Resolved is true when phase 1 + phase 2 ran inline (no readied reactions
	// matched, OR all reactions were resolved by NPCs inline).
	Resolved bool

	// Reactions is non-empty when one or more player reactors have a readied
	// reaction whose predicate matched. The orchestrator must surface each
	// trigger as InputRequired{reaction_prompt} on the corresponding reactor's
	// stream and call CompleteTakeAction once the reactors respond.
	Reactions []ReactionTrigger

	// AttackContext is the in-flight phase-1 state to persist between phases.
	// Non-nil iff Reactions is non-empty.
	AttackContext *PhasedAttackContext
}

// AttackInput is the encounter-SDK-side request shape for ResolveAttack.
// The orchestrator's resolver implementation translates this to the
// rulebook's native AttackInput shape (e.g., dnd5e/combat.AttackInput).
type AttackInput struct {
	// AttackerID is the entity making the attack — a player entity ID on
	// the TakeAction path, or a monster entity ID on the NPCAct path.
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

	// EventBus is the encounter-scoped dnd5e event bus. Wave 2.11c passes
	// this through so the resolver implementation can fire the attack chain
	// on the persistent bus (rather than creating a fresh per-attack bus).
	// Conditions subscribed at character rehydration — Sneak Attack,
	// Protection, etc. — observe attacks on this bus throughout the
	// encounter lifetime, enabling once-per-turn semantics to hold.
	//
	// May be nil for resolvers that manage their own bus internally.
	// rpg-api's Dnd5eCombatResolver MUST use this bus when non-nil.
	EventBus dnd5events.EventBus

	// Attacker and Defender are the SDK-held, already-hydrated runtime
	// entities for AttackerID / TargetID. #689: the encounter's LoadFromData
	// cascade hydrated these once and subscribed their conditions to EventBus;
	// the resolver MUST use them and MUST NOT re-load (re-loading on the same
	// bus is the #684 double-subscribe class). Either may be nil when the seat
	// carried no rehydratable DataJSON — in that case the resolver falls back
	// to its stat-snapshot stand-in path using the Attacker* fields above.
	//
	// Typed as combat.Combatant (the interface both *character.Character and
	// *monster.Monster satisfy and the resolver chain looks up via
	// CombatantLookup). The resolver type-asserts to the concrete type when it
	// needs richer surface (e.g. equipped weapon).
	Attacker combat.Combatant
	Defender combat.Combatant
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

	// Components is the optional per-source breakdown of the total Damage.
	// Rulebook resolvers populate this when they have rich breakdown data
	// (e.g. weapon damage + sneak attack). The encounter SDK forwards it to
	// DamageDealtEvent.Components without inspecting it; nil is valid and
	// means "no breakdown available".
	Components []encountercore.DamageComponent
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
