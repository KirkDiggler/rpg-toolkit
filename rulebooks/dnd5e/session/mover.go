// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resolution"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// moverSeam is this package's real encounter.Mover, the twin of [strikerSeam]
// and bound the same way: a STRUCT HOLDING A *writeScope rather than a closure
// over one, so it can be constructed BEFORE scope.enc exists — the composition
// needs a Mover to construct the very *encounter.Encounter this seam's own Move
// method later receives as a parameter.
//
// It is what makes an opportunity attack happen again. Until it existed
// resolution.NewMovement had no production caller at all: every walk on the
// live stack was a silent cell change, so the OA condition published triggers
// into a fold nobody entered (rpg-project#316, design rung 2).
type moverSeam struct {
	m     *Manager
	scope *writeScope
}

// compile-time proof the seam satisfies what it is handed to.
var _ encounter.Mover = moverSeam{}

// reactionName is the display name for each reaction ref this package can
// record a beat for.
//
// A TABLE RATHER THAN A DERIVATION, and it fails closed loudly when a ref is
// missing from it. The composition refuses a ReactionIdentity with an empty
// Name (encounter.Record, ErrInvalidData), so the choice is between naming
// every reaction here and inventing a name out of the ref's own id. Inventing
// one would ship "Uncanny Dodge" as whatever the id happened to spell, in the
// story every player reads, and nobody would find out from a test. One entry
// today because one free reaction exists (resolution.freeReactions); the day a
// second reaction reaches a movement fold, this line is where its author is
// stopped and asked what it is called.
var reactionName = map[string]string{
	refs.Conditions.OpportunityAttack().String(): "Opportunity Attack",
}

// Move announces mover's step from one cell to the next, resolves whatever
// reacted to it, and records the resulting beats itself — exactly as
// [strikerSeam.Strike] records its own, and through the same public verb.
//
// NEVER ADOPTS A NEW *encounter.Encounter, for [strikerSeam.Strike]'s reason
// and one sharper still: this is called from INSIDE a walk — the composition's
// own monster loop, or this package's runWalk — and swapping the encounter out
// from under the loop that is mid-path would orphan it. The world Resolve
// returns is read for its dirty sheets alone; hit points and conditions live on
// SHEETS, so nothing about a reaction requires swapping enc.
//
// Errors are mover malfunctions and abort the caller's whole verb. A step
// nothing reacted to returns nil having recorded nothing, which is the ordinary
// case.
func (s moverSeam) Move(
	ctx context.Context, enc *encounter.Encounter, mover encounter.MemberID, from, to spatial.Position,
) error {
	roster, err := enc.Members()
	if err != nil {
		return fmt.Errorf("move: %w", translate(err))
	}

	// THE WALKER'S OWN READIED SHEET, when this walk has one. A player's walk
	// is charged for before the first cell ([Manager.Move]), and a reaction to
	// one of its steps strikes that same member — so the blow must land on the
	// sheet that already paid, not on a second copy fetched behind its back
	// (see [writeScope.walker]). Nil for a monster's driven walk and for free
	// roam: neither spends anything, so there is no earlier edit to carry.
	//
	// The reaction's OWN price is not this seam's business either way. It is
	// charged on the reactor's ledger during the fold, by the condition that
	// decided to react.
	cast := s.m.walkCast(ctx, s.scope, roster)

	reactions := &reactionAttacks{
		ctx:      ctx,
		enc:      enc,
		mover:    mover,
		sheets:   sheetsByID(cast),
		answered: map[string]combatActions.Definition{},
	}

	machine, err := resolution.NewMovement(&resolution.MovementInput{
		Mover:     mover,
		MoverKind: moverKind(roster, mover),
		From:      from,
		To:        to,
		Reactions: reactions,
		Roller:    &diceSeam{roller: s.m.dice},
	})
	if err != nil {
		return fmt.Errorf("move: mover %q: %w: %v", mover, ErrInvalidWorld, err)
	}

	// A pure view for resolution's Input.World — a mid-verb read, never the
	// storage boundary (encounter v0.43.0, #1385).
	world := enc.WorldView()
	out, err := resolution.Resolve(ctx, &resolution.Input{
		World:        world,
		Participants: cast,
		Initiative:   s.m.initiative,
		Standing:     s.scope.standing,
		Sight:        &sightSeam{members: worldMembers(world)},
		TurnDriver:   s.m.turnDriver,
		// The concealment pair, bound to the same live scope every other
		// seam on this call is — the one-seam consistency law strikerSeam
		// states at the same place.
		CheckResolver: checkSeam(s),
		Witness:       witnessSeam{scope: s.scope},
		// NO COST. A step is not a declared action with a profile, and the
		// reaction's own price was already charged on the reactor's ledger
		// during the fold.
		Machine: machine,
		Roller:  &diceSeam{roller: s.m.dice},
	})
	if err != nil {
		return fmt.Errorf("move: %w", translateResolution(err))
	}

	moved, ok := out.Outcome.(resolution.MovementOutcome)
	if !ok {
		return fmt.Errorf("move: %w: movement produced %T", ErrInvalidWorld, out.Outcome)
	}

	// EVERY BEAT IS BUILT BEFORE ANY SHEET IS WRITTEN. The only way building
	// one can fail is a reaction this package cannot name, and failing after
	// the save would leave persisted damage that no beat in the story accounts
	// for. Built first, a refusal costs nothing durable.
	recorded := make([]*encounter.RecordInput, 0, len(moved.Reactions))
	for _, reaction := range moved.Reactions {
		name, known := reactionName[reaction.ConditionRef]
		if !known {
			return fmt.Errorf("move: reactor %q reacted with %q: %w: no display name",
				reaction.ReactorID, reaction.ConditionRef, ErrInvalidWorld)
		}
		beat := recordFor(
			&AttackInput{Attacker: reaction.ReactorID, Target: reaction.Against},
			reaction.Struck,
			reactions.answered[reaction.ReactorID],
		)
		// What the beat was taken AS. The numbers already crossed as an
		// ordinary strike; this is the only thing that explains why a fighter
		// dealt damage on a wolf's turn (encounter.ReactionIdentity).
		beat.Reaction = &encounter.ReactionIdentity{Ref: reaction.ConditionRef, Name: name}
		recorded = append(recorded, beat)
	}

	// Sheets first, then the beats — the ordering [Manager.saveDirty] states:
	// the composition's Record consults who is standing, standingSeam answers
	// out of exactly these two stores, and a consult run against sheets this
	// call has not written back is a consult about a world that no longer
	// exists.
	if err := s.m.saveDirty(ctx, s.scope, out); err != nil {
		return fmt.Errorf("move: %w", err)
	}

	for _, beat := range recorded {
		if _, err := enc.Record(beat); err != nil {
			return fmt.Errorf("move: %w", translate(err))
		}
	}
	return nil
}

