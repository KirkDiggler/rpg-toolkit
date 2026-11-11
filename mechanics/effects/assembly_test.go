// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package effects_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/events"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/effects"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/effects/machine"
)

type assemblySuite struct{ suite.Suite }

func TestAssemblySuite(t *testing.T) { suite.Run(t, new(assemblySuite)) }

func counterPart(key string, remaining int, scope effects.EndScope) effects.PartInput {
	vocab := outputVocabulary()
	vocab.End = nil
	return effects.PartInput{
		Key: key, OnTerminal: scope,
		Compose: effects.ComposeInput{
			HolderID: holder, MachineVocabulary: rageVocabulary(), OutputVocabulary: vocab,
			Recipe: machine.Recipe{
				Ref:     core.Ref{Module: "test", Type: "effects", ID: "counter"},
				Initial: "active", Bag: machine.Bag{"remaining": remaining},
				States: map[string]machine.State{
					"active": {Outputs: rageOutputs()[:1], Transitions: []machine.Transition{
						{On: beatTurnEnd, About: selSubject, When: &machine.Guard{Bag: &machine.BagLeaf{Key: "remaining", Op: machine.OpEq, Value: 1}}, To: "ended", Reason: "expired"},
						{On: beatTurnEnd, About: selSubject, Do: []machine.Act{{Add: &machine.AddAct{Key: "remaining", Delta: -1}}}},
					}},
					"ended": {Terminal: true},
				},
			},
		},
	}
}

func (s *assemblySuite) TestTwoPartsReactToOneBeat() {
	var changed []string
	definition := effects.AssemblyInput{Key: "attachment", Parts: []effects.PartInput{
		counterPart("one", 2, effects.EndPart), counterPart("two", 2, effects.EndPart),
	}, Lifecycle: effects.AssemblyLifecycle{Changed: func(_ context.Context, _ events.EventBus, change effects.AssemblyChange) error {
		changed = append(changed, change.PartKey)
		return nil
	}}}
	assembly, err := effects.Assemble(definition)
	s.Require().NoError(err)
	ctx, bus := context.Background(), events.NewEventBus()
	s.Require().NoError(assembly.Apply(ctx, bus))
	s.Require().NoError(turnEndTopic.On(bus).Publish(ctx, turnEnd{SubjectID: holder}))
	s.Equal([]string{"one", "two"}, changed)
	s.Equal(1, assembly.Snapshot().Parts["one"].Bag["remaining"])
	s.Equal(1, assembly.Snapshot().Parts["two"].Bag["remaining"])
	outputs := assembly.CurrentOutputs()
	s.Require().Len(outputs, 2)
	s.NotEqual(outputs[0].Output.ID(), outputs[1].Output.ID())
	s.Require().NoError(assembly.Remove(ctx, bus))
}

func (s *assemblySuite) TestPartCompletionDoesNotEndSibling() {
	var ends []effects.AssemblyEnd
	assembly, err := effects.Assemble(effects.AssemblyInput{Key: "attachment", Parts: []effects.PartInput{
		counterPart("short", 1, effects.EndPart), counterPart("long", 3, effects.EndPart),
	}, Lifecycle: effects.AssemblyLifecycle{End: func(_ context.Context, _ events.EventBus, end effects.AssemblyEnd) error {
		ends = append(ends, end)
		return nil
	}}})
	s.Require().NoError(err)
	ctx, bus := context.Background(), events.NewEventBus()
	s.Require().NoError(assembly.Apply(ctx, bus))
	s.Require().NoError(turnEndTopic.On(bus).Publish(ctx, turnEnd{SubjectID: holder}))
	s.Require().Len(ends, 1)
	s.Equal(effects.EndPart, ends[0].Scope)
	s.Equal("ended", assembly.Snapshot().Parts["short"].State)
	s.Equal(2, assembly.Snapshot().Parts["long"].Bag["remaining"])
	s.True(assembly.IsApplied())
	s.Require().Len(assembly.CurrentOutputs(), 1)
	s.Require().NoError(assembly.Remove(ctx, bus))
}

