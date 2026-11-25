// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/mind/machine"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// interactions.go is the root `interactions:` key (rpg-project#922, Gate S2
// of #769): the entrance that remembers, authored as one native
// `machine.Recipe` per interaction and judged by the engine's own door
// ([encounter.CheckInteraction]) at the author's path.

// interactionShapes judges every root interaction and returns what an option
// may name of each, by id. A refused interaction is kept as a nil entry, so
// a line naming it is not refused a second time for the one defect.
func interactionShapes(s *SingleRoomSpec, add errSink) map[string]*interactionShape {
	if len(s.Interactions) == 0 {
		return nil
	}
	out := make(map[string]*interactionShape, len(s.Interactions))
	for _, id := range sortedKeys(s.Interactions) {
		out[id] = nil
		path := "interactions." + id
		in, ok := interactionInputOf(id, s.Interactions[id], path, regionIDsOf(s), add)
		if !ok {
			continue
		}
		checked, err := encounter.CheckInteraction(&in)
		if err != nil {
			add(path+".recipe", err.Error())
			continue
		}
		shape := &interactionShape{states: map[string]bool{}, beats: map[string]bool{}, topics: map[string]bool{}}
		for _, topic := range checked.Topics {
			shape.topics[string(topic)] = true
		}
		for _, state := range checked.States {
			shape.states[state] = true
		}
		for _, beat := range checked.Beats {
			shape.beats[string(beat)] = true
		}
		out[id] = shape
	}
	return out
}

// interactionInputOf decodes one interaction's recipe with unknown members
// refused and its ref required, reporting a defect at path when it cannot.
func interactionInputOf(id string, spec InteractionSpec, path string, regions map[string]bool, add errSink) (encounter.InteractionInput, bool) {
	if spec.Recipe == nil {
		add(path+".recipe", "an interaction is its recipe, and this names none")
		return encounter.InteractionInput{}, false
	}
	var recipe machine.Recipe
	if err := decodeBehaviorValue(spec.Recipe, &recipe); err != nil {
		add(path+".recipe", err.Error())
		return encounter.InteractionInput{}, false
	}
	if err := recipe.Ref.IsValid(); err != nil {
		add(path+".recipe.ref", err.Error())
		return encounter.InteractionInput{}, false
	}
	in := encounter.InteractionInput{ID: encounter.InteractionID(id), Recipe: recipe}
	outcomes, ok := outcomesOf(spec.Outcomes, &recipe, path+".outcomes", regions, add)
	if !ok {
		return encounter.InteractionInput{}, false
	}
	in.Outcomes = outcomes
	if spec.Approach != nil {
		if spec.Approach.Within < 1 {
			add(path+".approach.within", "an approach is a distance of at least 1 cell")
			return encounter.InteractionInput{}, false
		}
		in.Approach = &encounter.InteractionApproach{Within: spec.Approach.Within}
	}
	return in, true
}

// interactionsOf lowers the root interactions into the field's, in id
// order; nil for none. Reached only after validation passed, so a recipe
// that cannot decode is not here to be dropped.
func interactionsOf(s *SingleRoomSpec) []encounter.InteractionInput {
	if len(s.Interactions) == 0 {
		return nil
	}
	out := make([]encounter.InteractionInput, 0, len(s.Interactions))
	for _, id := range sortedKeys(s.Interactions) {
		in, ok := interactionInputOf(id, s.Interactions[id], "", regionIDsOf(s), func(string, string) {})
		if ok {
			out = append(out, in)
		}
	}
	return out
}

