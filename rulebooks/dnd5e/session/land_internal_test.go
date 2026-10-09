// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/play/interrupt"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resolution"
)

// landProbe hears the repositories a landing writes through, so a test can
// tell which step ran when. Silent until armed.
type landProbe struct {
	armed bool
	hear  func(event string)
}

func (p *landProbe) heard(event string) {
	if p.armed && p.hear != nil {
		p.hear(event)
	}
}

type probedCharacters struct {
	*strikeCharacters
	probe *landProbe
	saved map[string]int
}

func (c *probedCharacters) GetCharacter(ctx context.Context, id string) (*character.Data, error) {
	c.probe.heard("read")
	return c.strikeCharacters.GetCharacter(ctx, id)
}

func (c *probedCharacters) SaveCharacter(ctx context.Context, data *character.Data) error {
	c.probe.heard("sheets")
	c.saved[data.ID]++
	return c.strikeCharacters.SaveCharacter(ctx, data)
}

type probedEncounters struct {
	*strikeEncounters
	probe *landProbe
}

func (e *probedEncounters) SaveEncounter(ctx context.Context, id string, data *encounter.EncounterData) error {
	e.probe.heard("commit")
	return e.strikeEncounters.SaveEncounter(ctx, id, data)
}

// LandSuite drives [Manager.land] directly inside one write scope, over an
// output built by hand so that every field it carries is one the test chose.
type LandSuite struct {
	suite.Suite

	probe      *landProbe
	sessions   *strikeSessions
	encounters *strikeEncounters
	characters *probedCharacters
	mgr        *Manager
	scope      *writeScope
}

func TestLandSuite(t *testing.T) { suite.Run(t, new(LandSuite)) }

func (s *LandSuite) SetupTest() {
	ctx := context.Background()
	s.probe = &landProbe{}
	s.sessions = &strikeSessions{byID: map[string]*SessionData{}}
	s.encounters = &strikeEncounters{byID: map[string]*encounter.EncounterData{}}
	s.characters = &probedCharacters{
		strikeCharacters: &strikeCharacters{byID: map[string]*character.Data{
			"fighter": strikeFixtureFighter("fighter"),
		}},
		probe: s.probe,
		saved: map[string]int{},
	}
	mgr, err := NewManager(&Config{Seats: newFakeSeats(),
		PresentationIDs: testPresentationIDs{}, Dice: &scriptedDice{rolls: []int{10, 10, 10, 10, 10, 10}}, TurnDriver: Pass{},
		Sessions: s.sessions, Encounters: &probedEncounters{strikeEncounters: s.encounters, probe: s.probe},
		Characters: s.characters, Events: DiscardEvents{},
	})
	s.Require().NoError(err)
	s.mgr = mgr

	world, err := encounter.NewEncounter(&encounter.SetupInput{
		Field: encounter.FieldInput{
			Canvas:  pointyCanvas(),
			Regions: []encounter.RegionInput{rectRegion("tomb", 0, 0, 12, 6)},
		},
		Endings:      []encounter.EndingInput{{Key: "withdraw", Trigger: encounter.TriggerExternal{}}},
		Retention:    encounter.RetentionUnbounded,
		Capabilities: encounter.RefusingCapabilities(),
	})
	s.Require().NoError(err)
	data := world.ToData()
	_, err = mgr.StartSession(ctx, &StartSessionInput{Session: "sess", Encounter: "world", World: &data})
	s.Require().NoError(err)
	_, err = mgr.Join(ctx, &JoinInput{Session: "sess", Member: "fighter",
		Position: encounter.HexCellAt(encounter.HexesArePointyTop(), 2, 0)})
	s.Require().NoError(err)
	_, err = mgr.Spawn(ctx, &SpawnInput{Session: "sess", ID: "goblin", Ref: refs.Monsters.Goblin().String(),
		Position: encounter.HexCellAt(encounter.HexesArePointyTop(), 3, 0)})
	s.Require().NoError(err)

	s.scope, err = s.mgr.openForChange(ctx, "sess")
	s.Require().NoError(err)
	// The area the output will close, open before the landing.
	s.Require().NoError(s.scope.enc.AddSightArea(&encounter.SightAreaInput{
		ID: "fog", SourceID: "fighter", Name: "Fog Cloud",
		Center: encounter.HexCellAt(encounter.HexesArePointyTop(), 8, 3), RadiusFeet: 10,
	}))
	s.sessions.saves, s.encounters.saves, s.characters.saves = 0, 0, 0
	s.characters.saved = map[string]int{}
}

