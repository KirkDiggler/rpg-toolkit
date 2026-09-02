// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"errors"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resolution"
)

// conceal.go is THE SEAM'S HALF OF CONCEALMENT (living-world slice 1, wave 1b
// — rpg-toolkit#1375; ruled on rpg-project#350 and #351): the two capabilities
// a concealed field refuses to build without, implemented from things this
// package already owns. The composition owns every rule about what concealment
// MEANS; what it cannot know is how a check resolves against a character
// (checkSeam — answered by handing the stored record to resolution, the one
// layer that lawfully loads sheets and folds chains) and how far the host's
// light-and-sight truth reaches (witnessSeam). Both are the
// [encounter.Standing] move again: the lookup lives here, the rule lives
// where the rules live.

// stagedCheck is one member's stored character record, staged for check
// resolution, with the verb's own context.
//
// STAGED BY THE VERB, BEFORE THE COMPOSITION ACTS, and that ordering is a
// secrecy law rather than a convenience. The composition consults
// [encounter.CheckResolver] only when the swept region actually holds an
// unfound concealed door — so a resolver that fetched the record lazily, at
// consult time, would fail a monster searcher (no loadable character) in
// exactly and only the rooms with something to find, and the refusal itself
// would answer the question the search asked. Staging up front makes "who
// can roll checks" decided identically for every region, empty or not — and
// the record is VALIDATED at staging for the same reason: a sheet that will
// not load must refuse here, uniformly, not at a consult whose very
// occurrence depends on what the room holds.
//
// WHAT IS STAGED IS A RECORD, NOT A SHEET. Check resolution moved behind
// resolution's door (resolution v0.27.0's MakeCheck, toolkit#1380; re-ruled
// on rpg-project#351): resolution loads the character, attaches their
// conditions, selects the best listed approach, and rolls with the
// AbilityCheckChain firing on its own lawful bus — this seam holds no sheet
// at roll time, no bus ever (TestNoBusLivesInThisModule), and no rule.
// Records in, answers out.
type stagedCheck struct {
	// ctx is the staging verb's own context, carried because the
	// composition's capability interface takes none — the consult happens
	// inside the verb's own synchronous call, so its lifetime is exactly
	// this context's.
	ctx context.Context

	data *character.Data
}

// stageCheck fetches one member's character record, proves it loads, and
// stages it on the scope for [checkSeam] to find.
//
// Characters are the only searchers in v1 (rpg-project#351): a member with no
// loadable character — a monster — is refused here, uniformly, before the
// composition ever looks at the region. See [stagedCheck] for why the refusal
// must not wait for the consult. The loaded sheet is deliberately discarded:
// it exists to make ErrBadCharacter fire at the same uniform point
// ErrNoCharacter does, and the sheet resolution rolls is the one RESOLUTION
// loads, inside its own interaction.
func (m *Manager) stageCheck(ctx context.Context, scope *writeScope, role, member string) error {
	data, err := m.fetchCharacterData(ctx, role, member)
	if err != nil {
		return err
	}
	if _, err := character.Load(ctx, data); err != nil {
		return fmt.Errorf("%s %q: %w: %v", role, member, ErrBadCharacter, err)
	}

	if scope.checks == nil {
		scope.checks = make(map[string]*stagedCheck, 1)
	}
	scope.checks[member] = &stagedCheck{ctx: ctx, data: data}
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

// ResolveCheck resolves one authored check for one member through
// [Manager.resolveStagedCheck] — shared with [Manager.Unlock] so the two
// verbs that resolve checks cannot drift.
func (c checkSeam) ResolveCheck(in *encounter.ResolveCheckInput) (*encounter.ResolveCheckOutput, error) {
	if in == nil {
		return nil, fmt.Errorf("resolve check: %w", ErrNilInput)
	}
	return c.m.resolveStagedCheck(c.scope, string(in.Member), in.Approaches)
}

// resolveStagedCheck is THE ONE PLACE A CHECK CROSSES TO RESOLUTION at this
// seam: the staged record goes down, [resolution.MakeCheck] loads the
// character with their conditions attached, selects the best listed approach
// (net pricing — the member's modifier against each route's own DC — ties to
// authored order; the mechanism ruling that briefly lived here, now
// resolution's own pin), rolls it with the AbilityCheckChain firing on
// resolution's bus, and the verdict comes back as data. Search's find checks
// (through [checkSeam]) and Unlock's lock checks both land here.
//
// A member with no staged record is a wiring fault: the verb that reached
// the composition without staging its actor is this package's own bug, and
// the error says so at the point of failure rather than rolling a check for
// nobody.
//
// A DirtyCharacter on the answer is written back through the same
// save-and-report path a swing's dirty sheets use ([Manager.saveDirty]'s
// shape): nil today — no condition yet spends itself on a check — but the
// day guidance does, the write is already in hand instead of silently lost.
func (m *Manager) resolveStagedCheck(
	scope *writeScope, member string, approaches []encounter.CheckApproach,
) (*encounter.ResolveCheckOutput, error) {
	staged, ok := scope.checks[member]
	if !ok {
		return nil, fmt.Errorf("resolve check for %q: no record was staged for this verb: %w",
			member, ErrNoSheet)
	}

	out, err := resolution.MakeCheck(staged.ctx, &resolution.CheckInput{
		Character:  staged.data,
		Approaches: approaches,
		Roller:     &diceSeam{roller: m.dice},
	})
	switch {
	case errors.Is(err, resolution.ErrBadCheck):
		// A check the rulebook cannot judge — a route naming no rulebook
		// skill or ability, an empty approach list — is a CONTENT defect:
		// the world that authored it is unusable, and the host matches the
		// same sentinel it matches for every other unreadable world.
		// Resolution's own account rides as text so the message still names
		// the offending ref (never a silent -5 from an unknown key).
		return nil, fmt.Errorf("member %q: %w: %v", member, ErrInvalidWorld, err)
	case err != nil:
		// Anything else is foreign (rpgerr): carried as text, never wrapped
		// into our chain (translate's own law).
		return nil, fmt.Errorf("member %q: check failed: %v", member, err)
	}

	if out.DirtyCharacter != nil {
		if err := m.characters.SaveCharacter(staged.ctx, out.DirtyCharacter); err != nil {
			report := SaveReport{
				Written: append([]string(nil), scope.written...),
				Failed:  []string{"character:" + out.DirtyCharacter.ID},
			}
			return nil, &SaveError{Report: report, Err: fmt.Errorf("saving checker: %w", err)}
		}
		scope.written = append(scope.written, "character:"+out.DirtyCharacter.ID)
	}

	return &encounter.ResolveCheckOutput{
		Beaten:  out.Result.Success,
		Applied: out.Applied,
		Total:   out.Result.Total,
	}, nil
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
