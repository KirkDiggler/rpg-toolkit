// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/play/clock"
	"github.com/KirkDiggler/rpg-toolkit/play/record"
)

// OutcomeKind names a rulebook result the story can carry.
//
// A CLOSED SET, and that is the point of the whole shape. The composition is
// the only thing that can write to an encounter's record, which is what keeps
// one transcript honest — every beat is stamped by the same hand, at the same
// clock, with the same audience rule. Letting a rulebook write whatever it
// liked would keep the transcript in one place and give up the reason that
// mattered: that a reader can trust what is in it.
//
// So a rulebook does not describe what happened. It names a kind the
// composition already knows and hands over numbers. Adding a kind is a change
// here, visible in a diff, and that is the cost being chosen on purpose.
type OutcomeKind string

const (
	// OutcomeStruck is an attack that landed.
	OutcomeStruck OutcomeKind = "struck"

	// OutcomeMissed is an attack that did not.
	OutcomeMissed OutcomeKind = "missed"

	// OutcomeDeathSave is one authoritative tabletop death-save result.
	OutcomeDeathSave OutcomeKind = "death_save"

	// OutcomeDown is a member the rulebook reports out of the fight — the
	// minimal death beat ruled on rpg-toolkit#959: this kind, and who.
	//
	// NOT ACCEPTABLE TO [Encounter.RecordTrain], deliberately, and it is the only
	// kind that is not. Down is something the composition NOTICES, by asking
	// [Standing] at the choke point where it already asks about sight; a caller
	// that could push the beat in would be a second system deciding the same
	// thing, and the first one would always win because it reaches the fact
	// first. prepareRecord's switch is therefore the CALLER-WRITABLE subset of this
	// enum rather than all of it — pushing this kind gets ErrInvalidData.
	//
	// [Encounter.RecordTrain] performing that ask itself (rpg-toolkit#1083) SHARPENS
	// this refusal rather than softening it. The verb a caller uses to report a
	// blow is now the verb that finds out what the blow did — so a caller gets
	// the down beat it wanted, in the same call, and still cannot write one. The
	// beat says what the rulebook answered; it has never said what a caller
	// claimed, and a kind here would be the only way to change that.
	OutcomeDown OutcomeKind = "down"

	// OutcomeBought is one item changing hands from a vendor to an actor —
	// a vendor purchase (rpg-project#369's Trade verb). Named for what the
	// record will say (design.md's own law, already applied to Unpack):
	// "Alice bought a longsword" is a truer statement than the generic
	// "Alice traded for one," the same reasoning that gives OutcomeSold its
	// own kind rather than a flipped flag on this one. Carries TradeDetail
	// the same way OutcomeDeathSave carries DeathSaveDetail: required
	// primitives this composition preserves without interpreting.
	//
	// Named OutcomeTraded through rpg-toolkit#1534; renamed once
	// OutcomeSold's own mirror made the generic word's imprecision visible
	// (rpg-toolkit#1537). OutcomeBartered is reserved — named here, not
	// built — for whenever item-for-item barter lands: it is a third
	// direction, not a fallback either of these two should absorb.
	OutcomeBought OutcomeKind = "bought"

	// OutcomeSold is the mirror of OutcomeBought from the actor's own
	// perspective — a player selling an item to a vendor (rpg-toolkit#1537's
	// Sell, no new verb: Trade's existing Give/Receive shape, the other
	// direction populated). Carries the same TradeDetail shape as
	// OutcomeBought — the data (item type/id/quantity) is identical between
	// the two directions; only the kind differs.
	OutcomeSold OutcomeKind = "sold"

	// OutcomeWarded is an attack a Sanctuary-style ward on the target
	// stopped before any roll against them — the ATTACKER's own failed save
	// against the warding caster's DC. Not a flag on OutcomeMissed: "warded
	// off" and "swung and missed" are different facts a reader should not
	// have to infer from a Warded detail's mere presence beside a struck/
	// missed beat's own fields, all of which stay meaningless here (there
	// was no attack roll). Carries WardedDetail the same way OutcomeDeathSave
	// carries DeathSaveDetail.
	OutcomeWarded OutcomeKind = "warded"

	// OutcomeExperienceGained is experience the party has already been paid,
	// arriving here only to be remembered (rpg-project#496).
	//
	// ONE BEAT PER CAUSE. Today the cause is a monster's fall: its worth is
	// authored on the monster, and the session SDK — which sees the fall at
	// commit time — divides that worth among the players on its roster,
	// writes the sheets, and THEN records this. A later slice pays an
	// authored reward at an ending and will name the ending as its cause
	// instead. That is one more cause, not one more kind.
	//
	// NOTHING HERE COMPUTES ANYTHING, and that division of labour is the
	// whole reason this is a kind rather than a rule. The rulebook holds
	// what a monster is worth and this module's go.mod cannot import it
	// (C1); the session applies the grant, because applying it is a write to
	// sheets this composition does not own; and the composition keeps the
	// record, because the record is the one thing it does own. It is exactly
	// the arrangement [OutcomeBought] already has with the Trade verb — the
	// numbers are preserved, never checked against anything.
	//
	// A GRANT OF ZERO IS NOT A BEAT. The session does not record one, and
	// this kind refuses one rather than writing a story about nothing
	// happening; an encounter whose fallen monster was worth nothing simply
	// has no experience beat in it.
	//
	// ITS ACTOR MAY BE A FORMER MEMBER, alone among the kinds: the fallen
	// monster that caused the grant may have exited by the time the session
	// records it. Any member this encounter ever held is accepted; a
	// never-member is refused (ErrNotMember).
	//
	// ACCEPTED AFTER THE ENCOUNTER HAS CLOSED, alone among the kinds. The
	// fall that pays can be the fall that ends the run — see
	// [Encounter.prepareRecord]'s refusal site for why that door has to be
	// open and why it is safe.
	OutcomeExperienceGained OutcomeKind = "experience_gained"
)

// OutcomeValue names one number a rulebook outcome carries.
//
// Deliberately generic — "the roll", "what it needed", "how much" — because
// the composition must not learn what an armour class is. The rulebook knows
// which of its numbers is which; the composition knows only that they are
// numbers, and refuses any name not on this list.
type OutcomeValue string

const (
	// ValueRoll is the die as rolled.
	ValueRoll OutcomeValue = "roll"

	// ValueTotal is the roll plus whatever the rules added to it.
	ValueTotal OutcomeValue = "total"

	// ValueAgainst is the number the total had to reach.
	ValueAgainst OutcomeValue = "against"

	// ValueAmount is how much was done — damage, healing, whatever the kind
	// means by it.
	ValueAmount OutcomeValue = "amount"
)

