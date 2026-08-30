// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resolution"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// moverSeam is this package's real encounter.Mover, bound to the live
// scope.enc a write verb is already operating on — the exact twin of
// [strikerSeam], and built the same way for the same reason.
//
// A STRUCT HOLDING A *writeScope RATHER THAN A CLOSURE OVER ONE, so it can be
// constructed BEFORE scope.enc exists: encounter.LoadEncounter needs a Mover
// to construct the very *encounter.Encounter this seam's own Move method later
// receives as a parameter. It only ever reads scope AT CALL TIME, well after
// that ordering is settled.
//
// It exists so a MONSTER's walk provokes what a player's walk provokes. Both
// paths reach one [resolution.NewMovement] through [Manager.announceStep], so
// "what does a step provoke" has a single answer rather than one per caller.
type moverSeam struct {
	m     *Manager
	scope *writeScope
}

// compile-time proof the seam satisfies what it is handed to.
var _ encounter.Mover = moverSeam{}

// Move announces one cell of a driven member's walk and resolves whatever
// reacts to it, recording each reaction's beat itself — the same obligation
// [strikerSeam.Strike] carries for a swing.
//
// NEVER ADOPTS A NEW *encounter.Encounter, for the reason strikerSeam gives at
// length: enc is the SAME live object the composition is mid-loop over, and
// swapping it out from inside a call enc itself made would orphan that loop.
// The world resolution returns is read for its dirty sheets alone and
// otherwise discarded.
func (s moverSeam) Move(
	ctx context.Context, enc *encounter.Encounter, mover encounter.MemberID, from, to spatial.Position,
) error {
	// The mover is always a monster, by the same argument strikerSeam gives
	// for its attacker: nothing outside this package's own TurnDriver-driven
	// turns reaches this method, and only an unplayed member — always
	// KindMonster today — has a TurnDriver at all.
	if _, err := s.m.announceStep(
		ctx, s.scope, enc, mover, string(KindMonster), from, to,
	); err != nil {
		return fmt.Errorf("move: %w", err)
	}
	return nil
}

// announceStep offers ONE cell of a walk to the rules and records what
// reacted. It is the single implementation both walk paths reach: a player's
// own walk through runWalk, and a driven member's through [moverSeam].
//
// Two paths, one machine, deliberately. "What does a step provoke" is one
// question, and answering it twice is how the two paths come to disagree —
// which is the asymmetry the Mover capability was added to prevent.
//
// THE CALLER TAKES THE STEP AFTERWARDS. Announce-before-step is
// [resolution.NewMovement]'s contract, not this function's preference: an
// opportunity attack fires because the mover is LEAVING reach, and the
// reactor's swing enforces melee reach against where the target IS. A caller
// that stepped first would hand the strike a departed target, the swing would
// refuse as out of range, and a refused reaction fails the whole interaction.
//
// COST IS NIL. The walk is priced once, up front, for the whole path
// (Manager.Move), so a per-cell charge here would bill it twice. Per-cell
// metering is its own slice.
func (m *Manager) announceStep(
	ctx context.Context, scope *writeScope, enc *encounter.Encounter,
	mover encounter.MemberID, kind string, from, to spatial.Position,
) (*resolution.MovementOutcome, error) {
	roster, err := enc.Members()
	if err != nil {
		return nil, translate(err)
	}

	// The WHOLE cast, by [Manager.castFor] — Kirk's law, and the reason this
	// machine exists rather than a bus at the call site: "a walk should load
	// everything. if there is a trap along the path it will need to be
	// loaded." Nobody here decides who is relevant to movement.
	cast, err := m.castFor(ctx, scope, roster, nil)
	if err != nil {
		return nil, err
	}

	attacks := &reactionAttacks{m: m, scope: scope, ctx: ctx, answered: map[string]combatActions.Definition{}}

	machine, err := resolution.NewMovement(&resolution.MovementInput{
		Mover:     mover,
		MoverKind: kind,
		From:      from,
		To:        to,
		Reactions: attacks,
		Roller:    &diceSeam{roller: m.dice},
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidWorld, err)
	}

	world := enc.ToData()
	out, err := resolution.Resolve(ctx, &resolution.Input{
		World:        world,
		Participants: cast,
		Initiative:   m.initiative,
		Standing:     scope.standing,
		Sight:        &sightSeam{members: append([]encounter.MemberData(nil), world.Members...)},
		TurnDriver:   m.turnDriver,
		Cost:         nil,
		Machine:      machine,
		Roller:       &diceSeam{roller: m.dice},
	})
	if err != nil {
		return nil, translateResolution(err)
	}

	moved, ok := out.Outcome.(resolution.MovementOutcome)
	if !ok {
		return nil, fmt.Errorf("%w: movement produced %T", ErrInvalidWorld, out.Outcome)
	}

	// BEFORE the beats, for the ordering [Manager.saveDirty] states: Record
	// consults who is standing, standingSeam answers out of these stores, and
	// a consult run against sheets this verb has not written back yet is a
	// consult about a world that no longer exists.
	if err := m.saveDirty(ctx, scope, out); err != nil {
		return nil, err
	}

	for _, reaction := range moved.Reactions {
		recorded, rerr := recordForReaction(reaction, attacks.answered[reaction.ReactorID])
		if rerr != nil {
			return nil, rerr
		}
		if _, err := enc.Record(recorded); err != nil {
			return nil, translate(err)
		}
	}

	return &moved, nil
}

