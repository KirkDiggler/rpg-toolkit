// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

// sneak_site_test.go plays the studio-authored sneak site (rpg-project#606,
// sneaking design R3, R6–R12, R16): a rogue declares sneaking out of the
// knight's sight, walks through the doorway into the knight's room, and the
// one roll its entry asks keeps it out of the knight's sight while the walk
// goes on.

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

const sneakSiteFixture = "testdata/sneak-site.yaml"

const (
	sneakSiteRogue  = encounter.MemberID("rogue")
	sneakSiteKnight = encounter.MemberID("knight")
	sneakSiteCook   = encounter.MemberID("cook")
)

// sneakSiteReset is the re-arm distance this host supplies for every
// watcher, in cells — the session's seed (design O6).
const sneakSiteReset = 2

// The site's cells, as offset [col, row] on the studio's floor.
var (
	// sneakSiteFarEnd is where the rogue's walk ends: two cells into the far
	// room, north of the doorway.
	sneakSiteFarEnd = castleCell(2, -1)
	// sneakSiteBesideCook is the near room's south-west, one cell from the
	// cook.
	sneakSiteBesideCook = castleCell(-6, 2)
)

// sneakSiteSight is the site's light: everybody sees twelve cells, the knight
// six — it watches its own room, and through the doorway only the near room's
// first cells — and the cook two: she is busy at her pot.
type sneakSiteSight struct{}

func (sneakSiteSight) Sight(members []encounter.MemberID) (map[encounter.MemberID]int, error) {
	out := make(map[encounter.MemberID]int, len(members))
	for _, id := range members {
		out[id] = 12
		switch id {
		case sneakSiteKnight:
			out[id] = 6
		case sneakSiteCook:
			out[id] = 2
		}
	}

	return out, nil
}

// sneakSiteSheets answers every sheet as the session will (S2): thirty feet,
// HALVED while the encounter reports the member sneaking (design R8 — the
// session's rule, never the encounter's), and the passive Perception and
// re-arm distance each watcher brings. speeds records each answer for the
// rogue, so a test can read the speed a fight's first turn was offered.
type sneakSiteSheets struct {
	enc    **encounter.Encounter
	speeds *[]int
}

func (s sneakSiteSheets) Sheets(members []encounter.MemberID) (map[encounter.MemberID]encounter.SheetFacts, error) {
	passive := map[encounter.MemberID]int{sneakSiteRogue: 12, sneakSiteKnight: 10, sneakSiteCook: 9}
	out := make(map[encounter.MemberID]encounter.SheetFacts, len(members))
	for _, id := range members {
		speed := 30
		if *s.enc != nil && (*s.enc).Sneaking(id) {
			speed /= 2
		}
		if id == sneakSiteRogue {
			*s.speeds = append(*s.speeds, speed)
		}
		out[id] = encounter.SheetFacts{
			SpeedFeet:         speed,
			Actions:           []encounter.ActionView{{Ref: kingSpear, RangeFeet: 5}},
			PassivePerception: passive[id],
			WatchResetCells:   sneakSiteReset,
		}
	}

	return out, nil
}

// sneakSiteStealth is the rogue's scripted Stealth, a +7: each call takes the
// next d20 and the resolver decides against the DC it was handed, as the
// session's check seam does. An unscripted call fails the scene.
type sneakSiteStealth struct {
	d20s  []int
	asked []encounter.ResolveCheckInput
}

func (r *sneakSiteStealth) ResolveCheck(*encounter.ResolveCheckInput) (*encounter.ResolveCheckOutput, error) {
	return nil, fmt.Errorf("the sneak site makes no explicit check")
}

func (r *sneakSiteStealth) ResolveDiscoveryCheck(in *encounter.ResolveCheckInput) (*encounter.ResolveCheckOutput, error) {
	r.asked = append(r.asked, *in)
	if len(r.d20s) == 0 {
		return nil, fmt.Errorf("roll %d was not scripted", len(r.asked))
	}
	total := r.d20s[0] + 7
	r.d20s = r.d20s[1:]
	applied := in.Approaches[0]

	return &encounter.ResolveCheckOutput{Beaten: total >= applied.DC, Applied: applied, Total: total}, nil
}

// sneakSite is one run of the site.
type sneakSite struct {
	t       *testing.T
	enc     *encounter.Encounter
	stealth *sneakSiteStealth
	speeds  *[]int
}

