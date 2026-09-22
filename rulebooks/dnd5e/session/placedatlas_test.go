// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

// placedatlas_test.go drives the seam's half of rpg-api-protos#351: the
// authored FOOTPRINTS a World Builder drew reach a client on their own
// atlas, and what reaches them is enough to draw the thing, offer Hold on
// it, and know whether it is on the floor at all.
//
// Why a file rather than a scene inside holdingsscenes_test.go: a placement
// is not a prop with a bigger number on it. It has no anchor cell, its pose
// is feet on a continuous plane instead of a coordinate, and the cells it
// stands on are the engine's own derivation — which is the one field a
// client would otherwise compute for itself and get subtly wrong. The
// fixture has to be authored in feet for any of that to be exercised.
//
// # The fixture
//
// One 10x6 hall in the authored frame, with the far two columns held by a
// concealment nobody in this file ever finds. Three rectangles stand in it:
//
//	ALTAR-SLAB   a one-foot box on the centre of the cell east of alice —
//	             holdable, blocking nothing, standing on exactly one cell
//	LONG-BENCH   a wide box drawn across its facing, so it stands on MORE
//	             than one cell and the reach question has a real answer
//	VAULT-PLINTH inside the concealment, blocking movement and sight — the
//	             per-observer clause's subject, and the one placement whose
//	             two flags are both true
//
// Everything a scene asserts about a pose is stated by the fixture in the
// same terms the fixture authored it in. Nothing here re-traces a rectangle
// against the hex grid: doing that would be the second geometry the Cells
// field exists to abolish, rebuilt inside the test that is supposed to prove
// it unnecessary.

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

const (
	slabID   = "altar-slab"
	benchID  = "long-bench"
	plinthID = "vault-plinth"
)

// slabCell is where the one-foot slab stands: the cell east of alice, so
// Hold's default adjacent range reaches it from where she already is.
func slabCell() spatial.Position { return hexCell(2, 1) }

// benchCell is the cell the wide bench's ORIGIN sits on. Which cells it
// STANDS on is the engine's answer and this file never guesses it.
func benchCell() spatial.Position { return hexCell(5, 1) }

// plinthCell is inside the concealed columns.
func plinthCell() spatial.Position { return hexCell(8, 2) }

// aBoxOn is a rectangle of the given size in feet, anchored on one cell's own
// centre in the hall's plane — hallPlane() is the same frame the field
// rasterises in, so a box authored here is a box the field agrees about.
//
// Width is ACROSS the facing and Depth ALONG it, spatial's vocabulary, which
// is the vocabulary that reaches this seam.
func aBoxOn(at spatial.Position, widthFeet, depthFeet float64) spatial.FootprintPlacement {
	return spatial.FootprintPlacement{
		Footprint: spatial.Footprint{Box: &spatial.Box{W: widthFeet, D: depthFeet}},
		Origin:    hallPlane().CellCentre(at),
	}
}

// placedWorld is the hall described at the top of this file.
func placedWorld(t fataler) *encounter.EncounterData {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Striker: encounter.RefusingStriker{}, Mover: encounter.RefusingMover{},
		Announcer: encQuietAnnouncer{}, Sight: encEveryoneSees{}, Equipment: encNoHandsObserved{},
		Initiative: encOrderAsGiven{}, TurnDriver: encPassDriver{},
		Standing:      encEveryoneStanding{},
		CheckResolver: encNeverResolves{},
		Witness:       encNeverWitnesses{},
		Field: encounter.FieldInput{Canvas: pointyCanvas(),
			Regions: []encounter.RegionInput{rectRegion("hall", 0, 0, 10, 6)},
			Concealments: []encounter.ConcealmentInput{{
				ID: vaultSecret, Checks: vaultFind(), Cells: rectCells(8, 0, 2, 6),
			}},
			Placed: []encounter.PlacedPropInput{
				{
					ID:        slabID,
					Placement: aBoxOn(slabCell(), 1, 1),
					Holdable:  true,
				},
				{
					// Twelve feet long on a five-foot cell: long enough that
					// the answer to "where does it stand" cannot be one cell,
					// and the scene below never has to say which ones.
					ID:        benchID,
					Placement: aBoxOn(benchCell(), 4, 12),
					Holdable:  true,
				},
				{
					ID:                plinthID,
					Placement:         aBoxOn(plinthCell(), 4, 4),
					BlocksMovement:    true,
					BlocksLineOfSight: true,
				},
			},
		},
		Members: []encounter.MemberInput{
			{ID: "alice", Kind: encounter.KindPlayer, Position: hexCell(1, 1)},
			{ID: "bob", Kind: encounter.KindPlayer, Position: hexCell(5, 4)},
		},
		Endings: []encounter.EndingInput{{Key: "out", Trigger: encounter.TriggerExternal{}}},
	})
	if err != nil {
		t.Fatalf("building placed world: %v", err)
	}
	data := enc.ToData()
	return &data
}

