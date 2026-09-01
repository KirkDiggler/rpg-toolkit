// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/checks"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
)

// conceal.go is THE SEAM'S HALF OF CONCEALMENT (living-world slice 1, wave 1b
// — rpg-toolkit#1375; ruled on rpg-project#350 and #351): the two capabilities
// a concealed field refuses to build without, implemented from things this
// package already owns. The composition owns every rule about what concealment
// MEANS; what it cannot know is what a DC means to a character sheet
// (checkSeam) and how far the host's light-and-sight truth reaches
// (witnessSeam). Both are the [encounter.Standing] move again: the lookup
// lives here, the rule lives where the rules live.

// stagedCheck is one member's sheet made ready to roll: loaded, and holding
// the verb's own context.
//
// STAGED BY THE VERB, BEFORE THE COMPOSITION ACTS, and that ordering is a
// secrecy law rather than a convenience. The composition consults
// [encounter.CheckResolver] only when the swept region actually holds an
// unfound concealed door — so a resolver that loaded the sheet lazily, at
// consult time, would fail a monster searcher (no loadable character) in
// exactly and only the rooms with something to find, and the refusal itself
// would answer the question the search asked. Staging up front makes "who can
// roll checks" decided identically for every region, empty or not.
//
// THERE IS DELIBERATELY NO BUS HERE, AND THE CHAIN THEREFORE DOES NOT FIRE —
// stated loudly because it is a scoped narrowing of the ruled contract, not
// an oversight. The check rolls through [checks.MakeAbilityCheck], the one
// function that owns dnd5e's AbilityCheckChain, under that function's own
// nil-bus contract ("no chain events are fired"). The examples' dnd5eresolver
// carries a bus because the examples have no other home for one; THIS module
// does — resolution — and its ratified structural pin
// (TestNoBusLivesInThisModule, the game-context slice) forbids a bus living
// here, tests included, precisely so no chain can ever fold at this seam.
// Today the narrowing is behaviorally lossless: rpg-toolkit#1357's own
// finding is that nothing yet subscribes to the chain on either side. The
// chain going LIVE is the already-named resolution.Resolve rung — "swaps
// depth, not rules" lands exactly there, where the bus lawfully lives and
// the whole cast attaches (R3). Until then a condition that would modify a
// check is not silently degraded by wiring; it is unreachable by module law.
// RULED on the rpg-toolkit#1375 friction report and recorded on
// rpg-project#351: "first production subscriber" narrows to first production
// CALLER of the check machinery, chain dormant until the resolution rung —
// which the resolution-rung slice inherits in writing.
type stagedCheck struct {
	// ctx is the staging verb's own context, carried because the
	// composition's capability interface takes none — the consult happens
	// inside the verb's own synchronous call, so its lifetime is exactly
	// this context's.
	ctx context.Context

	sheet *character.Character
}

// stageCheck loads one member's character, attaches it to a fresh bus, and
// stages it on the scope for [checkSeam] to find.
//
// Characters are the only searchers in v1 (rpg-project#351): a member with no
// loadable character — a monster — is refused here, uniformly, before the
// composition ever looks at the region. See [stagedCheck] for why the refusal
// must not wait for the consult.
func (m *Manager) stageCheck(ctx context.Context, scope *writeScope, role, member string) error {
	data, err := m.fetchCharacterData(ctx, role, member)
	if err != nil {
		return err
	}
	sheet, err := character.Load(ctx, data)
	if err != nil {
		return fmt.Errorf("%s %q: %w: %v", role, member, ErrBadCharacter, err)
	}

	if scope.checks == nil {
		scope.checks = make(map[string]*stagedCheck, 1)
	}
	scope.checks[member] = &stagedCheck{ctx: ctx, sheet: sheet}
	return nil
}

// checkSeam is this package's [encounter.CheckResolver], bound to the live
// write scope the way [strikerSeam] is and for the same chicken-and-egg
// reason: LoadEncounter needs the capability to construct the very
// *encounter.Encounter whose Search later consults it, so the seam reads
// scope AT CALL TIME, well after openForWrite has settled the fields.
type checkSeam struct {
	m     *Manager
	scope *writeScope
}

// compile-time proof the seam satisfies what it is handed to.
var _ encounter.CheckResolver = checkSeam{}