// RecordInput is one rulebook outcome, offered to the story.
//
// THERE IS NO PROSE FIELD HERE, and there will not be one. Every member is an
// ID the composition validates against its own roster, the kind is a closed
// enum, and the remaining primitives are catalog/rulebook facts under a closed
// shape. Prose is not something this input can express — not discouraged, not
// filtered, INEXPRESSIBLE — so no caller can narrate into a transcript that
// other players read, and no future caller can start.
//
// That is the difference between this and the general "append anything" hole
// it replaces the need for. The composition stays the only author of its
// record, and what a rulebook contributes is facts it can check.
type RecordInput struct {
	// Kind is what happened. Must be a kind this composition knows.
	Kind OutcomeKind

	// Actor is who did it. Must be a current member.
	Actor MemberID

	// Targets are who it was done to, if anyone. Each must be a current
	// member. Recorded sorted, for the same C8 reason every other list here
	// is: identical inputs must produce identical stories.
	Targets []MemberID

	// Values are the numbers, under names from the closed set. Absent is
	// legal — a kind that carries no numbers is a kind, not an error.
	Values map[OutcomeValue]int

	// Critical says whether this outcome was a critical hit. Meaningful only
	// for OutcomeStruck — a miss cannot crit — and false (the zero value) is
	// the ordinary case rather than an omission.
	Critical bool

	// Attack names what was swung, when this outcome is an attack. Present
	// for OutcomeStruck/OutcomeMissed, nil otherwise (rpg-toolkit#866,
	// rpg-toolkit#941). Every field is meant to be a catalog-owned
	// identifier the caller read off an already-compiled attack profile —
	// see AttackIdentity's own doc for exactly what this input does and
	// does not check about that.
	Attack *AttackIdentity

	// Reaction names what this outcome was taken AS, when it was a reaction
	// rather than a declared action. Present for an opportunity attack, nil
	// for an ordinary swing — see ReactionIdentity's own doc.
	Reaction *ReactionIdentity

	// Sequence names the sequence this swing was performed inside. Present on
	// a swing of a multiattack, nil on a lone swing and on every reaction.
	// Valid on OutcomeStruck, OutcomeMissed and OutcomeWarded; any other kind
	// is ErrInvalidData, as is an empty Ref or Name. Written as the beat's
	// "sequence": {"ref", "name"} key, and omitted when nil.
	Sequence *SequenceIdentity

	// DamageComponents are the ordered primitive facts that produced a struck
	// outcome. Meaning belongs to the rulebook; this composition preserves the
	// supplied order and values without interpreting them.
	DamageComponents []DamageComponent

	// AdvantageSources and DisadvantageSources identify the ordered sources the
	// attack fold reported. They carry identifiers only, never the rules engine's
	// human-readable Reason string.
	AdvantageSources    []AttackModifierSource
	DisadvantageSources []AttackModifierSource

	// Calculation is the authoritative sourced arithmetic for an attack. It
	// is valid only for struck/missed outcomes; when scalar roll/total values
	// are present they must agree with it.
	Calculation *RollCalculation

	// DeathSave carries the authoritative primitive facts for
	// OutcomeDeathSave. It is required for that kind and invalid for every
	// other kind; the encounter preserves it without interpreting thresholds.
	DeathSave *DeathSaveDetail

	// Trade carries the authoritative primitive facts for OutcomeBought and
	// OutcomeSold. It is required for either of those kinds and invalid for
	// every other kind; the encounter preserves it without interpreting
	// what the item does.
	Trade *TradeDetail

	// Warded carries the authoritative primitive facts for OutcomeWarded. It
	// is required for that kind and invalid for every other kind.
	Warded *WardedDetail

	// Experience carries the authoritative primitive facts for
	// OutcomeExperienceGained. It is required for that kind and invalid for
	// every other kind; the encounter preserves the amounts without asking
	// what anybody was worth or whether the shares add up.
	Experience *ExperienceDetail

	// PresentationID is the rulebook's opaque token for the ONE roll this
	// beat describes, carried verbatim so the actor who declared it and
	// every witness reading the same beat hold the same string for it.
	//
	// IT EXISTS BECAUSE THE SEQUENCE CANNOT DO THIS JOB. A story sequence is
	// recipient-local (rpg-toolkit#1377) — two members receiving one beat
	// count different numbers for it — so anything correlating a roller with
	// a witness has to be minted once and copied, which is exactly what this
	// is.
	//
	// Valid for OutcomeStruck and OutcomeMissed, and refused on every other
	// kind: OutcomeDeathSave carries its own inside [DeathSaveDetail], and a
	// second copy beside it would put a key on that beat its decoder does not
	// read. Empty is legal on an attack and means nobody declared this swing
	// — a monster's strike, an undeclared reaction, or a beat written before
	// this field existed — and the key is then omitted rather than written
	// empty, so those beats keep the shape they have always had.
	//
	// CHECKED FOR PRESENCE, NOT FOR MEANING, the same as [AttackIdentity]:
	// this composition cannot know a rulebook's token alphabet, and the
	// rulebook that mints one validates it before handing it over.
	PresentationID string

	// ConcentrationBreaks are the concentrations this outcome ended, in the
	// order the rulebook ended them. Their beats are appended AFTER this
	// outcome's own, so one blow produces one train — struck, the failed
	// check, the break, and the conditions it stripped — and a reader finds
	// the whole break inside the hit that caused it.
	//
	// Legal on every kind rather than only [OutcomeStruck]. Which outcomes can
	// break a concentration is a rulebook fact this module cannot import (C1),
	// exactly the reason the standing consult below runs for every kind, and a
	// list this composition refused on a kind it had guessed was harmless
	// would be encoding a rule it does not own. Empty is the ordinary case.
	ConcentrationBreaks []ConcentrationBreak

	// ConcentrationChecks are the concentrations this outcome tested and did
	// NOT break, in the order they were rolled. Their saved beats are appended
	// before any break's, so one blow reports every check it asked for and not
	// only the ones somebody lost.
	ConcentrationChecks []ConcentrationCheck
}

// DeathSaveDetail is the closed, rulebook-neutral story shape for one death
// save. Its fields are primitive facts supplied by the authoritative rulebook;
// this composition validates presence and preserves them verbatim.
type DeathSaveDetail struct {
	Roll              int              `json:"roll"`
	Outcome           string           `json:"outcome"`
	SuccessesAdded    int              `json:"successes_added"`
	FailuresAdded     int              `json:"failures_added"`
	Successes         int              `json:"successes"`
	Failures          int              `json:"failures"`
	SuccessesNeeded   int              `json:"successes_needed"`
	FailuresRemaining int              `json:"failures_remaining"`
	Stabilized        bool             `json:"stabilized"`
	Dead              bool             `json:"dead"`
	Recovered         bool             `json:"recovered"`
	HPRestored        int              `json:"hp_restored"`
	Continuation      string           `json:"continuation"`
	PresentationID    string           `json:"presentation_id"`
	Calculation       *RollCalculation `json:"calculation,omitempty"`
}

// WardedDetail is the closed, rulebook-neutral story shape for an attack a
// Sanctuary-style ward stopped: who warded the target off, and the
// attacker's own failed save against that warder's DC.
//
// REUSES CastSave RATHER THAN A FOURTH SAVE SHAPE — a ward-blocked save is
// the same fact as any other saving throw, just rolled by the ACTOR instead
// of the recipient. Save.Saver must equal the outcome's own Actor, never
// the warded Target; the inversion is the whole point of a ward.
type WardedDetail struct {
	// Source is the caster whose ward blocked this attempt. Must be a member
	// of this encounter now or at some point before — see
	// [Encounter.checkWardSource].
	Source MemberID `json:"source"`

	// Save is the ACTOR's own failed save against Source's DC.
	Save CastSave `json:"save"`
}

