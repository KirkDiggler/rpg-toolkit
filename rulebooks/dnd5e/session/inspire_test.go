// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// InspireSuite is rpg-project#397's grant half at this seam: a bard offers the
// die, to whom, at what price, and with what refusals.
type InspireSuite struct {
	suite.Suite

	sessions   *fakeSessions
	encounters *fakeEncounters
	characters *fakeCharacters
}

func TestInspireSuite(t *testing.T) {
	suite.Run(t, new(InspireSuite))
}

// levelOneBard is a bard with the feature on the sheet and two uses in the
// pool — the sheet a finalized draft produces, written directly so this suite
// does not depend on the draft flow.
func levelOneBard(id string, uses int) *character.Data {
	feature, err := json.Marshal(map[string]any{
		"ref":  refs.Features.BardicInspiration(),
		"id":   refs.Features.BardicInspiration().ID,
		"name": conditions.InspiredName,
	})
	if err != nil {
		panic(err)
	}
	return &character.Data{
		ID: id, PlayerID: "player-" + id, Name: id, Level: 1,
		ClassID: classes.Bard, RaceID: "human",
		AbilityScores: shared.AbilityScores{
			abilities.STR: 8, abilities.DEX: 14, abilities.CON: 12,
			abilities.INT: 10, abilities.WIS: 12, abilities.CHA: 16,
		},
		HitPoints: 9, MaxHitPoints: 9, ArmorClass: 12, ProficiencyBonus: 2,
		Features: []json.RawMessage{feature},
		Resources: map[coreResources.ResourceKey]character.RecoverableResourceData{
			resources.Inspiration: {Current: uses, Maximum: 2, ResetType: coreResources.ResetLongRest},
		},
	}
}

// bardWorld is a bard and a fighter on a turn clock with the bard active, the
// fighter standing `apart` cells away.
func bardWorld(t fataler, apart int) *encounter.EncounterData {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Striker: encounter.RefusingStriker{}, Mover: encounter.RefusingMover{},
		Announcer: encQuietAnnouncer{}, Sight: encEveryoneSees{},
		Initiative: encOrderAsGiven{}, TurnDriver: encPassDriver{},
		Standing: encEveryoneStanding{},
		Field: encounter.FieldInput{
			Canvas:  pointyCanvas(),
			Regions: []encounter.RegionInput{rectRegion("hall", 0, 0, 30, 8)},
		},
		Members: []encounter.MemberInput{
			{ID: "bard", Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}},
			{ID: "fighter", Kind: encounter.KindPlayer, Position: spatial.Position{X: float64(1 + apart), Y: 1}},
		},
		Endings:   []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
		Retention: encounter.RetentionUnbounded,
	})
	if err != nil {
		t.Fatalf("building the bard's scene: %v", err)
	}
	data := enc.ToData()
	return turnWorld(&data, []string{"bard", "fighter"}, 0)
}

// scene opens a session with the bard holding `uses` and the fighter `apart`
// cells away.
func (s *InspireSuite) scene(uses, apart int) *session.Manager {
	s.sessions, s.encounters = newFakeSessions(), newFakeEncounters()
	s.characters = newFakeCharacters(levelOneBard("bard", uses), armedFighter("fighter"))

	mgr, err := session.NewManager(&session.Config{
		PresentationIDs: testPresentationIDs{}, Dice: testDice{}, TurnDriver: session.Pass{},
		Sessions: s.sessions, Encounters: s.encounters,
		Characters: s.characters, Events: session.DiscardEvents{},
	})
	s.Require().NoError(err)
	_, err = mgr.StartSession(context.Background(), &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: bardWorld(s.T(), apart),
	})
	s.Require().NoError(err)
	return mgr
}

// inspirationRow is the bard's own Bardic Inspiration declaration.
func (s *InspireSuite) inspirationRow(mgr *session.Manager) session.Declaration {
	out, err := mgr.Afford(context.Background(), &session.AffordInput{Session: "sess", Member: "bard"})
	s.Require().NoError(err)
	for _, declaration := range out.Declarations {
		if declaration.Ability != nil &&
			declaration.Ability.Ref == refs.Features.BardicInspiration().String() {
			return declaration
		}
	}
	s.Require().Fail("the bard was offered no inspiration row")
	return session.Declaration{}
}

// candidateFor finds one member's row in a declaration's candidate universe.
func (s *InspireSuite) candidateFor(row session.Declaration, member string) (session.TargetCandidate, bool) {
	for _, candidate := range row.Candidates {
		if candidate.Member == member {
			return candidate, true
		}
	}
	return session.TargetCandidate{}, false
}

// grant activates the bard's inspiration on the named ally.
func (s *InspireSuite) grant(mgr *session.Manager, target string) (*session.ActivateOutput, error) {
	row := s.inspirationRow(mgr)
	return mgr.Activate(context.Background(), &session.ActivateInput{
		Session: "sess", Member: "bard", Target: target, DeclarationID: row.ID,
	})
}

// poolLeft reads the bard's stored uses.
func (s *InspireSuite) poolLeft() int {
	stored, err := s.characters.GetCharacter(context.Background(), "bard")
	s.Require().NoError(err)
	pool, ok := stored.Resources[resources.Inspiration]
	s.Require().True(ok, "the bard carries no inspiration pool")
	return pool.Current
}

