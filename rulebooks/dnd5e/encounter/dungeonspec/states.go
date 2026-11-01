// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec

import (
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/mind/machine"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// states.go is A CREATURE'S STATES (rpg-project#766 S4, design §4 of
// ideas/the-world-runs): the recipe a creature's machine runs, written in the
// studio's words and compiled to [machine.Recipe] for
// [encounter.MemberInput.Machine].
//
//	states:
//	  initial: posted
//	  posted:
//	    transitions:
//	      - { on: heard, within: 6, to: alert, do: [{ anchor: true }] }
//	  alert:
//	    transitions:
//	      - { on: time, rounds: 3, to: posted }
//	      - { on: time, do: [{ pick: time }] }
//
// A STATE'S PICK NAMES A TABLE KEY, so the author's existing vocabulary —
// rows, `when`, weights, temper — is the body of every state, and the states
// are only the skeleton that says which key applies now. A key a state picks
// is a legal key of any table in the same document (`march:` beside `time:`).
//
// LAYERED LIKE A TABLE, NEAREST WINS WHOLESALE (creature-table R4): a
// binding's `states` replaces its faction's entirely; neither means the
// default recipe, `watch: on time, pick time`.
//
// THE BEATS ([encounter.MachineBeats]): `time`, `heard`, `deed` (or
// `deed:<word>`), `seen`, `fact`, and `clock:<name>` — `clock:world`,
// `clock:fight`, `clock:bell:<prop>`. `within` (hexes) and `regions` are
// legal only on a beat that happened somewhere (`heard`, `deed`, `seen`).
// `rounds: N` holds once N rounds have passed since the creature's anchor
// (an `anchor` act). A `when` is one of `enemy`, `fact` or `not`.

// StatesSpec is a creature's recipe: the initial state and every state by
// name. Written flat — `initial` beside the states — so a state is never
// named `initial`.
type StatesSpec struct {
	// Initial is the state the creature starts in. REQUIRED, and a state
	// this block declares.
	Initial string
	// States is every state by name.
	States map[string]StateSpec
}

// StateSpec is one state: what it listens for, in order, the first whose
// conditions hold firing. Terminal ends the machine and carries none.
type StateSpec struct {
	Transitions []TransitionSpec `yaml:"transitions,omitempty" json:"transitions,omitempty"`
	Terminal    bool             `yaml:"terminal,omitempty" json:"terminal,omitempty"`
}

// TransitionSpec is one transition: on a beat, when its conditions hold, do
// its acts and go to a state.
//
//   - On is the beat ([encounter.MachineBeats]).
//   - When is one condition the creature holds: `enemy`, `fact` or `not`.
//   - Within is how many hexes from the creature the beat may have happened;
//     Regions how many regions away. Zero is not asked.
//   - Rounds holds once that many rounds have passed since the anchor.
//   - To is the state it enters; empty stays.
//   - Do is its acts: `pick` a table key, `wake_in` sleep that many rounds,
//     `anchor` stamp the round.
//   - Reason is why, shown on the state beat.
type TransitionSpec struct {
	On      string    `yaml:"on" json:"on"`
	When    *WhenSpec `yaml:"when,omitempty" json:"when,omitempty"`
	Within  int       `yaml:"within,omitempty" json:"within,omitempty"`
	Regions int       `yaml:"regions,omitempty" json:"regions,omitempty"`
	Rounds  int       `yaml:"rounds,omitempty" json:"rounds,omitempty"`
	To      string    `yaml:"to,omitempty" json:"to,omitempty"`
	Do      []ActSpec `yaml:"do,omitempty" json:"do,omitempty"`
	Reason  string    `yaml:"reason,omitempty" json:"reason,omitempty"`
}

// ActSpec is one act; exactly one field is set.
type ActSpec struct {
	Pick   string `yaml:"pick,omitempty" json:"pick,omitempty"`
	WakeIn int    `yaml:"wake_in,omitempty" json:"wake_in,omitempty"`
	Anchor bool   `yaml:"anchor,omitempty" json:"anchor,omitempty"`
}

// statesInitial is the key the initial state is written under.
const statesInitial = "initial"

// UnmarshalYAML reads the flat block: `initial` and one key per state,
// refusing an unknown key inside a state or a transition (Decode's
// strictness does not reach inside a custom unmarshaler).
func (s *StatesSpec) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind != yaml.MappingNode {
		return fmt.Errorf("line %d: `states` is `initial: <state>` and one key per state", value.Line)
	}
	out := StatesSpec{States: map[string]StateSpec{}}
	for i := 0; i+1 < len(value.Content); i += 2 {
		key, body := value.Content[i], value.Content[i+1]
		if key.Value == statesInitial {
			if body.Kind != yaml.ScalarNode {
				return fmt.Errorf("line %d: `initial` names one state", body.Line)
			}
			out.Initial = body.Value
			continue
		}
		var st StateSpec
		if err := decodeStrict(body, &st); err != nil {
			return fmt.Errorf("line %d: state %q: %w", body.Line, key.Value, err)
		}
		out.States[key.Value] = st
	}
	*s = out

	return nil
}

