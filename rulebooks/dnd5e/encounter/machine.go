// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"encoding/json"
	"fmt"
	"maps"
	"math"
	"slices"
	"sort"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/mind/behavior"
	"github.com/KirkDiggler/rpg-toolkit/mind/machine"
	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
	"github.com/KirkDiggler/rpg-toolkit/play/clock"
	"github.com/KirkDiggler/rpg-toolkit/play/record"
)

const (
	worldClockName              = "world"
	beatTime       machine.Beat = "time"
	beatTurn       machine.Beat = "turn"
)

type tickEv struct{ Round uint64 }

func machineVocabulary() machine.Vocabulary {
	scoped := machine.BeatSpec{InScope: inScope, Round: wakeRound}
	unscoped := machine.BeatSpec{Round: wakeRound}
	return machine.Vocabulary{
		Beats: map[machine.Beat]machine.BeatSpec{
			beatTime: {Timed: true, Round: wakeRound}, beatTurn: {Timed: true, Round: wakeRound},
			beatHeard: scoped, beatDeed: scoped, beatSeen: scoped, beatBarred: scoped, beatApproached: scoped,
			beatFact: unscoped, beatTopic: unscoped, clockBeat(worldClockName): unscoped, clockBeat(clockFight): unscoped,
		},
		Facts: map[machine.FactName]machine.FactSpec{
			factEnemy:      {Kind: machine.KindString, Read: readEnemy},
			"heard.kind":   {Kind: machine.KindString, Read: readHeardKind},
			factHeardClaim: {Kind: machine.KindString, Read: readHeardClaim},
		},
	}
}

func vocabularyFor(r *machine.Recipe) machine.Vocabulary {
	v := machineVocabulary()
	for _, st := range r.States {
		for _, t := range st.Transitions {
			name := string(t.On)
			switch {
			case strings.HasPrefix(name, deedPrefix):
				if word := strings.TrimPrefix(name, deedPrefix); slices.Contains(WhenDeeds, word) {
					v.Beats[t.On] = v.Beats[beatDeed]
				}
			case strings.HasPrefix(name, beatClock) && len(name) > len(beatClock):
				v.Beats[t.On] = machine.BeatSpec{Round: wakeRound}
			}
			addGuardFacts(v.Facts, t.When)
		}
	}
	return v
}

func addGuardFacts(facts map[machine.FactName]machine.FactSpec, g *machine.Guard) {
	if g == nil {
		return
	}
	if g.Fact != nil && strings.HasPrefix(string(g.Fact.Name), factPrefix) && len(g.Fact.Name) > len(factPrefix) {
		id := FactID(strings.TrimPrefix(string(g.Fact.Name), factPrefix))
		facts[g.Fact.Name] = machine.FactSpec{Kind: machine.KindBool, Read: func(env any) (any, bool) {
			me, ok := env.(*machineEnv)
			if !ok {
				return nil, false
			}
			return me.e.world.knowsFact(me.id, id), true
		}}
	}
	// A TOPIC HELD (rpg-project#929): true while the member holds the topic
	// about anybody, read from his OWN holdings — never the truth.
	if g.Fact != nil && strings.HasPrefix(string(g.Fact.Name), topicPrefix) && len(g.Fact.Name) > len(topicPrefix) {
		topic := perception.Topic(strings.TrimPrefix(string(g.Fact.Name), topicPrefix))
		facts[g.Fact.Name] = machine.FactSpec{Kind: machine.KindBool, Read: func(env any) (any, bool) {
			me, ok := env.(*machineEnv)
			if !ok {
				return nil, false
			}
			held, err := me.e.intelLog.OnTopic(sightMember(me.id), topic)
			if err != nil {
				return nil, false
			}
			return len(held) > 0, true
		}}
	}
	// WHAT THIS CREATURE HOLDS ABOUT THE ONE THIS BEAT IS ABOUT
	// (rpg-project#1000): the question a line's `holds` requirement asks,
	// asked from the same holder, about the one the beat names. A beat that
	// names nobody holds nothing about anybody: false.
	if g.Fact != nil && strings.HasPrefix(string(g.Fact.Name), holdsPrefix) && len(g.Fact.Name) > len(holdsPrefix) {
		topic := TopicID(strings.TrimPrefix(string(g.Fact.Name), holdsPrefix))
		facts[g.Fact.Name] = machine.FactSpec{Kind: machine.KindBool, Read: func(env any) (any, bool) {
			me, ok := env.(*machineEnv)
			if !ok {
				return nil, false
			}
			ev, ok := me.event.(wakeEv)
			if !ok || ev.Subject == "" {
				return false, true
			}
			return me.e.holdsAbout(me.id, ev.Subject, topic), true
		}}
	}
	// A RAISED VOICE (rpg-project#933): only a loudness this rulebook names
	// is registered, so a misspelt word is a fact the vocabulary refuses.
	if g.Fact != nil && strings.HasPrefix(string(g.Fact.Name), heardLoudnessPrefix) {
		if l, ok := loudnessOf(strings.TrimPrefix(string(g.Fact.Name), heardLoudnessPrefix)); ok && l != "" {
			facts[g.Fact.Name] = machine.FactSpec{Kind: machine.KindBool, Read: readHeardLoudness(l)}
		}
	}
	addGuardFacts(facts, g.Not)
	for i := range g.All {
		addGuardFacts(facts, &g.All[i])
	}
	for i := range g.Any {
		addGuardFacts(facts, &g.Any[i])
	}
}

