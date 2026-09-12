// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"errors"
	"fmt"
	"sort"

	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/play/intel"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resolution"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/spells"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// buildCastOffers compiles one offer per cantrip or leveled spell this member
// knows AND this build can actually cast.
//
// # A known ref with no cast profile mints no row
//
// The sheet's known cantrip and spell lists are what the character CHOSE;
// [spells.CastDefinition] is what this build can DO with each entry. The
// intersection is the row list, and a
// nil answer is skipped rather than turned into a disabled row — fail closed
// (design R9). A row that resolved to nothing would be the lie; a missing row
// is the truth, and the consequence is real: a bard who chose Mage Hand and
// Light is offered no Cast at all.
//
// This is the one place at this seam where an offer can be absent for a reason
// that is not about the turn. Every other verb keeps its row and explains
// itself, because every other verb's absence would be about budget or reach —
// facts that change as the turn goes on. "This build cannot cast Mage Hand"
// never changes, and a permanently disabled row is a menu item that is not a
// choice.
//
// # It compiles and projects the provider price, unlike buildActivationOffers
//
// An activation projects an answer the rulebook already assembled. A cast reads
// its bounds, price, range and effects from the complete rulebook definition,
// with only the caster-specific save DC supplied. So this compiles the same way
// Attack does — a complete priced [combatActions.Definition] whose serialized
// form IS the selector — with the caster's own save DC written into the gate
// before the hash, so a DC that moved makes the offer stale rather than making
// the click resolve against numbers the player never saw.
//
// Healing uses resolution's reach and living-recipient answers over known
// creatures, including the caster. Other creature casts retain their attack
// eligibility preflight. The seam does not infer healing eligibility from it.
func (m *Manager) buildCastOffers(
	ctx context.Context,
	enc *encounter.Encounter,
	session, member, spellTurn string,
	sheet *character.Character,
	roster []encounter.Member,
	positions map[string]spatial.Position,
	holdings []intel.Holding,
	participants []resolution.Participant,
	dependencyFailures []resolutionDependencyFailure,
) ([]compiledOffer, error) {
	known := append(sheet.KnownCantrips(), sheet.KnownSpells()...)
	if len(known) == 0 {
		return nil, nil
	}

	offers := make([]compiledOffer, 0, len(known))
	for _, ref := range known {
		if ref == nil {
			// Fail closed, exactly as buildActivationOffers does for a
			// nameless ability. A spell with no ref cannot be selected,
			// echoed back or executed, and dropping it quietly would leave a
			// button missing from a panel with no trace of why.
			return nil, fmt.Errorf("member %q knows a spell with no ref: %w", member, ErrBadCharacter)
		}

		definition := sheet.CastDefinition(spells.Spell(ref.ID))
		if definition == nil {
			// R9: this build has no cast content for the ref. Not an error,
			// and not a row.
			continue
		}
		if definition.Cast == nil {
			// A cast definition without a cast profile is a content defect
			// the door must not carry: the target rule and the range below
			// are read off it, and a nil here would compile a row nothing
			// could aim.
			return nil, fmt.Errorf("member %q: spell %q compiled no cast profile: %w",
				member, ref.String(), ErrBadCharacter)
		}
		offer, err := m.compileCastOffer(ctx, &compileCastOfferInput{
			Encounter: enc, SessionID: session, Member: member, Sheet: sheet, SpellTurn: spellTurn,
			Definition: *definition, Roster: roster, Positions: positions, Holdings: holdings,
			Participants: participants, DependencyFailures: dependencyFailures,
		})
		if err != nil {
			return nil, err
		}
		offers = append(offers, offer)
	}

	// Sorted by the spell's ref rather than by the order the sheet happens to
	// hold the choices in — the same within-verb tiebreak activations keep,
	// for the same reason: the panel's order is a fact about the character.
	sortCastOffers(offers)
	return offers, nil
}

// compileCastOfferInput carries one compiled and priced spell into the
// shared declaration compiler.
type compileCastOfferInput struct {
	Encounter          *encounter.Encounter
	SessionID          string
	SpellTurn          string
	Member             string
	Sheet              *character.Character
	Definition         combatActions.Definition
	Roster             []encounter.Member
	Positions          map[string]spatial.Position
	Holdings           []intel.Holding
	Participants       []resolution.Participant
	DependencyFailures []resolutionDependencyFailure
}