// openTheSneakSite seats the rogue at the party start and the cast where the
// file posts them, with the rogue's d20s scripted.
func openTheSneakSite(t *testing.T, d20s ...int) *sneakSite {
	t.Helper()
	compiled := loadContent(t, sneakSiteFixture)
	members := []encounter.MemberInput{{ID: sneakSiteRogue, Kind: encounter.KindPlayer, Position: compiled.PartyStart[0].At}}
	for _, m := range compiled.Monsters {
		members = append(members, encounter.MemberInput{
			ID: encounter.MemberID(m.MemberID), Kind: encounter.KindMonster, Position: m.At,
			Faction: encounter.FactionID(m.Faction), Table: m.Table, Facing: m.FacingDegrees,
		})
	}
	site := &sneakSite{t: t, stealth: &sneakSiteStealth{d20s: d20s}, speeds: &[]int{}}
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field:     compiled.Field,
		Members:   members,
		Endings:   append([]encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}}, compiled.Endings...),
		Retention: encounter.RetentionUnbounded,
		Capabilities: encounter.Capabilities{
			Sight:         sneakSiteSight{},
			Equipment:     encounter.UnobservedEquipment{},
			Conditions:    encounter.UnobservedEquipment{},
			Sheets:        sneakSiteSheets{enc: &site.enc, speeds: site.speeds},
			Participation: &castleStanding{down: map[encounter.MemberID]bool{}},
			Initiative:    orderAsGiven{},
			Driver:        encounter.TableDriver{Roller: rollsOne{}},
			Roller:        rollsOne{},
			CheckResolver: site.stealth,
			Witness:       nobodyPerceivesAnything{},
			Actors: encounter.Actors{
				Striker:   cellsStriker{calls: &[]cellsStrike{}},
				Mover:     quietMover{},
				Announcer: quietAnnouncer{},
			},
		},
	})
	require.NoError(t, err)
	site.enc = enc

	return site
}

// sneak declares the rogue's mode.
func (s *sneakSite) sneak() {
	s.t.Helper()
	out, err := s.enc.SetSneaking(&encounter.SetSneakingInput{Member: sneakSiteRogue, Sneaking: true})
	require.NoError(s.t, err)
	require.True(s.t, out.Sneaking)
}

// walkTo steps the rogue along the pathfinder's route to a cell, and stops
// early only if a fight takes the walk.
func (s *sneakSite) walkTo(to spatial.Position) {
	s.t.Helper()
	route, err := s.enc.Route(encounter.RouteInput{Mover: sneakSiteRogue, Policy: encounter.MoveToward, Anchor: to, Budget: 60})
	require.NoError(s.t, err)
	path := route.Path
	if len(path) == 0 || path[len(path)-1] != to {
		path = append(path, to)
	}
	for i, cell := range path {
		if positionOf(s.t, s.enc, sneakSiteRogue) == cell {
			continue
		}
		_, err := s.enc.Step(&encounter.StepInput{Member: sneakSiteRogue, To: cell, EndWalk: i == len(path)-1})
		require.NoError(s.t, err, "the rogue steps to %v", cell)
		if clockKind(s.t, s.enc, sneakSiteRogue) == encounter.ClockTurn {
			return
		}
	}
}

// holding is observer's sight holding of subject, or nil.
func (s *sneakSite) holding(observer, subject encounter.MemberID) *perception.Holding {
	s.t.Helper()
	held, err := s.enc.View(&encounter.ViewInput{Member: observer})
	require.NoError(s.t, err)
	for i := range held {
		if held[i].Channel == perception.Sight && encounter.MemberID(held[i].Subject) == subject {
			return &held[i]
		}
	}

	return nil
}

// snuck is every snuck beat a member's story holds.
func (s *sneakSite) snuck(audience encounter.MemberID) []map[string]any {
	s.t.Helper()
	story, err := s.enc.Story(&encounter.StoryInput{Audience: audience})
	require.NoError(s.t, err)
	var out []map[string]any
	for _, entry := range story {
		var body map[string]any
		require.NoError(s.t, json.Unmarshal(entry.Payload, &body))
		if body["beat"] == encounter.BeatSnuck {
			out = append(out, body)
		}
	}

	return out
}