type machineEnv struct {
	e     *Encounter
	id    MemberID
	event any
	facts *Facts
	err   error
}

func (e *Encounter) envFor(id MemberID) *machineEnv { return &machineEnv{e: e, id: id} }

// factHeardClaim names the guard fact that reads a caught checked line's
// verdict (rpg-project#1000): [ClaimHeld] or [ClaimFailed]. A plain line, a
// noise, or a voice whose words were not caught is neither.
const factHeardClaim machine.FactName = "heard.claim"

// holdsPrefix names a guard fact true while this creature holds a topic
// about the one the beat names (rpg-project#1000): `holds:<topic>`.
// `holds.anyone:<topic>` (the engine's `topic:<id>`) is the question
// about anybody.
const holdsPrefix = "holds:"

// readHeardClaim is `heard.claim`'s reader: the verdict the beat carries,
// unknown when it carries none.
func readHeardClaim(env any) (any, bool) {
	me, ok := env.(*machineEnv)
	if !ok {
		return nil, false
	}
	ev, ok := me.event.(wakeEv)
	if !ok || ev.Claim == "" {
		return nil, false
	}
	return ev.Claim, true
}

// namingBeats is the beats whose event names the one it is about, in the
// order a refusal lists them: what a `holds:<topic>` guard may be read on.
var namingBeats = []machine.Beat{beatHeard, beatSeen, beatBarred, beatApproached}

// GuardWordsRefusal is the refusal of one transition's guard words that
// read the beat itself (rpg-project#1000), or "" when they are sound: a
// `heard.claim` must name held or failed and be read on `heard`; a
// `holds:<topic>` must be read on a beat that names someone (`heard`,
// `seen`, `barred`, `approached`). The one sentence the engine and a
// content compiler both say.
func GuardWordsRefusal(on machine.Beat, g *machine.Guard) string {
	if g == nil {
		return ""
	}
	if g.Fact != nil {
		name := string(g.Fact.Name)
		switch {
		case g.Fact.Name == factHeardClaim:
			if v, ok := g.Fact.Value.(string); !ok || (v != ClaimHeld && v != ClaimFailed) {
				return fmt.Sprintf("`heard.claim` is %s or %s, and this names %q", ClaimHeld, ClaimFailed, fmt.Sprint(g.Fact.Value))
			}
			if on != beatHeard {
				return fmt.Sprintf("`heard.claim` reads what the creature heard, and `%s` is not hearing: it is read on `heard` only", on)
			}
		case strings.HasPrefix(name, holdsPrefix) && len(name) > len(holdsPrefix):
			if !slices.Contains(namingBeats, on) {
				return fmt.Sprintf("`%s` asks about the one this beat is about, and `%s` is about nobody: "+
					"it is read on heard, seen, barred and approached", name, on)
			}
		}
	}
	children := []*machine.Guard{g.Not}
	for i := range g.All {
		children = append(children, &g.All[i])
	}
	for i := range g.Any {
		children = append(children, &g.Any[i])
	}
	for _, c := range children {
		if refusal := GuardWordsRefusal(on, c); refusal != "" {
			return refusal
		}
	}
	return ""
}