type PlacedAtlasSuite struct {
	suite.Suite

	mgr *session.Manager
}

func TestPlacedAtlasSuite(t *testing.T) { suite.Run(t, new(PlacedAtlasSuite)) }

func (s *PlacedAtlasSuite) SetupTest() {
	mgr, err := session.NewManager(&session.Config{PresentationIDs: testPresentationIDs{},
		Dice: testDice{}, TurnDriver: session.Pass{},
		Sessions: newFakeSessions(), Encounters: newFakeEncounters(),
		Characters: testCharacters(), Events: session.DiscardEvents{},
	})
	s.Require().NoError(err)
	s.mgr = mgr

	_, err = mgr.StartSession(context.Background(), &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: placedWorld(s.T()),
	})
	s.Require().NoError(err)
}

// placedOf is one member's own placements, keyed by the author's id.
func (s *PlacedAtlasSuite) placedOf(member string) map[string]session.AtlasPlacedProp {
	s.T().Helper()
	atlas, err := s.mgr.Atlas(context.Background(), &session.AtlasInput{Session: "sess", Member: member})
	s.Require().NoError(err)

	out := make(map[string]session.AtlasPlacedProp, len(atlas.Placed))
	for _, p := range atlas.Placed {
		out[p.ID] = p
	}
	return out
}

// TestAPlacementReachesAClientWhole is the wire's own claim, driven through
// the SDK rather than through projectAtlas: the id, the pose in feet, both
// blocking answers, the offer, and the cells — read off a real session that
// loaded a stored world back out of its repository.
func (s *PlacedAtlasSuite) TestAPlacementReachesAClientWhole() {
	placed := s.placedOf("alice")
	s.Require().Contains(placed, slabID)

	slab := placed[slabID]
	s.Run("the pose is the one the fixture authored, in feet", func() {
		s.Equal(1.0, slab.Placement.Width, "spatial's W, the extent ACROSS the facing")
		s.Equal(1.0, slab.Placement.Depth)

		centre := hallPlane().CellCentre(slabCell())
		s.Equal(session.FootprintPoint{X: centre.X, Y: centre.Y}, slab.Placement.Origin,
			"feet on the plane, not a cell and not the atlas's axial frame")
		s.Zero(slab.Placement.Facing, "due east, which is a real facing and not an absence")
		s.Zero(slab.Placement.LocalOffset.X, "no nudge authored, so the box is centred on its origin")
		s.Zero(slab.Placement.LocalOffset.Y)
	})

	s.Run("a rectangle authored on one cell's centre stands on that cell", func() {
		// The fixture's own statement rather than a re-derivation: a one-foot
		// box on a five-foot cell's centre is inside that cell whatever the
		// rasteriser does at the edges, which is the whole reason the slab is
		// a foot wide.
		s.Equal([]spatial.Position{slabCell()}, slab.Cells)
	})

	s.Run("the offer and the two blocking answers are the author's", func() {
		s.True(slab.Holdable, "a client offers Hold here and never guesses from the id")
		s.False(slab.BlocksMovement, "a slab you walk around is not the only kind")
		s.False(slab.BlocksLineOfSight)

		s.Require().Contains(placed, benchID)
		s.True(placed[benchID].Holdable)
	})
}

// TestAWideRectangleStandsOnEveryCellItCovers is the field's reason for
// existing, stated where a client would read it: a rectangle bigger than a
// cell has NO single cell, and the answer it gets is a list.
//
// The claim is deliberately not a count. What the scene asserts is that the
// list is bigger than the one-cell case beside it and that the engine agrees
// with every entry — which a length assertion would not say, and which a
// number pinned today would start lying about the day the rasteriser's
// epsilon moves.
func (s *PlacedAtlasSuite) TestAWideRectangleStandsOnEveryCellItCovers() {
	bench := s.placedOf("alice")[benchID]
	s.Require().NotEmpty(bench.Cells)
	s.Greater(len(bench.Cells), len(s.placedOf("alice")[slabID].Cells),
		"twelve feet of bench on a five-foot cell cannot stand on as little as a one-foot box does")

	seen := map[spatial.Position]bool{}
	for _, c := range bench.Cells {
		s.False(seen[c], "%v is named twice: a placement stands on a SET of cells", c)
		seen[c] = true
	}
}