// checkWardSource refuses a ward source this encounter has never held.
//
// AN EVER-MEMBER, NOT A CURRENT ONE (rpg-toolkit#1965). A ward outlives the
// caster who cast it — Sanctuary does not end when its caster walks away, and
// the rulebook records the ward's DC at cast so nothing has to read the
// caster's sheet afterwards. A swing at the warded member after that caster
// Exited is the same fact as one before, and refusing it wedged the table: a
// driven monster turn is one of those swings. The departed source is the same
// notion [Encounter.Story] answers by — someone who was here keeps their name
// in the story. An id this encounter never held is still nobody.
//
// Errors: ErrNoMember when source is empty; ErrNotMember when it was never a
// member.
func (e *Encounter) checkWardSource(verb string, source MemberID) error {
	if source == "" {
		return fmt.Errorf("%s: warded source: %w", verb, ErrNoMember)
	}
	if !e.everMembers[source] {
		return fmt.Errorf("%s: warded source %q: %w", verb, source, ErrNotMember)
	}
	return nil
}

// TradeDetail is the closed, rulebook-neutral story shape for one traded
// item. Its fields are primitive facts supplied by the authoritative
// rulebook; this composition validates presence and preserves them
// verbatim.
//
// ItemType is a plain string, not a closed encounter-level enum, for the
// same reason AttackIdentity.DamageType is: this module's go.mod cannot
// import the rulebook package that owns that vocabulary (C1). Session,
// which already depends on that package, maps it onto and off of a closed
// Go type.
type TradeDetail struct {
	ItemType string `json:"item_type"`
	ItemID   string `json:"item_id"`
	Quantity int    `json:"quantity"`
}

// ExperienceDetail is the closed, rulebook-neutral story shape for one grant
// of experience: what caused it, and what each character was paid.
//
// Its fields are primitive facts supplied by the authoritative caller, which
// for [OutcomeExperienceGained] is the session SDK AFTER it applied the
// grant. This composition validates presence and preserves the numbers
// verbatim. What it never does is arithmetic — it does not divide, does not
// check that the shares sum to anything, and cannot ask what a monster was
// worth, because the rulebook that knows is a package its go.mod cannot
// import (C1).
type ExperienceDetail struct {
	// Member is the cause: the fallen monster's member id, the same id the
	// down beat carried.
	//
	// A PLAIN STRING RATHER THAN A MemberID, and checked for presence only.
	// A later slice pays an authored reward at an ending and will name the
	// ending here — a cause that is not a member and never will be. Reserving
	// a second field for that today would be building it early; writing an
	// ending key into this one later breaks nothing, because what this field
	// carries is an identifier either way.
	Member string `json:"member"`

	// Grants is who was paid and how much, one entry per character. At least
	// one is required: a grant nobody received is not a beat.
	//
	// Recorded sorted by Character, for the same C8 reason
	// [RecordInput.Targets] is sorted — a set of payees has no meaningful
	// order, a caller dividing a monster's worth across a Go map has no
	// stable one, and two runs of identical input must produce the identical
	// story. Only the ORDER is this composition's; the numbers are the
	// caller's, untouched.
	Grants []ExperienceGrant `json:"grants"`
}

// ExperienceGrant is one character's share of one grant.
type ExperienceGrant struct {
	// Character is who was paid.
	//
	// CHECKED FOR PRESENCE, NOT FOR MEMBERSHIP, unlike every MemberID on
	// [RecordInput]. Who the session pays is the session's roster fact and
	// this composition's roster is the encounter's — the two need not be the
	// same list, and a composition that refused the difference would be
	// deciding a question it was handed the answer to.
	Character string `json:"character"`

	// Amount is what this cause paid them. Must be positive — see
	// [OutcomeExperienceGained] on why a zero never reaches the story.
	Amount int `json:"amount"`

	// Total is what they hold after it: the session's own number, carried so
	// a reader can answer "how close am I" without summing every beat it
	// ever saw. Never checked against Amount, because this composition does
	// not know what they held before.
	Total int `json:"total"`
}

// preparedExperience validates the experience detail against the outcome kind
// and returns it as the story will keep it — nil for every kind that does not
// carry one, an ErrInvalidData refusal for a kind/detail mismatch either way.
//
// The refusals are the fail-closed floor this composition CAN hold without a
// rulebook: a cause, at least one payee, a name for each, and a positive
// amount. What it cannot hold — that the amounts are the right amounts — is
// exactly what the caller already settled before calling.
func preparedExperience(in *RecordInput) (*ExperienceDetail, error) {
	if in.Kind != OutcomeExperienceGained {
		if in.Experience != nil {
			return nil, fmt.Errorf(
				"record: experience detail does not match outcome kind %q: %w", in.Kind, ErrInvalidData)
		}
		return nil, nil
	}
	if in.Experience == nil {
		return nil, fmt.Errorf("record: experience detail is required: %w", ErrInvalidData)
	}
	if in.Experience.Member == "" {
		return nil, fmt.Errorf("record: experience member is required: %w", ErrInvalidData)
	}
	if len(in.Experience.Grants) == 0 {
		return nil, fmt.Errorf("record: experience grants are required: %w", ErrInvalidData)
	}
	for i, grant := range in.Experience.Grants {
		if grant.Character == "" {
			return nil, fmt.Errorf("record: experience grant %d character is required: %w", i, ErrInvalidData)
		}
		if grant.Amount <= 0 {
			return nil, fmt.Errorf(
				"record: experience grant %d amount %d must be positive: %w", i, grant.Amount, ErrInvalidData)
		}
	}

	// Copied rather than sorted in place: the caller's own slice is not this
	// composition's to rearrange behind its back.
	grants := append([]ExperienceGrant(nil), in.Experience.Grants...)
	sort.Slice(grants, func(i, j int) bool { return grants[i].Character < grants[j].Character })
	return &ExperienceDetail{Member: in.Experience.Member, Grants: grants}, nil
}

// experienceSubjects appends every character an experience grant paid to the
// beat's subject list, skipping anybody already on it.
//
// THE GRANTEES ARE WHAT THIS BEAT IS ABOUT. v1 hands every beat to the whole
// roster regardless of its subjects (see [Encounter.audienceFor]), so this
// changes nothing a reader can observe today — it is the answer
// rpg-toolkit#940's flip will need, recorded while the fact is still in front
// of us. A player who never saw the monster fall is owed their share all the
// same, and an audience worked out from the actor alone is the one shape that
// would drop it.
func experienceSubjects(subjects []MemberID, detail *ExperienceDetail) []MemberID {
	seen := make(map[MemberID]bool, len(subjects))
	for _, id := range subjects {
		seen[id] = true
	}
	for _, grant := range detail.Grants {
		id := MemberID(grant.Character)
		if seen[id] {
			continue
		}
		seen[id] = true
		subjects = append(subjects, id)
	}
	return subjects
}

// DamageComponent carries one ordered, rulebook-neutral damage contribution.
// It uses only primitives this module can persist without importing the
// rulebook packages that own their meaning.
//
// Everything roll-shaped lives in [Roll]: the provider-owned [RollSource]
// (canonical ref string and display name), the dice trace when this component
// rolled dice, and the modifier when this component contributed one — present
// even when its value is zero. The remaining fields are damage facts, not
// roll facts: the category, the damage type, and the multiplier. A component
// that exists only to carry a multiplier (resistance, vulnerability,
// immunity) has a sourced Roll with neither dice nor a modifier — the same
// shape the root rulebook's own trait producers emit.
//
// [Encounter.RecordTrain] validates Roll before appending: the source identity is
// required, and any dice trace present is replayed for structural and
// arithmetic consistency. The multiplier's MEANING is never interpreted here.
type DamageComponent struct {
	// Source is the damage category — "weapon", "monster_trait" — in the
	// rulebook's own words, carried through like DamageType below.
	Source string `json:"source"`

	// Roll records who provided this component's roll facts and what they
	// were. Roll.Dice records the pool as rolled — original faces, ordered
	// sourced rerolls, final faces, and the authoritative subtotal — and
	// Roll.Modifier is present whenever this component contributed a
	// modifier, even when that modifier's value is zero. Nil means the
	// modifier did not participate; a present zero is a real zero.
	Roll RollComponent `json:"roll"`

	DamageType string   `json:"damage_type"`
	Multiplier *float64 `json:"multiplier,omitempty"`
}