// outcomesOf judges and lowers an interaction's `outcomes` (rpg-project#928)
// at the author's own paths: every key a state of the recipe, every state
// naming at least one consequence, every consequence `give`, and every
// word of it one of the fixed ones. The engine judges the same again
// ([encounter.CheckInteraction]). Nil for none.
func outcomesOf(
	specs map[string][]OutcomeSpec, recipe *machine.Recipe, path string, regions map[string]bool, add errSink,
) (map[string][]encounter.Consequence, bool) {
	if len(specs) == 0 {
		return nil, true
	}
	ok := true
	fail := func(at, message string) { ok = false; add(at, message) }
	out := make(map[string][]encounter.Consequence, len(specs))
	for _, state := range sortedKeys(specs) {
		at := path + "." + state
		if _, declared := recipe.States[state]; !declared {
			fail(at, fmt.Sprintf("%q is not a state of this interaction's recipe", state))
			continue
		}
		if len(specs[state]) == 0 {
			fail(at, "an outcome lists what follows entering the state: write at least one `give`, `arrest` or `jail`")
			continue
		}
		for i, c := range specs[state] {
			itemAt := fmt.Sprintf("%s[%d]", at, i)
			switch {
			case kinds(c) > 1:
				fail(itemAt, "a consequence is one kind: `give`, `arrest` or `jail`, not two")
				continue
			case c.Jail != nil:
				switch {
				case c.Jail.Words == "":
					fail(itemAt+".jail.words", "a jail tells the jailed what happens to them: write its words")
				case c.Jail.Rounds < 0:
					fail(itemAt+".jail.rounds", "a jail's rounds are zero or more")
				default:
					out[state] = append(out[state], encounter.Consequence{Jail: &encounter.Jail{
						Words: c.Jail.Words, Rounds: c.Jail.Rounds, Escaped: encounter.FactID(c.Jail.Escaped),
					}})
				}
				continue
			case c.Arrest != nil:
				if arrest, ok := arrestOf(itemAt+".arrest", c.Arrest, regions, fail); ok {
					out[state] = append(out[state], encounter.Consequence{Arrest: &arrest})
				}
				continue
			case c.Give == nil:
				fail(itemAt, "a consequence is `give: { topic, to, about }`, `arrest: { cell, area }` or `jail: { words }`")
				continue
			}
			rec := givingOf(itemAt+".give", c.Give, true, fail)
			out[state] = append(out[state], encounter.Consequence{Recognize: &rec})
		}
	}
	return out, ok
}

// givingOf judges one authored `give:` at its path, wherever it hangs (an
// interaction's outcome, a line's success or failure row; rpg-project#928,
// #929, #932, #1005), and lowers it to the engine's [encounter.Recognition].
// carriers says whether the place names an interaction whose other carriers
// could hear it: where none is, `heard_by` is refused, since nobody would
// hear.
func givingOf(at string, r *GiveSpec, carriers bool, fail func(at, message string)) encounter.Recognition {
	if r.Topic == "" {
		fail(at+".topic", "a give names the topic given: write `topic: <topic>`")
	}
	// THE WHO-WORDS ARE THE PUBLISHED LIST'S (encounter.WhoWords,
	// rpg-project#1001): what `to`, `about` and `heard_by` accept is what
	// the list allows there, so the read-back cannot name another.
	tos, abouts, hearers := encounter.WhoWordsIn(encounter.WhoInGiveTo), encounter.WhoWordsIn(encounter.WhoInGiveAbout),
		encounter.WhoWordsIn(encounter.WhoInGiveHeardBy)
	if !slices.Contains(tos, r.To) {
		fail(at+".to", fmt.Sprintf("a topic is given `to: %s`, the creature the line was said to", strings.Join(tos, "`, `")))
	}
	if !slices.Contains(abouts, r.About) {
		fail(at+".about", subjectsSentence(encounter.WhoInGiveAbout))
	}
	switch {
	case r.HeardBy != "" && !slices.Contains(hearers, r.HeardBy):
		fail(at+".heard_by", fmt.Sprintf("the hearers are `%s`, or omit it for nobody", strings.Join(hearers, "`, `")))
	case r.HeardBy != "" && !carriers:
		fail(at+".heard_by", "this line names no interaction, so it has no other carriers to hear it: "+
			"omit heard_by, or give the line an interaction it requires or advances")
	}
	return encounter.Recognition{
		Topic: encounter.TopicID(r.Topic), As: r.As, Holder: encounter.RecognitionHolder(r.To),
		About: encounter.RecognitionSubjects(r.About), HeardBy: encounter.RecognitionHearers(r.HeardBy),
	}
}

// subjectsSentence says whom a note may be about where the who-word table
// allows `about`, joined from the table whatever it holds: each word with its
// label. THE ONE SENTENCE a conversation's give and a machine state's give
// share.
func subjectsSentence(place string) string {
	said := []string{}
	for _, w := range encounter.WhoWordsIn(place) {
		wd, _ := encounter.WhoWordAt(place, w)
		said = append(said, fmt.Sprintf("`%s`, %s", w, wd.Label))
	}
	return "the subjects are " + strings.Join(said, ", or ")
}

// lineGrantedTopics is every topic a line's success or failure row gives
// (rpg-project#929, #932), read straight off the document: a creature's own lines
// and its faction's, and every topic a behaviour definition's `outcomes` give
// on entering a state (rpg-project#989), since a topic a state gives counts as
// given.
func lineGrantedTopics(s *SingleRoomSpec) map[string]bool {
	out := map[string]bool{}
	for _, def := range s.BehaviorDefinitions {
		for _, consequences := range def.Outcomes {
			for _, c := range consequences {
				if c.Give != nil && c.Give.Topic != "" {
					out[c.Give.Topic] = true
				}
			}
		}
	}
	add := func(options []OptionSpec) {
		for _, o := range options {
			for _, row := range slices.Concat(o.Success, o.Failure) {
				if row.Give != nil && row.Give.Topic != "" {
					out[row.Give.Topic] = true
				}
			}
		}
	}
	for _, b := range s.Room.Gameplay.MonsterBindings {
		add(b.Options)
	}
	for _, f := range s.Factions {
		add(f.Options)
	}
	return out
}