// MarshalYAML writes the flat block back.
func (s StatesSpec) MarshalYAML() (any, error) {
	out := map[string]any{statesInitial: s.Initial}
	for name, st := range s.States {
		out[name] = st
	}

	return out, nil
}

// MarshalJSON writes the flat block, the YAML's own shape.
func (s StatesSpec) MarshalJSON() ([]byte, error) {
	out := map[string]any{statesInitial: s.Initial}
	for name, st := range s.States {
		out[name] = st
	}

	return json.Marshal(out)
}

// UnmarshalJSON reads the flat block.
func (s *StatesSpec) UnmarshalJSON(raw []byte) error {
	var flat map[string]json.RawMessage
	if err := json.Unmarshal(raw, &flat); err != nil {
		return err
	}
	out := StatesSpec{States: map[string]StateSpec{}}
	for k, v := range flat {
		if k == statesInitial {
			if err := json.Unmarshal(v, &out.Initial); err != nil {
				return fmt.Errorf("states.initial: %w", err)
			}
			continue
		}
		var st StateSpec
		if err := json.Unmarshal(v, &st); err != nil {
			return fmt.Errorf("states.%s: %w", k, err)
		}
		out.States[k] = st
	}
	*s = out

	return nil
}

// decodeStrict decodes a node refusing unknown fields, the dialect's
// `KnownFields(true)` for a node reached through a custom unmarshaler.
func decodeStrict(node *yaml.Node, out any) error {
	raw, err := yaml.Marshal(node)
	if err != nil {
		return err
	}
	dec := yaml.NewDecoder(strings.NewReader(string(raw)))
	dec.KnownFields(true)

	return dec.Decode(out)
}

// stateNames is a block's states in name order.
func (s *StatesSpec) stateNames() []string {
	names := make([]string, 0, len(s.States))
	for n := range s.States {
		names = append(names, n)
	}
	sort.Strings(names)

	return names
}

// picks is every table key the block's states pick.
func (s *StatesSpec) picks() []string {
	if s == nil {
		return nil
	}
	var out []string
	for _, st := range s.States {
		for _, t := range st.Transitions {
			for _, a := range t.Do {
				if a.Pick != "" && !slices.Contains(out, a.Pick) {
					out = append(out, a.Pick)
				}
			}
		}
	}
	sort.Strings(out)

	return out
}

// picksIn is every key any recipe in the document picks: a key a table may
// carry beside `time`.
func picksIn(factions []FactionSpec, bindings map[string]RoomMonsterBinding) map[string]bool {
	out := map[string]bool{}
	for _, fa := range factions {
		for _, k := range fa.States.picks() {
			out[k] = true
		}
	}
	for _, b := range bindings {
		for _, k := range b.States.picks() {
			out[k] = true
		}
	}

	return out
}