func (s *assemblySuite) TestApplicationEndRoutesOnceBeforeDetach() {
	ctx, bus := context.Background(), events.NewEventBus()
	var ends []effects.AssemblyEnd
	var during []int
	assembly, err := effects.Assemble(effects.AssemblyInput{Key: "attachment", Parts: []effects.PartInput{
		counterPart("first", 1, effects.EndApplication), counterPart("second", 1, effects.EndApplication),
	}, Lifecycle: effects.AssemblyLifecycle{End: func(ctx context.Context, bus events.EventBus, end effects.AssemblyEnd) error {
		ends = append(ends, end)
		got, err := damage(ctx, bus, testDamage{AttackerID: holder, Frame: strMelee(9), Total: 10})
		during = append(during, got)
		return err
	}}})
	s.Require().NoError(err)
	s.Require().NoError(assembly.Apply(ctx, bus))
	s.Require().NoError(turnEndTopic.On(bus).Publish(ctx, turnEnd{SubjectID: holder}))
	s.Require().Len(ends, 1)
	s.Equal("first", ends[0].PartKey)
	s.Equal("attachment", ends[0].AssemblyKey)
	s.Equal("expired", ends[0].Reason)
	s.Equal([]int{16}, during, "both contributions still answer during the end report")
	s.False(assembly.IsApplied())
	got, err := damage(ctx, bus, testDamage{AttackerID: holder, Frame: strMelee(9), Total: 10})
	s.Require().NoError(err)
	s.Equal(10, got)
	s.Error(assembly.Apply(ctx, bus), "a gameplay-ended application cannot be reattached")
}

func (s *assemblySuite) TestRestoredCompletedPartDoesNotRevive() {
	definition := effects.AssemblyInput{Key: "attachment", Parts: []effects.PartInput{
		counterPart("short", 1, effects.EndPart), counterPart("long", 3, effects.EndPart),
	}}
	assembly, err := effects.Assemble(definition)
	s.Require().NoError(err)
	ctx, bus := context.Background(), events.NewEventBus()
	s.Require().NoError(assembly.Apply(ctx, bus))
	s.Require().NoError(turnEndTopic.On(bus).Publish(ctx, turnEnd{SubjectID: holder}))
	snapshot := assembly.Snapshot()
	s.Require().NoError(assembly.Remove(ctx, bus))
	restored, err := effects.RestoreAssembly(effects.RestoreAssemblyInput{Definition: definition, Snapshot: &snapshot})
	s.Require().NoError(err)
	snapshot.Parts["long"].Bag["remaining"] = 99
	s.Require().NoError(restored.Apply(ctx, bus))
	s.Require().NoError(turnEndTopic.On(bus).Publish(ctx, turnEnd{SubjectID: holder}))
	s.Equal("ended", restored.Snapshot().Parts["short"].State)
	s.Equal(1, restored.Snapshot().Parts["long"].Bag["remaining"])
	s.Require().Len(restored.CurrentOutputs(), 1)
	s.Require().NoError(restored.Remove(ctx, bus))
}

func (s *assemblySuite) TestMissingOrTerminalMaintenanceSnapshotIsRefused() {
	definition := effects.AssemblyInput{Key: "attachment", Parts: []effects.PartInput{counterPart("required", 2, effects.EndApplication)},
		Lifecycle: effects.AssemblyLifecycle{End: func(context.Context, events.EventBus, effects.AssemblyEnd) error { return nil }}}
	for _, snapshot := range []*effects.AssemblySnapshot{
		nil,
		{Parts: map[string]effects.Snapshot{}},
		{Parts: map[string]effects.Snapshot{"foreign": {State: "active", Bag: machine.Bag{"remaining": 2}}}},
		{Parts: map[string]effects.Snapshot{"required": {State: "active", Bag: machine.Bag{}}}},
		{Parts: map[string]effects.Snapshot{"required": {State: "ended", Bag: machine.Bag{"remaining": 2}}}},
	} {
		got, err := effects.RestoreAssembly(effects.RestoreAssemblyInput{Definition: definition, Snapshot: snapshot})
		s.Error(err)
		s.Nil(got)
	}
}

