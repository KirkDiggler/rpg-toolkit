// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/play/interrupt"
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
	ctx context.Context, enc *encounter.Encounter, step encounter.MoveStep,
) error {
	roster, err := enc.Members()
	if err != nil {
		return fmt.Errorf("move: %w", translate(err))
	}

	// THE ONE PLACE A PLAYER IS ASKED INSTEAD OF SWUNG FOR. A monster walking
	// out of a player's reach is the case Kirk named (rpg-project#316 rung 3):
	// the player might want to save the swing for the second skeleton. Every
	// other combination stays automatic — a player's own walk provokes the
	// monsters around them, and a monster reactor answers instantly, because
	// there is nobody to ask.
	//
	// A WORLD NPC'S WALK IS NOT A MONSTER'S TURN. Ruling R4 scopes the window
	// to a driven monster turn, and a wandering placement is neither driven
	// nor in a fight, so it keeps rung 2's behaviour rather than freezing the
	// table on a stroll.
	reactions := &reactionAttacks{askPlayers: moverKind(roster, step.Mover) == string(KindMonster)}

	if err := s.offerStep(ctx, enc, step, roster, reactions); err != nil {
		return err
	}

	// EVERY PLAYER REACTOR OF THIS STEP IS ASKED AT ONCE (ruling R3), and the
	// step is NOT taken. Posed after the monsters' own reactions were recorded
	// above, because those happened: a skeleton that bit during the same step
	// bit whether or not the fighter is still deciding, and losing that beat
	// to the pause would be a swing nobody can account for.
	if len(reactions.asked) > 0 {
		return s.pose(step.From, step.To, step.Mover, reactions.asked)
	}
	return nil
}