// spellTurnIdentity scopes the active turn to its table and world. The active
// member distinguishes turns in the same round; combat exit clears the sheet's
// spell history before another fight can reuse a round number. Offers and the
// resolution payment door must use the same identity across repository reloads.
func spellTurnIdentity(sessionID, encounterID string, clock *encounter.ClockOfOutput) string {
	return fmt.Sprintf("%q/%q/%d/%q", sessionID, encounterID, clock.Round, clock.Active)
}

// compileCastOffer applies the shared budget, dependency, candidate, selector
// and projection rules to one spell — the cast twin of compileAttackOffer,
// over a candidate universe the profile's own target rule chooses.
func (m *Manager) compileCastOffer(
	ctx context.Context, input *compileCastOfferInput,
) (compiledOffer, error) {
	definition := input.Definition
	profile := definition.Cast
	slot := slotOf(definition.Cost)

	if profile.Casting == nil {
		return compiledOffer{}, fmt.Errorf("spell %q has no casting metadata: %w", definition.Ref.String(), ErrBadCast)
	}
	paymentErr := input.Sheet.CanPaySpell(character.SpellPayment{
		Turn: input.SpellTurn, Casting: *profile.Casting, Price: definition.Cost,
	})
	budgetOK := paymentErr == nil
	var budgetWhy *Shortfall
	if !budgetOK {
		shortfall := shortfallForPay(input.Sheet, definition.Cost, slot)
		if errors.Is(paymentErr, combat.ErrBonusActionSpell) {
			shortfall = Shortfall{Reason: ShortfallUnavailable, Text: paymentErr.Error()}
		} else if combat.CanPay(input.Sheet, definition.Cost) {
			return compiledOffer{}, fmt.Errorf("spell %q: %w: %v", definition.Ref.String(), ErrBadCost, paymentErr)
		}
		budgetWhy = &shortfall
	}

	id, variant, err := selectorIDFor(
		input.SessionID, input.Member, VerbCast, slot, nil, &definition, "", "",
	)
	if err != nil {
		return compiledOffer{}, err
	}
	spellRef := SpellRef{Ref: definition.Ref.String(), Name: definition.Name}
	cost, err := castCostComponents(definition.Cost)
	if err != nil {
		return compiledOffer{}, err
	}

	// A SELF-TARGETED CAST PROMPTS FOR NOBODY, and carries no candidates
	// rather than an empty universe that reads as "nobody is reachable" — the
	// same collapse targetKindOfAbility makes for an ability that selects
	// nobody.
	if profile.Target == combatActions.CastTargetSelf {
		declaration := Declaration{
			Verb: VerbCast, Slot: slot, Available: budgetOK, Why: budgetWhy, ID: id,
			Spell: &spellRef, TargetKind: TargetNone, Candidates: []TargetCandidate{},
			MinTargets: profile.MinTargets, MaxTargets: profile.MaxTargets,
			Options: castOptions(profile), Cost: cost,
		}
		return compiledOffer{
			declaration: declaration, spell: &definition, sheet: input.Sheet,
			cast: input.Participants, verb: VerbCast, slot: slot, variant: variant,
		}, nil
	}

	// AN AREA CAST PROMPTS FOR NOBODY EITHER, and takes its own arm rather than
	// falling into either of the two above.
	//
	// Not the self arm, because TargetKind would then say "this lands on you"
	// about a spell that lands on everyone but. Not the targeted arm below,
	// because that one refuses the whole declaration when nothing is in reach
	// (ShortfallNoTargetInReach) — correct when a player must pick somebody,
	// and wrong here: casting a thunderclap in an empty room is legal, spends
	// the action, and catches nobody, which the beat reports honestly.
	//
	// So availability is the budget alone. There is deliberately no "your
	// footprint is empty" verdict: a derived cast has no candidate list by
	// construction, nothing needs the preview yet, and an offer cannot say it
	// without Declaration growing a field for it. When a client wants to draw
	// the ring with the caught members lit up, that field arrives with it —
	// never by overloading Candidates, because a candidate is something you may
	// CHOOSE and nobody chooses these.
	if profile.Target == combatActions.CastTargetArea {
		declaration := Declaration{
			Verb: VerbCast, Slot: slot, Available: budgetOK, Why: budgetWhy, ID: id,
			Spell: &spellRef, TargetKind: areaTargetKind(profile.Area), Candidates: []TargetCandidate{},
			MinTargets: profile.MinTargets, MaxTargets: profile.MaxTargets,
			Options: castOptions(profile), Cost: cost,
		}
		return compiledOffer{
			declaration: declaration, spell: &definition, sheet: input.Sheet,
			cast: input.Participants, verb: VerbCast, slot: slot, variant: variant,
		}, nil
	}

	// The profile chooses physical touch or the established sight/range path.
	// Both use provider eligibility; world NPCs have no healable sheet.
	var candidates []targetPreflight
	if profile.Healing != nil {
		candidates, err = healingCandidates(ctx, input)
	} else {
		candidates, err = m.targetPreflight(
			input.Encounter, input.Positions,
			excludeWorldNPCs(input.Holdings, rosterKinds(input.Roster)),
			input.Member, profile.RangeFeet,
		)
		if err != nil {
			return compiledOffer{}, err
		}
		candidates, err = filterAttackTargets(ctx, candidates, input.Participants)
		if err != nil {
			return compiledOffer{}, err
		}
	}
	if err != nil {
		return compiledOffer{}, err
	}

	// Dependency failures annotate this offer's own working copy: two spells
	// share the same preflight facts and must not share the mutable
	// annotations, exactly as two Attack variants must not.
	candidates = cloneTargetPreflights(candidates)
	var dependencyWhy *Shortfall
	for _, failure := range input.DependencyFailures {
		why := Shortfall{
			Reason: ShortfallUnreadable,
			Text:   fmt.Sprintf("resolution participant %q is unreadable: %v", failure.member, failure.err),
		}
		if dependencyWhy == nil {
			copied := why
			dependencyWhy = &copied
		}
		for i := range candidates {
			if candidates[i].member == failure.member {
				candidates[i].available = false
				candidateWhy := why
				candidates[i].why = &candidateWhy
				break
			}
		}
	}

	// The same precedence Attack keeps: an unreadable dependency outranks a
	// budget refusal, which outranks "nobody in range", and no-one-in-range
	// does not erase the rows.
	anyReachable := anyAvailable(candidates)
	available := dependencyWhy == nil && budgetOK && anyReachable
	var why *Shortfall
	switch {
	case dependencyWhy != nil:
		why = dependencyWhy
	case !budgetOK:
		why = budgetWhy
	case !anyReachable:
		noTarget := Shortfall{Reason: ShortfallNoTargetInReach, Text: "no target in range"}
		why = &noTarget
	}

	targets := make(map[string]targetPreflight, len(candidates))
	for _, candidate := range candidates {
		targets[candidate.member] = candidate
	}

	return compiledOffer{
		declaration: Declaration{
			Verb: VerbCast, Slot: slot, Available: available, Why: why, ID: id,
			Spell: &spellRef, TargetKind: TargetMember, Candidates: projectCandidates(candidates),
			MinTargets: profile.MinTargets, MaxTargets: profile.MaxTargets,
			Options: castOptions(profile), Cost: cost,
		},
		spell: &definition, targets: targets, sheet: input.Sheet,
		cast: input.Participants, verb: VerbCast, slot: slot, variant: variant,
	}, nil
}

