// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
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

// Move announces mover's step from one cell to the next, resolves whatever
// reacted to it, and records the resulting beats itself — exactly as
// [strikerSeam.Strike] records its own, and through the same public verb.
//
// NEVER ADOPTS A NEW *encounter.Encounter: this is called from INSIDE a walk —
// the composition's own monster loop, or this package's runWalk — and swapping
// the encounter out from under the loop that is mid-path would orphan it.
//
// # The one place a player is asked instead of swung for
//
// A monster walking out of a player's reach is the case Kirk named
// (rpg-project#316 rung 3): the player might want to save the swing for the
// second skeleton. So a player reactor of a monster's step is ASKED — the
// machine poses a [resolution.PauseOpportunity] per player, all standing at
// once, and the step stops (ruling E8): the mover has not moved, and the
// encounter holds the step until the last asked player answered. Every other
// combination stays automatic. A WORLD NPC'S WALK IS NOT A MONSTER'S TURN
// (ruling R4), so a wandering placement never freezes the table.
//
// Whatever settled before the step stopped — a skeleton that bit during the
// same step — is told now, because it happened.
//
// Errors are mover malfunctions and abort the caller's whole verb, except
// [encounter.StepPausedError], which the composition reads as a checkpoint.
// A step nothing reacted to returns nil having recorded nothing.
func (s moverSeam) Move(
	ctx context.Context, enc *encounter.Encounter, step encounter.MoveStep,
) error {
	roster, err := enc.Members()
	if err != nil {
		return fmt.Errorf("move: %w", translate(err))
	}

	// THE WALKER'S OWN READIED SHEET, when this walk has one: a reaction to
	// one of its steps strikes the sheet that already paid for the walk, not
	// a second copy fetched behind its back (see [writeScope.walker]).
	cast := s.m.walkCast(ctx, s.scope, roster)
	kind := moverKind(roster, step.Mover)

	machine, err := resolution.NewMovement(&resolution.MovementInput{
		Mover:     step.Mover,
		MoverKind: kind,
		From:      step.From,
		To:        step.To,
		Reactions: &reactionAttacks{
			ctx:        ctx,
			enc:        enc,
			mover:      step.Mover,
			sheets:     sheetsByID(cast),
			askPlayers: kind == string(KindMonster),
		},
		Roller: &diceSeam{roller: s.m.dice},
		// THE ONE THING THIS SEAM ADDS TO THE STEP. The composition below
		// knows the creature did not choose to move and says so; the machine
		// above knows what an opportunity attack is.
		ForcedBy: forcedBy(step),
	})
	if err != nil {
		return fmt.Errorf("move: mover %q: %w: %v", step.Mover, ErrInvalidWorld, err)
	}

	out, err := resolution.Resolve(ctx, s.m.resolutionInput(ctx, s.scope, resolutionAsk{
		// A pure view — a mid-verb read, never the storage boundary.
		World:        enc.WorldView(),
		Participants: cast,
		// NO COST. A step is not a declared action; a reaction taken during
		// it is charged by resolution at the one door as it is taken.
		Machine: machine,
	}))
	if err != nil {
		return fmt.Errorf("move: %w", translateResolution(err))
	}

	// A PLAYER'S OWN WALK carries the rest of its path on the window a
	// reaction stops it with, so the last answer walks on. A monster's walk
	// is held by the encounter's own pause instead.
	story := windowStory{Kind: storyStep, Target: string(step.Mover)}
	if kind == "character" {
		story.WalkPath = append([]spatial.Position(nil), s.scope.walkContinuation...)
	}
	l, windows, err := s.m.stepLanding(s.scope, enc, story, out)
	if err != nil {
		return err
	}
	if _, err := s.m.land(ctx, s.scope, out, l); err != nil {
		return fmt.Errorf("move: %w", err)
	}
	if len(windows) > 0 {
		return &encounter.StepPausedError{Windows: windows}
	}
	return nil
}

