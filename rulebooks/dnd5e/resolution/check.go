// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package resolution

import (
	"context"
	"errors"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/checks"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
)

// CheckInput is one character against one authored check, on the way in.
//
// The checker is a RECORD, not a live sheet, for [ProjectCharacterInput]'s
// reason: the seam fetches records and only records, and truth is built down
// here. And it is a CHARACTER record by shape — characters are the only
// checkers in v1 (rpg-project#351). A monster checker is not refused at
// runtime; it is inexpressible, which is the stronger statement. The
// monster-Skills gap is rpg-toolkit#1358's own issue and is deliberately not
// half-solved here.
type CheckInput struct {
	// Character is the checker's sheet as the host stored it.
	Character *character.Data

	// Approaches are the check's authored routes, each with its own DC —
	// [encounter.CheckApproach]'s contract, carried in that vocabulary
	// because that is the vocabulary the authored world already speaks and a
	// second spelling of the same fact is the dual representation this repo
	// bans. This entry applies the character's best listed one (the standing
	// ruling: slice 1 pushes no approach choice to the player).
	//
	// TWO PAYLOAD SHAPES ARRIVE HERE and both are lawful: a find check lists
	// SKILL refs (perception, investigation), a lock check lists BARE ABILITY
	// refs (str, dex) with tool refs riding alongside. Which is which is not
	// a case split this entry makes — every route resolves through the same
	// dispatch, skill first, then ability (see [approachModifier]).
	//
	// An approach's Tool is carried unread: tool proficiency is shelved with
	// the tomb's authoring (rpg-project#269 §6.4), and a modifier invented
	// for it here would be a rule nobody wrote.
	Approaches []encounter.CheckApproach

	// Roller rolls the check. REQUIRED — supplied, never defaulted, for
	// [ErrNoRoller]'s standing reason: a silent default puts untestable
	// randomness into a result that looks fine.
	Roller dice.Roller
}

// CheckOutput is the check's verdict. All of it is data.
type CheckOutput struct {
	// Result is the roll, its total, whether it beat the DC — and the record
	// of *which* effects granted advantage, imposed disadvantage, or added a
	// bonus, in its source lists. Per-source for [SaveOutcome]'s reason: a
	// total alone can say "there was advantage" but not "Raging granted it",
	// and the difference is the whole point of a bus.
	//
	// The margin is the caller's to read off Total and DC if it wants one;
	// band vocabulary is campaign-scale and stays OUT of resolution — the
	// integration boundary holds.
	Result *checks.AbilityCheckResult

	// Applied is the route this entry selected and rolled — exactly one of
	// the listed approaches, carried so the verdict can name the DC that was
	// actually faced.
	Applied encounter.CheckApproach

	// DirtyCharacter is the checker's record after the check, when the check
	// changed it — nil means the sheet came through untouched, which is
	// today's every case. Returned rather than dropped because nothing may
	// survive this call that is not returned: the day a condition spends
	// itself on a check (guidance is the canonical one), the write-back is
	// already in the caller's hands instead of silently lost down here.
	DirtyCharacter *character.Data
}