// walkCast gathers every member a step can be noticed by.
//
// castFor WITH ONE ANSWER CHANGED: a character whose record cannot be read
// contributes nothing, instead of failing the call. A strike names two members
// and needs both — an attacker with no sheet has nothing to swing and the verb
// must say so. A step names ONE and offers itself to everybody; a member the
// repository cannot produce has nothing to notice it with, which is exactly the
// answer castFor already gives for a monster with no stored sheet, applied to
// the other kind. Failing instead would mean one unreadable bystander freezes
// every walk in the dungeon, including the walks of the members who are fine.
//
// The walker's own readied sheet ([writeScope.walker]) is preferred over a
// fetch wherever it appears, so a reaction lands on the sheet this walk already
// charged.
func (m *Manager) walkCast(
	ctx context.Context, scope *writeScope, roster []encounter.Member,
) []resolution.Participant {
	npcs := map[string]*monster.Data{}
	for i := range scope.data.NPCs {
		npcs[scope.data.NPCs[i].ID] = &scope.data.NPCs[i]
	}

	cast := make([]resolution.Participant, 0, len(roster))
	for _, member := range roster {
		id := string(member.ID)
		switch member.Kind {
		case encounter.MemberKind(KindMonster):
			sheet, stored := npcs[id]
			if !stored {
				continue
			}
			cast = append(cast, resolution.Participant{Monster: sheet})
		case encounter.MemberKind(KindWorld):
			// A placed world NPC has no sheet at all — see castFor.
			continue
		default:
			if scope.walker != nil && scope.walker.ID == id {
				cast = append(cast, resolution.Participant{Character: scope.walker})
				continue
			}
			data, err := m.fetchCharacterData(ctx, "participant", id)
			if err != nil {
				continue
			}
			cast = append(cast, resolution.Participant{Character: data})
		}
	}
	return cast
}

// moverKind is what the mover is, in the vocabulary events.MovementChainEvent
// speaks ("character", "monster"). Carried onto the chain so a subscriber can
// tell a player's step from a monster's without asking.
//
// Empty for a member the roster does not hold: the composition never calls a
// Mover for a stranger, and answering with a guess would put a kind on the
// chain that no rule could trust.
func moverKind(roster []encounter.Member, mover encounter.MemberID) string {
	for _, member := range roster {
		if member.ID != mover {
			continue
		}
		if member.Kind == encounter.MemberKind(KindPlayer) {
			return "character"
		}
		return string(member.Kind)
	}
	return ""
}

// castSheets is one member's stored sheet, exactly one arm populated.
type castSheets struct {
	character *character.Data
	monster   *monster.Data
}

// sheetsByID indexes the cast this call already gathered, so answering a
// reactor costs no second repository read. The cast IS the load: castFor
// fetched every member's sheet a moment ago, and fetching one again to compile
// its reaction would answer out of a different snapshot than the one the
// interaction is attaching effects to.
func sheetsByID(cast []resolution.Participant) map[string]castSheets {
	sheets := make(map[string]castSheets, len(cast))
	for _, participant := range cast {
		switch {
		case participant.Character != nil:
			sheets[participant.Character.ID] = castSheets{character: participant.Character}
		case participant.Monster != nil:
			sheets[participant.Monster.ID] = castSheets{monster: participant.Monster}
		}
	}
	return sheets
}

