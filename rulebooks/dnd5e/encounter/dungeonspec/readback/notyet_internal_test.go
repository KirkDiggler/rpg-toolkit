// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package readback

import (
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"
	"gopkg.in/yaml.v3"

	"github.com/KirkDiggler/rpg-toolkit/mind/machine"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
)

// NotYetSuite holds the not-yet-said guard to the dialect: every key a spec
// struct can carry is said or knowingly listed, and an unknown key at every
// walked level is listed at its path.
type NotYetSuite struct{ suite.Suite }

func TestNotYetSuite(t *testing.T) { suite.Run(t, new(NotYetSuite)) }

// specOf is, for each kind the walker checks, the spec struct whose keys it
// stands for.
var specOf = map[string]reflect.Type{
	"root":               reflect.TypeOf(dungeonspec.SingleRoomSpec{}),
	"party":              reflect.TypeOf(dungeonspec.PartySpec{}),
	"room":               reflect.TypeOf(dungeonspec.RoomSource{}),
	"room.room":          reflect.TypeOf(dungeonspec.RoomGameplaySource{}),
	"monsterDeclaration": reflect.TypeOf(dungeonspec.RoomMonsterSource{}),
	"monsterBinding":     reflect.TypeOf(dungeonspec.RoomMonsterBinding{}),
	"faction":            reflect.TypeOf(dungeonspec.FactionSpec{}),
	"disposition":        reflect.TypeOf(dungeonspec.DispositionSpec{}),
	"propBinding":        reflect.TypeOf(dungeonspec.RoomPropBinding{}),
	"doorBinding":        reflect.TypeOf(dungeonspec.RoomDoorBinding{}),
	"behaviorDefinition": reflect.TypeOf(dungeonspec.BehaviorDefinitionSpec{}),
	"interaction":        reflect.TypeOf(dungeonspec.InteractionSpec{}),
	"approach":           reflect.TypeOf(dungeonspec.InteractionApproachSpec{}),
	"outcome":            reflect.TypeOf(dungeonspec.OutcomeSpec{}),
	"arrest":             reflect.TypeOf(dungeonspec.ArrestSpec{}),
	"arrestArea":         reflect.TypeOf(dungeonspec.ArrestAreaSpec{}),
	"recognize":          reflect.TypeOf(dungeonspec.RecognizeSpec{}),
	"option":             reflect.TypeOf(dungeonspec.OptionSpec{}),
	"checkApproach":      reflect.TypeOf(dungeonspec.ApproachSpec{}),
	"advance":            reflect.TypeOf(dungeonspec.AdvanceSpec{}),
	"optionRow":          reflect.TypeOf(dungeonspec.AnswerSpec{}),
	"activity":           reflect.TypeOf(dungeonspec.ActivityCueSpec{}),
	"post":               reflect.TypeOf(dungeonspec.HoldsSpec{}),
	"recipe":             reflect.TypeOf(machine.Recipe{}),
	"state":              reflect.TypeOf(machine.State{}),
	"transition":         reflect.TypeOf(machine.Transition{}),
	"act":                reflect.TypeOf(machine.Act{}),
	"tableRow":           reflect.TypeOf(dungeonspec.AnswerSpec{}),
	"template":           reflect.TypeOf(dungeonspec.TemplateSpec{}),
	"ending":             reflect.TypeOf(dungeonspec.EndingSpec{}),
	"exit":               reflect.TypeOf(dungeonspec.RoomExit{}),
	"intel":              reflect.TypeOf(dungeonspec.IntelSpec{}),
	"region":             reflect.TypeOf(dungeonspec.RoomRegion{}),
}

// acknowledged is, per kind, the keys a spec struct carries that the
// read-back knowingly does not say: present in a file, each is listed under
// not yet said. A key here is a decision, not a gap.
var acknowledged = map[string][]string{
	"root":               {"concealments", "scenarios", "tables"},
	"monsterBinding":     {"table", "on", "states", "actions", "addActions", "dropActions"},
	"faction":            {"behaviors", "behaviorState", "blockingStates", "posts", "table", "on", "states"},
	"behaviorDefinition": {"ref", "parameters", "end", "body"},
	"optionRow":          {"flee", "when", "hold", "attack", "toward", "dash", "away", "search", "use", "with", "as"},
	"recipe":             {"bag"},
	"tableRow":           {"recognize"},
	"template":           {"addActions", "dropActions", "experience"},
}