// ResolveCheck rolls one authored check for one member, applying their best
// listed approach — the whole rule lives in [resolveApproaches], shared with
// [Manager.Unlock] so the two verbs that roll checks cannot drift.
//
// A member with no staged sheet is a wiring fault: the verb that reached the
// composition without staging its actor is this package's own bug, and the
// error says so at the point of failure rather than rolling a check for
// nobody.
func (c checkSeam) ResolveCheck(in *encounter.ResolveCheckInput) (*encounter.ResolveCheckOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("resolve check: %w", ErrNilInput)
	}
	staged, ok := c.scope.checks[string(in.Member)]
	if !ok {
		return nil, fmt.Errorf("resolve check for %q: no sheet was staged for this verb: %w",
			in.Member, ErrNoSheet)
	}
	return resolveApproaches(staged, string(in.Member), in.Approaches, c.m.dice)
}

// resolveApproaches is THE ONE PLACE A CHECK IS ROLLED at this seam: it picks
// the member's best listed approach, makes the real ability check through
// dnd5e's own machinery, and reports the verdict. Search's find checks
// (through [checkSeam]) and Unlock's lock checks both land here — the rules
// live once, and the later resolution.Resolve rung swaps this function's
// depth, not its callers.
//
// BEST is the approach that maximises the chance of success: the member's
// modifier for the route minus the route's own DC, highest wins, ties broken
// by authored order so the answer cannot move between calls. That is a
// mechanism ruling inside the ruled principle ("the resolver applies the
// character's best listed approach", rpg-project#350, which prices routes
// separately — so best cannot mean best modifier alone: a +1 route at DC 10
// beats a +3 route at DC 15). Approach choice by the PLAYER is postponed by
// the same ruling; nothing here forecloses it.
//
// An approach's Tool rides the authored data and is deliberately not read:
// tool proficiency is shelved with the tomb's authoring (rpg-project#269
// §6.4), and a modifier invented for it here would be a rule nobody wrote.
func resolveApproaches(
	staged *stagedCheck, member string, approaches []encounter.CheckApproach, roller Roller,
) (*encounter.ResolveCheckOutput, error) {
	if len(approaches) == 0 {
		// A check with no route through it is content this build cannot
		// judge — the composition validates this out of authored doors, so
		// reaching it here is a defect, not a failed roll.
		return nil, fmt.Errorf("member %q: check lists no approaches: %w", member, ErrInvalidWorld)
	}

	best := -1
	bestModifier := 0
	var bestSkill skills.Skill
	for i, a := range approaches {
		modifier, skill, err := approachModifier(staged.sheet, a)
		if err != nil {
			return nil, fmt.Errorf("member %q: %w", member, err)
		}
		if best < 0 || modifier-a.DC > bestModifier-approaches[best].DC {
			best, bestModifier, bestSkill = i, modifier, skill
		}
	}
	applied := approaches[best]

	// No EventBus, by module law — [stagedCheck]'s own account of the
	// narrowing. MakeAbilityCheck's nil-bus contract applies: modifier,
	// advantage arithmetic and the verdict are the rulebook's, and no chain
	// fires at this seam.
	check, err := checks.MakeAbilityCheck(staged.ctx, &checks.AbilityCheckInput{
		Roller:   &diceSeam{roller: roller},
		Skill:    bestSkill,
		DC:       applied.DC,
		Modifier: bestModifier,
	})
	if err != nil {
		// A foreign error (rpgerr): carried as text, never wrapped into our
		// chain (translate's own law).
		return nil, fmt.Errorf("member %q: check failed: %v", member, err)
	}

	return &encounter.ResolveCheckOutput{
		Beaten:  check.Success,
		Applied: applied,
		Total:   check.Total,
	}, nil
}

// approachModifier reads the member's modifier for one authored route off
// their real sheet: a skill ref rolls the skill (ability modifier plus
// proficiency, [character.Character.GetSkillModifier]'s own arithmetic), a
// bare ability ref rolls the raw ability. An identifier the rulebook has
// neither skill nor ability for is a CONTENT defect refused loudly — never a
// silent zero from an unknown key (the Copilot round on #1243's law).
func approachModifier(sheet *character.Character, a encounter.CheckApproach) (int, skills.Skill, error) {
	if skill, err := skills.GetByID(a.Ability); err == nil {
		return sheet.GetSkillModifier(skill), skill, nil
	}
	ability, err := abilities.GetByID(a.Ability)
	if err != nil {
		return 0, "", fmt.Errorf(
			"approach names no rulebook skill or ability (%q): %w", a.Ability, ErrInvalidWorld)
	}
	return sheet.GetAbilityModifier(ability), "", nil
}

