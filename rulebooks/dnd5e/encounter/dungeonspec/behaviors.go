// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec

import (
	"bytes"
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/mind/behavior"
	"github.com/KirkDiggler/rpg-toolkit/mind/machine"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
)

// BehaviorValue retains native component data as a source value. Its codecs
// preserve values rather than materialized graphs or private runtime progress.
type BehaviorValue struct{ raw jsontext.Value }

// UnmarshalYAML retains the authored native value without interpreting it.
func (v *BehaviorValue) UnmarshalYAML(node *yaml.Node) error {
	var value any
	if err := node.Decode(&value); err != nil {
		return err
	}
	raw, err := json.Marshal(value, json.Deterministic(true), json.WithMarshalers(json.MarshalToFunc(func(enc *jsontext.Encoder, number float64) error {
		if math.IsNaN(number) || math.IsInf(number, 0) {
			return fmt.Errorf("native behavior value must be finite")
		}
		literal := strconv.FormatFloat(number, 'g', -1, 64)
		if !strings.ContainsAny(literal, ".eE") {
			literal += ".0"
		}
		return enc.WriteValue(jsontext.Value(literal))
	})))
	if err != nil {
		return err
	}
	v.raw = raw
	return nil
}

// MarshalYAML writes the native value back, not a byte slice or runtime object.
func (v BehaviorValue) MarshalYAML() (any, error) {
	var out any
	if err := json.Unmarshal(v.raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// MarshalJSON carries the same value through source JSON round trips.
func (v BehaviorValue) MarshalJSON() ([]byte, error) { return bytes.Clone(v.raw), nil }

// UnmarshalJSON validates and retains source JSON without sharing the buffer.
func (v *BehaviorValue) UnmarshalJSON(raw []byte) error {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return err
	}
	v.raw = bytes.Clone(raw)
	return nil
}

// BehaviorDefinitionSpec is exactly one concrete recipe/source-table definition
// or one native finite sequence definition. A concrete recipe owns its Ref; a sequence owns
// Ref/Parameters/End/Body. Attachment IDs and progress are never definition data.
type BehaviorDefinitionSpec struct {
	Recipe     *BehaviorValue       `yaml:"recipe,omitempty" json:"recipe,omitempty"`
	Table      *BehaviorValue       `yaml:"table,omitempty" json:"table,omitempty"`
	table      TableSpec            // derived source grammar; never serialized
	Ref        string               `yaml:"ref,omitempty" json:"ref,omitempty"`
	Parameters *BehaviorValue       `yaml:"parameters,omitempty" json:"parameters,omitempty"`
	End        behavior.SequenceEnd `yaml:"end,omitempty" json:"end,omitempty"`
	Body       *BehaviorValue       `yaml:"body,omitempty" json:"body,omitempty"`
}

// BehaviorBindingSpec supplies an actor-local attachment ID, document-local
// definition key and explicit sequence values. Pointers retain absent versus
// explicitly empty items/with so unsupported concrete-binding inputs refuse.
type BehaviorBindingSpec struct {
	ID    string         `yaml:"id" json:"id"`
	Use   string         `yaml:"use" json:"use"`
	Items *[]RoomCell    `yaml:"items,omitempty" json:"items,omitempty"`
	With  *BehaviorValue `yaml:"with,omitempty" json:"with,omitempty"`
}

type behaviorLowering struct {
	Input          *encounter.BehaviorInput
	BlockingStates []string
	Posts          map[string]encounter.Post
}

type behaviorSourceInput struct {
	Definitions map[string]BehaviorDefinitionSpec
	Bindings    *[]BehaviorBindingSpec
	Body        string
	Blocking    *[]string
	Posts       map[string]HoldsSpec
	Path        string
	Key         string
	Cells       singleRoomCells
	Doors       map[string]bool
	Vocabulary  machine.Vocabulary

	// Topics is every topic a root interaction's outcomes grant
	// (rpg-project#928): what a post's `pass` may recognize on.
	Topics map[string]bool
}

func behaviorSourceError(path, message string) error {
	return &ValidationError{Errors: []FieldError{{Path: path, Message: message}}}
}

func decodeBehaviorValue(value *BehaviorValue, out any) error {
	if value == nil {
		return fmt.Errorf("required native value is absent")
	}
	return json.Unmarshal(value.raw, out, json.RejectUnknownMembers(true))
}

func checkBehaviorDefinition(key string, definition BehaviorDefinitionSpec) error {
	path := "behaviorDefinitions." + key
	if key == "" {
		return behaviorSourceError("behaviorDefinitions", "a definition needs a name")
	}
	if definition.Recipe != nil {
		if definition.Body != nil || definition.Ref != "" || definition.End != "" || definition.Parameters != nil {
			return behaviorSourceError(path, "choose concrete recipe/table or sequence ref/parameters/end/body, not both")
		}
		var recipe machine.Recipe
		if err := decodeBehaviorValue(definition.Recipe, &recipe); err != nil {
			return behaviorSourceError(path+".recipe", err.Error())
		}
		if err := recipe.Ref.IsValid(); err != nil {
			return behaviorSourceError(path+".recipe.ref", err.Error())
		}
		return nil
	}
	if definition.Table != nil {
		return behaviorSourceError(path+".table", "a concrete table requires its recipe")
	}
	if _, err := core.ParseString(definition.Ref); err != nil {
		return behaviorSourceError(path+".ref", err.Error())
	}
	if definition.End != behavior.SequenceRepeat && definition.End != behavior.SequenceStop {
		return behaviorSourceError(path+".end", "a sequence explicitly ends as repeat or stop")
	}
	if definition.Body == nil {
		return behaviorSourceError(path+".body", "a sequence needs its native recipe/table body")
	}
	return nil
}

func bindBehaviorSource(in behaviorSourceInput) (behaviorLowering, error) {
	if in.Bindings == nil && in.Body == "" && in.Blocking == nil && in.Posts == nil {
		return behaviorLowering{}, nil
	}
	out := behaviorLowering{Input: &encounter.BehaviorInput{BodyInstance: in.Body}}
	stateNames := map[string]map[string][]string{}
	if in.Bindings != nil {
		for index, binding := range *in.Bindings {
			path := fmt.Sprintf("%s.behaviors[%d]", in.Path, index)
			if binding.ID == "" {
				return behaviorLowering{}, behaviorSourceError(path+".id", "attachment ID is required")
			}
			if _, duplicate := stateNames[binding.ID]; duplicate {
				return behaviorLowering{}, behaviorSourceError(path+".id", "duplicate attachment ID on this actor")
			}
			definition, known := in.Definitions[binding.Use]
			if !known {
				return behaviorLowering{}, behaviorSourceError(path+".use", fmt.Sprintf("definition %q is not declared in this document", binding.Use))
			}
			if err := checkBehaviorDefinition(binding.Use, definition); err != nil {
				return behaviorLowering{}, err
			}
			bound := encounter.BoundBehavior{}
			names := map[string][]string{}
			if definition.Recipe != nil {
				if binding.Items != nil || binding.With != nil {
					return behaviorLowering{}, behaviorSourceError(path, "items/with are not supported on a concrete definition")
				}
				bound.Instance.ID = binding.ID
				if err := decodeBehaviorValue(definition.Recipe, &bound.Instance.Recipe); err != nil {
					return behaviorLowering{}, behaviorSourceError(path+".use", err.Error())
				}
				bound.Instance.Table = tableOf(definition.table, encounter.HexesArePointyTop())
				for state := range bound.Instance.Recipe.States {
					names[state] = []string{state}
				}
			} else {
				if binding.Items == nil || len(*binding.Items) == 0 {
					return behaviorLowering{}, behaviorSourceError(path+".items", "a sequence requires a nonempty finite item list")
				}
				items := make([]jsontext.Value, len(*binding.Items))
				for i, cell := range *binding.Items {
					answer := in.Cells.cellAt(authoredCell{hex: &cell})
					if answer.refusal != "" {
						return behaviorLowering{}, behaviorSourceError(fmt.Sprintf("%s.items[%d]", path, i), answer.refusal)
					}
					raw, err := json.Marshal(answer.cell)
					if err != nil {
						return behaviorLowering{}, behaviorSourceError(path+".items", err.Error())
					}
					items[i] = raw
				}
				ref, err := core.ParseString(definition.Ref)
				if err != nil {
					return behaviorLowering{}, behaviorSourceError(path+".use", err.Error())
				}
				var values map[string]any
				if binding.With != nil {
					// JSON is YAML's subset: this reads integer tokens as int,
					// decimal tokens as floats, without coercing scalar kinds.
					if err := yaml.Unmarshal(binding.With.raw, &values); err != nil {
						return behaviorLowering{}, behaviorSourceError(path+".with", err.Error())
					}
				}
				var schema []behavior.ParameterSpec
				if definition.Parameters != nil {
					if err := decodeBehaviorValue(definition.Parameters, &schema); err != nil {
						return behaviorLowering{}, behaviorSourceError("behaviorDefinitions."+binding.Use+".parameters", err.Error())
					}
				}
				expanded, err := behavior.ExpandSequence(&behavior.ExpandSequenceInput{
					ID: binding.ID, Definition: behavior.SequenceDefinition{Ref: *ref, Parameters: schema, End: definition.End, Body: bytes.Clone(definition.Body.raw)},
					Items: items, Parameters: values, Vocabulary: in.Vocabulary,
				})
				if err != nil {
					return behaviorLowering{}, behaviorSourceError(path, fmt.Sprintf("use %q: %v", binding.Use, err))
				}
				bound.Instance, bound.Completions = expanded.Instance, expanded.Completions
				for state, source := range expanded.StateSources {
					names[source.Name] = append(names[source.Name], state)
				}
				for name := range names {
					slices.Sort(names[name])
				}
			}
			for key, rows := range bound.Instance.Table {
				if err := behaviorCells(rows, in.Cells); err != nil {
					return behaviorLowering{}, behaviorSourceError(path+".use", fmt.Sprintf("definition %q table %q: %v", binding.Use, key, err))
				}
			}
			if _, err := encounter.CheckBehaviors(&encounter.CheckBehaviorsInput{Behaviors: encounter.BehaviorInput{Instances: []encounter.BoundBehavior{bound}}}); err != nil {
				return behaviorLowering{}, behaviorSourceError(path+".use", fmt.Sprintf("definition %q: %v", binding.Use, err))
			}
			stateNames[binding.ID] = names
			out.Input.Instances = append(out.Input.Instances, bound)
		}
	}
	body := in.Body
	if body == "" && len(out.Input.Instances) == 1 {
		body = out.Input.Instances[0].Instance.ID
	}
	if body != "" {
		if _, known := stateNames[body]; !known {
			return behaviorLowering{}, behaviorSourceError(in.Path+".behaviorState", "must name an attached behavior")
		}
	}
	resolveState := func(name string) ([]string, error) {
		if body == "" {
			return nil, behaviorSourceError(in.Path+".behaviorState", "state-bound blocking/posts require an unambiguous body instance")
		}
		states := stateNames[body][name]
		if len(states) == 0 {
			return nil, behaviorSourceError(in.Path+".behaviorState", fmt.Sprintf("body %q has no source state %q", body, name))
		}
		return states, nil
	}
	if in.Blocking != nil {
		for _, name := range *in.Blocking {
			states, err := resolveState(name)
			if err != nil {
				return behaviorLowering{}, err
			}
			out.BlockingStates = append(out.BlockingStates, states...)
		}
	}
	for _, name := range sortedKeys(in.Posts) {
		post := in.Posts[name]
		states, err := resolveState(name)
		if err != nil {
			return behaviorLowering{}, err
		}
		path := in.Path + ".posts." + name
		if !in.Doors[post.Door] {
			return behaviorLowering{}, behaviorSourceError(path+".door", "must name a door in this site")
		}
		if at, refusal := passRefusal(path+".pass", post.Pass, in.Topics); refusal != "" {
			return behaviorLowering{}, behaviorSourceError(at, refusal)
		}
		lowered := encounter.Post{Door: encounter.DoorID(in.Key + "/" + post.Door), Against: encounter.PostDirection(post.Against), Pass: passOf(post.Pass)}
		if post.Against != "" && post.Against != "both" && post.Against != "entering" && post.Against != "leaving" {
			return behaviorLowering{}, behaviorSourceError(path+".against", "must be both, entering or leaving")
		}
		for i, cell := range post.At {
			answer := in.Cells.cellAt(authoredCell{hex: &cell})
			if answer.refusal != "" {
				return behaviorLowering{}, behaviorSourceError(fmt.Sprintf("%s.at[%d]", path, i), answer.refusal)
			}
			lowered.Cells = append(lowered.Cells, answer.cell)
		}
		if out.Posts == nil {
			out.Posts = map[string]encounter.Post{}
		}
		for _, state := range states {
			out.Posts[state] = lowered
		}
	}
	if _, err := encounter.CheckBehaviors(&encounter.CheckBehaviorsInput{Behaviors: *out.Input, BlockingStates: out.BlockingStates, Posts: out.Posts}); err != nil {
		return behaviorLowering{}, behaviorSourceError(in.Path+".behaviors", err.Error())
	}
	return out, nil
}

func behaviorCells(rows []behavior.Answer, frame singleRoomCells) error {
	for _, row := range rows {
		for _, selector := range []*behavior.Selector{row.Toward, row.Dash, row.Away, row.Attack} {
			if selector == nil {
				continue
			}
			if selector.At != nil && !frame.onFloor(*selector.At) {
				return fmt.Errorf("target %v is not floor", *selector.At)
			}
			for _, cell := range selector.Path {
				if !frame.onFloor(cell) {
					return fmt.Errorf("route cell %v is not floor", cell)
				}
			}
		}
		if err := behaviorCells(row.Rows, frame); err != nil {
			return err
		}
	}
	return nil
}

func roomBehaviorInput(s *SingleRoomSpec, id string) (behaviorLowering, error) {
	binding := s.Room.Gameplay.MonsterBindings[id]
	uses, body, blocks, posts := binding.Behaviors, binding.BehaviorState, binding.BlockingStates, binding.Posts
	if uses == nil {
		for _, faction := range s.Factions {
			if faction.ID != sideOf(binding.Faction) {
				continue
			}
			uses = faction.Behaviors
			if body == "" {
				body = faction.BehaviorState
			}
			if blocks == nil {
				blocks = faction.BlockingStates
			}
			if posts == nil {
				posts = faction.Posts
			}
			break
		}
	}
	vocab, err := roomBehaviorVocabulary(s)
	if err != nil {
		return behaviorLowering{}, err
	}
	return bindBehaviorSource(behaviorSourceInput{Definitions: s.BehaviorDefinitions, Bindings: uses, Body: body,
		Blocking: blocks, Posts: posts, Path: "room.room.monsterBindings." + id, Key: s.Key,
		Cells: roomCells(&s.Room.Gameplay), Doors: roomDoorIDs(&s.Room.Gameplay), Vocabulary: vocab,
		Topics: grantedTopicsOf(s)})
}

func roomBehaviorVocabulary(s *SingleRoomSpec) (machine.Vocabulary, error) {
	facts := make([]encounter.FactID, 0)
	for fact := range roomFacts(s) {
		facts = append(facts, encounter.FactID(fact))
	}
	var clocks []string
	for prop := range s.Room.Gameplay.PropBindings {
		clocks = append(clocks, "bell:"+prop)
	}
	// Literal named clocks in native definitions remain explicit declarations.
	// This is vocabulary registration, not reference substitution or execution.
	for _, definition := range s.BehaviorDefinitions {
		for _, value := range []*BehaviorValue{definition.Recipe, definition.Body} {
			if value == nil {
				continue
			}
			var node any
			if err := json.Unmarshal(value.raw, &node); err != nil {
				return machine.Vocabulary{}, err
			}
			clocks = append(clocks, behaviorClockNames(node)...)
		}
	}
	// A GUARD MAY READ A TOPIC HELD (rpg-project#929), `topic:<id>`, for any
	// topic the document grants.
	var topics []encounter.TopicID
	for _, topic := range sortedKeys(grantedTopicsOf(s)) {
		topics = append(topics, encounter.TopicID(topic))
	}
	vocab, err := encounter.BehaviorVocabulary(&encounter.BehaviorVocabularyInput{Facts: facts, Clocks: clocks, Topics: topics})
	if err != nil {
		return machine.Vocabulary{}, err
	}
	return vocab.Vocabulary, nil
}

func behaviorSourceValues(s *SingleRoomSpec, add errSink) {
	report := func(err error) {
		if err == nil {
			return
		}
		if detailed, ok := err.(*ValidationError); ok {
			for _, field := range detailed.Errors {
				add(field.Path, field.Message)
			}
			return
		}
		add("behaviorDefinitions", err.Error())
	}
	for _, name := range sortedKeys(s.BehaviorDefinitions) {
		report(checkBehaviorDefinition(name, s.BehaviorDefinitions[name]))
	}
	vocab, err := roomBehaviorVocabulary(s)
	if err != nil {
		report(err)
		return
	}
	for i, faction := range s.Factions {
		path := fmt.Sprintf("factions[%d]", i)
		if faction.On != nil {
			add(path+".on", "use explicit behaviors; old actor tables are not imported")
		}
		if faction.Table != "" {
			add(path+".table", "use explicit behaviors; old actor tables are not imported")
		}
		if faction.States != nil {
			add(path+".states", "use a concrete behavior definition and behaviorState; old machines are not imported")
		}
		_, err := bindBehaviorSource(behaviorSourceInput{Definitions: s.BehaviorDefinitions, Bindings: faction.Behaviors,
			Body: faction.BehaviorState, Blocking: faction.BlockingStates, Posts: faction.Posts,
			Path: path, Key: s.Key, Cells: roomCells(&s.Room.Gameplay), Doors: roomDoorIDs(&s.Room.Gameplay), Vocabulary: vocab,
			Topics: grantedTopicsOf(s)})
		report(err)
	}
	for _, actor := range s.Room.Gameplay.MonsterDeclarations {
		binding := s.Room.Gameplay.MonsterBindings[actor.ID]
		path := "room.room.monsterBindings." + actor.ID
		if binding.On != nil {
			add(path+".on", "use explicit behaviors; old actor tables are not imported")
		}
		if binding.Table != "" {
			add(path+".table", "use explicit behaviors; old actor tables are not imported")
		}
		if binding.States != nil {
			add(path+".states", "use a concrete behavior definition and behaviorState; old machines are not imported")
		}
		_, err := roomBehaviorInput(s, actor.ID)
		report(err)
	}
}

func behaviorBindingsShape(parent *yaml.Node, path string, add errSink) {
	optionalReference(parent, "behaviorState", path, add)
	optionalNode(parent, "blockingStates", path, add)
	if posts := optionalNode(parent, "posts", path, add); posts != nil && posts.Kind == yaml.MappingNode {
		for i := 0; i+1 < len(posts.Content); i += 2 {
			at := path + ".posts." + posts.Content[i].Value
			post := resolveNode(posts.Content[i+1])
			if post == nil || post.Kind != yaml.MappingNode {
				add(at, "expected a post binding")
				continue
			}
			optionalReference(post, "door", at, add)
			if cells := optionalNode(post, "at", at, add); cells != nil && cells.Kind == yaml.SequenceNode {
				for j, cell := range cells.Content {
					cellShape(cell, fmt.Sprintf("%s.at[%d]", at, j), add)
				}
			}
		}
	}
	bindings := optionalNode(parent, "behaviors", path, add)
	if bindings == nil {
		return
	}
	if bindings.Kind != yaml.SequenceNode {
		add(path+".behaviors", "expected an ordered list of bindings")
		return
	}
	for i, binding := range bindings.Content {
		at := fmt.Sprintf("%s.behaviors[%d]", path, i)
		binding = resolveNode(binding)
		if binding == nil || binding.Kind != yaml.MappingNode {
			add(at, "expected a behavior binding")
			continue
		}
		optionalReference(binding, "id", at, add)
		optionalReference(binding, "use", at, add)
		optionalNode(binding, "with", at, add)
		if items := optionalNode(binding, "items", at, add); items != nil && items.Kind == yaml.SequenceNode {
			for j, cell := range items.Content {
				cellShape(cell, fmt.Sprintf("%s.items[%d]", at, j), add)
			}
		}
	}
}

func behaviorClockNames(node any) []string {
	var names []string
	switch value := node.(type) {
	case string:
		if strings.HasPrefix(value, "clock:") && len(value) > len("clock:") {
			names = append(names, strings.TrimPrefix(value, "clock:"))
		}
	case []any:
		for _, child := range value {
			names = append(names, behaviorClockNames(child)...)
		}
	case map[string]any:
		for _, child := range value {
			names = append(names, behaviorClockNames(child)...)
		}
	}
	return names
}