// guardWordsOf is the first refusal of a recipe's guard words
// ([GuardWordsRefusal]), naming the state and the transition, or nil.
func guardWordsOf(r *machine.Recipe) error {
	for _, name := range slices.Sorted(maps.Keys(r.States)) {
		for i, t := range r.States[name].Transitions {
			if refusal := GuardWordsRefusal(t.On, t.When); refusal != "" {
				return fmt.Errorf("state %q transition %d: %s", name, i, refusal)
			}
		}
	}
	return nil
}

func readHeardKind(env any) (any, bool) {
	me, ok := env.(*machineEnv)
	if !ok {
		return nil, false
	}
	ev, ok := me.event.(wakeEv)
	if !ok || ev.Kind == "" {
		return nil, false
	}
	return ev.Kind, true
}

// heardLoudnessPrefix names a guard fact that is true while the heard
// occurrence that woke the creature was at or above a loudness
// (rpg-project#933): `heard.loudness:shout` is a shout or a scream. A sound
// that states no loudness is at or above nothing.
const heardLoudnessPrefix = "heard.loudness:"

// readHeardLoudness is a `heard.loudness:<word>` fact's reader.
func readHeardLoudness(least Loudness) func(env any) (any, bool) {
	return func(env any) (any, bool) {
		me, ok := env.(*machineEnv)
		if !ok {
			return nil, false
		}
		ev, ok := me.event.(wakeEv)
		if !ok {
			return nil, false
		}
		return atOrAbove(ev.Loudness, least), true
	}
}

func readEnemy(env any) (any, bool) {
	me, ok := env.(*machineEnv)
	if !ok {
		return nil, false
	}
	if me.facts == nil && me.err == nil {
		f, err := me.e.factsFor(me.id)
		if err != nil {
			me.err = err
			return nil, false
		}
		me.facts = &f
	}
	if me.err != nil {
		return nil, false
	}
	switch {
	case me.facts.EnemyInReach, me.facts.EnemySeen:
		return string(EnemySeen), true
	case me.facts.EnemyRemembered, me.facts.EnemyLost:
		return string(EnemyRemembered), true
	}
	return string(EnemyNone), true
}

// compileRecipe validates a standalone source recipe. Live instances compile
// through NewComposer with their own completion vocabulary, never this runner.
func compileRecipe(r *machine.Recipe) (*machine.Program, error) {
	p, err := r.Compile(vocabularyFor(r))
	if err != nil {
		return nil, err
	}
	if err := guardWordsOf(r); err != nil {
		return nil, err
	}
	for _, key := range picksOf(r) {
		if slices.Contains(OptionKeys, key) || IsRetiredTableKey(key) {
			return nil, fmt.Errorf("pick %q is not an ordinary behavior table", key)
		}
	}
	return p, nil
}

func picksOf(r *machine.Recipe) []AnswerKey {
	if r == nil {
		return nil
	}
	seen := map[AnswerKey]bool{}
	for _, st := range r.States {
		for _, t := range st.Transitions {
			for _, a := range t.Do {
				if a.Pick != "" {
					seen[AnswerKey(a.Pick)] = true
				}
			}
		}
	}
	out := make([]AnswerKey, 0, len(seen))
	for key := range seen {
		out = append(out, key)
	}
	slices.Sort(out)
	return out
}

func validateBlockingStates(r *machine.Recipe, states []string) error {
	if len(states) == 0 {
		return nil
	}
	if r == nil {
		return fmt.Errorf("blocking states need an explicit behavior body instance")
	}
	for _, name := range states {
		if _, ok := r.States[name]; !ok {
			return fmt.Errorf("blocking state %q is not in its body recipe", name)
		}
	}
	return nil
}

func (e *Encounter) memberBlocks(id MemberID) bool {
	m, ok := e.members[id]
	return ok && m.blocksIn(m.bodyState())
}

