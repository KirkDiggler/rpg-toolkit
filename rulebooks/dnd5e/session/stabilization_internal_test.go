// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreCombat "github.com/KirkDiggler/rpg-toolkit/core/combat"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resolution"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/saves"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
	"github.com/stretchr/testify/suite"
)

type StabilizationSessionSuite struct{ suite.Suite }

func TestStabilizationSessionSuite(t *testing.T) { suite.Run(t, new(StabilizationSessionSuite)) }

type stabilizationStream struct{ events []Event }

func (s *stabilizationStream) Publish(_ context.Context, events []Event) error {
	s.events = append(s.events, events...)
	return nil
}

func stabilizationProfile() actions.Definition {
	return actions.Definition{Ref: core.Ref{Module: "test", Type: "spells", ID: "stabilize"}, Name: "Stabilization", Cost: &combat.SpendProfile{Slots: map[coreCombat.ActionType]int{coreCombat.ActionStandard: 1}},
		Cast: &actions.CastProfile{Casting: &combat.SpellCasting{Time: combat.SpellCastingAction}, Target: actions.CastTargetTouch, RangeFeet: 5, MinTargets: 1, MaxTargets: 1, Stabilize: true}}
}

// Content remains disabled in this release. Exercise the same compiler, machine,
// persistence and recording seams with a generic profile, without a catalog override.
func (s *StabilizationSessionSuite) TestCompiledProfilePersistsAndDeliversReplayableResult() {
	for _, stable := range []bool{false, true} {
		s.Run(map[bool]string{false: "dying", true: "already stable"}[stable], func() {
			ctx := context.Background()
			alice, bob := strikeFixtureFighter("alice"), strikeFixtureFighter("bob")
			alice.ActionEconomy = &character.ActionEconomyData{TurnNumber: 1, ActionsRemaining: 1, BonusActionsRemaining: 1, ReactionsRemaining: 1, MovementRemaining: 30}
			alice.Resources = map[coreResources.ResourceKey]character.RecoverableResourceData{
				resources.SpellSlotLevel1: {Current: 2, Maximum: 2, ResetType: coreResources.ResetLongRest},
			}
			bob.HitPoints = 0
			bob.DeathSaveState = &saves.DeathSaveState{Successes: 1, Failures: 2, Stabilized: stable}
			characters := &strikeCharacters{byID: map[string]*character.Data{"alice": alice, "bob": bob}}
			sessions := &strikeSessions{byID: map[string]*SessionData{}}
			encounters := &strikeEncounters{byID: map[string]*encounter.EncounterData{}}
			stream := &stabilizationStream{}
			dice := &scriptedDice{}
			mgr, err := NewManager(&Config{PresentationIDs: testPresentationIDs{}, Dice: dice, TurnDriver: Pass{}, Sessions: sessions, Encounters: encounters, Characters: characters, Events: stream})
			s.Require().NoError(err)
			world, err := encounter.NewEncounter(&encounter.SetupInput{
				Striker: encounter.RefusingStriker{}, Mover: encounter.RefusingMover{}, Announcer: encQuietAnnouncer{}, Sight: aggregateRecordEveryoneSees{}, Equipment: encNoHandsObserved{},
				Initiative: aggregateRecordOrderAsGiven{}, TurnDriver: passDriver{}, Standing: aggregateRecordEveryoneStanding{},
				Field:   encounter.FieldInput{Canvas: pointyCanvas(), Regions: []encounter.RegionInput{rectRegion("hall", 0, 0, 5, 5)}},
				Members: []encounter.MemberInput{{ID: "alice", Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}}, {ID: "bob", Kind: encounter.KindPlayer, Position: spatial.Position{X: 2, Y: 1}}},
				Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
			})
			s.Require().NoError(err)
			data := world.ToData()
			_, err = mgr.StartSession(ctx, &StartSessionInput{Session: "sess", Encounter: "world", World: &data})
			s.Require().NoError(err)
			scope, err := mgr.openForChange(ctx, "sess")
			s.Require().NoError(err)
			roster, err := scope.enc.Members()
			s.Require().NoError(err)
			positions := map[string]spatial.Position{}
			for _, member := range roster {
				positions[string(member.ID)] = member.Position
			}
			sheet, err := character.Load(ctx, alice)
			s.Require().NoError(err)
			participants := []resolution.Participant{{Character: alice}, {Character: bob}}
			definition := stabilizationProfile()
			input := &compileCastOfferInput{Encounter: scope.enc, SessionID: "sess", Member: "alice", SpellTurn: "turn1", Sheet: sheet, Definition: definition, Roster: roster, Positions: positions,
				Holdings: []perception.Holding{{Subject: "bob"}}, Participants: participants}
			// No CurrentVia: touch uses known identity, not current sight.
			offer, err := mgr.compileCastOffer(ctx, input)
			s.Require().NoError(err)
			s.True(offer.declaration.Available)
			s.Require().Len(offer.declaration.Candidates, 1)
			s.True(offer.targets["bob"].available)
			targets, err := castTargets(&definition, offer, []string{"bob"}, castAim{})
			s.Require().NoError(err)
			machine, err := resolution.NewAction(&resolution.ActionInput{Definition: definition, AttackerID: "alice", TargetIDs: targets, Roller: &diceSeam{roller: dice}})
			s.Require().NoError(err)
			out, err := resolution.Resolve(ctx, &resolution.Input{World: scope.enc.WorldView(), Participants: participants, Machine: machine,
				Cost: &resolution.Cost{PayerID: "alice", Profile: definition.Cost, SpellTurn: "turn1", Turn: &resolution.Turn{Number: 1}}, Roller: &diceSeam{roller: dice},
				Initiative: mgr.initiative, TurnDriver: scope.driver, Standing: scope.standing, Sight: aggregateRecordEveryoneSees{}, Equipment: encNoHandsObserved{}})
			s.Require().NoError(err)
			results, pushes, err := castOutcome(out.Outcome, "alice", *offer.declaration.Spell)
			s.Require().NoError(err)
			s.Empty(pushes)
			s.Require().NoError(mgr.adopt(ctx, scope, out.World))
			s.Require().NoError(mgr.saveDirty(ctx, scope, out))
			_, err = scope.enc.RecordCast(&encounter.RecordCastInput{Actor: "alice", Spell: encounter.SpellIdentity{Ref: definition.Ref.String(), Name: definition.Name}, Targets: results})
			s.Require().NoError(err)
			_, _, err = mgr.commit(ctx, scope)
			s.Require().NoError(err)
			s.Zero(dice.next)
			s.Zero(characters.byID["alice"].ActionEconomy.ActionsRemaining)
			s.Equal(alice.Resources, characters.byID["alice"].Resources)
			s.Zero(characters.byID["bob"].HitPoints)
			s.Equal(&saves.DeathSaveState{Stabilized: true}, characters.byID["bob"].DeathSaveState)
			var live *StabilizedBody
			for _, event := range stream.events {
				if event.Recipient == "alice" && event.Kind == EventActivationResult {
					body := event.Body.(ActivationResultBody)
					live = body.Stabilized
				}
			}
			s.Require().NotNil(live)
			s.Equal(LifeStateStabilized, live.After)
			s.True(live.Progress.Stabilized)
			s.Zero(live.Progress.Failures)
			for id, stored := range encounters.byID {
				encounters.byID[id], err = cloneFixture(stored)
				s.Require().NoError(err)
			}
			for id, stored := range sessions.byID {
				sessions.byID[id], err = cloneFixture(stored)
				s.Require().NoError(err)
			}
			for id, stored := range characters.byID {
				characters.byID[id], err = cloneFixture(stored)
				s.Require().NoError(err)
			}
			replay, err := mgr.Story(ctx, &StoryInput{Session: "sess", Member: "alice"})
			s.Require().NoError(err)
			found := false
			for _, event := range replay {
				if event.Kind == EventActivationResult {
					s.Equal(live, event.Body.(ActivationResultBody).Stabilized)
					found = true
				}
			}
			s.True(found)
			// Recompilation rejects a target whose authoritative state changed.
			for _, state := range []struct {
				hp   int
				dead bool
			}{{hp: 1}, {dead: true}} {
				bob.HitPoints = state.hp
				bob.DeathSaveState = &saves.DeathSaveState{Dead: state.dead}
				fresh, compileErr := mgr.compileCastOffer(ctx, input)
				s.Require().NoError(compileErr)
				s.False(fresh.declaration.Available)
				_, targetErr := castTargets(&definition, fresh, targets, castAim{})
				s.ErrorIs(targetErr, ErrStaleDeclaration)
			}
			bob.HitPoints = 0
			bob.DeathSaveState = &saves.DeathSaveState{Stabilized: true}
			blocked := scope.enc.WorldView()
			blocked.Field.Walls = append(blocked.Field.Walls, encounter.BoundaryData{
				From: encounter.PositionData{X: 1, Y: 1}, To: encounter.PositionData{X: 2, Y: 1}, BlocksMovement: true,
			})
			s.Require().NoError(mgr.adopt(ctx, scope, blocked))
			input.Encounter = scope.enc
			fresh, err := mgr.compileCastOffer(ctx, input)
			s.Require().NoError(err)
			s.False(fresh.declaration.Available)
			s.Equal(ShortfallTargetOutOfReach, fresh.targets["bob"].why.Reason)
			_, err = castTargets(&definition, fresh, targets, castAim{})
			s.ErrorIs(err, ErrStaleDeclaration)
			paid, err := character.Load(ctx, characters.byID["alice"])
			s.Require().NoError(err)
			input.Sheet = paid
			fresh, err = mgr.compileCastOffer(ctx, input)
			s.Require().NoError(err)
			s.False(fresh.declaration.Available, "paid action cannot be offered again")
		})
	}
}

