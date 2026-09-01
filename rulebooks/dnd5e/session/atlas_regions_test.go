// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"fmt"
	"math"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// AtlasRegionsSuite is rpg-project#256 at the seam: the map carries its
// regions, and the map of a world nobody has started is the map of the same
// world started.
type AtlasRegionsSuite struct {
	suite.Suite

	mgr *session.Manager
}

func TestAtlasRegionsSuite(t *testing.T) { suite.Run(t, new(AtlasRegionsSuite)) }

func (s *AtlasRegionsSuite) SetupTest() {
	mgr, err := session.NewManager(&session.Config{
		Dice: testDice{}, TurnDriver: session.Pass{}, Sessions: newFakeSessions(), Encounters: newFakeEncounters(),
		Characters: testCharacters(), Events: session.DiscardEvents{},
	})
	s.Require().NoError(err)
	s.mgr = mgr
}

// tombRegions is the three areas of a small tomb in the AUTHORED frame: an
// entry hall, a crypt, and an L-shaped gallery wrapping the crypt's corner.
// Each carries its own archetype and a different light level, so a
// projection that carried the wrong region's facts — or everybody's through
// one region — is visible.
//
// The gallery is the L: a shape no rectangle describes, which is the whole
// reason regions replaced rooms, and the shape whose pixel picture tells a
// cell converted once from a cell converted twice (see
// TestARegionCellDrawsWhereItWasAuthored).
var tombRegions = []struct {
	id, archetype string
	intensity     float64
	cells         []spatial.Position
}{
	{"hall", "entry", 1.0, rectCells(0, 0, 4, 3)},
	{"crypt", "crypt", 0.2, rectCells(4, 0, 3, 3)},
	{"gallery", "gallery", 0.6, append(rectCells(4, 3, 3, 1), rectCells(7, 0, 1, 4)...)},
}

// tomb builds the three-region tomb under the given orientation.
func tomb(o encounter.Orientation) *encounter.EncounterData {
	regions := make([]encounter.RegionInput, 0, len(tombRegions))
	for _, r := range tombRegions {
		regions = append(regions, encounter.RegionInput{
			ID: r.id, Name: r.id, Cells: r.cells, Archetype: r.archetype,
			Lighting: &encounter.Lighting{Intensity: r.intensity},
		})
	}
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Striker: encounter.RefusingStriker{}, Announcer: encQuietAnnouncer{}, Sight: encEveryoneSees{}, Initiative: encOrderAsGiven{},
		TurnDriver: encPassDriver{}, Standing: encEveryoneStanding{},
		Field: encounter.FieldInput{
			Canvas:  encounter.CanvasInput{Void: encounter.VoidIsOpaque(), Orientation: o},
			Regions: regions,
		},
		Members: []encounter.MemberInput{
			{ID: "alice", Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}},
		},
		Endings: []encounter.EndingInput{{Key: "out", Trigger: encounter.TriggerExternal{}}},
	})
	if err != nil {
		panic(fmt.Sprintf("building the tomb: %v", err))
	}
	data := enc.ToData()
	return &data
}

func (s *AtlasRegionsSuite) started(world *encounter.EncounterData) *session.Atlas {
	ctx := context.Background()
	_, err := s.mgr.StartSession(ctx, &session.StartSessionInput{Session: "sess", Encounter: "world", World: world})
	s.Require().NoError(err)
	atlas, err := s.mgr.Atlas(ctx, &session.AtlasInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)
	return atlas
}

// TestTheAtlasCarriesEveryRegion is the headline: three regions, each with
// the archetype and light level it was authored with, whose cells together
// are exactly the map's cells.
func (s *AtlasRegionsSuite) TestTheAtlasCarriesEveryRegion() {
	atlas := s.started(tomb(encounter.HexesArePointyTop()))

	s.Require().Len(atlas.Regions, 3)
	byID := map[string]session.AtlasRegion{}
	for _, r := range atlas.Regions {
		byID[r.ID] = r
	}
	s.Equal([]string{"crypt", "gallery", "hall"}, []string{atlas.Regions[0].ID, atlas.Regions[1].ID, atlas.Regions[2].ID},
		"sorted by id, the composition's own order")

	for _, want := range tombRegions {
		got, ok := byID[want.id]
		s.Require().True(ok, "region %q is on the map", want.id)
		s.Equal(want.id, got.Name, "the display name is carried verbatim")
		s.Equal(want.archetype, got.Archetype, "%s keeps its own archetype", want.id)
		s.Equal(want.intensity, got.Lighting.Intensity, "%s keeps its own light level", want.id)
		s.Len(got.Cells, len(want.cells), "%s owns every cell it was painted with", want.id)
	}

	// Every floor cell appears in exactly one region's cells.
	owned := map[spatial.Position]string{}
	for _, r := range atlas.Regions {
		for _, c := range r.Cells {
			s.Empty(owned[c], "cell %v is owned by both %s and %s", c, owned[c], r.ID)
			owned[c] = r.ID
		}
	}
	s.Len(owned, len(atlas.Cells), "the regions' cells union to the map's cells")
	for _, c := range atlas.Cells {
		s.NotEmpty(owned[c], "map cell %v belongs to some region", c)
	}
}