// keysOf is a struct's authored keys: its yaml tags, else its json tags
// (the machine recipe is authored in its json names), else the lower-cased
// field name for a struct a custom decoder reads.
func keysOf(t reflect.Type) []string {
	var out []string
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		name := ""
		for _, tag := range []string{"yaml", "json"} {
			if v, ok := f.Tag.Lookup(tag); ok {
				name = strings.Split(v, ",")[0]
				break
			}
		}
		if name == "-" {
			continue
		}
		if name == "" {
			name = strings.ToLower(f.Name[:1]) + f.Name[1:]
		}
		out = append(out, name)
	}
	return out
}

// TestEveryKeyIsClassified: a spec struct that gains a key fails here until
// the key is said or acknowledged.
func (s *NotYetSuite) TestEveryKeyIsClassified() {
	for kind, t := range specOf {
		said := map[string]bool{}
		for _, k := range saidKeys[kind] {
			said[k] = true
		}
		for _, k := range acknowledged[kind] {
			said[k] = true
		}
		for _, k := range keysOf(t) {
			s.True(said[k], "%s.%s (on %s) is neither said nor acknowledged as not yet said", kind, k, t)
		}
	}
	for kind := range saidKeys {
		s.Contains(specOf, kind, "the walked kind %s names no spec struct", kind)
	}
}

// TestAnUnknownKeyAtEveryLevelIsListed injects a key the read-back does not
// know at every level the walker visits — by calling the walker on the
// castle's tree directly, since the compiler refuses an unknown key before
// the read-back runs — and finds each one listed at its path.
func (s *NotYetSuite) TestAnUnknownKeyAtEveryLevelIsListed() {
	raw, err := os.ReadFile("../testdata/world-builder-v4-castle.yaml")
	s.Require().NoError(err)
	doc := newDocument(raw)
	levels := []string{
		"", "room", "room.room", "room.room.regions[1]", "room.room.monsterDeclarations[0]",
		"room.room.monsterBindings.gate-1", "room.room.monsterBindings.gate-1.options[0]",
		"room.room.monsterBindings.gate-1.options[1].check[0]", "room.room.monsterBindings.gate-1.options[3].advance",
		"room.room.monsterBindings.gate-1.options[0].success[0]",
		"room.room.monsterBindings.gate-1.options[0].success[0].recognize",
		"room.room.monsterBindings.prisoner-1.options[0].success[0].activity",
		"room.room.monsterBindings.gate-1.posts.alert", "factions[0]", "factions[1].options[0]", "dispositions[0]",
		"room.room.propBindings.altar", "room.room.propBindings.altar.options[0]", "room.room.doorBindings.gate-wall-door",
		"behaviorDefinitions.gate-1-policy", "behaviorDefinitions.gate-1-policy.recipe",
		"behaviorDefinitions.gate-1-policy.recipe.states.alert",
		"behaviorDefinitions.gate-1-policy.recipe.states.alert.transitions[2]",
		"behaviorDefinitions.gate-1-policy.recipe.states.alert.transitions[2].do[0]",
		"behaviorDefinitions.gate-1-policy.table.time[0]", "interactions.front-gate", "interactions.front-gate.approach",
		"interactions.front-gate.recipe", "interactions.front-gate.recipe.states.fresh",
		"interactions.front-gate.recipe.states.fresh.transitions[0]", "interactions.front-gate.outcomes.admitted[0]",
		"interactions.front-gate.outcomes.admitted[0].recognize", "templates.guard", "endings[0]", "exits[0]", "intel[0]",
	}
	for _, at := range levels {
		n := doc.root
		if at != "" {
			n = doc.node(at)
		}
		s.Require().NotNil(n, at)
		s.Require().Equal(yaml.MappingNode, n.Kind, at)
		n.Content = append(n.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "zz-unknown"}, &yaml.Node{Kind: yaml.ScalarNode, Value: "1"})
	}
	b := &builder{doc: doc}
	listed, want := map[string]bool{}, map[string]bool{}
	for _, u := range b.notYetSaid() {
		listed[u.Path] = true
	}
	for _, at := range levels {
		p := "zz-unknown"
		if at != "" {
			p = at + ".zz-unknown"
		}
		want[p] = true
	}
	s.Equal(want, listed, "every injected key is listed at its path, and the castle lists nothing else")
}