// validateDamageComponentRoll checks the roll facts of one persisted damage
// component. Its source identity is always required, and it must contribute
// something — dice, a modifier, or a multiplier — because a component that
// contributes nothing has no story to tell. Whether the multiplier's FACTOR is
// the right rule (resistance, vulnerability, immunity) is never interpreted
// here; only a dice trace's structural and arithmetic consistency is.
func validateDamageComponentRoll(component DamageComponent) error {
	if err := validateRollComponentData(component.Roll); err != nil {
		return err
	}
	if component.Roll.Dice == nil && component.Roll.Modifier == nil && component.Multiplier == nil {
		return fmt.Errorf("must contain dice, a modifier, or a multiplier")
	}
	return nil
}

// AttackModifierSource identifies an entity/content source without carrying
// prose. Empty strings mean that identifier was absent.
type AttackModifierSource struct {
	SourceRef string `json:"source_ref,omitempty"`
	SourceID  string `json:"source_id,omitempty"`
}

// AttackIdentity names what was swung — ref, display name, and damage type —
// carried on a [RecordInput] whose Kind is OutcomeStruck or OutcomeMissed so
// the beat can answer "with a longsword" and "6 slashing" for every witness,
// not only the one whose own verb response held the compiled profile.
//
// PLAIN STRINGS, MEANT TO BE USED LIKE MemberID IS ON THIS INPUT — CHECKED
// FOR PRESENCE THE WAY MemberID IS, BUT NOT FOR MEANING (Copilot, PR #1172).
// [Encounter.RecordTrain] refuses ErrInvalidData when Ref or Name is empty, the
// same as it refuses an empty Actor — that is the minimum this composition
// CAN check without a catalog. It does not, and cannot, validate that Ref or
// Name names anything real, or that DamageType is one of the rulebook's own
// words: this module's go.mod cannot import the catalog that would answer
// whether "longsword" is real (C1). The intended values are the catalog's
// own identifier and display label — "longsword", "Longsword" — and the
// rulebook's own word for the damage dealt, fixed by a sealed weapon/action
// catalog rather than composed at the call site, and it is session — which
// already depends on that package — that maps DamageType onto a closed Go
// type. The composition guarantees presence; the rulebook guarantees
// meaning.
type AttackIdentity struct {
	// Ref is the catalog ref the attack compiled from — "longsword",
	// "unarmed-strike".
	Ref string

	// Name is the catalog's display name for Ref — "Longsword", "Unarmed
	// Strike".
	Name string

	// DamageType is the kind of damage the attack deals, in the rulebook's
	// own words — "slashing", "bludgeoning". A string here, not a closed
	// encounter-level enum: this module's go.mod cannot import the damage
	// package that owns that vocabulary (C1), so the value is carried
	// through exactly as Actor and Targets already are, and it is
	// session — which already depends on that package — that maps it onto
	// a closed Go type.
	DamageType string
}

// ReactionIdentity names what an outcome was taken AS, when the actor was
// reacting rather than acting — carried on a [RecordInput] whose Kind is
// OutcomeStruck or OutcomeMissed so the beat can answer "opportunity attack"
// for every witness.
//
// It exists because without it the story is honestly confusing rather than
// wrong. An opportunity attack IS a strike and records through this same verb,
// so every number already crosses; what does not is why an actor dealt damage
// during somebody else's turn. A reader sees a fighter hit on a wolf's turn
// and has nothing to explain it with (rpg-project#316).
//
// PLAIN STRINGS, CHECKED FOR PRESENCE AND NOT FOR MEANING, exactly as
// [AttackIdentity] is and for the identical reason: [Encounter.RecordTrain] refuses
// ErrInvalidData when Ref or Name is empty — the minimum this composition CAN
// check — and cannot validate that either names anything real, because this
// module's go.mod cannot import the rulebook that would answer (C1). The
// composition guarantees presence; the rulebook guarantees meaning.
//
// AN OPEN SET, so a string rather than a kind on [OutcomeKind]'s closed enum.
// The closed enum answers "what happened", which this composition must know;
// this answers "under which of the rulebook's rules", which it must not. Shield
// and Uncanny Dodge join without a change here.
type ReactionIdentity struct {
	// Ref is the rulebook ref of the condition or feature that reacted —
	// "dnd5e:conditions:opportunity_attack".
	Ref string

	// Name is the display name for Ref — "Opportunity Attack".
	Name string
}

// SequenceIdentity names the sequence a swing was performed inside — the goblin
// boss's Multiattack — so the story can tell a Multiattack's swing from a lone
// pick. Plain strings checked for presence, as [AttackIdentity] and
// [ReactionIdentity] are: this module cannot resolve a rulebook ref (C1).
type SequenceIdentity struct {
	// Ref is the sequence definition's own ref, not the component it swung.
	Ref string

	// Name is the display name for Ref — "Multiattack".
	Name string
}

// TrainUnit is one told unit of a landing: exactly one of Outcome or
// Activation. Each carries its own concentration checks and breaks
// ([RecordInput.ConcentrationChecks] and [RecordInput.ConcentrationBreaks],
// [RecordActivationInput]'s same fields), told behind that unit's own beat.
type TrainUnit struct {
	// Outcome is a rulebook outcome: a swing, a death save, a trade, a ward,
	// an experience grant.
	Outcome *RecordInput

	// Activation is an activation told inside a landing: a post-hit
	// reaction's retaliation, today.
	Activation *RecordActivationInput
}

// RecordTrainInput is every unit one landing tells, in story order.
type RecordTrainInput struct {
	Units []TrainUnit
}

// TrainLanded is where one unit's beats landed.
type TrainLanded struct {
	// Seq is the unit's own beat: the outcome beat, or the activation's
	// activated beat.
	Seq uint64

	// FollowUpSeqs are every other beat the unit appended, in append order: an
	// activation's save and results, then the unit's concentration checks,
	// then its breaks' trains.
	FollowUpSeqs []uint64
}

// RecordTrainOutput reports one [TrainLanded] per input unit, in input order,
// and the intel changes the one standing consult produced.
type RecordTrainOutput struct {
	Units []TrainLanded

	// IntelDeltas maps member IDs to their updated percepts after any driven
	// monster turns caused by noticing the train's consequences.
	IntelDeltas map[MemberID]*IntelDelta
}