func (s *assemblySuite) TestLaterAttachFailureUnwindsEarlierParts() {
	bad := counterPart("bad", 2, effects.EndPart)
	refused := errors.New("attachment refused")
	kind := bad.Compose.OutputVocabulary.Kinds[kindDamageBonus]
	kind.Attach = func(context.Context, events.EventBus, *effects.Bound) (effects.Detach, error) { return nil, refused }
	bad.Compose.OutputVocabulary.Kinds[kindDamageBonus] = kind
	assembly, err := effects.Assemble(effects.AssemblyInput{Key: "attachment", Parts: []effects.PartInput{
		counterPart("good", 2, effects.EndPart), bad,
	}})
	s.Require().NoError(err)
	ctx, bus := context.Background(), events.NewEventBus()
	s.ErrorIs(assembly.Apply(ctx, bus), refused)
	s.False(assembly.IsApplied())
	got, err := damage(ctx, bus, testDamage{AttackerID: holder, Frame: strMelee(9), Total: 10})
	s.Require().NoError(err)
	s.Equal(10, got)
}

func (s *assemblySuite) TestRemoveOnlyDetachesAndSnapshotsAreOwned() {
	ends := 0
	assembly, err := effects.Assemble(effects.AssemblyInput{Key: "attachment", Parts: []effects.PartInput{counterPart("part", 3, effects.EndPart)},
		Lifecycle: effects.AssemblyLifecycle{End: func(context.Context, events.EventBus, effects.AssemblyEnd) error { ends++; return nil }}})
	s.Require().NoError(err)
	before := assembly.Snapshot()
	before.Parts["part"].Bag["remaining"] = 99
	delete(before.Parts, "part")
	ctx, bus := context.Background(), events.NewEventBus()
	s.Require().NoError(assembly.Apply(ctx, bus))
	s.Error(assembly.Apply(ctx, bus))
	s.Require().NoError(assembly.Remove(ctx, bus))
	s.Require().NoError(assembly.Remove(ctx, bus))
	s.Zero(ends)
	s.Equal(3, assembly.Snapshot().Parts["part"].Bag["remaining"])
	s.Require().NoError(assembly.Apply(ctx, bus))
	s.Require().NoError(assembly.Remove(ctx, bus))
}

func (s *assemblySuite) TestDuplicateKeysOrNestedLifecycleHooksAreRefused() {
	for _, change := range []func(*effects.AssemblyInput){
		func(in *effects.AssemblyInput) { in.Key = "" },
		func(in *effects.AssemblyInput) { in.Parts = append(in.Parts, in.Parts[0]) },
		func(in *effects.AssemblyInput) { in.Parts[0].Key = " " },
		func(in *effects.AssemblyInput) { in.Parts[0].Compose.OutputVocabulary.End = outputVocabulary().End },
		func(in *effects.AssemblyInput) { in.Parts[0].OnTerminal = effects.EndApplication },
		func(in *effects.AssemblyInput) { in.Parts[0].OnTerminal = "unknown" },
		func(in *effects.AssemblyInput) {
			other := counterPart("other", 2, effects.EndPart)
			other.Compose.HolderID = "another holder"
			in.Parts = append(in.Parts, other)
		},
	} {
		input := effects.AssemblyInput{Key: "attachment", Parts: []effects.PartInput{counterPart("part", 2, effects.EndPart)}}
		change(&input)
		got, err := effects.Assemble(input)
		s.Error(err)
		s.Nil(got)
	}
}
