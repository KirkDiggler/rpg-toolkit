// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"fmt"
	"sort"

	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	"github.com/KirkDiggler/rpg-toolkit/play/intel"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resolution"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/spells"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// castPrice is what one cantrip costs: the standard action, and nothing else.
//
// # It is compiled HERE rather than read off content, and that is the ruling
//
// A [combatActions.CastProfile] declares what a cast DOES and says nothing
// about what it costs, deliberately — the same profile is free for a monster's
// innate cast and an action for a player's. So the price belongs to whoever
// mints the declaration, which is this seam.
//
// # One action, no pool, no slot
//
// That is the whole of a cantrip's price at level 1, which is why the cast door
// is the first verb at this seam that is really paid at the door (design R10):
// there is nothing underneath it to charge a second time. A levelled spell adds
// a pool entry to this same profile and changes nothing about who charges it.
func castPrice() *combat.SpendProfile {
	return &combat.SpendProfile{
		Slots: map[coreCombat.ActionType]int{coreCombat.ActionStandard: 1},
	}
}

// buildCastOffers compiles one offer per cantrip this member knows AND this
// build can actually cast.
//
// # A known ref with no cast profile mints no row
//
// The sheet's [character.Character.KnownCantrips] is what the bard CHOSE;
// [spells.CastDefinition] is what this build can DO with it, and nine of the
// bard's eleven cantrips answer nil. The intersection is the row list, and a
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
// # It compiles and prices, unlike buildActivationOffers
//
// An activation projects an answer the rulebook already assembled. A cast has
// no such answer to project: nothing on the sheet knows what Vicious Mockery
// reaches, what it costs, or what DC it demands. So this compiles the same way
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
	known := sheet.KnownCantrips()
	if len(known) == 0 {
		return nil, nil
	}

	// The caster answers its own DC once, before any spell is compiled: it is
	// 8 + proficiency + spellcasting modifier, a property of the caster known
	// before any machine starts (design R4), so it does not vary between two
	// cantrips on the same sheet.
	dc := sheet.SpellSaveDC()

	offers := make([]compiledOffer, 0, len(known))
	for _, ref := range known {
		if ref == nil {
			// Fail closed, exactly as buildActivationOffers does for a
			// nameless ability. A cantrip with no ref cannot be selected,
			// echoed back or executed, and dropping it quietly would leave a
			// button missing from a panel with no trace of why.
			return nil, fmt.Errorf("member %q knows a cantrip with no ref: %w", member, ErrBadCharacter)
		}

		definition := spells.CastDefinition(spells.Spell(ref.ID), dc)
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
		definition.Cost = castPrice()

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

// compileCastOfferInput carries one compiled and priced cantrip into the
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
// and projection rules to one cantrip — the cast twin of compileAttackOffer,
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

	// A SELF-TARGETED CAST PROMPTS FOR NOBODY, and carries no candidates
	// rather than an empty universe that reads as "nobody is reachable" — the
	// same collapse targetKindOfAbility makes for an ability that selects
	// nobody.
	if profile.Target == combatActions.CastTargetSelf {
		declaration := Declaration{
			Verb: VerbCast, Slot: slot, Available: budgetOK, Why: budgetWhy, ID: id,
			Spell: &spellRef, TargetKind: TargetNone, Candidates: []TargetCandidate{},
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

	// Dependency failures annotate this offer's own working copy: two cantrips
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
		},
		spell: &definition, targets: targets, sheet: input.Sheet,
		cast: input.Participants, verb: VerbCast, slot: slot, variant: variant,
	}, nil
}

// sortCastOffers orders compiled Cast rows by the spell's ref. Ranking across
// verbs is sortDeclarations' job and stays there; this is the within-verb
// tiebreak verb ranking alone cannot provide now that one verb has two rows.
func sortCastOffers(offers []compiledOffer) {
	sort.SliceStable(offers, func(i, j int) bool {
		return offers[i].declaration.Spell.Ref < offers[j].declaration.Spell.Ref
	})
}