// castOptions projects the content's menu onto the seam's own twin, in the
// content's order. That order is the only one there is: a picker draws the
// words in the order the spell authored them, and sorting them here would
// invent a ranking no one wrote down.
//
// A fresh slice per row rather than the profile's own, for the reason every
// offer clones its candidate annotations: two compiled rows share the same
// catalog definition and must not share a slice header into it.
func castOptions(profile *combatActions.CastProfile) []CastOption {
	if len(profile.Options) == 0 {
		// Nil, which the omitempty tag on Declaration.Options renders exactly
		// as an empty slice would — the two are the same answer on the wire,
		// and an earlier version of this comment claimed a distinction the
		// encoding does not make. Returning nil allocates nothing for the many
		// rows that offer no choice.
		return nil
	}
	out := make([]CastOption, 0, len(profile.Options))
	for _, option := range profile.Options {
		out = append(out, CastOption{ID: option.ID, Label: option.Label})
	}
	return out
}

// castCostComponents projects the executable slot and pool price without
// exposing private pool keys. Pool labels come from the rulebook catalog; an
// unknown key is refused rather than displayed as persistence bytes.
func castCostComponents(profile *combat.SpendProfile) ([]CostComponent, error) {
	if profile == nil {
		return []CostComponent{}, nil
	}
	if len(profile.Capacity) != 0 {
		return nil, fmt.Errorf("%w: cast price contains unprojectable capacity", ErrBadCost)
	}

	out := make([]CostComponent, 0, len(profile.Slots)+len(profile.Pools))
	for _, entry := range []struct {
		key      coreCombat.ActionType
		currency Currency
	}{
		{coreCombat.ActionStandard, CurrencyAction},
		{coreCombat.ActionBonus, CurrencyBonus},
		{coreCombat.ActionReaction, CurrencyReaction},
	} {
		if needed := profile.Slots[entry.key]; needed > 0 {
			out = append(out, CostComponent{Currency: entry.currency, Needed: needed})
		}
	}

	keys := make([]coreResources.ResourceKey, 0, len(profile.Pools))
	for key := range profile.Pools {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	for _, key := range keys {
		needed := profile.Pools[key]
		if needed <= 0 {
			continue
		}
		label, ok := resources.DisplayName(key)
		if !ok {
			return nil, fmt.Errorf("%w: cast price pool has no provider display name", ErrBadCost)
		}
		out = append(out, CostComponent{
			Currency: CurrencyCharges, Needed: needed, Label: label,
		})
	}
	return out, nil
}

// sortCastOffers orders compiled Cast rows by the spell's ref. Ranking across
// verbs is sortDeclarations' job and stays there; this is the within-verb
// tiebreak verb ranking alone cannot provide now that one verb has two rows.
func sortCastOffers(offers []compiledOffer) {
	sort.SliceStable(offers, func(i, j int) bool {
		return offers[i].declaration.Spell.Ref < offers[j].declaration.Spell.Ref
	})
}

// areaTargetKind is which of the two derived shapes this area cast is: one
// settled by the profile alone, or one the caster has to point.
//
// THE ORIGIN IS THE WHOLE QUESTION, and it is the CONTENT's answer rather than
// this seam's. An area anchored on the caster's own edge has a direction and
// nothing in the profile supplies it; every other origin this build knows
// hangs off the caster's cell and is complete the moment it is compiled.
//
// A profile with no area at all falls to [TargetArea], which is what it was
// before this function existed. It is not a case worth an arm: CastProfile's
// own Validate binds the target rule and the shape together, deriveAreaMembers
// refuses the same content at the door, and a shape-less area cast that
// prompted for a cell would ask the player to aim nothing.
func areaTargetKind(area *combatActions.CastArea) TargetKind {
	if area != nil && area.Footprint.Origin == combatActions.AreaOriginCasterEdge {
		return TargetCell
	}
	return TargetArea
}

// healingCandidates projects provider answers over known creatures, including
// the caster. The provider distinguishes touch from ranged sight requirements.
func healingCandidates(ctx context.Context, input *compileCastOfferInput) ([]targetPreflight, error) {
	ids := []string{input.Member}
	seen := map[string]bool{input.Member: true}
	for _, holding := range excludeWorldNPCs(input.Holdings, rosterKinds(input.Roster)) {
		id := string(holding.Subject)
		if _, present := input.Positions[id]; present && !seen[id] {
			ids = append(ids, id)
			seen[id] = true
		}
	}
	sort.Strings(ids)
	room, err := input.Encounter.Canvas()
	if err != nil {
		return nil, err
	}
	targets := resolution.HealingTargetsInput{Room: room, CasterID: input.Member, Candidates: ids, Participants: input.Participants}
	var answers map[string]bool
	if input.Definition.Cast.Target == combatActions.CastTargetTouch {
		answers, err = resolution.HealingTargets(ctx, &targets)
	} else {
		answers, err = resolution.RangedHealingTargets(ctx, &resolution.RangedHealingTargetsInput{
			HealingTargetsInput: targets, Encounter: input.Encounter, RangeFeet: input.Definition.Cast.RangeFeet,
		})
	}
	if err != nil {
		return nil, translateResolution(err)
	}
	out := make([]targetPreflight, 0, len(answers))
	for _, id := range ids {
		reachable, eligible := answers[id]
		if !eligible {
			continue
		}
		candidate := targetPreflight{member: id, available: reachable}
		if !reachable {
			candidate.why = &Shortfall{Reason: ShortfallTargetOutOfReach, Text: "Target is not within touch"}
			if input.Definition.Cast.Target != combatActions.CastTargetTouch {
				candidate.why.Text = "Target is not within sight and range"
			}
		}
		out = append(out, candidate)
	}
	return out, nil
}
