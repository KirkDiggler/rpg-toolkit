// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

import (
	"context"
	"encoding/json"
	jsonv2 "encoding/json/v2"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"
	"gopkg.in/yaml.v3"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

type behaviorSourceSuite struct {
	suite.Suite
	raw []byte
}

func TestBehaviorSourceSuite(t *testing.T) { suite.Run(t, new(behaviorSourceSuite)) }

func (s *behaviorSourceSuite) SetupTest() {
	var err error
	s.raw, err = os.ReadFile("testdata/behavior-patrol-consumer.yaml")
	s.Require().NoError(err)
}

func (s *behaviorSourceSuite) source() *dungeonspec.SingleRoomSpec {
	decoded, err := dungeonspec.DecodeSingleRoom(dungeonspec.SingleRoomDecodeInput{Source: s.raw})
	s.Require().NoError(err)
	return decoded.Spec
}

func (s *behaviorSourceSuite) TestBindingsPreserveDefinitionAndAttachmentIdentity() {
	compiled, err := dungeonspec.Load(s.raw)
	s.Require().NoError(err)
	s.Require().Len(compiled.Monsters, 2)
	west, court := compiled.Monsters[0], compiled.Monsters[1]
	s.Equal("west", west.MemberID)
	s.Equal("court", court.MemberID)
	s.Require().NotNil(west.Behaviors)
	s.Require().NotNil(court.Behaviors)
	w, c := west.Behaviors.Instances[1], court.Behaviors.Instances[1]
	s.Equal("duty", w.Instance.ID)
	s.Equal("duty", c.Instance.ID)
	s.Equal("example:behaviors:patrol", w.Instance.Recipe.Ref.String())
	s.Equal(w.Instance.Recipe.Ref, c.Instance.Recipe.Ref)
	s.Contains(c.Instance.Recipe.States, "item/2/walk")
	s.NotContains(w.Instance.Recipe.States, "item/2/walk")
	s.Equal(spatial.Position{X: 6, Y: 1}, *w.Instance.Table["item/0/move"][0].Toward.At)
	s.Equal(spatial.Position{X: 14}, *c.Instance.Table["item/0/move"][0].Toward.At)
}

func (s *behaviorSourceSuite) TestNotificationSelectionsRefuseBeforeSourceAcceptance() {
	for _, on := range []string{"clock:world", "clock:fight", "heard", "seen", "deed", "fact"} {
		for _, form := range []string{"sequence", "concrete"} {
			s.Run(form+"/"+on, func() {
				from, to := "{on: time, do: [{pick: move}]}", "{on: '"+on+"', do: [{pick: move}]}"
				if form == "concrete" {
					from, to = "{on: time, do: [{pick: look}]}", "{on: '"+on+"', do: [{pick: look}]}"
				}
				bad := strings.Replace(string(s.raw), from, to, 1)
				s.Require().NotEqual(string(s.raw), bad)
				_, err := dungeonspec.Load([]byte(bad))
				s.ErrorIs(err, dungeonspec.ErrBadSpec)
			})
		}
	}
	for _, on := range []string{"time", "turn", "barred"} {
		s.Run("authorized/"+on, func() {
			raw := strings.Replace(string(s.raw), "{on: time, do: [{pick: move}]}", "{on: "+on+", do: [{pick: move}]}", 1)
			_, err := dungeonspec.Load([]byte(raw))
			s.NoError(err, "world/turn opportunities and the explicitly authorized barred reaction retain selection")
		})
	}
}

func (s *behaviorSourceSuite) TestConcreteDefinitionReusesSourceBundles() {
	raw := "bundles:\n  inspect:\n    params: {target: null}\n    rows: [{toward: {at: $target}}]\n" + string(s.raw)
	raw = strings.Replace(raw, "- {weight: 1, toward: {at: {q: 2, r: 2}}}", "- {use: inspect, with: {target: {q: 2, r: 2}}}", 1)
	decoded, err := dungeonspec.DecodeSingleRoom(dungeonspec.SingleRoomDecodeInput{Source: []byte(raw)})
	s.Require().NoError(err)
	compiled, err := dungeonspec.CompileSingleRoom(dungeonspec.CompileSingleRoomInput{Spec: decoded.Spec})
	s.Require().NoError(err)
	row := compiled.Monsters[0].Behaviors.Instances[0].Instance.Table["look"][0]
	s.Equal("inspect", row.From)
	s.Equal(spatial.Position{X: 2, Y: 2}, *row.Rows[0].Toward.At)
	encoded, err := yaml.Marshal(decoded.Spec)
	s.Require().NoError(err)
	s.Contains(string(encoded), "use: inspect", "source keeps the reusable bundle reference")
}

func (s *behaviorSourceSuite) TestConcreteBundleCompilationIsDeterministic() {
	raw := "bundles:\n  inspect:\n    params: {target: null}\n    rows: [{toward: {at: $target}}]\n" + string(s.raw)
	raw = strings.Replace(raw, "- {weight: 1, toward: {at: {q: 2, r: 2}}}", "- {use: inspect, with: {target: {q: 2, r: 2}}}", 1)
	var want string
	for i := 0; i < 48; i++ {
		compiled, err := dungeonspec.Load([]byte(raw))
		s.Require().NoError(err)
		encoded, err := json.Marshal(compiled.Monsters)
		s.Require().NoError(err)
		if i == 0 {
			want = string(encoded)
		} else {
			s.Require().Equal(want, string(encoded), "same source must not change parameter descriptors or configuration identity")
		}
	}
}

func (s *behaviorSourceSuite) TestDuplicateAttachmentIsLocalToActor() {
	spec := s.source()
	west := spec.Room.Gameplay.MonsterBindings["west"]
	(*west.Behaviors)[1].ID = "investigate"
	_, err := dungeonspec.CompileSingleRoom(dungeonspec.CompileSingleRoomInput{Spec: spec})
	s.Require().Error(err)
	s.Contains(err.Error(), "room.room.monsterBindings.west.behaviors[1].id")
	s.Contains(err.Error(), "duplicate attachment")
	_, err = dungeonspec.Load(s.raw)
	s.Require().NoError(err, "same local duty ID on two actors is not a duplicate")
}

func (s *behaviorSourceSuite) TestCellItemsAreLoweredOnce() {
	compiled, err := dungeonspec.Load(s.raw)
	s.Require().NoError(err)
	at := compiled.Monsters[0].Behaviors.Instances[1].Instance.Table["item/0/move"][0].Toward.At
	s.Equal(spatial.Position{X: 6, Y: 1}, *at, "axial row one must not receive an offset conversion")
	bad := strings.Replace(string(s.raw), "items: [{q: 6, r: 1}", "items: [{q: 60, r: 1}", 1)
	_, err = dungeonspec.Load([]byte(bad))
	s.Require().Error(err)
	s.Contains(err.Error(), "room.room.monsterBindings.west.behaviors[1].items[0]")
	s.Contains(err.Error(), "not floor")
}

func (s *behaviorSourceSuite) TestBindingsCannotAliasDefinitionOrEachOther() {
	spec := s.source()
	before, err := yaml.Marshal(spec)
	s.Require().NoError(err)
	compiled, err := dungeonspec.CompileSingleRoom(dungeonspec.CompileSingleRoomInput{Spec: spec})
	s.Require().NoError(err)
	compiled.Monsters[0].Behaviors.Instances[1].Instance.Table["item/0/move"][0].Toward.At.X = 99
	compiled.Monsters[0].Behaviors.Instances[0].Instance.Table["look"][0].Toward.At.X = 99
	s.Equal(float64(2), compiled.Monsters[1].Behaviors.Instances[0].Instance.Table["look"][0].Toward.At.X)
	after, err := yaml.Marshal(spec)
	s.Require().NoError(err)
	s.Equal(string(before), string(after), "compilation and output mutation leave source untouched")
	again, err := dungeonspec.CompileSingleRoom(dungeonspec.CompileSingleRoomInput{Spec: spec})
	s.Require().NoError(err)
	s.Equal(float64(6), again.Monsters[0].Behaviors.Instances[1].Instance.Table["item/0/move"][0].Toward.At.X)
}

func (s *behaviorSourceSuite) TestSourceRoundTripKeepsUsesAndValues() {
	for _, kind := range []string{"yaml", "json"} {
		s.Run(kind, func() {
			spec := s.source()
			west, court := spec.Room.Gameplay.MonsterBindings["west"], spec.Room.Gameplay.MonsterBindings["court"]
			west.Behaviors = nil // inherit, not explicit empty
			empty := []dungeonspec.BehaviorBindingSpec{}
			court.Behaviors = &empty
			spec.Room.Gameplay.MonsterBindings["west"], spec.Room.Gameplay.MonsterBindings["court"] = west, court
			var encoded []byte
			var err error
			var restored dungeonspec.SingleRoomSpec
			if kind == "yaml" {
				encoded, err = yaml.Marshal(spec)
			} else {
				encoded, err = json.Marshal(spec)
			}
			s.Require().NoError(err)
			if kind == "yaml" {
				err = yaml.Unmarshal(encoded, &restored)
			} else {
				err = json.Unmarshal(encoded, &restored)
			}
			s.Require().NoError(err)
			s.Nil(restored.Room.Gameplay.MonsterBindings["west"].Behaviors)
			s.Require().NotNil(restored.Room.Gameplay.MonsterBindings["court"].Behaviors)
			s.Empty(*restored.Room.Gameplay.MonsterBindings["court"].Behaviors)
			s.Equal("investigate", (*restored.Factions[0].Behaviors)[0].Use)
			compiled, err := dungeonspec.CompileSingleRoom(dungeonspec.CompileSingleRoomInput{Spec: &restored})
			s.Require().NoError(err)
			s.Len(compiled.Monsters[0].Behaviors.Instances, 1)
			s.Empty(compiled.Monsters[1].Behaviors.Instances)
			s.NotContains(string(encoded), "item/0/walk")
			s.NotContains(string(encoded), "progress")
		})
	}
	encoded, err := json.Marshal(s.source())
	s.Require().NoError(err)
	var restored dungeonspec.SingleRoomSpec
	s.Require().NoError(json.Unmarshal(encoded, &restored))
	_, err = dungeonspec.CompileSingleRoom(dungeonspec.CompileSingleRoomInput{Spec: &restored})
	s.Require().NoError(err, "source JSON retains integer with values rather than decoding them as float inputs")
}

func (s *behaviorSourceSuite) TestBodyBindingNamesSourceStatesNotPrivateDutyStates() {
	spec := s.source()
	var body dungeonspec.BehaviorDefinitionSpec
	s.Require().NoError(json.Unmarshal([]byte(`{"recipe":{"ref":"example:behaviors:body","initial":"posted","states":{"posted":{},"open":{}}}}`), &body))
	spec.BehaviorDefinitions["body"] = body
	west := spec.Room.Gameplay.MonsterBindings["west"]
	bindings := append([]dungeonspec.BehaviorBindingSpec{{ID: "body", Use: "body"}}, (*west.Behaviors)...)
	west.Behaviors, west.BehaviorState = &bindings, "body"
	blocking := []string{"posted"}
	west.BlockingStates = &blocking
	spec.Room.Gameplay.MonsterBindings["west"] = west
	compiled, err := dungeonspec.CompileSingleRoom(dungeonspec.CompileSingleRoomInput{Spec: spec})
	s.Require().NoError(err)
	s.Equal("body", compiled.Monsters[0].Behaviors.BodyInstance)
	s.Equal([]string{"posted"}, compiled.Monsters[0].BlockingStates)
	west.BehaviorState = "duty"
	spec.Room.Gameplay.MonsterBindings["west"] = west
	_, err = dungeonspec.CompileSingleRoom(dungeonspec.CompileSingleRoomInput{Spec: spec})
	s.Require().Error(err)
	s.Contains(err.Error(), "room.room.monsterBindings.west.behaviorState")
	s.Contains(err.Error(), "posted")
	west.BehaviorState = ""
	spec.Room.Gameplay.MonsterBindings["west"] = west
	_, err = dungeonspec.CompileSingleRoom(dungeonspec.CompileSingleRoomInput{Spec: spec})
	s.Require().Error(err)
	s.Contains(err.Error(), "unambiguous body")
}

func (s *behaviorSourceSuite) TestScalarAbsenceFalseZeroAndEmptyStringSurviveSource() {
	raw := strings.Replace(string(s.raw), "    end: repeat", "      - {name: enabled, kind: bool, required: true}\n      - {name: count, kind: int, required: true}\n      - {name: label, kind: string, required: true}\n    end: repeat", 1)
	raw = strings.ReplaceAll(raw, "with: {weight: 1}", "with: {weight: 1, enabled: false, count: 0, label: ''}")
	decoded, err := dungeonspec.DecodeSingleRoom(dungeonspec.SingleRoomDecodeInput{Source: []byte(raw)})
	s.Require().NoError(err)
	encoded, err := json.Marshal(decoded.Spec)
	s.Require().NoError(err)
	var restored dungeonspec.SingleRoomSpec
	s.Require().NoError(json.Unmarshal(encoded, &restored))
	_, err = dungeonspec.CompileSingleRoom(dungeonspec.CompileSingleRoomInput{Spec: &restored})
	s.Require().NoError(err, "false, zero and empty remain supplied, not missing required values")
	yamlSource, err := yaml.Marshal(&restored)
	s.Require().NoError(err)
	_, err = dungeonspec.Load(yamlSource)
	s.Require().NoError(err)
}

func (s *behaviorSourceSuite) TestOldActorPolicyFieldsRefuseInsteadOfBeingIgnored() {
	for _, old := range []string{"on: {time: [{hold: {}}]}", "table: old", "states: {initial: old, old: {}}"} {
		raw := strings.Replace(string(s.raw), "      west:\n", "      west:\n        "+old+"\n", 1)
		_, err := dungeonspec.Load([]byte(raw))
		s.Require().Error(err)
		s.Contains(err.Error(), "room.room.monsterBindings.west.")
		s.Contains(err.Error(), "not imported")
	}
}

func (s *behaviorSourceSuite) TestBindingRefusalsNameTheOriginalSource() {
	for _, tc := range []struct{ name, old, replacement, path string }{
		{"unknown use", "use: patrol", "use: absent", "room.room.monsterBindings.west.behaviors[1].use"},
		{"wrong scalar", "with: {weight: 1}", "with: {weight: nope}", "room.room.monsterBindings.west.behaviors[1]"},
		{"decimal is not integer", "with: {weight: 1}", "with: {weight: 1.0}", "room.room.monsterBindings.west.behaviors[1]"},
		{"missing scalar", "with: {weight: 1}", "with: {}", "room.room.monsterBindings.west.behaviors[1]"},
		{"fractional coordinate", "{q: 6, r: 1}", "{q: 6.5, r: 1}", "room.room.monsterBindings.west.behaviors[1].items[0].q"},
		{"fractional post coordinate", "      west:\n", "      west:\n        behaviorState: investigate\n        posts: {idle: {door: absent, at: [{q: 1.5, r: 1}]}}\n", "room.room.monsterBindings.west.posts.idle.at[0].q"},
		{"missing coordinate", "{q: 6, r: 1}", "{q: 6}", "room.room.monsterBindings.west.behaviors[1].items[0].r"},
		{"concrete with", "{id: investigate, use: investigate}", "{id: investigate, use: investigate, with: {}}", "behaviors[0]"},
		{"shared ID", "  investigate:\n    recipe:", "  investigate:\n    id: shared\n    recipe:", "behaviorDefinitions.investigate.id"},
		{"runtime progress", "            use: patrol", "            progress: {}\n            use: patrol", "room.room.monsterBindings.west.behaviors[1].progress"},
		{"unknown recipe field", "        initial: walk", "        initial: walk\n        madeUp: true", "room.room.monsterBindings.west.behaviors[1]"},
	} {
		s.Run(tc.name, func() {
			s.Require().Contains(string(s.raw), tc.old)
			_, err := dungeonspec.Load([]byte(strings.Replace(string(s.raw), tc.old, tc.replacement, 1)))
			s.Require().Error(err)
			s.Contains(err.Error(), tc.path)
		})
	}
}

type behaviorSourceSheets struct{}

func (behaviorSourceSheets) Sheets(ids []encounter.MemberID) (map[encounter.MemberID]encounter.SheetFacts, error) {
	out := map[encounter.MemberID]encounter.SheetFacts{}
	for _, id := range ids {
		speed := 10
		if id == "hero" {
			speed = 5
		}
		out[id] = encounter.SheetFacts{SpeedFeet: speed, PassivePerception: 10}
	}
	return out, nil
}

type behaviorSourceDie struct{ calls int }

func (d *behaviorSourceDie) Roll(_ context.Context, sides int) (int, error) {
	if sides != 100 || d.calls >= 6 {
		return 0, fmt.Errorf("unexpected d%d at call %d", sides, d.calls+1)
	}
	d.calls++
	return 1, nil
}
func (*behaviorSourceDie) RollN(context.Context, int, int) ([]int, error) {
	return nil, fmt.Errorf("unexpected RollN")
}

func (s *behaviorSourceSuite) TestAuthoredSiteDrivesRealMovementAndReload() {
	compiled, err := dungeonspec.Load(s.raw)
	s.Require().NoError(err)
	members := []encounter.MemberInput{{ID: "hero", Kind: encounter.KindPlayer, Position: compiled.PartyStart[0].At}}
	for _, m := range compiled.Monsters {
		members = append(members, encounter.MemberInput{ID: encounter.MemberID(m.MemberID), Kind: encounter.KindMonster,
			Position: m.At, Faction: encounter.FactionID(m.Faction), Behaviors: m.Behaviors,
			BlockingStates: m.BlockingStates, Posts: m.Posts})
	}
	caps := facingCapabilities()
	die := &behaviorSourceDie{}
	caps.Driver, caps.Roller, caps.Sheets = encounter.BehaviorDriver{}, die, behaviorSourceSheets{}
	e, err := encounter.NewEncounter(&encounter.SetupInput{Field: compiled.Field, Members: members,
		Retention: encounter.RetentionUnbounded, // the real session host's capture policy
		Endings:   []encounter.EndingInput{{Key: "stop", Trigger: encounter.TriggerExternal{}}}, Capabilities: caps})
	s.Require().NoError(err)
	for _, cell := range []spatial.Position{{X: 1, Y: 2}, {X: 0, Y: 2}, {X: 1, Y: 2}} {
		_, err := e.Step(&encounter.StepInput{Member: "hero", To: cell})
		s.Require().NoError(err)
		raw, err := json.Marshal(e.ToData())
		s.Require().NoError(err)
		var data encounter.EncounterData
		s.Require().NoError(json.Unmarshal(raw, &data))
		e, err = encounter.LoadEncounter(&encounter.LoadEncounterInput{Data: data, Capabilities: caps})
		s.Require().NoError(err)
	}
	membersOut, err := e.Members()
	s.Require().NoError(err)
	positions := map[string]spatial.Position{}
	for _, member := range membersOut {
		positions[string(member.ID)] = member.Position
	}
	s.Equal(spatial.Position{X: 6, Y: 1}, positions["west"])
	s.Equal(spatial.Position{X: 14}, positions["court"])
	s.Equal(6, die.calls, "three real opportunities, two guards, no selection on reload or completion")
	// Runtime symbols must describe the real compiler's bound configuration,
	// not a separate fixture representation or a new expansion on reload.
	record, err := e.ReadRecord(&encounter.ReadRecordInput{})
	s.Require().NoError(err)
	symbols := map[string]json.RawMessage{}
	count := 0
	for _, entry := range record.Entries {
		var symbol struct {
			Beat          string          `json:"beat"`
			Member        string          `json:"member"`
			Configuration json.RawMessage `json:"configuration"`
		}
		s.Require().NoError(json.Unmarshal(entry.Payload, &symbol))
		if symbol.Beat == encounter.BeatBehaviorConfig {
			count++
			symbols[symbol.Member] = symbol.Configuration
		}
	}
	s.Require().Equal(2, count, "one exact configuration for each actor, not one per reload")
	for _, monster := range compiled.Monsters {
		expected, err := jsonv2.Marshal(monster.Behaviors)
		s.Require().NoError(err)
		s.JSONEq(string(expected), string(symbols[monster.MemberID]))
	}
}
