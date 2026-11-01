// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/mind/machine"
	"github.com/KirkDiggler/rpg-toolkit/play/clock"
	"github.com/KirkDiggler/rpg-toolkit/play/record"
)

// machine.go is A CREATURE IS A MACHINE THAT LISTENS (rpg-project#766 S3,
// design §2 and §4 of ideas/the-world-runs).
//
// # The raise wakes who is due
//
// A creature with orders carries a machine blob ([memberRecord.Machine]): its
// current state, its bag, its wake and its anchor. The recipe it runs is
// compiled once per encounter into a [machine.Program]; the blob is the only
// part that persists. Two indexes are derived from every blob at construction
// and at load and never stored: the due heap ([Encounter.due]) answers "who
// acts at this round" and the listener index ([Encounter.listeners]) answers
// "who listens for this beat". A raise of the world clock pops the due heap
// ([Encounter.wake]); nothing walks the roster when the clock moves (R1).
//
// # The default recipe is today's turn
//
// Every creature with a table runs the default recipe (R4):
//
//	initial: watch
//	watch:   on time  do pick time
//
// which is the world turn the census gave it: due every round, roll the
// `time` table, execute. So the castle plays as it did. A state with no
// `time` transition is never due; that is the point of the machine, and the
// authored recipes that use it are S4's.
//
// # A dwell is not a wake
//
// A patrol standing out a waypoint's wait (dwell.go) stays due every round on
// the default recipe: it is asked, its patrol row holds, and the rows above
// it still fire (a `heard` row walks a standing guard toward a clang). So the
// dwell stays the member's own (memberRecord.Dwell) and this slice never sets
// a wake; only an authored `wake_in` act does (S4). DueAt honours a wake
// wherever it comes from.

// worldClockName is the name of the default clock (R5): every member is
// seated on it unless content names another, and a member's empty
// [memberRecord.Clock] means it.
const worldClockName = "world"

// beatTime is the beat a raise of the clock lands on the machines it wakes:
// one round of the clock the creature is seated on (design §3). Timed, so the
// due heap answers who hears it.
const beatTime machine.Beat = "time"

// defaultRecipeRef names the default recipe (rpg-project#773's finding: a
// core.Ref is module:type:id).
const defaultRecipeRef = "dnd5e:machines:default"

// defaultInitial is the default recipe's one state.
const defaultInitial = "watch"

// tickEv is the `time` beat's event: the round the machine is asked in.
type tickEv struct {
	Round uint64
}

// machineVocabulary is what this slice supplies the machine: the `time` beat
// alone, carrying its round. Facts and the scoped beats (heard, deed, seen,
// fact, clock) are S4's.
func machineVocabulary() machine.Vocabulary {
	return machine.Vocabulary{
		Beats: map[machine.Beat]machine.BeatSpec{
			beatTime: {
				Timed: true,
				Round: func(ev any) (uint64, bool) {
					t, ok := ev.(tickEv)
					return t.Round, ok
				},
			},
		},
	}
}

// defaultRecipe is the recipe every creature with a table and no recipe of
// its own runs: one state, `watch`, whose one transition rolls the `time`
// table on every round of its clock.
func defaultRecipe() (machine.Recipe, error) {
	ref, err := core.ParseString(defaultRecipeRef)
	if err != nil {
		return machine.Recipe{}, fmt.Errorf("default recipe ref: %w", err)
	}

	return machine.Recipe{
		Ref:     *ref,
		Initial: defaultInitial,
		States: map[string]machine.State{
			defaultInitial: {Transitions: []machine.Transition{
				{On: beatTime, Do: []machine.Act{{Pick: string(AnswerTime)}}},
			}},
		},
	}, nil
}

// compilePrograms compiles every recipe this build ships against the
// encounter's vocabulary, keyed by the recipe's ref.
//
// Errors: a recipe the vocabulary refuses, which is this build's defect.
func compilePrograms() (map[string]*machine.Program, error) {
	recipe, err := defaultRecipe()
	if err != nil {
		return nil, err
	}
	program, err := recipe.Compile(machineVocabulary())
	if err != nil {
		return nil, fmt.Errorf("compile %s: %w", defaultRecipeRef, err)
	}

	return map[string]*machine.Program{defaultRecipeRef: program}, nil
}

