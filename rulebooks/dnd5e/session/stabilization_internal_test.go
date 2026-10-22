// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

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
)

type StabilizationSessionSuite struct{ suite.Suite }

func TestStabilizationSessionSuite(t *testing.T) { suite.Run(t, new(StabilizationSessionSuite)) }

type stabilizationStream struct{ acts []Act }

func (s *stabilizationStream) PublishActs(_ context.Context, acts []Act) error {
	s.acts = append(s.acts, acts...)
	return nil
}

// stabilizedIn is the stabilization alice's acts carry, or nil.
func stabilizedIn(acts []Act) *Stabilization {
	for _, act := range acts {
		if act.Recipient != "alice" {
			continue
		}
		activate, ok := act.Body.(ActivateAct)
		if !ok {
			continue
		}
		for _, result := range activate.Results {
			if result.Stabilization != nil {
				return result.Stabilization
			}
		}
	}
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
			mgr, err := NewManager(&Config{Record: DiscardRecord{}, Acts: stream, Seats: newFakeSeats(), Explorations: newFakeExplorations(), PresentationIDs: testPresentationIDs{}, Dice: dice, TurnDriver: Pass{}, Sessions: sessions, Encounters: encounters, Characters: characters})
			s.Require().NoError(err)
			sc := scene{
				Field:   encounter.FieldInput{Canvas: pointyCanvas(), Regions: []encounter.RegionInput{rectRegion("hall", 0, 0, 5, 5)}},
				Party:   []sceneSeat{seatAt("alice", 1, 1), seatAt("bob", 2, 1)},
				Endings: []encounter.EndingInput{{Key: "done", Trigger: encounter.TriggerExternal{}}},
			}
			launchScene(s.T(), mgr, sc)
			// Launch rests whoever it seats, so the dying sheet is written
			// once they are on the board: it is the state under test.
			characters.byID["bob"] = bob
			scope, err := mgr.openForChange(ctx, "recheck", "sess")
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
			out, err := resolution.Resolve(ctx, &resolution.Input{
				World:        scope.enc.WorldView(),
				Participants: participants,
				Machine:      machine,
				Cost:         &resolution.Cost{PayerID: "alice", Profile: definition.Cost, SpellTurn: "turn1", Turn: &resolution.Turn{Number: 1}},
				Capabilities: encounter.Capabilities{
					Roller:        &diceSeam{roller: dice},
					Initiative:    mgr.initiative,
					Driver:        scope.driver,
					Participation: scope.standing,
					Sight:         aggregateRecordEveryoneSees{},
					Equipment:     encNoHandsObserved{},
					Conditions:    encNoHandsObserved{},
					Sheets:        sheetsBeside(scope.standing),
					Actors:        resolution.Actors,
				},
			})
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
			live := stabilizedIn(stream.acts)
			s.Require().NotNil(live)
			s.Equal(string(LifeStateStabilized), live.After)
			s.True(live.Stabilized)
			s.Zero(live.Failures)
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
			replay, err := mgr.Acts(ctx, &ActsInput{Session: "sess", Member: "alice"})
			s.Require().NoError(err)
			s.Equal(live, stabilizedIn(replay.Acts), "catch-up reads the stabilization live delivery carried")
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