// placeStates refuses a states block this build could not run, each at its
// own path: no initial or an initial it lacks, a terminal state with
// transitions, a beat this build does not supply, a `to` it lacks, a
// `within`/`regions` on a beat that happened nowhere, a `when` other than
// `enemy`, `fact` or `not`, an act that is not exactly one of `pick`,
// `wake_in` above zero or `anchor`, and finally anything the composition's
// own compile refuses ([encounter.ValidateMachine]).
func (g *grammar) placeStates(path string, s *StatesSpec, owner string) {
	if s == nil {
		return
	}
	at := path + ".states"
	switch _, ok := s.States[s.Initial]; {
	case s.Initial == "":
		g.fail(at+".initial", "a creature's states name the one it starts in: write `initial: <state>`")
	case !ok:
		g.fail(at+".initial", "%q is not a state this block declares: they are %s",
			s.Initial, strings.Join(s.stateNames(), ", "))
	}
	failed := false
	fail := func(p, format string, args ...any) {
		failed = true
		g.fail(p, format, args...)
	}
	for _, name := range s.stateNames() {
		st := s.States[name]
		sp := at + "." + name
		if st.Terminal && len(st.Transitions) > 0 {
			fail(sp+".terminal", "a terminal state ends the machine and listens for nothing")
		}
		for i, t := range st.Transitions {
			tp := fmt.Sprintf("%s.transitions[%d]", sp, i)
			if !encounter.ValidMachineBeat(t.On) {
				fail(tp+".on", "%q is not a beat a state listens for: they are %s, `deed:<word>` and `clock:<name>`",
					t.On, strings.Join(encounter.MachineBeats, ", "))
			}
			if t.To != "" {
				if _, ok := s.States[t.To]; !ok {
					fail(tp+".to", "%q is not a state this block declares", t.To)
				}
			}
			if (t.Within != 0 || t.Regions != 0) && !encounter.MachineBeatScoped(t.On) {
				fail(tp+".within", "`%s` happened nowhere, so there is nothing to be within: "+
					"`within` and `regions` are for heard, deed and seen", t.On)
			}
			if t.Within < 0 || t.Regions < 0 || t.Rounds < 0 {
				fail(tp, "a distance or a span is counted from 1")
			}
			if t.When != nil {
				if _, err := guardOf(t.When); err != nil {
					fail(tp+".when", "%s", err)
				}
			}
			for j, a := range t.Do {
				ap := fmt.Sprintf("%s.do[%d]", tp, j)
				set := 0
				if a.Pick != "" {
					set++
				}
				if a.WakeIn != 0 {
					set++
				}
				if a.Anchor {
					set++
				}
				if set != 1 {
					fail(ap, "an act is one of { pick: <key> }, { wake_in: N } or { anchor: true }")
				}
				if a.WakeIn < 0 {
					fail(ap+".wake_in", "a sleep is counted from 1")
				}
			}
		}
	}
	if failed || s.Initial == "" {
		return
	}
	r, err := recipeOf(s, owner)
	if err != nil {
		g.fail(at, "%s", err)
		return
	}
	if err := encounter.ValidateMachine(r); err != nil {
		g.fail(at, "%s", err)
	}
}

// guardOf compiles a state's one condition to a machine guard.
//
// Errors: a condition a state cannot read (a deed, a sound or a threat are a
// row's conditions; a state listens for those as beats).
func guardOf(w *WhenSpec) (*machine.Guard, error) {
	switch {
	case w.Not != nil:
		inner, err := guardOf(w.Not)
		if err != nil {
			return nil, err
		}
		return &machine.Guard{Not: inner}, nil
	case w.Enemy != "":
		if !slices.Contains([]string{string(encounter.EnemySeen), string(encounter.EnemyRemembered), string(encounter.EnemyNone)}, w.Enemy) {
			return nil, fmt.Errorf("a state reads `enemy` as seen, remembered or none, and this names %q", w.Enemy)
		}
		return &machine.Guard{Fact: &machine.FactLeaf{Name: encounter.MachineEnemyFact, Op: machine.OpEq, Value: w.Enemy}}, nil
	case w.Fact != "":
		return &machine.Guard{Fact: &machine.FactLeaf{Name: encounter.MachineFact(w.Fact), Op: machine.OpEq, Value: true}}, nil
	}

	return nil, fmt.Errorf("a state's `when` is one of { enemy }, { fact } or { not }: a deed, a sound and a threat are beats a state listens for (`on: deed`, `on: heard`)")
}