func (s *StabilizationSessionSuite) TestResultDecodingIsStrictAndMappingDetached() {
	provider := character.StabilizeOutput{Before: combat.LifeStateDying, After: combat.LifeStateStabilized, Progress: character.DeathSaveProgress{Stabilized: true, SuccessesNeeded: 3, FailuresRemaining: 3}}
	ref := stabilizationProfile().Ref
	results := activationResults([]resolution.ActivationEffect{{Kind: resolution.EffectStabilized, TargetID: "bob", Ref: ref.String(), Name: "Stabilization", Stabilization: provider}})
	s.Require().Len(results, 1)
	s.Equal(stabilizationDetail(provider), results[0].Stabilization)
	provider.Progress.Failures = 99
	s.Zero(results[0].Stabilization.Failures)
	_, err := imposedResult(resolution.ImposedEffect{Kind: resolution.ImposedStabilized, RecipientID: "bob"}, SpellRef{})
	s.ErrorIs(err, ErrInvalidWorld)
	valid := `{"beat":"activation-result","actor":"alice","result":{"kind":"stabilized","target":"bob","ref":"test:spells:stabilize","name":"Stabilization","stabilization":{"before":"dying","after":"stabilized","hit_points":0,"successes":0,"failures":0,"successes_needed":3,"failures_remaining":3,"stabilized":true,"dead":false}}}`
	kind, body := decodeBeat([]byte(valid))
	s.Equal(EventActivationResult, kind)
	s.Require().NotNil(body)
	for _, bad := range []string{
		strings.Replace(valid, `"hit_points":0`, `"hit_points":null`, 1),
		strings.Replace(valid, `"hit_points":0,`, ``, 1),
		strings.Replace(valid, `"hit_points":0`, `"hit_points":0,"hit_points":0`, 1),
		strings.Replace(valid, `"dead":false`, `"dead":false,"extra":0`, 1),
		strings.Replace(valid, `"before":"dying"`, `"before":""`, 1),
		strings.Replace(valid, `"kind":"stabilized"`, `"kind":"healing-applied"`, 1),
		strings.Replace(valid, `"kind":"stabilized"`, `"kind":"stabilized","calculation":null`, 1),
		strings.Replace(valid, `"kind":"stabilized"`, `"kind":"stabilized","amount":0`, 1),
	} {
		_, decoded := decodeBeat([]byte(bad))
		s.Nil(decoded, bad)
	}
	encoded, err := json.Marshal(body)
	s.Require().NoError(err)
	s.Contains(string(encoded), `"hit_points":0`)
	s.NotContains(string(encoded), "healing_applied")
}