// witnessSeam is this package's [encounter.Witness]: who currently perceives
// an open concealed door, answered FROM THE ONE SIGHT SEAM.
//
// The consistency obligation (review-1373, on the record): Sight and Witness
// must agree, because a Sight that sees through an open concealed door while
// Witness denies perception would deliver a hidden-floor percept with no
// reveal. This seam earns the agreement structurally rather than by promise —
// the three predicates below are [encounter]'s own percept predicates,
// answered by the same instruments:
//
//   - reach comes from the SAME *sightSeam pointer the live encounter holds
//     (scope.sight — a member placed by this very verb is already in it,
//     sight.go's own pointer argument);
//   - line of sight is asked of the encounter's own canvas
//     ([encounter.Encounter.Canvas]), the same geometry rebuildPercepts
//     walks;
//   - the range test is rebuildPercepts's own strictly-greater cut ("a
//     member exactly at the edge of your sight is inside it"), applied
//     through [encounter.Encounter.Distance] — the same grid metric.
//
// A member perceives the door when they see EITHER endpoint of ANY of its
// crossings — the cells the composition hands over as "the cells a perceiver
// would have to see".
//
// Calling back into the live encounter from inside its own verb is the
// established reentrancy pattern ([strikerSeam] reads Members and Records
// from inside a driven turn); everything read here is read-only.
type witnessSeam struct {
	scope *writeScope
}

// compile-time proof the seam satisfies what it is handed to.
var _ encounter.Witness = witnessSeam{}

// Perceivers reports which members currently perceive the door. See the
// type's own doc for where each predicate comes from.
func (w witnessSeam) Perceivers(in *encounter.PerceiversInput) ([]encounter.MemberID, error) {
	if in == nil {
		return nil, fmt.Errorf("perceivers: %w", ErrNilInput)
	}
	enc := w.scope.enc

	roster, err := enc.Members()
	if err != nil {
		return nil, err
	}
	canvas, err := enc.Canvas()
	if err != nil {
		return nil, err
	}
	reach, err := w.scope.sight.Sight(rosterIDs(roster))
	if err != nil {
		return nil, err
	}

	var out []encounter.MemberID
	for _, member := range roster {
		cells := reach[member.ID]
		for _, edge := range in.Edges {
			seesFrom := enc.Distance(member.Position, edge.From) <= float64(cells) &&
				!canvas.IsLineOfSightBlocked(member.Position, edge.From)
			seesTo := enc.Distance(member.Position, edge.To) <= float64(cells) &&
				!canvas.IsLineOfSightBlocked(member.Position, edge.To)
			if seesFrom || seesTo {
				out = append(out, member.ID)
				break
			}
		}
	}
	return out, nil
}

// refusingCheckResolver is the read-path stand-in, [encounter.RefusingStriker]'s
// pattern one capability over: a read verb loads worlds it never drives, so a
// check resolved on one is this package's own bug, reported at the point of
// failure rather than answered with an invented roll.
type refusingCheckResolver struct{}

// compile-time proof the stand-in satisfies what it is handed to.
var _ encounter.CheckResolver = refusingCheckResolver{}

// ResolveCheck always refuses: no read verb rolls a check.
func (refusingCheckResolver) ResolveCheck(*encounter.ResolveCheckInput) (*encounter.ResolveCheckOutput, error) {
	return nil, fmt.Errorf("a check was resolved on a read-only world: %w", ErrInvalidWorld)
}

// refusingWitness is refusingCheckResolver's twin for perception: reads
// refresh no sight, so a witness consulted on a read path is a bug, not an
// event.
type refusingWitness struct{}

// compile-time proof the stand-in satisfies what it is handed to.
var _ encounter.Witness = refusingWitness{}

// Perceivers always refuses: no read verb refreshes sight.
func (refusingWitness) Perceivers(*encounter.PerceiversInput) ([]encounter.MemberID, error) {
	return nil, fmt.Errorf("a witness was consulted on a read-only world: %w", ErrInvalidWorld)
}