// TestARegionCellDrawsWhereItWasAuthored is the pixel-formula pin for a
// region's cells: a client that takes them as axial and applies the STANDARD
// pixel formula for Atlas.Layout draws the authored L, under both layouts.
//
// An external reference, not a round-trip (rpg-toolkit#1150's lesson): a
// cell converted twice — offset to axial at construction, then again here as
// if it were still offset — passes every consistency check this package can
// write and draws the gallery somewhere else. The offset picture the author
// drew is the reference: odd rows shove right under pointy-top, odd columns
// shove down under flat-top, which is the convention spatial's conversion
// pins (encounter's TestOffsetAndAxialAgreeWithSpatial).
func (s *AtlasRegionsSuite) TestARegionCellDrawsWhereItWasAuthored() {
	cases := []struct {
		name        string
		orientation encounter.Orientation
		layout      session.HexLayout
		axialPixel  func(q, r float64) (x, y float64)
		offsetPixel func(col, row float64) (x, y float64)
	}{
		{
			"pointy_top", encounter.HexesArePointyTop(), session.HexLayoutPointyTop,
			// Circumradius 1: x = √3·(q + r/2), y = 1.5·r.
			func(q, r float64) (float64, float64) { return math.Sqrt(3) * (q + r/2), 1.5 * r },
			// odd-r: odd rows shove half a cell right.
			func(col, row float64) (float64, float64) {
				return math.Sqrt(3) * (col + 0.5*float64(int(row)&1)), 1.5 * row
			},
		},
		{
			"flat_top", encounter.HexesAreFlatTop(), session.HexLayoutFlatTop,
			// Circumradius 1: x = 1.5·q, y = √3·(r + q/2).
			func(q, r float64) (float64, float64) { return 1.5 * q, math.Sqrt(3) * (r + q/2) },
			// odd-q: odd columns shove half a cell down.
			func(col, row float64) (float64, float64) {
				return 1.5 * col, math.Sqrt(3) * (row + 0.5*float64(int(col)&1))
			},
		},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			s.SetupTest()
			atlas := s.started(tomb(tc.orientation))
			s.Require().Equal(tc.layout, atlas.Layout)

			var gallery *session.AtlasRegion
			for i := range atlas.Regions {
				if atlas.Regions[i].ID == "gallery" {
					gallery = &atlas.Regions[i]
				}
			}
			s.Require().NotNil(gallery)

			// The gallery's corner cell, authored at [7,3]: the one cell
			// where the L's two arms meet, and off both axes, so a cell
			// converted twice lands somewhere else on both layouts.
			wantX, wantY := tc.offsetPixel(7, 3)
			found := false
			for _, c := range gallery.Cells {
				x, y := tc.axialPixel(c.X, c.Y)
				if math.Abs(x-wantX) < 1e-9 && math.Abs(y-wantY) < 1e-9 {
					found = true
				}
			}
			s.True(found, "some gallery cell draws exactly where the author put [7,3]")

			// And the whole L draws where it was authored: every region cell
			// lands on an authored pixel, and every authored pixel is hit.
			hit := make([]bool, len(tombRegions[2].cells))
			for _, c := range gallery.Cells {
				x, y := tc.axialPixel(c.X, c.Y)
				matched := false
				for i, a := range tombRegions[2].cells {
					ax, ay := tc.offsetPixel(a.X, a.Y)
					if math.Abs(x-ax) < 1e-9 && math.Abs(y-ay) < 1e-9 {
						hit[i], matched = true, true
					}
				}
				s.True(matched, "gallery cell %v draws on no authored cell", c)
			}
			for i, h := range hit {
				s.True(h, "authored gallery cell %v is drawn by nobody", tombRegions[2].cells[i])
			}
		})
	}
}