// RecordTrain puts one landing's units into the story and then lets the world
// notice them, once.
//
// It exists because a strike resolved outside this module was INVISIBLE:
// resolution returns an outcome value and writes no beat, appendBeat is
// unexported, and the SDK builds every client event by reading each member's
// story — so an attack produced nothing to render and nothing to re-read
// (rpg-toolkit#966). One transcript is the product; a rule that resolves
// somewhere else still has to land in it.
//
// The audience is every current member, at the current clock reading — the
// same convention the clock beats use. An outcome is not secret: a fight is
// localized but visible, and a client that learned about a strike only from
// the striker's own response could not render the scene the party is in.
//
// # One landing, one train, one consult
//
// A landing writes every dirty sheet BEFORE it records, so the standing
// consult answers from the end state of the whole output. Asking between two
// units would therefore report a fall ahead of the blow that caused it, and a
// last-standing member's fall would close the encounter between two swings of
// one multiattack. The train is the unit of telling: every unit's beats first,
// the question once, after the last.
//
// Every unit is prepared before anything is appended. Then, for each unit in
// order: its own beat, its concentration checks' saved beats, its breaks'
// trains, and, for a struck or missed outcome, its attack deed. After the last
// unit, one pass: the down beats, then the party-defeat ending if the rulebook
// answers one, then the ending a deed asked for, then removals, the fight
// forming a deed started (its first slot driven now), and the declared
// member-down endings; then the observed-standing refresh and each activation
// unit's world-action price in unit order.
//
// A deed lands in place, so a stance it turns is told beside the blow that
// turned it, and a fight it forms is told forming (bubble-formed). The ENDING
// that stance would trigger (an authored stance ending, a fact's, an arrival's)
// is parked and closes the encounter once, after the last unit and the down
// beats (T5), and a fight formed mid-train is driven then too: nothing is
// consulted, closed or driven between two units. Which ending wins when
// several apply is the order above: party defeat, then the parked ending, then
// declared member-down endings, because an ending already closed is not
// evaluated again.
//
// # It records, and then the world notices what it recorded
//
// The consult is [Encounter.noticeDown], the one place noticing happens. It
// runs for EVERY kind rather than only for [OutcomeStruck]: which outcomes can
// drop somebody is a rulebook fact and this module cannot import the rulebook
// (C1), so a verb that decided for itself which beats were worth looking after
// would be encoding that rule, and would miss the first route to zero nobody
// has written yet. The composition ASKS and the rulebook answers, the answer is
// pulled and never remembered, and the story is the ledger that keeps the news
// from being told twice. The alternative on offer was a caller pushing the
// beat in, and that is a different thing entirely: see [OutcomeDown], which is
// still refused, for why.
//
// A unit's own beat is the cause. [TrainLanded.Seq] is therefore the unit's
// beat and never the last one written; a caller asked for one thing to be
// recorded and is told where that thing landed.
//
// # On error
//
// Errors, every one before anything is appended: [ErrNilInput] for a nil input;
// [ErrInvalidData] for zero units or a unit with both or neither field set;
// [ErrClosed] for a closed encounter, except a train whose every unit is an
// [OutcomeExperienceGained]; and whatever the preparation of any unit refuses,
// wrapped with the unit's index — [ErrNoMember], [ErrNotMember] (an experience
// beat's actor may be any former member), [ErrInvalidData] (a kind or value
// name this composition does not know, missing or mismatched DeathSave, Trade,
// Warded, Experience or Sequence detail, an Attack, Reaction or Sequence whose
// Ref or Name is empty, a damage component whose roll facts are missing or
// internally inconsistent, a non-finite damage multiplier JSON cannot
// represent), and anything the [Participation] capability answers with.
//
// A failure in a deed or the consult happens after the append, so a rulebook
// that cannot answer leaves the in-memory encounter holding a train whose
// consequences were never worked out. That is R5's documented limit rather
// than a hole in it, and the caller's obligation is doc.go's whole answer to
// it — drop the encounter unsaved.
func (e *Encounter) RecordTrain(in *RecordTrainInput) (*RecordTrainOutput, error) {
	const verb = "record train"
	if in == nil {
		return nil, fmt.Errorf("%s: %w", verb, ErrNilInput)
	}
	if len(in.Units) == 0 {
		return nil, fmt.Errorf("%s: no units: %w", verb, ErrInvalidData)
	}

	// Prepared in full before the first append: a bad unit anywhere refuses
	// the whole train and costs the rulebook nothing.
	prepared := make([]preparedUnit, 0, len(in.Units))
	for i, unit := range in.Units {
		p, err := e.prepareUnit(i, unit)
		if err != nil {
			return nil, err
		}
		prepared = append(prepared, p)
	}

	landed, intelDeltas, err := e.appendTrainAndNotice(verb, prepared)
	if err != nil {
		return nil, err
	}
	return &RecordTrainOutput{Units: landed, IntelDeltas: intelDeltas}, nil
}

// preparedUnit is one unit, validated and marshalled, nothing appended.
type preparedUnit struct {
	// beats is the unit's own beat first when headed, then its follow-ups.
	beats []preparedActivationBeat

	// headed says beats[0] is the unit's own beat. False for a
	// [Encounter.TellConcentration], which has none.
	headed bool

	// land is a struck or missed outcome's attack deed; nil otherwise.
	land func() error

	// pass carries a stabilized or recovered death save's deferReconcile.
	pass participationPassInput

	// worldActor is an activation's actor, who pays the world-action price;
	// empty otherwise.
	worldActor MemberID
}

// prepareUnit validates and marshals one unit without mutating the story. It is
// the validation/mutation boundary for [Encounter.RecordTrain]: pure, so a bad
// unit anywhere in a train can be refused before anything is appended.
func (e *Encounter) prepareUnit(index int, unit TrainUnit) (preparedUnit, error) {
	switch {
	case unit.Outcome != nil && unit.Activation != nil:
		return preparedUnit{}, fmt.Errorf("record train: unit %d: both outcome and activation: %w", index, ErrInvalidData)
	case unit.Outcome == nil && unit.Activation == nil:
		return preparedUnit{}, fmt.Errorf("record train: unit %d: neither outcome nor activation: %w", index, ErrInvalidData)
	}

	if in := unit.Activation; in != nil {
		beats, err := e.prepareActivation(in)
		if err != nil {
			return preparedUnit{}, fmt.Errorf("record train: unit %d: %w", index, err)
		}
		return preparedUnit{beats: beats, headed: true, worldActor: in.Actor}, nil
	}

	in := unit.Outcome
	beats, err := e.prepareRecord(in)
	if err != nil {
		return preparedUnit{}, fmt.Errorf("record train: unit %d: %w", index, err)
	}
	prepared := preparedUnit{beats: beats, headed: true}

	// A stabilized or recovered Death Save carries an explicit turn
	// continuation. Recording it happens inside the already-active turn, so it
	// neither auto-passes that slot nor reconciles a retained one-sided bubble
	// in this same call. Stabilized explicitly reaches EndTurn; recovered keeps
	// control until the eventual turn-settlement boundary.
	if in.Kind == OutcomeDeathSave && (in.DeathSave.Stabilized || in.DeathSave.Recovered) {
		prepared.pass.deferReconcile = true
	}
	if in.Kind == OutcomeStruck || in.Kind == OutcomeMissed {
		// subjects[1:] is the validated, sorted target list for these two
		// kinds: only [OutcomeExperienceGained] puts anything else in there,
		// and it never lands an attack.
		targets := beats[0].subjects[1:]
		prepared.land = func() error { return e.landAttack(in.Actor, targets) }
	}
	return prepared, nil
}