func (m *memberRecord) blocksIn(state string) bool {
	if len(m.BlockingStates) == 0 {
		return m.BlocksMovement
	}
	return slices.Contains(m.BlockingStates, state)
}

func onDefaultClock(m *memberRecord) bool { return m.Clock == "" || m.Clock == worldClockName }

// relistBehavior conservatively unions actor dispatch without narrowing away
// any instance's scope. Each real recipe guard checks its own scope on delivery.
func (e *Encounter) relistBehavior(m *memberRecord) {
	if m.behaviors == nil {
		e.listeners.Set(m.ID, nil)
		return
	}
	status, _ := m.behaviors.composer.Inspect(&behavior.InspectComposerInput{})
	seen := map[machine.Beat]bool{}
	var listens []machine.Listen
	for _, instance := range status.Instances {
		for _, listen := range instance.Listens {
			if !seen[listen.Beat] {
				seen[listen.Beat] = true
				listens = append(listens, machine.Listen{Beat: listen.Beat})
			}
		}
	}
	e.listeners.Set(m.ID, listens)
}

// behaviorDue asks only world-timed instances. A fight-only role never arms the
// world heap, even if another independent role is also attached to the actor.
func (m *memberRecord) behaviorDue(now uint64) (uint64, bool) {
	if m.behaviors == nil {
		return 0, false
	}
	status, _ := m.behaviors.composer.Inspect(&behavior.InspectComposerInput{Now: now})
	var earliest uint64
	found := false
	for _, instance := range status.Instances {
		if instance.DueAt == nil || !slices.Contains(instance.Timed, beatTime) {
			continue
		}
		at := *instance.DueAt
		if at <= now {
			at = now + 1
		}
		if !found || at < earliest {
			earliest, found = at, true
		}
	}
	return earliest, found
}