// reactionAttacks answers what a reactor swings when a step triggers it —
// resolution.ReactionAttacks, asked PER REACTOR at the moment the trigger
// fires.
//
// THIS IS WHERE HOSTILITY IS DECIDED, and it closes the shape of
// rpg-toolkit#899 (an opportunity attack ignores hostility) and rpg-toolkit#766
// (a reaction fires between allies in free roam). The OA condition's own
// predicate is geometry and economy — is this mover leaving my reach, have I a
// reaction left — and it is right not to hold a side: who is an enemy of whom
// is the RUN's answer, folded from the dungeon's factions and whatever the run
// has since learned (rpg-project#375). So the condition publishes and this
// capability, which can ask the run, declines to hand over a weapon when the
// mover is a friend.
//
// Readiness is NOT re-asked here. It is the condition's own gate
// (gamectx.IsReactionReady) and it was already passed — and already SPENT,
// since the condition marks its meter and bills the reactor's economy before
// this capability is ever reached. A second readiness question here could only
// be asked of the outer context, which carries no readiness map at all, so it
// would refuse every reaction in the game while looking like a safety check.
type reactionAttacks struct {
	// ctx is the caller's, captured because ReactionAttacks.AttackFor takes
	// none. Safe because resolution asks this capability synchronously, inside
	// the Resolve call this struct was built for, and never after it returns.
	ctx context.Context

	enc   *encounter.Encounter
	mover encounter.MemberID

	// sheets is the cast, indexed. See [sheetsByID].
	sheets map[string]castSheets

	// answered remembers the definition handed to each reactor, so the beat
	// recorded afterwards names the weapon that actually swung. The outcome
	// carries the numbers and not the definition, and re-compiling one to
	// record it would be a second answer to a question already settled.
	answered map[string]combatActions.Definition
}

var _ resolution.ReactionAttacks = (*reactionAttacks)(nil)

// AttackFor answers the melee attack reactorID swings at the mover, or false.
//
// FALSE IS AN ANSWER, not a failure, at every gate below: an ally, a stranger
// to this run, a caster with no melee weapon and a monster whose whole action
// list is a ranged spit each simply do not get an opportunity attack.
func (r *reactionAttacks) AttackFor(reactorID string) (combatActions.Definition, bool) {
	// A member of the run. A reactor with no sheet in the cast is not someone
	// this call can compile an attack for, and the cast is every member the
	// roster held a moment ago.
	sheets, member := r.sheets[reactorID]
	if !member {
		return combatActions.Definition{}, false
	}

	// HOSTILE, AND KNOWN TO BE. A pair the run cannot answer for is not a
	// licence to swing: "unknown" is the absent value that says the fold has
	// nothing on this pair, and refusing there is what keeps a wandering NPC
	// from biting a player it has no quarrel with (rpg-toolkit#766).
	hostile, known := r.enc.IsHostile(encounter.MemberID(reactorID), r.mover)
	if !known || !hostile {
		return combatActions.Definition{}, false
	}

	definition, ok := r.meleeAttackFor(reactorID, sheets)
	if !ok {
		return combatActions.Definition{}, false
	}

	r.answered[reactorID] = definition
	return definition, true
}

// meleeAttackFor compiles the reactor's melee swing off its stored sheet, the
// same way the declared version of that swing is compiled: a character's
// through character.AssembleAttack on the main hand (offers.go), a monster's by
// reading its stored action list (striker.go).
//
// NO COST ON EITHER. A declared swing is priced by the action economy before
// assembly; a reaction was already billed by the condition that fired it, and
// pricing it again here would charge a character twice for one swing.
func (r *reactionAttacks) meleeAttackFor(
	reactorID string, sheets castSheets,
) (combatActions.Definition, bool) {
	if sheets.monster != nil {
		for i := range sheets.monster.Actions {
			action := sheets.monster.Actions[i]
			if action.Attack != nil && action.Attack.Delivery.IsMelee() {
				return action.Clone(), true
			}
		}
		return combatActions.Definition{}, false
	}

	loaded, err := character.Load(r.ctx, sheets.character)
	if err != nil {
		// A sheet that will not reconstitute cannot swing. It is not this
		// capability's business to fail the walk over it: the same sheet is in
		// the cast resolution is attaching, so a load this broken has already
		// been reported where it is actionable.
		return combatActions.Definition{}, false
	}
	definition, err := character.AssembleAttack(loaded, &character.AssembleAttackInput{
		Slot: character.SlotMainHand,
	})
	if err != nil {
		return combatActions.Definition{}, false
	}
	if definition.Attack == nil || !definition.Attack.Delivery.IsMelee() {
		return combatActions.Definition{}, false
	}
	return definition, true
}