// appendTrainAndNotice is the ONLY body that appends a train and runs the
// post-append consult, for [Encounter.RecordTrain] and
// [Encounter.TellConcentration] alike. ONE BODY, so the verbs cannot drift — a
// check told without an outcome lands exactly as the same check told behind
// one, and the world notices it the same way.
//
// Each unit rides in at the current clock reading: its own beat (when headed),
// then its follow-ups, then its deed. No consult runs between units: the sheets
// were written before the record, so asking early would tell a fall ahead of
// the blow that caused it.
//
// A DEED CAN ASK FOR AN ENDING, and the ending waits (T5). A deed turns pairs
// hostile, and an authored stance ending, a fact or an arrival's ending can
// hang on that. The stance beat is told in place, behind the blow, but the
// close is parked, carried by this call alone, and runs once after the last
// unit and the down beats, before any turn transfer or driven monster turn.
//
// A CLOSED ENCOUNTER STOPS AFTER THE APPEND. Only a train of
// [OutcomeExperienceGained] reaches this on one — every other unit is refused
// in preparation — and a settled world has nothing left to notice: no sight to
// refresh, no standing to consult, no ending left to fire. Nothing between the
// first append and the consult can close the encounter: a deed's ending is
// parked until after the last unit.
//
// Otherwise the world finds out what the train just changed, AFTER the last
// append, never before: the train is the cause, and a down beat ahead of what
// explains it would be a story told backwards. Then the observed-standing
// refresh, then each activation unit's world-action price, in unit order.
// verb names the caller in every error.
func (e *Encounter) appendTrainAndNotice(
	verb string, units []preparedUnit,
) ([]TrainLanded, map[MemberID]*IntelDelta, error) {
	landed := make([]TrainLanded, len(units))
	var pass participationPassInput
	var held *heldEnding
	var drives []*clock.Turn

	// A deed can turn a stance, and an authored ending can wait on that stance.
	// The stance change is told in place, behind the blow that caused it, but
	// no ending closes between two units: the first one asked for is parked
	// and evaluated once, after the last unit (T5).
	// A nested train (a driven strike recorded from inside a deed's consult)
	// runs under its own hold: it saves the outer's and restores it on the way
	// out, so neither can see or consume the other's.
	outerHold, outerHeld, outerDrives := e.holdEndings, e.heldEnding, e.deferredDrives
	e.holdEndings, e.heldEnding, e.deferredDrives = false, nil, nil
	defer func() { e.holdEndings, e.heldEnding, e.deferredDrives = outerHold, outerHeld, outerDrives }()

	for i, unit := range units {
		for j, beat := range unit.beats {
			appended, err := e.appendBeat(&record.AppendInput{
				At:       uint64(e.clock.ToData().HighWater),
				Audience: e.audienceFor(subjectBeat, beat.subjects...),
				Tags:     map[string]string{"tag": "outcome"},
				Payload:  beat.payload,
			})
			if err != nil {
				return nil, nil, fmt.Errorf("%s: unit %d: beat %d: %w", verb, i, j, err)
			}
			if unit.headed && j == 0 {
				landed[i].Seq = appended.Seq
			} else {
				landed[i].FollowUpSeqs = append(landed[i].FollowUpSeqs, appended.Seq)
			}
		}
		if e.outcome == nil && unit.land != nil {
			e.holdEndings = true
			err := unit.land()
			e.holdEndings = false
			if err != nil {
				return nil, nil, fmt.Errorf("%s: unit %d: %w", verb, i, err)
			}
			// Taken off the encounter at once: from here the parked ending
			// belongs to this train alone.
			if held == nil {
				held = e.heldEnding
			}
			e.heldEnding = nil
			drives = append(drives, e.deferredDrives...)
			e.deferredDrives = nil
		}
		if unit.pass.deferReconcile {
			pass.deferReconcile = true
		}
	}

	if e.outcome != nil {
		return landed, nil, nil
	}

	// The deed's parked ending rides this pass and fires right after the down
	// beats, before any transfer or driven turn (see noticeDown).
	pass.held = held
	pass.newlyActive = drives
	participation, intelDeltas, nerr := e.noticeDown(pass)
	if nerr != nil {
		return nil, nil, fmt.Errorf("%s: %w", verb, nerr)
	}
	observed, err := e.refreshChangedStanding(participation)
	if err != nil {
		return nil, nil, fmt.Errorf("%s observed standing: %w", verb, err)
	}

	// THE WORLD'S PRICE FOR AN ACTION, paid after the outcome has landed and
	// before anything else refreshes (design §5, worldtime.go): one round on
	// the world clock for the actor, and the world thinks on it. Nothing at
	// all for a member inside a fight, where the round is what prices time.
	for _, unit := range units {
		if unit.worldActor == "" {
			continue
		}
		if err := e.spendWorldAction(unit.worldActor); err != nil {
			return nil, nil, err
		}
	}
	return landed, mergeIntelDeltas(intelDeltas, observed), nil
}

// TellConcentrationOutput is what [Encounter.TellConcentration] appended.
type TellConcentrationOutput struct {
	// Seqs lists every beat appended, checks first, then breaks, in order.
	Seqs []uint64

	// IntelDeltas maps member IDs to their updated percepts after any driven
	// monster turns caused by noticing the concentration's consequences.
	IntelDeltas map[MemberID]*IntelDelta
}

// TellConcentrationInput is the concentration a rule changed with no outcome
// to carry it: the checks it rolled and the concentrations it ended.
type TellConcentrationInput struct {
	// Actor is the member whose rule caused them — the caster whose new spell
	// ended their own earlier concentration, say. Must be a current member. It
	// is a subject of every beat, as [RecordInput.Actor] is of a train.
	Actor MemberID

	// Checks are the concentrations tested and NOT broken, in the order they
	// were rolled, with [RecordInput.ConcentrationChecks]' meaning.
	Checks []ConcentrationCheck

	// Breaks are the concentrations ended, in the order the rule ended them,
	// with [RecordInput.ConcentrationBreaks]' meaning.
	Breaks []ConcentrationBreak
}

// TellConcentration appends the beats [Encounter.RecordTrain] appends behind a
// unit's own beat, with no unit: every check's saved beat, then every break's
// train (the failed check if there was one, the break, the conditions it
// stripped).
//
// IT IS NOT A KIND OF OUTCOME, and a [TrainUnit] still requires one. Some
// concentration has no causing unit to ride behind — a cast that ends its
// caster's own earlier concentration and then pauses, a turn boundary — and
// before this verb it was dropped by name. It shares the train's preparation
// (the same validation, under the verb name "tell concentration") and the
// train's append-and-consult body, so the same checks and breaks land as the
// same beats in the same order either way.
//
// [TellConcentrationOutput.Seqs] lists every beat appended, in order; there is
// no unit beat to name.
//
// Errors: [ErrNilInput]; [ErrClosed]; [ErrNoMember] or [ErrNotMember] for the
// actor; [ErrInvalidData] when both lists are empty; whatever the shared
// preparation refuses. Every refusal runs before anything is appended.
func (e *Encounter) TellConcentration(in *TellConcentrationInput) (*TellConcentrationOutput, error) {
	const verb = "tell concentration"
	if in == nil {
		return nil, fmt.Errorf("%s: %w", verb, ErrNilInput)
	}
	if e.outcome != nil {
		return nil, fmt.Errorf("%s: %w", verb, ErrClosed)
	}
	if in.Actor == "" {
		return nil, fmt.Errorf("%s: actor: %w", verb, ErrNoMember)
	}
	if _, ok := e.members[in.Actor]; !ok {
		return nil, fmt.Errorf("%s: actor %q: %w", verb, in.Actor, ErrNotMember)
	}
	if len(in.Checks) == 0 && len(in.Breaks) == 0 {
		return nil, fmt.Errorf("%s: nothing to tell: %w", verb, ErrInvalidData)
	}

	checkBeats, err := e.prepareConcentrationChecks(verb, in.Actor, in.Checks)
	if err != nil {
		return nil, err
	}
	breakBeats, err := e.prepareConcentrationBreaks(verb, in.Actor, in.Breaks)
	if err != nil {
		return nil, err
	}

	landed, intelDeltas, err := e.appendTrainAndNotice(
		verb, []preparedUnit{{beats: append(checkBeats, breakBeats...)}})
	if err != nil {
		return nil, err
	}
	return &TellConcentrationOutput{Seqs: landed[0].FollowUpSeqs, IntelDeltas: intelDeltas}, nil
}

