// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"encoding/json"
	"errors"
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

// machineVocabulary is the machine's vocabulary, read from [MachineWords]:
// every whole beat word with its timing and scope, and every whole fact
// word with its kind and its reader ([machineFactReaders]).
func machineVocabulary() machine.Vocabulary {
	v := machine.Vocabulary{Beats: map[machine.Beat]machine.BeatSpec{}, Facts: map[machine.FactName]machine.FactSpec{}}
	for _, b := range machineBeatWords {
		if b.Open != SlotNone {
			continue
		}
		v.Beats[machine.Beat(b.Word)] = beatSpecOf(b)
	}
	for _, c := range machineConditionWords {
		if !c.Fact || c.Open != SlotNone {
			continue
		}
		v.Facts[machine.FactName(c.Word)] = machine.FactSpec{Kind: scalarKindOf(c.Kind), Read: machineFactReaders[c.Word]}
	}
	return v
}

// beatSpecOf is a table beat as the machine compiles it.
func beatSpecOf(b BeatWord) machine.BeatSpec {
	spec := machine.BeatSpec{Timed: b.Timed, Round: wakeRound}
	if b.Scoped {
		spec.InScope = inScope
	}
	return spec
}

// scalarKindOf is a table kind as the machine names it.
func scalarKindOf(kind string) machine.ScalarKind {
	switch kind {
	case "string":
		return machine.KindString
	case "int":
		return machine.KindInt
	}
	return machine.KindBool
}