// TestTheOfferIsABonusActionAimedAtAnAlly is the panel the bard sees.
func (s *InspireSuite) TestTheOfferIsABonusActionAimedAtAnAlly() {
	row := s.inspirationRow(s.scene(2, 1))

	s.True(row.Available)
	s.Equal(session.VerbActivate, row.Verb)
	s.Equal(session.SlotBonus, row.Slot, "a bonus action, read off the feature's own shape")
	s.Equal(session.TargetMember, row.TargetKind)
	s.Equal(conditions.InspiredName, row.Ability.Name)

	candidate, found := s.candidateFor(row, "fighter")
	s.Require().True(found, "the ally is offered")
	s.True(candidate.Available)

	_, self := s.candidateFor(row, "bard")
	s.False(self, "a creature other than yourself")
}

// TestItReachesSixtyFeet — the reach is RAW, and a fighter eleven cells away
// (55 feet) is still in it.
func (s *InspireSuite) TestItReachesSixtyFeet() {
	row := s.inspirationRow(s.scene(2, 11))

	candidate, found := s.candidateFor(row, "fighter")
	s.Require().True(found)
	s.True(candidate.Available, "55 feet is inside 60")
}

// TestAnAllyTooFarKeepsTheirRowWithAReason — the panel shows who is there and
// why they cannot be reached, rather than a list that changes length.
func (s *InspireSuite) TestAnAllyTooFarKeepsTheirRowWithAReason() {
	row := s.inspirationRow(s.scene(2, 13))

	candidate, found := s.candidateFor(row, "fighter")
	s.Require().True(found, "the row stays")
	s.False(candidate.Available, "65 feet is outside 60")
	s.Require().NotNil(candidate.Why)
	s.Equal(session.ShortfallTargetOutOfReach, candidate.Why.Reason)
}

// TestAnEmptyPoolGreysTheRowOut — refused before anything moves, with the
// feature's own words on it.
func (s *InspireSuite) TestAnEmptyPoolGreysTheRowOut() {
	mgr := s.scene(0, 1)

	row := s.inspirationRow(mgr)
	s.False(row.Available)
	s.Require().NotNil(row.Why)
	s.Equal(session.CurrencyCharges, row.Why.Currency)

	_, err := s.grant(mgr, "fighter")
	s.Require().Error(err, "and the door refuses it too")
	s.Equal(0, s.poolLeft())
}

// TestTheGrantSpendsOneUseAndLandsTheDie is the walk's first beat.
func (s *InspireSuite) TestTheGrantSpendsOneUseAndLandsTheDie() {
	mgr := s.scene(2, 1)

	out, err := s.grant(mgr, "fighter")

	s.Require().NoError(err)
	s.Equal(1, s.poolLeft(), "the pool drops by one")
	s.Equal(refs.Features.BardicInspiration().String(), out.Ability)

	stored, err := s.characters.GetCharacter(context.Background(), "fighter")
	s.Require().NoError(err)
	s.True(carriesInspiration(s.T(), stored), "the die is on the ALLY's sheet")

	// And the story says so, as an activation with one condition-applied
	// result — the beat the shipped Activate path already writes.
	s.Equal(1, s.countBeat(mgr, "activated"))
	s.Equal(1, s.countBeat(mgr, "activation-result"))
}

// TestASecondDieIsRefusedRatherThanReplacing is R7. Replacing would spend a
// use to overwrite a use — a charge with no change — so the candidate carries
// the reason before it is chosen.
func (s *InspireSuite) TestASecondDieIsRefusedRatherThanReplacing() {
	mgr := s.scene(2, 1)
	_, err := s.grant(mgr, "fighter")
	s.Require().NoError(err)

	row := s.inspirationRow(mgr)
	candidate, found := s.candidateFor(row, "fighter")
	s.Require().True(found, "the ally keeps their row")
	s.False(candidate.Available)
	s.Require().NotNil(candidate.Why)
	s.Equal("already inspired", candidate.Why.Text)

	_, err = s.grant(mgr, "fighter")
	s.Require().Error(err, "and choosing it anyway is refused")
	s.Equal(1, s.poolLeft(), "with nothing further spent")
}

// TestTheBardCannotInspireThemselves — RAW, and refused at the door as well as
// absent from the candidates.
func (s *InspireSuite) TestTheBardCannotInspireThemselves() {
	mgr := s.scene(2, 1)

	_, err := s.grant(mgr, "bard")

	s.Require().Error(err)
	s.Equal(2, s.poolLeft(), "nothing spent on a refused grant")
}

// countBeat counts beats of one kind in the bard's story.
func (s *InspireSuite) countBeat(mgr *session.Manager, kind string) int {
	entries, err := mgr.Story(context.Background(), &session.StoryInput{Session: "sess", Member: "bard"})
	s.Require().NoError(err)
	count := 0
	for _, entry := range entries {
		var beat struct {
			Beat string `json:"beat"`
		}
		s.Require().NoError(json.Unmarshal(entry.Payload, &beat))
		if beat.Beat == kind {
			count++
		}
	}
	return count
}