// MakeCheck is the lawful way to make an ability check for a stored character:
// load the character, attach their conditions, select their best listed
// approach, and roll it with the AbilityCheckChain firing on resolution's own
// bus. Records in, answers out — the caller holds no sheet, no bus, and no
// rule.
//
// THERE IS NO UNAIDED CHECK. For a real character nobody can prove no
// condition applies, so a check made without the chain is a claim the rules
// packages refuse to express (rpg-toolkit#1357, the checks package's own
// door) — and this entry is where the lawful call lives, because the bus
// lives here and nowhere else (ADR-0038). The session's CheckResolver
// capability is implemented by calling this entry; the seam carries refs and
// records, never sheets (rpg-toolkit#1380).
//
// BEST is the approach that maximises the chance of success: the character's
// modifier for the route minus the route's own DC, highest wins, ties broken
// by authored order so the answer cannot move between calls. That is a
// mechanism ruling inside the ruled principle (rpg-project#350, which prices
// routes separately — best cannot mean best modifier alone: a +1 route at
// DC 10 beats a +3 route at DC 15). Selection reads the raw sheet and the
// fold modifies the roll: a condition can change what the d20 lands on, not
// which route is walked. Approach choice by the PLAYER is postponed by the
// same ruling; nothing here forecloses it.
//
// # It reads strictly, unlike the projection, and the difference is the ruling
//
// [ProjectCharacter] drops a condition blob it cannot parse and warns,
// because a projection only reads and refusing would put one unreadable blob
// between a player and their AC display. This entry REFUSES the same record:
// its output is a rules verdict, and a check rolled past a condition that
// could not load is exactly the unaided check the ruling forbids — silently
// missing disadvantage is a found door that should have stayed hidden. One
// attach mechanism, policy per entry; this entry keeps [attachAllInput]'s
// zero-value strictness.
//
// # There is no world here, and that is v1's honest answer
//
// The room this installs is ABSENT, [ProjectCharacter]'s move: what crosses
// the seam is a record and a route list, and inventing a room to keep a shape
// tidy would answer "there is nothing here", which is a different and false
// statement. A check effect that needs the world fails closed with ErrNoRoom
// readable where it failed, and the first such effect brings the world input
// with it — load-bearing or deferred, not defaulted.
func MakeCheck(ctx context.Context, in *CheckInput) (*CheckOutput, error) {
	// One bus for this check, created here and nowhere else — the same claim
	// Resolve and ProjectCharacter make, made the same way.
	return makeCheckOn(ctx, in, newSurface(events.NewEventBus()))
}

// makeCheckOn is [MakeCheck] with the surface handed in, so a test can hold
// the bus underneath and check what is left on it afterwards. Unexported for
// resolveOn's reason: a caller supplying its own bus would be a caller
// keeping one alive, which is the thing this package exists to prevent.
func makeCheckOn(ctx context.Context, in *CheckInput, surf *surface) (*CheckOutput, error) {
	if in == nil {
		return nil, ErrNilInput
	}
	if in.Roller == nil {
		return nil, fmt.Errorf("%w: a check rolls a d20", ErrNoRoller)
	}
	if len(in.Approaches) == 0 {
		// A check with no route through it is content this entry cannot
		// judge — the composition validates this out of authored doors, so
		// reaching it here is a defect, not a failed roll.
		return nil, fmt.Errorf("%w: check lists no approaches", ErrBadCheck)
	}

	// The same validation every participant gets, from the same function, for
	// the same reason: a sheet with no ID cannot be read back out of the cast
	// it was just put into.
	one := Participant{Character: in.Character}
	if err := one.validate(); err != nil {
		return nil, err
	}

	cast, err := attachAll(ctx, surf, &attachAllInput{
		Participants: []Participant{one},
		// This entry builds its own participant and never builds a monster
		// one, so attach's roller — reached only through the monster branch —
		// is unreachable by construction. The CHECK's roller is in.Roller,
		// handed to the rules entry below; conflating the two would let a
		// trait roll with the caller's check dice.
		Roller: refusingRoller{},
		// DropUnreadable stays at its zero value: this entry refuses what it
		// cannot read. See the strictness section of [MakeCheck]'s doc.
	})
	if err != nil {
		// Tear down whatever did attach before giving up, exactly as every
		// other entry does: leaving revocation to an error path's silence is
		// how leaks become normal.
		_ = surf.teardown(ctx)

		return nil, err
	}

	// THE SAME DOOR, with a cast of one and no world. Not a second installer
	// and not a lighter one — there is exactly one function that may do this,
	// and this is a caller of it.
	ctx = installTruth(ctx, nil, cast, nil)

	ch, ok := cast.Character(one.ID())
	if !ok {
		// attachAll put it there one statement ago, under this exact ID, so
		// this is unreachable rather than unlikely. It refuses anyway: the day
		// that stops being true, a nil dereference is the worst available way
		// to find out.
		return nil, errors.Join(
			fmt.Errorf("%w: %q attached but is not in the cast", ErrBadParticipant, one.ID()),
			surf.teardown(ctx),
		)
	}

	applied, modifier, skill, selErr := bestApproach(ch, in.Approaches)
	if selErr != nil {
		return nil, errors.Join(
			fmt.Errorf("resolution: check for %q: %w", one.ID(), selErr),
			surf.teardown(ctx),
		)
	}

	// The chain folds exactly once, inside the rules entry that owns the
	// arithmetic, on THIS interaction's bus — the surface delegates every
	// publish to the one bus this entry created. Same custody shape as the
	// save machine's Gather (see save.go): the rules package requires the bus
	// and this package is the only lawful supplier.
	result, checkErr := checks.MakeAbilityCheck(ctx, &checks.AbilityCheckInput{
		Roller:    in.Roller,
		EventBus:  surf,
		CheckerID: one.ID(),
		Skill:     skill,
		DC:        applied.DC,
		Modifier:  modifier,
	})

	// Revoked on every exit whether or not the roll worked, because a
	// subscription that outlives its interaction is the leak this package
	// exists to prevent.
	tearErr := surf.teardown(ctx)

	if checkErr != nil {
		return nil, errors.Join(
			fmt.Errorf("resolution: check for %q: %w", one.ID(), checkErr),
			tearErr,
		)
	}
	if tearErr != nil {
		return nil, fmt.Errorf("resolution: teardown: %w", tearErr)
	}

	out := &CheckOutput{Result: result, Applied: applied}
	if ch.IsDirty() {
		out.DirtyCharacter = ch.ToData()
	}

	return out, nil
}