// prepareRecord validates and marshals an outcome without mutating Story.
// Cast transactions reuse it so an invalid later result cannot strand an attack.
func (e *Encounter) prepareRecord(in *RecordInput) ([]preparedActivationBeat, error) {
	if in == nil {
		return nil, fmt.Errorf("record: %w", ErrNilInput)
	}
	// THE ONE DOOR LEFT OPEN AFTER THE CLOSE. Every mutating verb in this
	// module refuses a closed encounter and so does this one — except for
	// [OutcomeExperienceGained], which is a consequence of the very fall that
	// ended things.
	//
	// The sequence is not hypothetical: a boss going down fires its
	// [TriggerMemberDown] ending inside [Encounter.noticeDown], which runs
	// INSIDE the train that reported the killing blow, so the encounter is
	// already settled by the time the session has divided the monster's worth
	// and written the sheets. Refusing here would mean the one death that
	// mattered most is the only death nobody was paid for, on the record.
	//
	// It is safe because this kind cannot change anything. It is bookkeeping
	// the session has already applied, it moves no clock, it names no target,
	// and [Encounter.RecordTrain] skips the standing consult entirely once the
	// encounter is closed — a settled world has nothing left to notice. The
	// ending-reward slice [ExperienceDetail.Member] already anticipates comes
	// through this same door. Every other kind stays refused.
	if e.outcome != nil && in.Kind != OutcomeExperienceGained {
		return nil, fmt.Errorf("record: %w", ErrClosed)
	}

	switch in.Kind {
	case OutcomeStruck, OutcomeMissed, OutcomeDeathSave, OutcomeBought, OutcomeSold, OutcomeWarded,
		OutcomeExperienceGained:
	default:
		return nil, fmt.Errorf("record: outcome kind %q: %w", in.Kind, ErrInvalidData)
	}
	if in.Kind == OutcomeDeathSave {
		if in.DeathSave == nil {
			return nil, fmt.Errorf("record: death save detail is required: %w", ErrInvalidData)
		}
		if in.DeathSave.Outcome == "" {
			return nil, fmt.Errorf("record: death save outcome is required: %w", ErrInvalidData)
		}
		if in.DeathSave.Continuation == "" {
			return nil, fmt.Errorf("record: death save continuation is required: %w", ErrInvalidData)
		}
		if in.DeathSave.PresentationID == "" {
			return nil, fmt.Errorf("record: death save presentation id is required: %w", ErrInvalidData)
		}
	} else if in.DeathSave != nil {
		return nil, fmt.Errorf("record: death save detail does not match outcome kind %q: %w", in.Kind, ErrInvalidData)
	}
	if in.Kind == OutcomeBought || in.Kind == OutcomeSold {
		if in.Trade == nil {
			return nil, fmt.Errorf("record: trade detail is required: %w", ErrInvalidData)
		}
		if in.Trade.ItemID == "" {
			return nil, fmt.Errorf("record: trade item id is required: %w", ErrInvalidData)
		}
		if in.Trade.Quantity <= 0 {
			return nil, fmt.Errorf("record: trade quantity must be positive: %w", ErrInvalidData)
		}
	} else if in.Trade != nil {
		return nil, fmt.Errorf("record: trade detail does not match outcome kind %q: %w", in.Kind, ErrInvalidData)
	}
	if in.Kind == OutcomeWarded {
		if in.Warded == nil {
			return nil, fmt.Errorf("record: warded detail is required: %w", ErrInvalidData)
		}
		if err := e.checkWardSource("record", in.Warded.Source); err != nil {
			return nil, err
		}
		save := in.Warded.Save
		// THE INVERSION IS THE WHOLE POINT OF A WARD: an ordinary CastSave's
		// saver is whoever the effect targeted, but here it is the ACTOR —
		// the one whose attempt the ward is testing, not the one it protects.
		if save.Saver != in.Actor {
			return nil, fmt.Errorf(
				"record: warded save saver %q must be the actor %q: %w", save.Saver, in.Actor, ErrInvalidData)
		}
		if save.Ability == "" {
			return nil, fmt.Errorf("record: warded save ability: %w", ErrInvalidData)
		}
		if save.Roll < 1 || save.Roll > 20 {
			return nil, fmt.Errorf("record: warded save roll %d is not a d20: %w", save.Roll, ErrInvalidData)
		}
		if save.DC < 1 {
			return nil, fmt.Errorf("record: warded save dc %d: %w", save.DC, ErrInvalidData)
		}
		if save.Succeeded {
			return nil, fmt.Errorf("record: warded save succeeded: %w", ErrInvalidData)
		}
		if err := validateRecordedD20(save.Calculation, save.Roll, save.Total); err != nil {
			return nil, fmt.Errorf("record: warded save calculation: %v: %w", err, ErrInvalidData)
		}
	} else if in.Warded != nil {
		return nil, fmt.Errorf("record: warded detail does not match outcome kind %q: %w", in.Kind, ErrInvalidData)
	}
	experience, expErr := preparedExperience(in)
	if expErr != nil {
		return nil, expErr
	}
	if in.PresentationID != "" && in.Kind != OutcomeStruck && in.Kind != OutcomeMissed {
		return nil, fmt.Errorf("record: presentation id does not match outcome kind %q: %w", in.Kind, ErrInvalidData)
	}
	if in.Sequence != nil && in.Kind != OutcomeStruck && in.Kind != OutcomeMissed && in.Kind != OutcomeWarded {
		return nil, fmt.Errorf("record: sequence does not match outcome kind %q: %w", in.Kind, ErrInvalidData)
	}
	if in.Calculation != nil && in.Kind != OutcomeStruck && in.Kind != OutcomeMissed {
		return nil, fmt.Errorf("record: calculation does not match outcome kind %q: %w", in.Kind, ErrInvalidData)
	}

	if in.Actor == "" {
		return nil, fmt.Errorf("record: actor: %w", ErrNoMember)
	}
	// AN EXPERIENCE BEAT MAY NAME A FORMER MEMBER. Its actor is the cause —
	// the monster whose fall paid — and a fallen monster can exit before the
	// session settles the act. It is the same notion [Encounter.Story] and
	// [Encounter.checkWardSource] answer by: someone who was here keeps their
	// name in the story. An id this encounter never held is still nobody.
	// Every other kind's actor must be a current member.
	if in.Kind == OutcomeExperienceGained {
		if !e.everMembers[in.Actor] {
			return nil, fmt.Errorf("record: actor %q: %w", in.Actor, ErrNotMember)
		}
	} else if _, ok := e.members[in.Actor]; !ok {
		return nil, fmt.Errorf("record: actor %q: %w", in.Actor, ErrNotMember)
	}

	targets := append([]MemberID(nil), in.Targets...)
	for _, id := range targets {
		if id == "" {
			return nil, fmt.Errorf("record: target: %w", ErrNoMember)
		}
		if _, ok := e.members[id]; !ok {
			return nil, fmt.Errorf("record: target %q: %w", id, ErrNotMember)
		}
		// AN NPC IS NOT A TARGET (rpg-project#493, R4), and only a swing is
		// refused: a trade and a death save name members too, and neither is
		// an attack. Struck and missed are exactly the two kinds that land a
		// deed through [Encounter.landAttack].
		if in.Kind == OutcomeStruck || in.Kind == OutcomeMissed {
			if err := e.attackable("record", id); err != nil {
				return nil, err
			}
		}
	}
	sort.Slice(targets, func(i, j int) bool { return targets[i] < targets[j] })

	// Values are marshalled through a sorted slice rather than the map: a Go
	// map has no order, and a beat whose bytes differ between two runs of the
	// same input is a transcript that cannot be compared (C8).
	names := make([]OutcomeValue, 0, len(in.Values))
	for name := range in.Values {
		switch name {
		case ValueRoll, ValueTotal, ValueAgainst, ValueAmount:
			names = append(names, name)
		default:
			return nil, fmt.Errorf("record: value %q: %w", name, ErrInvalidData)
		}
	}
	sort.Slice(names, func(i, j int) bool { return names[i] < names[j] })

	if in.Kind == OutcomeStruck {
		for i, component := range in.DamageComponents {
			if err := validateDamageComponentRoll(component); err != nil {
				return nil, fmt.Errorf("record: damage component %d roll: %v: %w", i, err, ErrInvalidData)
			}
			if component.Multiplier != nil &&
				(math.IsNaN(*component.Multiplier) || math.IsInf(*component.Multiplier, 0)) {
				return nil, fmt.Errorf("record: damage component %d multiplier: %w", i, ErrInvalidData)
			}
		}
	}

	payload := map[string]interface{}{
		"beat":  string(in.Kind),
		"actor": string(in.Actor),
	}
	if len(targets) > 0 {
		out := make([]string, 0, len(targets))
		for _, id := range targets {
			out = append(out, string(id))
		}
		payload["targets"] = out
	}
	for _, name := range names {
		payload[string(name)] = in.Values[name]
	}

	roll, hasRoll := in.Values[ValueRoll]
	total, hasTotal := in.Values[ValueTotal]
	if hasRoll || hasTotal || in.Calculation != nil {
		if in.Kind != OutcomeStruck && in.Kind != OutcomeMissed {
			return nil, fmt.Errorf("record: roll calculation does not match outcome kind %q: %w", in.Kind, ErrInvalidData)
		}
		if !hasRoll || !hasTotal || in.Calculation == nil {
			return nil, fmt.Errorf("record: attack roll, total, and calculation are required together: %w", ErrInvalidData)
		}
		if err := validateRecordedD20(in.Calculation, roll, total); err != nil {
			return nil, fmt.Errorf("record: attack calculation: %v: %w", err, ErrInvalidData)
		}
		payload["calculation"] = in.Calculation
	}

	// Critical is only ever true for a struck outcome — a miss cannot crit —
	// but it is written unconditionally rather than only when true: false
	// beside a hit is itself the answer ("not a critical"), the same
	// false-vs-absent law the wire seam keeps for its own bools, and a typed
	// reader downstream must not have to treat a missing key as a third
	// state.
	if in.Kind == OutcomeStruck {
		payload["critical"] = in.Critical
		if len(in.DamageComponents) > 0 {
			payload["damage_components"] = in.DamageComponents
		}
		if len(in.AdvantageSources) > 0 {
			payload["advantage_sources"] = in.AdvantageSources
		}
		if len(in.DisadvantageSources) > 0 {
			payload["disadvantage_sources"] = in.DisadvantageSources
		}
	}
	if in.DeathSave != nil {
		if in.DeathSave.Calculation != nil {
			if err := validateRecordedD20(
				in.DeathSave.Calculation, in.DeathSave.Roll, in.DeathSave.Calculation.Total,
			); err != nil {
				return nil, fmt.Errorf("record: death save calculation: %v: %w", err, ErrInvalidData)
			}
		}
		payload["death_save"] = in.DeathSave
	}
	if in.Trade != nil {
		payload["trade"] = in.Trade
	}
	if in.Warded != nil {
		payload["warded"] = in.Warded
	}
	if experience != nil {
		payload["experience"] = experience
	}
	// Omitted when empty, unlike critical's unconditional false: absent and
	// "" say the identical thing here — this swing has no shared roll to
	// correlate — so writing the key would change nothing but the bytes of
	// every beat that predates it.
	if in.PresentationID != "" {
		payload["presentation_id"] = in.PresentationID
	}
	if in.Attack != nil {
		if in.Attack.Ref == "" {
			return nil, fmt.Errorf("record: attack ref: %w", ErrInvalidData)
		}
		if in.Attack.Name == "" {
			return nil, fmt.Errorf("record: attack name: %w", ErrInvalidData)
		}
		payload["attack"] = map[string]string{
			"ref":         in.Attack.Ref,
			"name":        in.Attack.Name,
			"damage_type": in.Attack.DamageType,
		}
	}
	if in.Reaction != nil {
		if in.Reaction.Ref == "" {
			return nil, fmt.Errorf("record: reaction ref: %w", ErrInvalidData)
		}
		if in.Reaction.Name == "" {
			return nil, fmt.Errorf("record: reaction name: %w", ErrInvalidData)
		}
		payload["reaction"] = map[string]string{
			"ref":  in.Reaction.Ref,
			"name": in.Reaction.Name,
		}
	}
	if in.Sequence != nil {
		if in.Sequence.Ref == "" {
			return nil, fmt.Errorf("record: sequence ref: %w", ErrInvalidData)
		}
		if in.Sequence.Name == "" {
			return nil, fmt.Errorf("record: sequence name: %w", ErrInvalidData)
		}
		payload["sequence"] = map[string]string{
			"ref":  in.Sequence.Ref,
			"name": in.Sequence.Name,
		}
	}

	beatBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("record: outcome payload: %w", err)
	}

	// Validated and marshalled BEFORE the outcome beat is appended, with every
	// other refusal above it: a break with an unknown caster or no stated
	// reason must cost the rulebook nothing, and it would cost it a stranded
	// struck beat if it were checked on the way out.
	checkBeats, checkErr := e.prepareConcentrationChecks("record", in.Actor, in.ConcentrationChecks)
	if checkErr != nil {
		return nil, checkErr
	}
	breakBeats, breakErr := e.prepareConcentrationBreaks("record", in.Actor, in.ConcentrationBreaks)
	if breakErr != nil {
		return nil, breakErr
	}
	breakBeats = append(checkBeats, breakBeats...)

	subjects := append([]MemberID{in.Actor}, targets...)
	if experience != nil {
		subjects = experienceSubjects(subjects, experience)
	}
	prepared := []preparedActivationBeat{{payload: beatBytes, subjects: subjects}}
	return append(prepared, breakBeats...), nil
}