// grantedTopicsOf is every topic the document gives — a root interaction's
// `outcomes` and a line's success and failure rows — read straight off it: the
// topics a post's `pass` may ask `noted` of.
func grantedTopicsOf(s *SingleRoomSpec) map[string]bool {
	out := lineGrantedTopics(s)
	for _, spec := range s.Interactions {
		for _, cs := range spec.Outcomes {
			for _, c := range cs {
				if c.Give != nil && c.Give.Topic != "" {
					out[c.Give.Topic] = true
				}
			}
		}
	}
	return out
}

// topicGuards refuses a routine waiting on a topic nothing in the document
// gives (rpg-project#929): a `noted.anyone:<topic>` guard in a behavior
// definition's recipe whose topic no outcome and no line's row gives could
// never be true, so the transition could never fire. Refused at the
// definition's recipe; the transition itself is the compiler's to name
// beyond that. A guard still written in the retired `topic:<id>` is refused
// naming the new word (rpg-project#1005), in a recipe or a sequence's body;
// a `heard.claim` or a `noted:<topic>` on a beat that does not carry what
// it reads is refused in the engine's sentence ([encounter.GuardWordsRefusal],
// rpg-project#1000).
func topicGuards(s *SingleRoomSpec, add errSink) {
	granted, values := grantedTopicsOf(s), givenValuesOf(s)
	for _, id := range sortedKeys(s.BehaviorDefinitions) {
		def := s.BehaviorDefinitions[id]
		if def.Body != nil && bytes.Contains(def.Body.raw, []byte(`"`+retiredTopicGuardPrefix)) {
			var body any
			if err := json.Unmarshal(def.Body.raw, &body); err == nil {
				for _, name := range retiredGuardNames(body) {
					add(fmt.Sprintf("behaviorDefinitions.%s.body", id), topicGuardRetired(name))
				}
			}
		}
		if def.Recipe == nil {
			continue
		}
		var recipe machine.Recipe
		if err := json.Unmarshal(def.Recipe.raw, &recipe); err != nil {
			continue
		}
		for _, state := range sortedKeys(recipe.States) {
			for i, t := range recipe.States[state].Transitions {
				at := fmt.Sprintf("behaviorDefinitions.%s.recipe.states.%s.transitions[%d].when", id, state, i)
				// A GUARD THAT READS THE BEAT reads one that carries it
				// (rpg-project#1000), in the engine's own sentence.
				if refusal, leaf := encounter.GuardWordsRefusalAt(t.On, t.When); refusal != "" {
					add(strings.TrimSuffix(at, ".when")+leafPath(leaf), refusal)
				}
				held, retired, valued := guardTopics(t.When)
				for _, name := range retired {
					add(at, topicGuardRetired(name))
				}
				for _, topic := range held {
					if !granted[topic] {
						add(at, fmt.Sprintf("%q is a topic no interaction's `outcomes` and no line's success or failure row gives, "+
							"so nobody could ever note it", topic))
					}
				}
				// A VALUE NOTHING GIVES (rpg-project#1007): a topic given, but
				// never as the value the step waits for.
				for _, v := range valued {
					if granted[v.topic] && !values.givesAs(v.topic, v.as) {
						add(at, errNothingGivesValue("this step", v.topic, v.as))
					}
				}
			}
		}
	}
}

// retiredTopicGuardPrefix is the guard fact a machine asked about anybody
// with before rpg-project#1005. The engine still reads it in a saved run's
// recipe; content may no longer write it.
const retiredTopicGuardPrefix = "topic:"

// topicGuardRetired is the sentence a content guard still written
// `topic:<id>` is refused with.
func topicGuardRetired(name string) string {
	id := strings.TrimPrefix(name, retiredTopicGuardPrefix)
	return fmt.Sprintf("`%s` is now `%s%s`: a machine asks whether the creature has noted %q about anyone with `%s<topic>`",
		name, notedAnyonePrefix, id, id, notedAnyonePrefix)
}