// bestApproach selects the character's best listed route and reads their
// modifier for it: modifier minus DC, highest wins, ties to authored order —
// the strictly-greater comparison IS the tie rule, and a test pins it.
//
// Every route is validated whether or not it wins, deliberately: an authored
// list where route three names no rulebook skill or ability is broken content
// today, not broken content on the day somebody's modifiers make route three
// best.
func bestApproach(
	ch *character.Character, approaches []encounter.CheckApproach,
) (encounter.CheckApproach, int, skills.Skill, error) {
	best := -1
	bestModifier := 0
	var bestSkill skills.Skill

	for i, a := range approaches {
		if a.DC < 1 {
			// CheckApproach's own well-formedness rule ("a check with nothing
			// to beat is not a check, and zero is what an undeclared one would
			// look like"), enforced where the DC is about to be faced.
			return encounter.CheckApproach{}, 0, "", fmt.Errorf(
				"%w: approach %q has no difficulty (DC %d)", ErrBadCheck, a.Ability, a.DC)
		}

		modifier, skill, err := approachModifier(ch, a)
		if err != nil {
			return encounter.CheckApproach{}, 0, "", err
		}

		if best < 0 || modifier-a.DC > bestModifier-approaches[best].DC {
			best, bestModifier, bestSkill = i, modifier, skill
		}
	}

	return approaches[best], bestModifier, bestSkill, nil
}

// approachModifier reads the character's modifier for one authored route off
// their real sheet: a skill ref rolls the skill (ability modifier plus
// proficiency, [character.Character.GetSkillModifier]'s own arithmetic), a
// bare ability ref rolls the raw ability. An identifier the rulebook has
// neither skill nor ability for is a CONTENT defect refused loudly — never a
// silent zero from an unknown key.
//
// The returned skill is what the chain event carries: empty for a bare
// ability route, honestly, because "no skill" is what a raw STR check is —
// a subscriber whose predicate reads the skill declines an empty one on its
// own, which is applicability staying the effect's business.
func approachModifier(
	ch *character.Character, a encounter.CheckApproach,
) (int, skills.Skill, error) {
	if skill, err := skills.GetByID(a.Ability); err == nil {
		return ch.GetSkillModifier(skill), skill, nil
	}

	ability, err := abilities.GetByID(a.Ability)
	if err != nil {
		return 0, "", fmt.Errorf(
			"%w: approach names no rulebook skill or ability (%q)", ErrBadCheck, a.Ability)
	}

	return ch.GetAbilityModifier(ability), "", nil
}