// TestTheCellsTheAtlasNamesAreTheCellsHoldJudges is the agreement the field
// is for: an offer a client makes from this list and a refusal the engine
// gives cannot disagree, because both read the same derivation.
//
// Driven off the atlas's OWN list rather than a cell this file picked —
// a wrong list puts the joiner in the wrong place, and the hold fails.
func (s *PlacedAtlasSuite) TestTheCellsTheAtlasNamesAreTheCellsHoldJudges() {
	ctx := context.Background()
	bench := s.placedOf("alice")[benchID]
	s.Require().NotEmpty(bench.Cells)

	// STANDING ON ONE IS DISTANCE ZERO AND IN REACH. Range 0 means adjacent,
	// and the placement's own footing counts as adjacency — the rule
	// AtlasPlacedProp.Cells documents, checked rather than restated.
	_, err := s.mgr.Join(ctx, &session.JoinInput{
		Session: "sess", Member: "carol", Position: bench.Cells[0]})
	s.Require().NoError(err, "a non-blocking rectangle's footing is standable floor")

	_, err = s.mgr.Hold(ctx, &session.HoldInput{
		Session: "sess", Member: "carol", Target: benchID})
	s.Require().NoError(err, "a member standing on a cell the atlas named is in reach of it")

	// And the converse, so the scene is a claim about the LIST rather than
	// about Hold being generous: bob is across the hall from every cell on
	// it, and the same verb refuses him.
	_, err = s.mgr.Hold(ctx, &session.HoldInput{
		Session: "sess", Member: "bob", Target: slabID})
	s.Require().ErrorIs(err, session.ErrOutOfRange,
		"the reach the offer was made from is the reach the refusal measures")
}

// TestAHeldPlacementIsOnNobodysMap is the absence law's second clause,
// driven through the SDK: a rectangle somebody picked up left the floor for
// EVERY member, and nothing marks it — it is simply gone.
func (s *PlacedAtlasSuite) TestAHeldPlacementIsOnNobodysMap() {
	ctx := context.Background()
	s.Require().Contains(s.placedOf("bob"), slabID, "it stands there to begin with")

	// Alice is already beside it, so the default range — adjacent — is the
	// reach being exercised.
	_, err := s.mgr.Hold(ctx, &session.HoldInput{
		Session: "sess", Member: "alice", Target: slabID})
	s.Require().NoError(err)

	for _, who := range []string{"alice", "bob"} {
		placed := s.placedOf(who)
		s.NotContains(placed, slabID,
			"%s's map still shows a rectangle that is in somebody's hands", who)
		s.Contains(placed, benchID, "%s: and the one nobody touched still stands", who)
	}
}

// TestAConcealedPlacementIsWithheldWhole is the per-observer clause — the
// only one of the three that can differ between two members of one party.
//
// The plinth stands inside a concealment neither player has found, so
// neither is told about it, while the UNSCOPED read — the builder's own,
// which answers the host's whole truth — still has it. That pairing is what
// makes this a claim about the filter rather than about the fixture having
// no plinth in it.
func (s *PlacedAtlasSuite) TestAConcealedPlacementIsWithheldWhole() {
	ctx := context.Background()

	for _, who := range []string{"alice", "bob"} {
		s.NotContains(s.placedOf(who), plinthID,
			"%s has not found the vault and is told nothing about what stands in it", who)
	}

	authored, err := s.mgr.AtlasOf(ctx, &session.AtlasOfInput{World: placedWorld(s.T())})
	s.Require().NoError(err)

	whole := map[string]session.AtlasPlacedProp{}
	for _, p := range authored.Placed {
		whole[p.ID] = p
	}
	s.Require().Contains(whole, plinthID, "the builder's read answers for authored content, concealed or not")
	s.True(whole[plinthID].BlocksMovement, "and both of its flags cross: all four combinations are real")
	s.True(whole[plinthID].BlocksLineOfSight)
	s.False(whole[plinthID].Holdable, "a rectangle nobody declared holdable is scenery")

	// PRESENTED WHOLE, NEVER TRIMMED. A placement the filter lets through
	// says the same cells the unscoped read gives it — a footing cut down to
	// what a recipient can see would be geometry the author never drew.
	alice := s.placedOf("alice")
	for _, id := range []string{slabID, benchID} {
		s.Require().Contains(alice, id)
		s.Equal(whole[id].Cells, alice[id].Cells, "%s: the whole footing, or none of it", id)
		s.Equal(whole[id].Placement, alice[id].Placement, "%s: and the same pose", id)
	}
}