// guardTopics is every topic a guard asks `noted.anyone:<topic>`,
// `noted:<topic>` (rpg-project#1000) or `noted.as:<topic>` (rpg-project#1007)
// of, nested included, every value a `noted.as:<topic>` waits for, and every
// guard fact still written in the retired `topic:<id>`.
func guardTopics(g *machine.Guard) (held, retired []string, valued []topicValue) {
	if g == nil {
		return nil, nil, nil
	}
	if g.Fact != nil {
		name := string(g.Fact.Name)
		if id, ok := strings.CutPrefix(name, string(encounter.MachineNotedAs(""))); ok && id != "" {
			held = append(held, id)
			if as, isWord := g.Fact.Value.(string); isWord && as != "" {
				valued = append(valued, topicValue{topic: id, as: as})
			}
		}
		if id, ok := strings.CutPrefix(name, notedAnyonePrefix); ok && id != "" {
			held = append(held, id)
		}
		if id, ok := strings.CutPrefix(name, string(encounter.MachineNoted(""))); ok && id != "" {
			held = append(held, id)
		}
		if id, ok := strings.CutPrefix(name, retiredTopicGuardPrefix); ok && id != "" {
			retired = append(retired, name)
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
		h, r, v := guardTopics(c)
		held, retired, valued = append(held, h...), append(retired, r...), append(valued, v...)
	}
	return held, retired, valued
}

// topicValue is one value a topic is given with or asked as; the empty value
// is a give with none.
type topicValue struct{ topic, as string }

// topicValues is every value each topic is given with, the document read
// straight off (rpg-project#1007): a topic with a value everywhere it is
// given, or nowhere. A give with none counts under the empty value, and so
// does an arrest or a jail, which give the engine's own topics with none.
type topicValues map[string]map[string]bool

// add records one give of topic as value.
func (v topicValues) add(topic, as string) {
	if v[topic] == nil {
		v[topic] = map[string]bool{}
	}
	v[topic][as] = true
}

// givesAs reports whether some give carries topic with exactly the value
// asked. A nil index gives nothing, and so judges nothing.
func (v topicValues) givesAs(topic, as string) bool { return v[topic][as] }

// givenAt is one give where it is written: the path of the give, its topic and
// its value. An arrest and a jail are gives with no path to carry a value.
type givenAt struct {
	at, topic, as string
}

// givesOf is every give the document writes, in file order, with the path of
// each: the lines' success and failure rows, a behaviour definition's state
// outcomes and a root interaction's outcomes, whose arrest and jail give the
// engine's own topics with none.
func givesOf(s *SingleRoomSpec) []givenAt {
	var out []givenAt
	for _, id := range sortedKeys(s.BehaviorDefinitions) {
		def := s.BehaviorDefinitions[id]
		for _, state := range sortedKeys(def.Outcomes) {
			for i, c := range def.Outcomes[state] {
				if c.Give != nil && c.Give.Topic != "" {
					at := fmt.Sprintf("behaviorDefinitions.%s.outcomes.%s[%d].give", id, state, i)
					out = append(out, givenAt{at: at, topic: c.Give.Topic, as: c.Give.As})
				}
			}
		}
	}
	lines := func(path string, options []OptionSpec) {
		for j, o := range options {
			for _, verdict := range []struct {
				key  string
				rows []AnswerSpec
			}{{"success", o.Success}, {"failure", o.Failure}} {
				for k, row := range verdict.rows {
					if row.Give != nil && row.Give.Topic != "" {
						at := fmt.Sprintf("%s.options[%d].%s[%d].give", path, j, verdict.key, k)
						out = append(out, givenAt{at: at, topic: row.Give.Topic, as: row.Give.As})
					}
				}
			}
		}
	}
	for _, id := range sortedKeys(s.Room.Gameplay.MonsterBindings) {
		lines("room.room.monsterBindings."+id, s.Room.Gameplay.MonsterBindings[id].Options)
	}
	for i, f := range s.Factions {
		lines(fmt.Sprintf("factions[%d]", i), f.Options)
	}
	for _, id := range sortedKeys(s.Interactions) {
		spec := s.Interactions[id]
		for _, state := range sortedKeys(spec.Outcomes) {
			for i, c := range spec.Outcomes[state] {
				switch {
				case c.Give != nil && c.Give.Topic != "":
					at := fmt.Sprintf("interactions.%s.outcomes.%s[%d].give", id, state, i)
					out = append(out, givenAt{at: at, topic: c.Give.Topic, as: c.Give.As})
				case c.Arrest != nil:
					out = append(out, givenAt{topic: string(encounter.TopicArrested)}, givenAt{topic: string(encounter.TopicWanted)})
				case c.Jail != nil:
					out = append(out, givenAt{topic: string(encounter.TopicJailed)})
				}
			}
		}
	}
	return out
}

// givenValuesOf is every value each topic the document gives is given with.
func givenValuesOf(s *SingleRoomSpec) topicValues {
	out := topicValues{}
	for _, g := range givesOf(s) {
		out.add(g.topic, g.as)
	}
	return out
}

// topicValueRefusals refuses a value where the engine would (rpg-project#1007),
// at the `as` of the give that wrote it, in the engine's sentences: a value on
// `arrested`, `wanted` or `jailed`, which are the engine's own and have none;
// and a topic given with a value in one place and with none in another.
func topicValueRefusals(s *SingleRoomSpec, add errSink) {
	gives := givesOf(s)
	values := topicValues{}
	for _, g := range gives {
		values.add(g.topic, g.as)
	}
	for _, g := range gives {
		switch {
		case g.as == "":
		case slices.Contains([]string{string(encounter.TopicArrested), string(encounter.TopicWanted), string(encounter.TopicJailed)}, g.topic):
			add(g.at+".as", fmt.Sprintf("this give gives %q as %q: %q is the engine's own topic and has no value", g.topic, g.as, g.topic))
		case values[g.topic][""]:
			add(g.at+".as", fmt.Sprintf("the topic %q is given as %q in one place and with no value in another: "+
				"a topic has a value everywhere it is given, or nowhere", g.topic, g.as))
		}
	}
}

// errNothingGivesValue is the sentence for a question about a value no give of
// the topic carries, in the engine's words.
func errNothingGivesValue(what, topic, as string) string {
	return fmt.Sprintf("%s asks whether %q is noted as %q, and nothing gives it that value", what, topic, as)
}

// retiredGuardNames is every guard fact name in a decoded native value still
// written in the retired `topic:<id>`.
func retiredGuardNames(node any) []string {
	var out []string
	switch v := node.(type) {
	case map[string]any:
		if fact, ok := v["fact"].(map[string]any); ok {
			if name, ok := fact["name"].(string); ok && strings.HasPrefix(name, retiredTopicGuardPrefix) &&
				len(name) > len(retiredTopicGuardPrefix) {
				out = append(out, name)
			}
		}
		for _, k := range sortedKeys(v) {
			out = append(out, retiredGuardNames(v[k])...)
		}
	case []any:
		for _, child := range v {
			out = append(out, retiredGuardNames(child)...)
		}
	}
	return out
}

// regionIDsOf is every region id the document declares.
func regionIDsOf(s *SingleRoomSpec) map[string]bool {
	out := map[string]bool{}
	for _, r := range s.Room.Gameplay.Regions {
		out[r.ID] = true
	}
	return out
}

// arrestOf judges one authored `arrest:` at its path (rpg-project#925) and
// lowers it: the cell a declared region, the area naming at least one region
// or cell, every region declared, the cells in canonical row runs. The area's
// cells reach the engine in the authored frame a region's cells do, and the
// engine refuses one that is not floor ([encounter.ErrBadArrest]).
func arrestOf(at string, a *ArrestSpec, regions map[string]bool, fail func(at, message string)) (encounter.Arrest, bool) {
	ok := true
	bad := func(where, message string) { ok = false; fail(where, message) }
	switch {
	case a.Cell == "":
		bad(at+".cell", "an arrest names the region the arrested are held in")
	case !regions[a.Cell]:
		bad(at+".cell", fmt.Sprintf("%q is not a region of this site", a.Cell))
	}
	if len(a.Area.Regions) == 0 && len(a.Area.Cells) == 0 {
		bad(at+".area", "an arrest's area names its regions or its cells: no area is chosen for you")
	}
	out := encounter.Arrest{Cell: encounter.RegionID(a.Cell)}
	for i, r := range a.Area.Regions {
		if !regions[r] {
			bad(fmt.Sprintf("%s.area.regions[%d]", at, i), fmt.Sprintf("%q is not a region of this site", r))
			continue
		}
		out.Area.Regions = append(out.Area.Regions, encounter.RegionID(r))
	}
	if len(a.Area.Cells) > 0 {
		for _, c := range expandRows(a.Area.Cells, at+".area.cells", func(where, message string) { bad(where, message) }) {
			out.Area.Cells = append(out.Area.Cells, axialOffset(c, spatial.HexOrientationPointyTop))
		}
	}
	return out, ok
}

// kinds is how many consequence kinds one outcome item names.
func kinds(c OutcomeSpec) int {
	n := 0
	for _, set := range []bool{c.Give != nil, c.Arrest != nil, c.Jail != nil} {
		if set {
			n++
		}
	}
	return n
}