// TestSneakSite is the sneaking slice's scenario, on a site authored in the
// studio: one roll per entry into the knight's zone, a won roll keeping the
// rogue out of the knight's sight while the walk goes on, a second roll only
// after the rogue stood the re-arm distance away and came back — and, in the
// variants, a loss that forms the fight at full speed and a loss against the
// cook that forms none.
func TestSneakSite(t *testing.T) {
	t.Run("past the knight", func(t *testing.T) {
		s := openTheSneakSite(t, 15, 15)
		start := positionOf(t, s.enc, sneakSiteRogue)
		require.Nil(t, s.holding(sneakSiteKnight, sneakSiteRogue), "the rogue starts out of the knight's sight")
		s.sneak()
		require.Empty(t, s.stealth.asked, "declared out of every watcher's sight: nothing rolls")

		s.walkTo(sneakSiteFarEnd)
		require.Equal(t, sneakSiteFarEnd, positionOf(t, s.enc, sneakSiteRogue), "the walk continues to its end (R16)")
		require.Len(t, s.stealth.asked, 1, "one entry, one roll")
		require.Equal(t, sneakSiteRogue, s.stealth.asked[0].Member)
		require.Equal(t, []encounter.CheckApproach{{Ability: "stealth", DC: 10}}, s.stealth.asked[0].Approaches)
		beats := s.snuck(sneakSiteRogue)
		require.Len(t, beats, 1)
		require.Equal(t, string(sneakSiteKnight), beats[0]["watcher"], "the rogue sees the knight it beat")
		require.Equal(t, true, beats[0]["beaten"])
		require.Empty(t, s.snuck(sneakSiteKnight), "the roll is the rogue's alone")
		require.Empty(t, s.snuck(sneakSiteCook))
		require.Nil(t, s.holding(sneakSiteKnight, sneakSiteRogue), "the knight's holdings never gained the rogue")
		require.Equal(t, encounter.ClockWorld, clockKind(t, s.enc, sneakSiteRogue), "no fight formed")

		// Out past the re-arm distance, behind the wall, and back in.
		s.walkTo(start)
		require.Len(t, s.stealth.asked, 1, "leaving rolls nothing")
		s.walkTo(sneakSiteFarEnd)
		require.Len(t, s.stealth.asked, 2, "the second entry rolls again")
		require.Len(t, s.snuck(sneakSiteRogue), 2)
		require.Nil(t, s.holding(sneakSiteKnight, sneakSiteRogue))
		require.Equal(t, encounter.ClockWorld, clockKind(t, s.enc, sneakSiteRogue))
		require.Contains(t, *s.speeds, 15, "the session's half pace was asked while the rogue sneaked")
	})

	t.Run("a loss forms the fight at full speed", func(t *testing.T) {
		s := openTheSneakSite(t, 2)
		s.sneak()
		s.walkTo(sneakSiteFarEnd)
		require.Len(t, s.stealth.asked, 1)
		require.Equal(t, false, s.snuck(sneakSiteRogue)[0]["beaten"])
		require.Equal(t, encounter.ClockTurn, clockKind(t, s.enc, sneakSiteRogue), "a lost roll is first contact")
		h := s.holding(sneakSiteKnight, sneakSiteRogue)
		require.NotNil(t, h)
		require.NotEmpty(t, h.CurrentVia, "the knight sees the rogue")
		require.False(t, s.enc.Sneaking(sneakSiteRogue), "formation ended the mode")
		require.Nil(t, s.enc.ToData().SneakVerdicts)

		// The fight's first turn reads the rogue's speed after formation:
		// full, because the mode ended before any budget asked.
		*s.speeds = (*s.speeds)[:0]
		_, err := s.enc.EndTurn(&encounter.EndTurnInput{Member: sneakSiteRogue})
		if err != nil {
			require.ErrorIs(t, err, encounter.ErrNotActive)
		}
		require.NotEmpty(t, *s.speeds)
		for _, speed := range *s.speeds {
			require.Equal(t, 30, speed, "a caught sneaker fights at full speed")
		}
		story, err := s.enc.Story(&encounter.StoryInput{Audience: sneakSiteRogue})
		require.NoError(t, err)
		formed := false
		for _, entry := range story {
			var body map[string]any
			require.NoError(t, json.Unmarshal(entry.Payload, &body))
			formed = formed || body["beat"] == encounter.BeatFightStarted
		}
		require.True(t, formed, "the fight formed by today's contact law")
	})

	t.Run("a loss against the cook forms no fight", func(t *testing.T) {
		// A 1 is 8 against the cook's passive 9.
		s := openTheSneakSite(t, 1)
		s.sneak()
		s.walkTo(sneakSiteBesideCook)
		require.Len(t, s.stealth.asked, 1, "the cook's zone was entered")
		require.Equal(t, []encounter.CheckApproach{{Ability: "stealth", DC: 9}}, s.stealth.asked[0].Approaches)
		require.Equal(t, false, s.snuck(sneakSiteRogue)[0]["beaten"])
		require.Equal(t, encounter.ClockWorld, clockKind(t, s.enc, sneakSiteRogue), "a civil watcher forms no fight")
		h := s.holding(sneakSiteCook, sneakSiteRogue)
		require.NotNil(t, h)
		require.NotEmpty(t, h.CurrentVia, "the cook holds the rogue")
	})
}