func (e *Encounter) rebuildWake() error {
	e.due = machine.Due[MemberID]{}
	e.listeners = machine.Listeners[MemberID]{}
	e.dueAt = map[MemberID]uint64{}
	now := uint64(e.clock.ToData().HighWater)
	ids := make([]MemberID, 0, len(e.members))
	for id := range e.members {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	for _, id := range ids {
		e.relistBehavior(e.members[id])
		on, err := e.onWorldClock(id)
		if err != nil {
			return err
		}
		if !on {
			continue
		}
		budget, err := e.clock.Budget(&clock.BudgetInput{ID: core.EntityID(id)})
		if err != nil {
			return err
		}
		e.armOwed(id, now, budget)
	}
	return nil
}

func (e *Encounter) armOwed(id MemberID, now uint64, budget int) {
	base := now
	if budget > 0 {
		base -= uint64(budget)
	}
	e.armAt(id, base)
}

func (e *Encounter) armAt(id MemberID, now uint64) {
	m := e.members[id]
	if m == nil {
		e.disarm(id)
		return
	}
	at, due := m.behaviorDue(now)
	if !due {
		e.disarm(id)
		return
	}
	e.due.Arm(id, at)
	e.dueAt[id] = at
}

func (e *Encounter) disarm(id MemberID) {
	e.due.Disarm(id)
	delete(e.dueAt, id)
}

func (e *Encounter) jumpWake(span uint64) error {
	for id, at := range e.dueAt {
		if at > math.MaxUint64-span {
			return fmt.Errorf("member %q: %w", id, behavior.ErrBadWakeDelay)
		}
	}
	for _, m := range e.members {
		if m.behaviors == nil {
			continue
		}
		if _, err := m.behaviors.composer.DelayWakes(&behavior.DelayWakesInput{Span: span}); err != nil {
			return err
		}
	}
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
	return nil
}

// stepMachine offers one paid world opportunity through the same driver-first
// selection path as fight turns. The name remains the clock adapter's, not a
// separate Program.Step runner.
func (e *Encounter) stepMachine(id MemberID, m *memberRecord, round uint64) error {
	budget, err := e.clock.Budget(&clock.BudgetInput{ID: core.EntityID(id)})
	if err != nil {
		return err
	}
	pay := budget - int(uint64(e.clock.ToData().HighWater)-round) //nolint:gosec // bounded world rounds
	pay = max(1, min(pay, budget))
	if err := e.runBehaviorOpportunity(id, m, behaviorOpportunity{on: beatTime, event: tickEv{Round: round}, now: round}); err != nil {
		return err
	}
	on, err := e.onWorldClock(id)
	if err != nil || !on {
		return err
	}
	if pay > 0 {
		if _, err := e.clock.Spend(&clock.SpendInput{ID: core.EntityID(id), Amount: pay}); err != nil {
			return err
		}
	}
	e.armAt(id, round)
	return nil
}

func (e *Encounter) runBehaviorOpportunity(id MemberID, m *memberRecord, opportunity behaviorOpportunity) error {
	sheet, err := e.sheetOf(id)
	if err != nil {
		return err
	}
	budget := TurnBudget{MovementFeet: sheet.SpeedFeet}
	previous := e.opportunity
	e.opportunity = &opportunity
	defer func() { e.opportunity = previous }()
	_, err = e.runIntents(nil, core.EntityID(id), m, 0, &budget, 0, intentBound(budget), ClockWorld)
	return err
}

func (e *Encounter) appendStateBeat(id MemberID, stepped machine.Stepped) error {
	payload, err := json.Marshal(map[string]interface{}{
		"beat": BeatState, "member": string(id), "from": stepped.From, "to": stepped.To, "reason": stepped.Reason,
	})
	if err != nil {
		return err
	}
	_, err = e.appendBeat(&record.AppendInput{At: uint64(e.clock.ToData().HighWater),
		Audience: e.audienceFor(subjectBeat, id), Tags: map[string]string{"tag": BeatState}, Payload: payload})
	return err
}

// MachineData recognizes the retired saved field so loading refuses it instead
// of silently resetting it. No live member writes or runs this representation.
type MachineData struct {
	State  string         `json:"state,omitempty"`
	Bag    map[string]any `json:"bag,omitempty"`
	WakeAt *uint64        `json:"wake_at,omitempty"`
	Anchor *uint64        `json:"anchor,omitempty"`
}

func keyOr(key AnswerKey) AnswerKey {
	if key == "" {
		return AnswerTime
	}
	return key
}

// MachineBeats names the composition's public occurrence/opportunity vocabulary.
var MachineBeats = []string{string(beatTime), string(beatTurn), string(beatHeard), string(beatDeed), string(beatSeen), string(beatFact), string(beatTopic), beatClock + worldClockName, beatClock + clockFight, string(beatBarred), string(beatApproached)}

// ValidMachineBeat admits a declared beat, named deed or named clock notification.
func ValidMachineBeat(on string) bool {
	switch {
	case slices.Contains(MachineBeats, on):
		return true
	case strings.HasPrefix(on, deedPrefix):
		return slices.Contains(WhenDeeds, strings.TrimPrefix(on, deedPrefix))
	case strings.HasPrefix(on, beatClock):
		return len(on) > len(beatClock)
	}
	return false
}

// MachineBeatScoped identifies inputs whose occurrence carries a place.
func MachineBeatScoped(on string) bool {
	return on == string(beatHeard) || on == string(beatDeed) || on == string(beatSeen) || on == string(beatBarred) || on == string(beatApproached) || strings.HasPrefix(on, deedPrefix) && ValidMachineBeat(on)
}

// MachineFact names an observer-local learned fact.
func MachineFact(id string) machine.FactName { return machine.FactName(factPrefix + id) }

// MachineTopic names a topic for a recipe guard (rpg-project#929): `topic:<id>`,
// true while the member holds that topic about anybody, read from its own
// holdings.
func MachineTopic(id string) machine.FactName { return machine.FactName(topicPrefix + id) }

// MachineHolds names a guard fact true while the creature holds a topic
// about the one the beat names (rpg-project#1000): `holds:<id>`.
func MachineHolds(id string) machine.FactName { return machine.FactName(holdsPrefix + id) }

// MachineEnemyFact reads the actor's own enemy band, never authoritative intel.
const MachineEnemyFact = factEnemy

// ValidateMachine validates standalone authored recipe vocabulary without play.
func ValidateMachine(r *machine.Recipe) error {
	if r == nil {
		return nil
	}
	_, err := compileRecipe(r)
	return err
}