// fullOutput is one output carrying one of everything a landing lands: a
// world marked by the goblin's name, one dirty character, one dirty monster,
// one closed and one opened area, and one concentration check and break.
func (s *LandSuite) fullOutput() *resolution.Output {
	world := s.scope.enc.WorldView()
	for i := range world.Members {
		if world.Members[i].ID == "goblin" {
			world.Members[i].Name = "marked goblin"
		}
	}
	fighter, err := s.characters.strikeCharacters.GetCharacter(context.Background(), "fighter")
	s.Require().NoError(err)
	fighter.HitPoints = 3
	var goblin *monster.Data
	for i := range s.scope.data.NPCs {
		if s.scope.data.NPCs[i].ID == "goblin" {
			copied := s.scope.data.NPCs[i]
			goblin = &copied
		}
	}
	s.Require().NotNil(goblin)
	goblin.HitPoints = 1
	spell := encounter.SpellIdentity{Ref: refs.Spells.FogCloud().String(), Name: "Fog Cloud"}
	return &resolution.Output{
		World:           world,
		DirtyCharacters: []*character.Data{fighter},
		DirtyMonsters:   []*monster.Data{goblin},
		ClosedAreas:     []string{"fighter"},
		OpenedAreas: []encounter.SightAreaInput{{
			ID: "cloud", SourceID: "goblin", Name: "Fog Cloud",
			Center: encounter.HexCellAt(encounter.HexesArePointyTop(), 6, 3), RadiusFeet: 10,
		}},
		ConcentrationChecks: []encounter.ConcentrationCheck{{Spell: spell}},
		ConcentrationBreaks: []encounter.ConcentrationBreak{{Caster: "fighter", Spell: spell}},
	}
}

// poseOpen opens one window on the scope's ledger for a landing to answer.
func (s *LandSuite) poseOpen() interrupt.Window {
	posed, err := s.scope.ledger.Pose(&interrupt.PoseInput{
		Audience: core.EntityID("fighter"),
		Options:  []interrupt.Option{interrupt.Option(ReactStrike), interrupt.Option(ReactHold)},
		Payload:  []byte(`{}`),
		At:       s.scope.baseline,
	})
	s.Require().NoError(err)
	return posed.Window
}

func areaIDs(enc *encounter.Encounter) map[string]bool {
	ids := map[string]bool{}
	for _, area := range enc.WorldView().SightAreas {
		ids[area.ID] = true
	}
	return ids
}

func (s *LandSuite) isOpen(id interrupt.WindowID) bool {
	open, err := s.scope.ledger.Open()
	s.Require().NoError(err)
	for _, w := range open {
		if w.ID == id {
			return true
		}
	}
	return false
}

func (s *LandSuite) goblinName(enc *encounter.Encounter) string {
	for _, member := range enc.WorldView().Members {
		if member.ID == "goblin" {
			return member.Name
		}
	}
	return ""
}

// TestEveryOutputFieldLandsOnce: every field of one output lands exactly once,
// in the one order.
//
// The order is read off what each step leaves behind, probed at every point
// the landing calls out of itself: each repository read and write, and the
// Record, Window and Continue closures. adopt shows as the scope's encounter
// changing, the areas as the area set changing, the answer as the window
// closing; each is logged the first time a probe sees it. No probe sits
// between the area step and the answer, so their order is pinned by
// TestTheAreasLandBeforeTheAnswer instead.
func (s *LandSuite) TestEveryOutputFieldLandsOnce() {
	ctx := context.Background()
	before := s.scope.enc
	answering := s.poseOpen()
	out := s.fullOutput()
	wantChecks, wantBreaks := out.ConcentrationChecks, out.ConcentrationBreaks

	var order []string
	seen := map[string]bool{}
	observe := func() {
		note := func(step string, happened bool) {
			if happened && !seen[step] {
				seen[step] = true
				order = append(order, step)
			}
		}
		note("adopt", s.scope.enc != before)
		areas := areaIDs(s.scope.enc)
		note("areas", areas["cloud"] && !areas["fog"])
		note("answer", !s.isOpen(answering.ID))
	}
	s.probe.hear = func(event string) {
		observe()
		if event != "read" {
			order = append(order, event)
		}
	}

	records, windows, continues := 0, 0, 0
	var posedID interrupt.WindowID
	var told concentration
	var recordedOn *encounter.Encounter
	s.probe.armed = true
	result, err := s.mgr.land(ctx, s.scope, out, &landing{
		Record: func(enc *encounter.Encounter, c concentration) error {
			observe()
			order = append(order, "record")
			records++
			told, recordedOn = c, enc
			return nil
		},
		Answer: &windowAnswer{Window: answering, Choice: ReactHold},
		Window: func(*encounter.Encounter) error {
			observe()
			order = append(order, "window")
			windows++
			posedID = s.poseOpen().ID
			return nil
		},
		Continue: func(*encounter.Encounter) error {
			observe()
			order = append(order, "continue")
			continues++
			return nil
		},
	})
	s.probe.armed = false
	s.Require().NoError(err)
	s.Require().NotNil(result)

	s.NotSame(before, s.scope.enc, "the scope adopted the output's world")
	s.Equal("marked goblin", s.goblinName(s.scope.enc), "the adopted world is the output's")
	s.Same(s.scope.enc, recordedOn, "a verb's landing records on the adopted encounter")
	s.Equal(1, s.characters.saved["fighter"], "the dirty character is saved once")
	s.Equal(3, s.characters.byID["fighter"].HitPoints)
	goblinHP := -1
	for _, npc := range s.scope.data.NPCs {
		if npc.ID == "goblin" {
			goblinHP = npc.HitPoints
		}
	}
	s.Equal(1, goblinHP, "the dirty monster is folded into the session record")
	s.Equal(1, records)
	s.Equal(wantChecks, told.Checks, "Record received exactly the output's checks")
	s.Equal(wantBreaks, told.Breaks, "Record received exactly the output's breaks")
	areas := areaIDs(s.scope.enc)
	s.True(areas["cloud"], "the opened area landed")
	s.False(areas["fog"], "the closed area ended")
	s.False(s.isOpen(answering.ID), "the resumed window was answered")
	s.Equal(1, windows)
	s.True(s.isOpen(posedID), "the posed window is open")
	s.Equal(s.scope.ledger.ToData(), s.scope.data.Windows, "the session record carries the ledger")
	s.Equal(1, continues)
	s.Equal(1, s.encounters.saves, "the encounter is saved once")
	s.Equal(1, s.sessions.saves, "the session is saved once")
	s.Equal([]string{"adopt", "sheets", "record", "areas", "answer", "window", "continue", "commit"}, order)
}