// programFor is the program a member's blob runs: the default recipe for a
// monster with a table, nil for anyone else. A creature with no table has no
// machine and is never armed, the census's own filter ("a creature nobody
// wrote orders for holds, and holding is indistinguishable from not being
// asked").
func (e *Encounter) programFor(m *memberRecord) *machine.Program {
	if m == nil || m.Kind != KindMonster || len(m.Table) == 0 {
		return nil
	}

	return e.programs[defaultRecipeRef]
}

// onDefaultClock reports whether a member is seated on the world clock by
// name. Every member is in this slice; S7 seats members on area clocks.
func onDefaultClock(m *memberRecord) bool {
	return m.Clock == "" || m.Clock == worldClockName
}

// rebuildWake derives the due heap and the listener index from every
// member's blob: the machines' own answers (DueAt, Listens) at the world's
// now. Called once at construction and once at load, the moment the field
// builds its placement index; neither index is persisted.
//
// A member in a fight is not armed: the fight prices its time, and the
// dissolve or the transfer that returns it re-arms it.
//
// Errors: a clock read failed.
func (e *Encounter) rebuildWake() error {
	e.due = machine.Due[MemberID]{}
	e.listeners = machine.Listeners[MemberID]{}
	e.dueAt = map[MemberID]uint64{}

	now := uint64(e.clock.ToData().HighWater)
	ids := make([]MemberID, 0, len(e.members))
	for id := range e.members {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, id := range ids {
		m := e.members[id]
		p := e.programFor(m)
		if p == nil {
			continue
		}
		e.listeners.Set(id, p.Listens(&m.Machine))
		on, err := e.onWorldClock(id)
		if err != nil {
			return err
		}
		if !on {
			continue
		}
		budget, err := e.clock.Budget(&clock.BudgetInput{ID: core.EntityID(id)})
		if err != nil {
			return fmt.Errorf("world wakes budget %q: %w", id, err)
		}
		e.armOwed(id, now, budget)
	}

	return nil
}

// armOwed arms a member at its first OWED round: the clock's persisted budget
// is the rounds it was granted and has not spent (a raise that came inside
// another pass, saved before the next one took it), so a member owed b rounds
// at the world's now is due at now - b + 1 and the next raise pops it b + 1
// times, round by round, as the census spent the budget a turn at a time. A
// member owed nothing, or whose machine carries a wake of its own, is armed
// from its machine ([Encounter.armAt]). The budget never exceeds the
// high-water (clock.LoadTick refuses it), so the round is at least 1.
func (e *Encounter) armOwed(id MemberID, now uint64, budget int) {
	m := e.members[id]
	if budget <= 0 || m.Machine.WakeAt != nil {
		e.armAt(id, now)
		return
	}
	p := e.programFor(m)
	if _, due := p.DueAt(&m.Machine, now); !due {
		e.disarm(id)
		return
	}
	at := now - uint64(budget) + 1
	e.due.Arm(id, at)
	e.dueAt[id] = at
}

// armAt puts a member on the due heap at the round its machine wants after
// now ([machine.Program.DueAt]), or takes it off when the machine wants none.
//
// NEVER AT OR BEFORE NOW. A member seated at the world's now acts at the next
// raise, as the census gave a newcomer no budget until the clock rose; a wake
// already behind the clock (a record loaded long after) is due at the next round, once, rather than owed every
// round it missed.
func (e *Encounter) armAt(id MemberID, now uint64) {
	m := e.members[id]
	p := e.programFor(m)
	if p == nil {
		e.disarm(id)
		return
	}
	at, ok := p.DueAt(&m.Machine, now)
	if !ok {
		e.disarm(id)
		return
	}
	if at <= now {
		at = now + 1
	}
	e.due.Arm(id, at)
	e.dueAt[id] = at
}

// disarm takes a member off the due heap: it joined a fight, left the run or
// its machine wants no time.
func (e *Encounter) disarm(id MemberID) {
	e.due.Disarm(id)
	delete(e.dueAt, id)
}

// jumpWake moves every armed round and every wake by span: the elapse's own
// rule ([Encounter.elapseWorld]) that nothing is owed afterwards. A creature
// due at the next round is due at the round after the jump, and an authored
// wake keeps the rounds it had left.
func (e *Encounter) jumpWake(span uint64) {
	ids := make([]MemberID, 0, len(e.dueAt))
	for id := range e.dueAt {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, id := range ids {
		at := e.dueAt[id] + span
		e.due.Arm(id, at)
		e.dueAt[id] = at
	}
	for _, m := range e.members {
		if m.Machine.WakeAt != nil {
			at := *m.Machine.WakeAt + span
			m.Machine.WakeAt = &at
		}
	}
}

// stepMachine asks one woken member's machine the time at round, runs what
// it picks, and re-arms it: the world turn the census's thinkFor ran, one
// unit at a time.
//
// THE PICK IS TODAY'S UNIT, unchanged: the sheet's speed of this moment, a
// world budget with no attack, the fight's own anti-spin bound, one
// [Encounter.runIntents] on the world clock, one round spent. Its own pick
// can take it off the world clock (the alarm it raised formed a fight with
// it in); then the fight prices its time, nothing is spent, and it is not
// re-armed: the fight's end re-arms it.
//
// A transition that names a state lands a [BeatState] and re-lists what the
// member listens for, before the pick runs. The default recipe has one state
// and never does.
//
// Errors: the machine refused its blob, a pick of a key other than `time`
// (the only key this slice's vocabulary picks), or the turn failed.
func (e *Encounter) stepMachine(id MemberID, m *memberRecord, round uint64) error {
	p := e.programFor(m)
	if p == nil {
		return nil
	}
	stepped, err := p.Step(&m.Machine, beatTime, tickEv{Round: round}, nil, round)
	if err != nil {
		return fmt.Errorf("world wakes %q: %w", id, err)
	}
	// A TRANSITION IS TRUE THE MOMENT IT FIRES, whatever its pick then does:
	// the beat and the relist land before the pick, so a pick that takes the
	// member off the world clock cannot leave its state changed silently.
	if stepped.To != "" {
		if err := e.appendStateBeat(id, stepped); err != nil {
			return err
		}
		e.listeners.Set(id, p.Listens(&m.Machine))
	}
	for _, act := range stepped.Acts {
		if act.Pick == "" {
			continue
		}
		if act.Pick != string(AnswerTime) {
			return fmt.Errorf("world wakes %q: pick %q: only the time table is picked on the world clock: %w",
				id, act.Pick, ErrInvalidData)
		}
		sheet, serr := e.sheetOf(id)
		if serr != nil {
			return fmt.Errorf("world wakes %q: %w", id, serr)
		}
		turnBudget := TurnBudget{AttacksLeft: 0, MovementFeet: sheet.SpeedFeet}
		// The same anti-spin bound a fight's own turn uses: one terminating
		// Pass plus however many cells this member could ever ask for one at
		// a time. It is also the bound on the beats a pick lands (design §2):
		// the machine's own acts never raise a beat.
		bound := intentBound(turnBudget)
		if _, rerr := e.runIntents(nil, core.EntityID(id), m, 0, &turnBudget, 0, bound, ClockWorld); rerr != nil {
			return fmt.Errorf("world wakes %q: %w", id, rerr)
		}
	}

	on, err := e.onWorldClock(id)
	if err != nil {
		return err
	}
	if !on {
		return nil
	}
	if _, err := e.clock.Spend(&clock.SpendInput{ID: core.EntityID(id), Amount: 1}); err != nil {
		return fmt.Errorf("world wakes spend %q: %w", id, err)
	}
	e.armAt(id, round)

	return nil
}

// appendStateBeat writes a machine entering a state: the member, the state it
// left, the state it entered and the transition's reason (design §2, the
// transition beat). Its audience is the member's own subject audience, as
// the pick beat's is.
func (e *Encounter) appendStateBeat(id MemberID, stepped machine.Stepped) error {
	payload, err := json.Marshal(map[string]interface{}{
		"beat":   BeatState,
		"member": string(id),
		"from":   stepped.From,
		"to":     stepped.To,
		"reason": stepped.Reason,
	})
	if err != nil {
		return fmt.Errorf("state beat: %w", err)
	}
	if _, err := e.appendBeat(&record.AppendInput{
		At:       uint64(e.clock.ToData().HighWater),
		Audience: e.audienceFor(subjectBeat, id),
		Tags:     map[string]string{"tag": BeatState},
		Payload:  payload,
	}); err != nil {
		return fmt.Errorf("state beat: %w", err)
	}

	return nil
}

// MachineData is the persistent representation of a member's machine blob
// (rpg-project#766 S3): its current state, the bag keys that differ from
// their declared zero, the round it asked to be woken and the round it
// anchored. Absent on a member means the fresh blob: the recipe's initial
// state, an empty bag, no wake, no anchor. Which recipe it runs is not
// stored: the encounter derives it from the member (today, the default
// recipe for every monster with a table).
type MachineData struct {
	// State is the machine's current state; empty is the initial one.
	State string `json:"state,omitempty"`
	// Bag holds the bag keys that differ from their declared zero: bool,
	// whole number or string values.
	Bag map[string]any `json:"bag,omitempty"`
	// WakeAt is the world round the machine asked to be woken (an authored
	// wake_in act), or nil.
	WakeAt *uint64 `json:"wake_at,omitempty"`
	// Anchor is the round an anchor act stamped, or nil.
	Anchor *uint64 `json:"anchor,omitempty"`
}

// machineDataFrom renders a blob, nil when it is the fresh blob of its
// program, so a member that never left its initial state writes nothing and a
// record from before machines existed round-trips byte for byte.
func machineDataFrom(p *machine.Program, b machine.Blob) *MachineData {
	state := b.State
	if p != nil && state == p.NewBlob().State {
		state = ""
	}
	if state == "" && len(b.Bag) == 0 && b.WakeAt == nil && b.Anchor == nil {
		return nil
	}
	out := &MachineData{State: state, Bag: map[string]any(b.Bag.Clone())}
	if b.WakeAt != nil {
		at := *b.WakeAt
		out.WakeAt = &at
	}
	if b.Anchor != nil {
		at := *b.Anchor
		out.Anchor = &at
	}

	return out
}

// machineFromData resolves a stored blob against the program the member
// runs. A JSON whole number in the bag lands as int, the machine's kind.
//
// Errors (each wrapped in [ErrInvalidData] by the caller's load): a machine
// on a member that runs none, a state the recipe lacks, a bag value that is
// not a bool, a whole number or a string.
func machineFromData(p *machine.Program, d *MachineData) (machine.Blob, error) {
	if d == nil {
		return machine.Blob{}, nil
	}
	if p == nil {
		return machine.Blob{}, errors.New("a machine on a member that runs none (no table)")
	}
	if d.State != "" {
		probe := machine.Blob{State: d.State}
		if _, err := p.Step(&probe, beatTime, tickEv{}, nil, 0); errors.Is(err, machine.ErrUnknownState) {
			return machine.Blob{}, fmt.Errorf("machine state %q is not in its recipe", d.State)
		}
	}
	b := machine.Blob{State: d.State}
	if len(d.Bag) > 0 {
		b.Bag = machine.Bag{}
		for k, v := range d.Bag {
			scalar, ok := bagScalar(v)
			if !ok {
				return machine.Blob{}, fmt.Errorf("machine bag %q holds %v (%T), not a bool, whole number or string", k, v, v)
			}
			b.Bag[k] = scalar
		}
	}
	if d.WakeAt != nil {
		at := *d.WakeAt
		b.WakeAt = &at
	}
	if d.Anchor != nil {
		at := *d.Anchor
		b.Anchor = &at
	}

	return b, nil
}

// bagScalar is a stored bag value in the machine's kinds: a bool, a string,
// or a whole number as int (encoding/json decodes every number as float64).
func bagScalar(v any) (any, bool) {
	switch x := v.(type) {
	case bool, string, int:
		return x, true
	case float64:
		if x == math.Trunc(x) && x >= math.MinInt && x <= math.MaxInt {
			return int(x), true
		}
	}

	return nil, false
}
