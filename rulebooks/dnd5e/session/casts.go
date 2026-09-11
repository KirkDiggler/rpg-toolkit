// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
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
// # Every candidate rule is Attack's, and that is the finding rather than a
// shortcut
//
// The design's R6 asks for "a member not the caster, held in the caster's
// sight, within range", which is [Manager.targetPreflight] over the same
// world-NPC-excluded holdings Attack uses, followed by the same participation
// filter. There is no hostility predicate here, because [combatActions.CastProfile]
// declares none: a session that decided True Strike may only be pointed at an
// enemy would be deciding a content rule in the one place that cannot see the
// content (ADR-0045), and RAW 2014 agrees with the profile — both cantrips
// this build ships name "a creature", not "a hostile creature".
func (m *Manager) buildCastOffers(
	ctx context.Context,
	enc *encounter.Encounter,
	session, member string,
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

	// The caster answers its own DC once, before any spell is compiled: it is
	// 8 + proficiency + spellcasting modifier, a property of the caster known
	// before any machine starts (design R4), so it does not vary between two
	// known spells on the same sheet.
	dc := sheet.SpellSaveDC()

	offers := make([]compiledOffer, 0, len(known))
	for _, ref := range known {
		if ref == nil {
			// Fail closed, exactly as buildActivationOffers does for a
			// nameless ability. A spell with no ref cannot be selected,
			// echoed back or executed, and dropping it quietly would leave a
			// button missing from a panel with no trace of why.
			return nil, fmt.Errorf("member %q knows a spell with no ref: %w", member, ErrBadCharacter)
		}

		definition := spells.CastDefinition(spells.CastDefinitionInput{
			Spell: spells.Spell(ref.ID), SpellSaveDC: dc,
		})
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
			Encounter: enc, SessionID: session, Member: member, Sheet: sheet,
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
	Member             string
	Sheet              *character.Character
	Definition         combatActions.Definition
	Roster             []encounter.Member
	Positions          map[string]spatial.Position
	Holdings           []intel.Holding
	Participants       []resolution.Participant
	DependencyFailures []resolutionDependencyFailure
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

	budgetOK := combat.CanPay(input.Sheet, definition.Cost)
	var budgetWhy *Shortfall
	if !budgetOK {
		shortfall := shortfallForPay(input.Sheet, definition.Cost, slot)
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
			MinTargets: profile.MinTargets, MaxTargets: profile.MaxTargets, Cost: cost,
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
			Spell: &spellRef, TargetKind: TargetArea, Candidates: []TargetCandidate{},
			MinTargets: profile.MinTargets, MaxTargets: profile.MaxTargets, Cost: cost,
		}
		return compiledOffer{
			declaration: declaration, spell: &definition, sheet: input.Sheet,
			cast: input.Participants, verb: VerbCast, slot: slot, variant: variant,
		}, nil
	}

	// The candidate universe, by Attack's own rules over this spell's range:
	// every live sighting except the caster, within RangeFeet, world NPCs
	// excluded, then the participation filter that removes the dead and the
	// defeated while keeping the Dying and the Stabilized.
	candidates, err := m.targetPreflight(
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
			MinTargets: profile.MinTargets, MaxTargets: profile.MaxTargets, Cost: cost,
		},
		spell: &definition, targets: targets, sheet: input.Sheet,
		cast: input.Participants, verb: VerbCast, slot: slot, variant: variant,
	}, nil
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