// recordForReaction turns one fired reaction into the beat the composition
// stamps. It is [recordFor] with the reaction's own identity added, rather
// than a second way to describe a strike: an opportunity attack IS a strike,
// and the numbers reach the story through the same shape a declared swing's do.
//
// The definition is the one the capability ANSWERED WITH, carried back rather
// than re-derived, so the weapon in the beat is provably the weapon that was
// swung — re-assembling it here would be a second answer that could drift.
func recordForReaction(
	reaction resolution.ReactionOutcome, definition combatActions.Definition,
) (*encounter.RecordInput, error) {
	parsed, err := core.ParseString(reaction.ConditionRef)
	if err != nil {
		return nil, fmt.Errorf("%w: reaction ref %q: %v", ErrInvalidWorld, reaction.ConditionRef, err)
	}

	// An unknown ref is a HARD ERROR, matching what conditions.DisplayFor's own
	// doc says the character projection does with a false result. A reaction
	// the rulebook cannot name is a reaction whose beat would read "hit for 7"
	// during someone else's turn with nothing to explain it — which is the
	// exact confusion this identity was added to end, so failing loudly beats
	// recording it nameless.
	display, ok := conditions.DisplayFor(*parsed)
	if !ok {
		return nil, fmt.Errorf("%w: no display name for reaction %q", ErrInvalidWorld, reaction.ConditionRef)
	}

	recorded := recordFor(
		&AttackInput{Attacker: reaction.ReactorID, Target: reaction.Against},
		reaction.Struck,
		definition,
	)
	recorded.Reaction = &encounter.ReactionIdentity{Ref: parsed.String(), Name: display.Name}

	return recorded, nil
}

// reactionAttacks answers what a triggered reactor swings, for BOTH kinds.
//
// ONE IMPLEMENTATION, not one per kind, because the question is one question.
// A character swings the weapon in their main hand, assembled by the same
// character.AssembleAttack the declared-attack path compiles an offer from; a
// monster swings the first melee action on its sheet, read exactly as
// [strikerSeam] reads one. Neither is a special case of the other, and a
// reactor with nothing to swing is an ANSWER (false), not a failure — an
// unarmed caster simply does not get an opportunity attack.
//
// NO COST is assembled into the character's definition. The reaction's price
// is the reaction itself, and the OA condition has already published its own
// SpendRequested by the time this is asked; pricing the swing again here would
// bill a character twice for one reaction.
//
// IT MEMOISES WHAT IT ANSWERED, in answered, so the beat can record the weapon
// that was actually swung rather than re-deriving one.
//
// The context is held on the struct because [resolution.ReactionAttacks] takes
// none: it is asked mid-fold, inside the same call this was constructed for,
// so the ctx it carries is that call's own and outlives nothing.
type reactionAttacks struct {
	m     *Manager
	scope *writeScope
	ctx   context.Context //nolint:containedctx // asked mid-fold inside one call; see the type's doc

	answered map[string]combatActions.Definition
}

var _ resolution.ReactionAttacks = (*reactionAttacks)(nil)

// AttackFor returns what reactorID swings, or false if it has nothing.
func (r *reactionAttacks) AttackFor(reactorID string) (combatActions.Definition, bool) {
	if definition, ok := r.answered[reactorID]; ok {
		return definition, true
	}

	definition, ok := r.assemble(reactorID)
	if !ok {
		return combatActions.Definition{}, false
	}
	r.answered[reactorID] = definition

	return definition, true
}

func (r *reactionAttacks) assemble(reactorID string) (combatActions.Definition, bool) {
	for i := range r.scope.data.NPCs {
		if r.scope.data.NPCs[i].ID != reactorID {
			continue
		}
		return meleeActionOf(&r.scope.data.NPCs[i])
	}

	data, err := r.m.fetchCharacterData(r.ctx, "reactor", reactorID)
	if err != nil {
		// A reactor whose sheet will not load has no attack to offer. False is
		// the honest answer at this seam — the alternative is failing a whole
		// walk over a reaction nobody asked for.
		return combatActions.Definition{}, false
	}
	sheet, err := character.Load(r.ctx, data)
	if err != nil {
		return combatActions.Definition{}, false
	}

	definition, err := character.AssembleAttack(sheet, &character.AssembleAttackInput{
		Slot: character.SlotMainHand,
	})
	if err != nil {
		return combatActions.Definition{}, false
	}

	return definition, true
}

// meleeActionOf picks the melee action a monster reacts with — the first in
// its own stored order, which is the order the content author wrote and the
// same one [encounter.MonsterView] presents.
func meleeActionOf(sheet *monster.Data) (combatActions.Definition, bool) {
	for i := range sheet.Actions {
		if sheet.Actions[i].Attack == nil {
			continue
		}
		if sheet.Actions[i].Attack.Delivery.IsMelee() {
			return sheet.Actions[i].Clone(), true
		}
	}
	return combatActions.Definition{}, false
}

// positionOf reads where a member currently stands, off the composition that
// owns the answer.
//
// Read per step rather than carried forward from the last cell, for the reason
// runWalk already reads each landing off Step's own answer instead of echoing
// its input: the two agree today, and the day a step can land somewhere other
// than where it was aimed, reading the answer keeps being right without anyone
// noticing it had to change.
func positionOf(enc *encounter.Encounter, member string) (spatial.Position, error) {
	members, err := enc.Members()
	if err != nil {
		return spatial.Position{}, translate(err)
	}
	for _, m := range members {
		if string(m.ID) == member {
			return m.Position, nil
		}
	}
	return spatial.Position{}, fmt.Errorf("member %q: %w", member, ErrNoMember)
}