// TestASeamLandingAdoptsAndCommitsNothing: a Live landing records on the
// encounter that called the seam, writes the sheets, and neither adopts nor
// commits — the calling verb does.
func (s *LandSuite) TestASeamLandingAdoptsAndCommitsNothing() {
	live := s.scope.enc
	out := s.fullOutput()
	var recordedOn *encounter.Encounter
	result, err := s.mgr.land(context.Background(), s.scope, out, &landing{
		Live: live,
		Record: func(enc *encounter.Encounter, _ concentration) error {
			recordedOn = enc
			return nil
		},
	})
	s.Require().NoError(err)
	s.Equal(&landed{}, result)
	s.Same(live, s.scope.enc, "a seam's landing adopts nothing")
	s.Same(live, recordedOn, "Record received Live")
	s.NotEqual("marked goblin", s.goblinName(s.scope.enc))
	s.Equal(1, s.characters.saved["fighter"], "the sheets are still written")
	s.Zero(s.encounters.saves, "nothing commits the encounter")
	s.Zero(s.sessions.saves, "nothing commits the session")
}

// TestAnUntoldLandingDropsConcentrationByName: a landing that declares it
// tells no concentration lands an output that carries some, and commits.
func (s *LandSuite) TestAnUntoldLandingDropsConcentrationByName() {
	out := s.fullOutput()
	s.Require().NotEmpty(out.ConcentrationBreaks)
	_, err := s.mgr.land(context.Background(), s.scope, out, &landing{Untold: true})
	s.Require().NoError(err)
	s.Equal(1, s.encounters.saves, "the untold landing commits")
}

// TestAnUndeclaredLandingRefusesConcentration: concentration with nothing to
// tell it and no declaration that it is untold refuses, and commits nothing.
func (s *LandSuite) TestAnUndeclaredLandingRefusesConcentration() {
	out := s.fullOutput()
	_, err := s.mgr.land(context.Background(), s.scope, out, &landing{})
	s.Require().ErrorIs(err, ErrInvalidWorld)
	s.Zero(s.encounters.saves, "a refused landing commits nothing")
	s.Zero(s.sessions.saves)
}

// TestTheAreasLandBeforeTheAnswer pins the one pair of steps the order test
// cannot probe between: an answer that fails finds the areas already landed,
// and the failure reports the sheets that landed before it (R10).
func (s *LandSuite) TestTheAreasLandBeforeTheAnswer() {
	out := s.fullOutput()
	gone := interrupt.Window{ID: 999, Audience: core.EntityID("fighter")}
	_, err := s.mgr.land(context.Background(), s.scope, out, &landing{
		Record: func(*encounter.Encounter, concentration) error { return nil },
		Answer: &windowAnswer{Window: gone, Choice: ReactHold},
	})
	s.Require().ErrorIs(err, ErrInvalidSession)
	var saveErr *SaveError
	s.Require().ErrorAs(err, &saveErr, "a failure after the sheets landed reports them")
	s.Contains(saveErr.Report.Written, "character:fighter")
	areas := areaIDs(s.scope.enc)
	s.True(areas["cloud"], "the areas landed before the answer was attempted")
	s.False(areas["fog"])
	s.Zero(s.encounters.saves, "a failed landing commits nothing")
}