// machineFactReaders reads each whole fact word of [MachineWords] from the
// creature's event or world. A table fact without a reader is caught by the
// table's own test.
var machineFactReaders = map[string]func(env any) (any, bool){
	string(factEnemy):      readEnemy,
	"heard.kind":           readHeardKind,
	string(factHeardClaim): readHeardClaim,
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
	// WHAT THIS CREATURE HAS NOTED ABOUT THE ONE THIS BEAT IS ABOUT
	// (rpg-project#1000): the question a line's `noted` requirement asks,
	// asked from the same holder, about the one the beat names. Nothing is
	// noted about a beat that names nobody: false.
	if g.Fact != nil && strings.HasPrefix(string(g.Fact.Name), notedPrefix) && len(g.Fact.Name) > len(notedPrefix) {
		topic := TopicID(strings.TrimPrefix(string(g.Fact.Name), notedPrefix))
		facts[g.Fact.Name] = machine.FactSpec{Kind: machine.KindBool, Read: func(env any) (any, bool) {
			me, ok := env.(*machineEnv)
			if !ok {
				return nil, false
			}
			ev, ok := me.event.(wakeEv)
			if !ok || ev.Subject == "" {
				return false, true
			}
			return me.e.hasNoted(me.id, ev.Subject, topic), true
		}}
	}
	// A RAISED VOICE (rpg-project#933): only a loudness this rulebook names
	// is registered, so a misspelt word is a fact the vocabulary refuses,
	// and so is none at all (`heard.loudness:` names no loudness; the
	// machine words table says the loudness is required, rpg-project#1001).
	if word, ok := strings.CutPrefix(string(guardFactName(g)), heardLoudnessPrefix); ok && word != "" {
		if l, ok := loudnessOf(word); ok {
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

// notedPrefix names a guard fact true while this creature has noted a topic
// about the one the beat names (rpg-project#1000): `noted:<topic>`.
// `noted.anyone:<topic>` (the engine's `topic:<id>`) is the question
// about anybody.
const notedPrefix = "noted:"

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

// GuardWordsRefusal is the refusal of one transition's guard words that
// read the beat itself (rpg-project#1000), or "" when they are sound: a
// `heard.claim` must name one of its values and be read on a beat the table
// allows it on; a `noted:<topic>` must be read on a beat that names someone.
// The one sentence the engine and a content compiler both say, joined from
// [MachineWords].
func GuardWordsRefusal(on machine.Beat, g *machine.Guard) string {
	refusal, _ := GuardWordsRefusalAt(on, g)
	return refusal
}

// GuardWordsRefusalAt is [GuardWordsRefusal] with the place of the refused
// leaf inside the step, as the machine names places: `when`, then `all[i]`,
// `any[i]`, `not`, and `fact`. Nil with "" when the words are sound.
func GuardWordsRefusalAt(on machine.Beat, g *machine.Guard) (string, []machine.Segment) {
	return guardWordsAt(on, g, []machine.Segment{{Name: "when"}})
}

// guardWordsAt is [GuardWordsRefusalAt] below the leaf at.
func guardWordsAt(on machine.Beat, g *machine.Guard, at []machine.Segment) (string, []machine.Segment) {
	if g == nil {
		return "", nil
	}
	if g.Fact != nil {
		name := string(g.Fact.Name)
		word, known := ConditionWordOf(name)
		here := append(slices.Clone(at), machine.Segment{Name: "fact"})
		switch {
		case known && word.Word == string(factHeardClaim):
			if v, ok := g.Fact.Value.(string); !ok || !slices.Contains(word.Values, v) {
				return fmt.Sprintf("`%s` is %s, and this names %q", word.Word, strings.Join(word.Values, " or "), fmt.Sprint(g.Fact.Value)), here
			}
			if allowed := word.allowedOn(); !beatAllowed(allowed, on) {
				return fmt.Sprintf("`%s` reads what the creature heard, and `%s` is not hearing: it is read on `%s` only",
					word.Word, on, strings.Join(allowed, "`, `")), here
			}
		case known && word.Word == notedPrefix:
			if !beatAllowed(word.allowedOn(), on) {
				return fmt.Sprintf("`%s` asks about the one this beat is about, and `%s` is about nobody: "+
					"it is read on %s", name, on, joinWords(word.allowedOn(), "and")), here
			}
		}
	}
	type child struct {
		g   *machine.Guard
		seg machine.Segment
	}
	children := []child{{g.Not, machine.Segment{Name: "not"}}}
	for i := range g.All {
		children = append(children, child{&g.All[i], machine.Segment{Name: "all", Index: i, Indexed: true}})
	}
	for i := range g.Any {
		children = append(children, child{&g.Any[i], machine.Segment{Name: "any", Index: i, Indexed: true}})
	}
	for _, c := range children {
		if refusal, leaf := guardWordsAt(on, c.g, append(slices.Clone(at), c.seg)); refusal != "" {
			return refusal, leaf
		}
	}
	return "", nil
}

// joinWords joins words as "a, b and c" (or "a, b or c").
func joinWords(words []string, conj string) string {
	switch len(words) {
	case 0:
		return ""
	case 1:
		return words[0]
	}
	return strings.Join(words[:len(words)-1], ", ") + " " + conj + " " + words[len(words)-1]
}

// StepRefusal is a refusal of the encounter's own about one step of a
// machine (rpg-project#997): the state, the step, the place inside it as the
// machine names places ([machine.Segment]), and the sentence. Its text is
// "state %q transition %d: <message>", as it always read.
type StepRefusal struct {
	// State is the state the refused step is in.
	State string
	// Transition is the step's index within State.
	Transition int
	// Leaf is the place inside the step; empty for the step itself.
	Leaf []machine.Segment
	// Message is the sentence, without the place.
	Message string
}

// Error is the refusal as it has always read.
func (r *StepRefusal) Error() string {
	return fmt.Sprintf("state %q transition %d: %s", r.State, r.Transition, r.Message)
}

// StepRefusals is every [*StepRefusal] in err, however it is joined or
// wrapped, in order; none for nil or an error holding none.
func StepRefusals(err error) []*StepRefusal {
	var out []*StepRefusal
	var walk func(error)
	walk = func(e error) {
		switch v := e.(type) {
		case nil:
		case *StepRefusal:
			out = append(out, v)
		case interface{ Unwrap() []error }:
			for _, kid := range v.Unwrap() {
				walk(kid)
			}
		default:
			walk(errors.Unwrap(e))
		}
	}
	walk(err)
	return out
}

// guardWordsOf is the first refusal of a recipe's guard words
// ([GuardWordsRefusal]), as a [*StepRefusal] naming its state, step and
// leaf, or nil.
func guardWordsOf(r *machine.Recipe) error {
	for _, name := range slices.Sorted(maps.Keys(r.States)) {
		for i, t := range r.States[name].Transitions {
			if refusal, leaf := GuardWordsRefusalAt(t.On, t.When); refusal != "" {
				return &StepRefusal{State: name, Transition: i, Leaf: leaf, Message: refusal}
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

// guardFactName is the name of g's fact leaf, or "" for none.
func guardFactName(g *machine.Guard) machine.FactName {
	if g == nil || g.Fact == nil {
		return ""
	}
	return g.Fact.Name
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

// appendRoleStateBeat writes [BeatRoleState] for one of m's roles other than
// its body: the role's instance id and its recipe, the state it left and
// entered, and why. Its audience is m alone.
func (e *Encounter) appendRoleStateBeat(m *memberRecord, role string, stepped machine.Stepped) error {
	body := map[string]interface{}{
		"beat": BeatRoleState, "member": string(m.ID), "role": role,
		"from": stepped.From, "to": stepped.To, "reason": stepped.Reason,
	}
	// A ROLE NAMES ITS RECIPE OR THE BEAT IS NOT WRITTEN: a step from an
	// instance the creature does not have is a broken composition.
	i, ok := m.behaviors.byID[role]
	if !ok {
		return fmt.Errorf("role state: %s has no role %q", m.ID, role)
	}
	body["recipe"] = m.behaviors.input.Instances[i].Instance.Recipe.Ref.String()
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	_, err = e.appendBeat(&record.AppendInput{At: uint64(e.clock.ToData().HighWater),
		Audience: []MemberID{m.ID}, Tags: map[string]string{"tag": BeatRoleState}, Payload: payload})
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

// MachineBeats names the composition's whole beat words, in the order a
// refusal lists them: [MachineWords]' beats.
var MachineBeats = beatsWhere(func(BeatWord) bool { return true })

// ValidMachineBeat admits a whole beat word, a deed word this rulebook names
// (`deed:<word>`) or a named clock (`clock:<name>`), by [MachineWords].
func ValidMachineBeat(on string) bool {
	b, ok := BeatWordOf(on)
	switch {
	case !ok:
		return false
	case b.Open == SlotDeed:
		return slices.Contains(WhenDeeds, strings.TrimPrefix(on, b.Word))
	}
	return true
}

// MachineBeatScoped identifies a beat whose occurrence carries a place: the
// table's Scoped.
func MachineBeatScoped(on string) bool {
	b, ok := BeatWordOf(on)
	return ok && b.Scoped && ValidMachineBeat(on)
}

// MachineFact names an observer-local learned fact.
func MachineFact(id string) machine.FactName { return machine.FactName(factPrefix + id) }

// MachineTopic names a topic for a recipe guard (rpg-project#929): `topic:<id>`,
// true while the member holds that topic about anybody, read from its own
// holdings.
func MachineTopic(id string) machine.FactName { return machine.FactName(topicPrefix + id) }

// MachineNoted names a guard fact true while the creature has noted a topic
// about the one the beat names (rpg-project#1000): `noted:<id>`.
func MachineNoted(id string) machine.FactName { return machine.FactName(notedPrefix + id) }

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