// offerStep offers one announced step to the rules and records whatever reacted
// to it. It is the body [moverSeam.Move] and [Manager.React] share.
//
// TWO CALLERS, ONE MACHINE, and that is the point of the split rather than a
// convenience. An answered window resolves THE SAME STEP a second time — same
// mover, same two cells, same fold — with only the ReactionAttacks capability
// differing, and a second hand-written copy of this would be a second answer to
// what a step means.
//
// The capability is the caller's to build: Move's asks players, React's answers
// for exactly one of them. Everything else here — the cast, the machine, the
// beats, the save order — is identical either way.
func (s moverSeam) offerStep(
	ctx context.Context, enc *encounter.Encounter, step encounter.MoveStep,
	roster []encounter.Member, reactions *reactionAttacks,
) error {
	// THE WALKER'S OWN READIED SHEET, when this walk has one. A player's walk
	// is charged for before the first cell ([Manager.Move]), and a reaction to
	// one of its steps strikes that same member — so the blow must land on the
	// sheet that already paid, not on a second copy fetched behind its back
	// (see [writeScope.walker]). Nil for a monster's driven walk and for free
	// roam: neither spends anything, so there is no earlier edit to carry.
	//
	// The reaction's OWN price is not this seam's business either way. It is
	// charged on the reactor's ledger by the condition that offered it, when
	// the movement machine reports the reaction TAKEN (ruling R1).
	cast := s.m.walkCast(ctx, s.scope, roster)

	reactions.ctx = ctx
	reactions.enc = enc
	reactions.mover = step.Mover
	reactions.sheets = sheetsByID(cast)
	reactions.answered = map[string]combatActions.Definition{}

	machine, err := resolution.NewMovement(&resolution.MovementInput{
		Mover:     step.Mover,
		MoverKind: moverKind(roster, step.Mover),
		From:      step.From,
		To:        step.To,
		Reactions: reactions,
		Roller:    &diceSeam{roller: s.m.dice},
		// THE ONE THING THIS SEAM ADDS TO THE STEP. The composition below
		// knows the creature did not choose to move and says so; the machine
		// above knows what an opportunity attack is. Neither can reach the
		// other, and this line is the whole of the translation.
		ForcedBy: forcedBy(step),
	})
	if err != nil {
		return fmt.Errorf("move: mover %q: %w: %v", step.Mover, ErrInvalidWorld, err)
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
		Equipment:    equipmentBeside(s.scope.standing),
		TurnDriver:   s.m.turnDriver,
		// The concealment pair, bound to the same live scope every other
		// seam on this call is — the one-seam consistency law strikerSeam
		// states at the same place.
		CheckResolver: checkSeam(s),
		Witness:       witnessSeam{scope: s.scope},
		// NO COST. A step is not a declared action with a profile, and the
		// reaction's own price is charged on the reactor's own ledger.
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
		// NO PRESENTATION TOKEN, for the reason a monster's strike carries
		// none: a reaction is a roll the server took on the reactor's behalf
		// inside somebody else's Move, so no client simulated its die — see
		// recordFor.
		beat := recordFor(
			&AttackInput{Attacker: reaction.ReactorID, Target: reaction.Against},
			reaction.Struck,
			reactions.answered[reaction.ReactorID],
			"",
			// NO CONCENTRATION LISTS ON THE PER-REACTION BEAT. One move is one
			// interaction and can produce several opportunity attacks, so the
			// checks and breaks arrive once for the whole of it; copying them
			// onto every beat would record one broken spell as many. They are
			// attached below, to one beat.
			&resolution.Output{},
		)
		// What the beat was taken AS. The numbers already crossed as an
		// ordinary strike; this is the only thing that explains why a fighter
		// dealt damage on a skeleton's turn (encounter.ReactionIdentity).
		beat.Reaction = &encounter.ReactionIdentity{Ref: reaction.ConditionRef, Name: name}
		recorded = append(recorded, beat)
	}

	// # The interaction's concentration consequences, attached to ONE beat
	//
	// A move deals no damage of its own; what breaks a walker's concentration
	// during one is an opportunity attack, and resolution reports every check
	// and break the whole move produced as one flat pair of lists with no beat
	// attribution in them. So they ride the LAST reaction beat: by then every
	// swing of this move has landed, which is the earliest point the record can
	// honestly say the consequences were all in.
	//
	// WITH ONE REACTION — which is the case the fight actually produces — that
	// beat is the swing that caused it and the placement is exact. With two it
	// is a small ordering imprecision inside a single interaction rather than a
	// wrong fact, and it is named here rather than hidden. Splitting the lists
	// per reactor is resolution's attribution to make, not this seam's.
	if len(out.ConcentrationChecks) > 0 || len(out.ConcentrationBreaks) > 0 {
		if len(recorded) == 0 {
			// A move that broke a concentration and swung at nobody has no
			// beat for the break to ride, and dropping it would end a spell
			// that nothing in the story ever mentions. Refused rather than
			// recorded silently.
			return fmt.Errorf("move: %w: the walk ended a concentration with no beat to carry it",
				ErrInvalidWorld)
		}
		last := recorded[len(recorded)-1]
		last.ConcentrationChecks = out.ConcentrationChecks
		last.ConcentrationBreaks = out.ConcentrationBreaks
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

// pose opens one window per player reactor of this step and reports the pause.
//
// THE RETURN IS THE POINT. resolution.ReactionAttacks has no channel for
// "suspend" — its one method answers a definition or false, with no error at
// all — so the decision to ask is remembered during Resolve and acted on here,
// where this seam still has an error to return. [encounter.StepPausedError] is
// what the composition reads as a checkpoint rather than a malfunction: it
// leaves the mover standing on from, stores the rest of the turn, and narrates
// the beat.
//
// The ledger is written into the session aggregate HERE rather than at commit,
// so a verb that poses nothing writes nothing — the freeze costs exactly the
// walks that actually stop.
func (s moverSeam) pose(
	from, to spatial.Position, mover encounter.MemberID, asked []askedReactor,
) error {
	reaction, err := poseableReaction()
	if err != nil {
		return fmt.Errorf("move: %w", err)
	}

	windows := make([]encounter.PausedWindow, 0, len(asked))
	for _, ask := range asked {
		payload, perr := marshalWindowPayload(windowPayload{
			Mover:      string(mover),
			From:       from,
			To:         to,
			Reactor:    ask.reactor,
			Reaction:   reaction.Ref,
			Definition: ask.definition,
		})
		if perr != nil {
			return fmt.Errorf("move: %w: %v", ErrInvalidSession, perr)
		}
		if _, oerr := s.scope.ledger.Pose(&interrupt.PoseInput{
			Audience: core.EntityID(ask.reactor),
			Options:  []interrupt.Option{interrupt.Option(ReactStrike), interrupt.Option(ReactHold)},
			Payload:  payload,
			// The sequence this verb started from, which is the only story
			// coordinate this seam holds: the window_opened beat the
			// composition appends for these windows lands after it, so a
			// reader holding the story can still order the two.
			At: s.scope.baseline,
		}); oerr != nil {
			return fmt.Errorf("move: %w: %v", ErrInvalidSession, oerr)
		}
		windows = append(windows, encounter.PausedWindow{
			Audience: encounter.MemberID(ask.reactor),
			Reaction: encounter.ReactionIdentity{Ref: reaction.Ref, Name: reaction.Name},
		})
	}

	s.scope.data.Windows = s.scope.ledger.ToData()
	s.scope.touched = true

	return &encounter.StepPausedError{Windows: windows}
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

	// askPlayers turns a player reactor's swing into a QUESTION. When it is
	// set, a character who passes every gate below is not handed a weapon —
	// they are remembered in asked, and [moverSeam.pose] opens them a window
	// after Resolve returns.
	//
	// It is not a readiness or a hostility question and does not replace one:
	// every gate still runs, and a reactor who would not have swung is not
	// asked either. The only thing it changes is WHO decides, and the answer
	// is only ever "the player" when the mover is a monster (ruling R4).
	askPlayers bool

	// only, when non-empty, is the single reactor this capability will answer
	// for — [Manager.React]'s second pass, where the swing being resolved is
	// one specific player's accepted offer and every other reactor of the same
	// step either already swung or is still deciding.
	only string

	// definition is the attack `only` was OFFERED, replayed rather than
	// recompiled. The offer a player accepted is the offer that was made: a
	// sheet edited between the question and the answer must not silently
	// change what they swing with.
	definition combatActions.Definition

	// asked is every player reactor this step must ask, in the order the
	// machine reached them. Read by [moverSeam.pose] after Resolve returns.
	asked []askedReactor
}

// askedReactor is one player who was offered a swing and has not answered.
type askedReactor struct {
	// reactor is the member being asked, and definition the melee attack
	// they were offered — both frozen into the window's payload so the
	// answer resolves the same swing the question described.
	reactor    string
	definition combatActions.Definition
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

	// AN ANSWERED WINDOW SWINGS FOR ITS OWN AUDIENCE AND NOBODY ELSE. On
	// React's replay the same step is offered to the rules a second time, so
	// every reactor's condition triggers again — including the monsters that
	// already bit during the first pass and the players still deciding. Only
	// the member who said yes gets a weapon, and they get the one they were
	// offered rather than a freshly compiled twin.
	if r.only != "" {
		if reactorID != r.only {
			return combatActions.Definition{}, false
		}
		r.answered[reactorID] = r.definition
		return r.definition, true
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

	// THE PLAYER IS ASKED, LAST, AFTER EVERY OTHER GATE. Asked before them, a
	// window would open for an ally, for a friend of the run, or for a caster
	// with nothing to swing — three questions with one answer, put to a person
	// who has to read them. Every gate above has to say yes before the pause
	// is worth anybody's turn.
	//
	// FALSE HERE COSTS THE REACTOR NOTHING. The condition bills only when the
	// machine reports a reaction TAKEN (ruling R1), and declining is not
	// taking — which is exactly what makes `hold` free.
	if r.askPlayers && sheets.character != nil {
		r.asked = append(r.asked, askedReactor{reactor: reactorID, definition: definition})
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