// TestAtlasOfIsTheSameMapAStartedSessionAnswers pins the one-producer claim
// a dungeon registry relies on: the map it previews for a world nobody has
// started is, field for field, the map a session on that world plays from.
func (s *AtlasRegionsSuite) TestAtlasOfIsTheSameMapAStartedSessionAnswers() {
	world := tomb(encounter.HexesArePointyTop())

	preview, err := s.mgr.AtlasOf(context.Background(), &session.AtlasOfInput{World: world})
	s.Require().NoError(err)

	played := s.started(world)
	s.Equal(played, preview, "one projection, one producer")
	s.Len(preview.Regions, 3, "and it is not trivially equal by being empty")
}

// TestAtlasOfRefusesWhatStartSessionRefuses: the two share a load, so a world
// that cannot be previewed is a world that cannot be started, in the same
// vocabulary.
func (s *AtlasRegionsSuite) TestAtlasOfRefusesWhatStartSessionRefuses() {
	ctx := context.Background()

	_, err := s.mgr.AtlasOf(ctx, nil)
	s.ErrorIs(err, session.ErrNilInput)

	_, err = s.mgr.AtlasOf(ctx, &session.AtlasOfInput{})
	s.ErrorIs(err, session.ErrInvalidWorld, "a nil world is not a map")

	broken := tomb(encounter.HexesArePointyTop())
	broken.Field.Canvas.Orientation = "sideways"
	_, err = s.mgr.AtlasOf(ctx, &session.AtlasOfInput{World: broken})
	s.ErrorIs(err, session.ErrInvalidWorld, "a world that will not load is refused")
	s.NotErrorIs(err, encounter.ErrNoField, "in this package's vocabulary, not the composition's")

	_, err = s.mgr.StartSession(ctx, &session.StartSessionInput{Session: "sess", Encounter: "world", World: broken})
	s.ErrorIs(err, session.ErrInvalidWorld, "and StartSession refuses the same world the same way")
}

// untouchableStores is every repository a Manager is wired with, each
// failing the test the moment anything reaches it.
type untouchableStores struct{ t *testing.T }

func (u untouchableStores) touched(what string) {
	u.t.Helper()
	u.t.Fatalf("AtlasOf reached a store: %s", what)
}

func (u untouchableStores) GetSession(context.Context, string) (*session.SessionData, error) {
	u.touched("GetSession")
	return nil, nil
}
func (u untouchableStores) SaveSession(context.Context, *session.SessionData) error {
	u.touched("SaveSession")
	return nil
}
func (u untouchableStores) GetEncounter(context.Context, string) (*encounter.EncounterData, error) {
	u.touched("GetEncounter")
	return nil, nil
}
func (u untouchableStores) SaveEncounter(context.Context, string, *encounter.EncounterData) error {
	u.touched("SaveEncounter")
	return nil
}
func (u untouchableStores) GetCharacter(context.Context, string) (*character.Data, error) {
	u.touched("GetCharacter")
	return nil, nil
}
func (u untouchableStores) SaveCharacter(context.Context, *character.Data) error {
	u.touched("SaveCharacter")
	return nil
}
func (u untouchableStores) Publish(context.Context, []session.Event) error {
	u.touched("Publish")
	return nil
}

// TestAtlasOfTouchesNoStore pins what a dungeon registry relies on when it
// calls AtlasOf for every file at boot and on every validate-only keystroke
// from a builder: the projection is pure construction, and reaches no
// session, encounter, character store or event stream. A load does not
// consult standing, sight or initiative — those fire from play — and this
// is the test that keeps it so from this side of the seam.
func (s *AtlasRegionsSuite) TestAtlasOfTouchesNoStore() {
	stores := untouchableStores{t: s.T()}
	mgr, err := session.NewManager(&session.Config{
		Dice: testDice{}, TurnDriver: session.Pass{},
		Sessions: stores, Encounters: stores, Characters: stores, Events: stores,
	})
	s.Require().NoError(err)

	atlas, err := mgr.AtlasOf(context.Background(), &session.AtlasOfInput{World: tomb(encounter.HexesArePointyTop())})
	s.Require().NoError(err)
	s.Len(atlas.Regions, 3)
}