// forcedBy names the effect suppressing this step's opportunity attacks, or
// nothing at all for a step somebody chose.
//
// TWO FIELDS COLLAPSE INTO ONE ANSWER, and the collapse is the rule.
// [encounter.MoveStep] carries Forced and Cause separately because the
// composition has no idea what either is for; [resolution.MovementInput] asks
// one question — are the triggers suppressed, and by what — because the fold
// needs a name to refuse in. A prevention source reading "something" is not a
// name, so a forced step with no cause still hands over the zero ref rather
// than nil: the step was forced, and the honest record of a forced step by
// nobody is a forced step by nobody.
//
// NIL IS THE PROVOKING CASE on both sides of this line, which is why nothing
// here reads Cause on its own. A forced move that PROVOKES — Dissonant
// Whispers sends its target running and the running provokes — reaches this
// seam with Forced false, because encounter inverts Direct's own Provokes flag
// before it builds the step. This function must not second-guess that: reading
// a non-zero Cause as "suppress" would silence the one directive whose whole
// point is that it does not.
//
// THAT DAY CAME. Dissonant Whispers ships, so the paragraph above describes
// live traffic rather than a case being held open — and it is the reason
// [Manager.React] can replay a held flee with the two fields its window does
// not store. A mutant that read Cause when Forced is false would swallow every
// swing the whisper is supposed to buy.
func forcedBy(step encounter.MoveStep) *core.Ref {
	if !step.Forced {
		return nil
	}
	cause := step.Cause
	return &cause
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
			data, err := m.sheetsFor(nil).load(ctx, "participant", id)
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
// (gamectx.IsReactionReady) and it was already passed; the price is
// resolution's, charged at its one door when a swing is taken.
type reactionAttacks struct {
	// ctx is the caller's, captured because ReactionAttacks.AttackFor takes
	// none. Safe because resolution asks this capability synchronously, inside
	// the Resolve call this struct was built for, and never after it returns.
	ctx context.Context

	enc   *encounter.Encounter
	mover encounter.MemberID

	// sheets is the cast, indexed. See [sheetsByID].
	sheets map[string]castSheets

	// askPlayers turns a player reactor's swing into a QUESTION: a character
	// who passes every gate below is answered [resolution.ReactionAsk], and
	// the machine poses them a pause with the definition frozen inside it.
	//
	// It is not a readiness or a hostility question and does not replace one:
	// every gate still runs, and a reactor who would not have swung is not
	// asked either. The only thing it changes is WHO decides, and the answer
	// is only ever "the player" when the mover is a monster (ruling R4).
	askPlayers bool
}

var _ resolution.ReactionAttacks = (*reactionAttacks)(nil)

// AttackFor answers the melee attack reactorID swings at the mover, and
// whether to swing now or ask.
//
// [resolution.ReactionNone] IS AN ANSWER, not a failure, at every gate below:
// an ally, a stranger to this run, a caster with no melee weapon and a monster
// whose whole action list is a ranged spit each simply do not get an
// opportunity attack, and it costs them nothing.
func (r *reactionAttacks) AttackFor(reactorID string) (combatActions.Definition, resolution.ReactionAnswer) {
	// A member of the run. A reactor with no sheet in the cast is not someone
	// this call can compile an attack for.
	sheets, member := r.sheets[reactorID]
	if !member {
		return combatActions.Definition{}, resolution.ReactionNone
	}

	// HOSTILE, AND KNOWN TO BE. A pair the run cannot answer for is not a
	// licence to swing: "unknown" is the absent value that says the fold has
	// nothing on this pair, and refusing there is what keeps a wandering NPC
	// from biting a player it has no quarrel with (rpg-toolkit#766).
	hostile, known := r.enc.IsHostile(encounter.MemberID(reactorID), r.mover)
	if !known || !hostile {
		return combatActions.Definition{}, resolution.ReactionNone
	}

	definition, ok := r.meleeAttackFor(reactorID, sheets)
	if !ok {
		return combatActions.Definition{}, resolution.ReactionNone
	}

	// THE PLAYER IS ASKED, LAST, AFTER EVERY OTHER GATE. Asked before them, a
	// window would open for an ally, for a friend of the run, or for a caster
	// with nothing to swing — three questions with one answer. The definition
	// rides the pause, so the answer swings what the question described.
	if r.askPlayers && sheets.character != nil {
		return definition, resolution.ReactionAsk
	}
	return definition, resolution.ReactionSwing
}

// meleeAttackFor compiles the reactor's melee swing off its stored sheet, the
// same way the declared version of that swing is compiled: a character's
// through character.AssembleAttack on the main hand (offers.go), a monster's by
// reading its stored action list (striker.go).
//
// NO COST ON EITHER. A declared swing is priced by the action economy before
// assembly; a reaction's price is resolution's, charged at its door when the
// swing is taken, and pricing it here would charge a character twice.
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