// recipeOf compiles a states block to the machine's recipe, named after the
// placement or faction that authored it.
//
// Errors: a `when` a state cannot read ([guardOf]).
func recipeOf(s *StatesSpec, owner string) (*machine.Recipe, error) {
	ref, err := core.ParseString("dnd5e:machines:" + owner)
	if err != nil {
		ref, _ = core.ParseString("dnd5e:machines:authored")
	}
	out := &machine.Recipe{Ref: *ref, Initial: s.Initial, States: map[string]machine.State{}}
	for name, st := range s.States {
		ms := machine.State{Terminal: st.Terminal}
		for _, t := range st.Transitions {
			var parts []machine.Guard
			if t.Within > 0 || t.Regions > 0 {
				parts = append(parts, machine.Guard{Scope: &machine.ScopeLeaf{Within: t.Within, Regions: t.Regions}})
			}
			if t.Rounds > 0 {
				parts = append(parts, machine.Guard{Rounds: &machine.RoundsLeaf{Op: machine.OpGte, N: t.Rounds}})
			}
			if t.When != nil {
				g, gerr := guardOf(t.When)
				if gerr != nil {
					return nil, gerr
				}
				parts = append(parts, *g)
			}
			mt := machine.Transition{On: machine.Beat(t.On), To: t.To, Reason: t.Reason}
			switch len(parts) {
			case 0:
			case 1:
				mt.When = &parts[0]
			default:
				mt.When = &machine.Guard{All: parts}
			}
			for _, a := range t.Do {
				mt.Do = append(mt.Do, machine.Act{Pick: a.Pick, WakeIn: a.WakeIn, Anchor: a.Anchor})
			}
			ms.Transitions = append(ms.Transitions, mt)
		}
		out.States[name] = ms
	}

	return out, nil
}

// machineOf is a creature's compiled recipe: its own states, else its
// faction's — nearest wins wholesale — else nil for the default. A block
// the grammar refused never reaches here.
func machineOf(own, faction *StatesSpec, owner, side string) *machine.Recipe {
	s, name := own, owner
	if s == nil {
		s, name = faction, side
	}
	if s == nil {
		return nil
	}
	r, err := recipeOf(s, name)
	if err != nil {
		return nil
	}

	return r
}

// placeCreatureKeys refuses, at one creature's own path, a key of its
// resolved table that the machine it resolves to never picks: the
// encounter's door judges the table against that creature's machine, not
// the document's ([encounter.MemberInput.Machine]), so a `march:` legal for
// the reserves is refused on a creature whose own states win and pick no
// `march`. own is the creature's own orders, already laid over the table its
// binding names; ownStates is its own `states`. The faction's orders and
// states are the layer beneath, nearest winning wholesale, as compiled.
//
// A key no state anywhere picks is [grammar.placeOn]'s refusal at the
// table's own path, and is not reported a second time here.
func (g *grammar) placeCreatureKeys(path, faction string, own map[string][]AnswerSpec, ownStates *StatesSpec) {
	side := sideOf(faction)
	var sideOn map[string][]AnswerSpec
	var sideStates *StatesSpec
	for _, fa := range g.declaredFactions {
		if fa.ID == side {
			sideOn = layerSpecs(rootTableOn(g.tables, fa.Table), fa.On)
			sideStates = fa.States
		}
	}
	states := ownStates
	if states == nil {
		states = sideStates
	}
	picked := states.picks()
	for _, key := range sortedKeys(layerSpecs(sideOn, own)) {
		if knownTableKey(key) || !g.picked[key] || slices.Contains(picked, key) {
			continue
		}
		if len(picked) == 0 {
			g.fail(path, "this creature's table carries %q and the states it runs pick no key "+
				"(it runs the default, which picks time): give it states that pick %q, or a table without it", key, key)
			continue
		}
		g.fail(path, "this creature's table carries %q and the states it runs never pick it: they pick %s",
			key, strings.Join(picked, ", "))
	}
}
