// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// conceal_test.go is living-world slice 1, wave 1b (rpg-toolkit#1371): the
// composition acts on concealment. The scenes here are the ruled design's
// own pins (rpg-project#350/#351), one per law — search's audience-of-one,
// the answer that never leaks the question, the two knowledge moments,
// presence piercing, reveal beats for perceivers and late arrivals, and the
// blob the knowledge rides.
//
// The fixture is one hall with a concealed door in each of two seams:
//
//	x: 0..4 hall | 5..8 annex | 9..11 vault (CONCEALED)
//	cellar at (0,8)..(3,11), below the hall — no concealed structure at all
//
// The hall|annex seam is walled at HEIGHT 2 except row 3 (veil-door,
// concealed, closed — a concealed door between two VISIBLE spaces, the
// masquerade case) and row 6 (an open, doorless gap, so the annex is
// honestly reachable). The annex|vault seam is walled at default height
// except row 3 (vault-door, concealed, closed — the door a hidden room
// hides behind).
const (
	concealRow  = 3
	concealGap  = 6
	seamHeight  = 2.0
	veilDoor    = "veil-door"
	vaultDoor   = "vault-door"
	hallRegion  = "hall"
	annexRegion = "annex"
	vaultRegion = "vault"
	cellarRgn   = "cellar"
)

var (
	seeker = core.EntityID("seeker")
	buddy  = core.EntityID("buddy")
	loner  = core.EntityID("loner")
	lurker = core.EntityID("lurker")

	seekerCell = spatial.Position{X: 2, Y: concealRow}
	buddyCell  = spatial.Position{X: 6, Y: concealRow}
	lonerCell  = spatial.Position{X: 1, Y: 9}
	lurkerCell = spatial.Position{X: 10, Y: concealRow}
)

// veilFind and vaultFind are the authored find checks, multi-approach on the
// veil so the sweep hands the resolver the whole authored list.
func veilFind() []encounter.CheckApproach {
	return []encounter.CheckApproach{
		{Ability: "perception", DC: 15},
		{Ability: "investigation", Tool: "dnd5e:item:magnifying-glass", DC: 12},
	}
}

func vaultFind() []encounter.CheckApproach {
	return []encounter.CheckApproach{{Ability: "perception", DC: 12}}
}

// withHeight authors a height onto every wall of a seam — the fixture for
// the mask-height pin, where the expectation is this authored value.
func withHeight(walls []encounter.WallInput, h float64) []encounter.WallInput {
	out := make([]encounter.WallInput, len(walls))
	for i, w := range walls {
		w.Height = h
		out[i] = w
	}
	return out
}

// concealField is the fixture dungeon described in the file comment.
func concealField() encounter.FieldInput {
	return encounter.FieldInput{
		Canvas: pointyCanvas(),
		Regions: []encounter.RegionInput{
			rectRegion(hallRegion, 0, 0, 5, 8),
			rectRegion(annexRegion, 5, 0, 4, 8),
			func() encounter.RegionInput {
				r := rectRegion(vaultRegion, 9, 0, 3, 8)
				r.Concealed = true
				return r
			}(),
			rectRegion(cellarRgn, 0, 8, 4, 4),
		},
		Walls: append(
			withHeight(seamWallExcept(4, 8, concealRow, concealGap), seamHeight),
			seamWallExcept(8, 8, concealRow)...),
		Doors: []encounter.DoorInput{
			{ID: veilDoor, Edges: doorEdgesAcross(4, concealRow), State: encounter.DoorIsClosed(), Concealed: veilFind()},
			{ID: vaultDoor, Edges: doorEdgesAcross(8, concealRow), State: encounter.DoorIsClosed(), Concealed: vaultFind()},
		},
	}
}

// twinField is the dungeon a non-knower's atlas claims to be: the same
// field HONESTLY AUTHORED with no vault, no doors, and a real height-2 wall
// where the veil-door hides — the never-authored yardstick made literal.
// The annex|vault seam walls cannot exist here at all (their far endpoints
// would be off floor), which is exactly why the projection withholds them.
func twinField() encounter.FieldInput {
	return encounter.FieldInput{
		Canvas: pointyCanvas(),
		Regions: []encounter.RegionInput{
			rectRegion(hallRegion, 0, 0, 5, 8),
			rectRegion(annexRegion, 5, 0, 4, 8),
			rectRegion(cellarRgn, 0, 8, 4, 4),
		},
		Walls: withHeight(seamWallExcept(4, 8, concealGap), seamHeight),
	}
}

type ConcealSuite struct {
	suite.Suite

	witness *scriptedWitness
}

func TestConcealSuite(t *testing.T) {
	suite.Run(t, new(ConcealSuite))
}

func (s *ConcealSuite) SetupTest() {
	s.witness = &scriptedWitness{perceivers: map[encounter.DoorID][]encounter.MemberID{}}
}

// partyMembers is the standing cast: seeker in the hall, buddy in the
// annex, loner in the cellar. Scenes that need somebody inside the vault
// append lurker.
func partyMembers(inVault bool) []encounter.MemberInput {
	members := []encounter.MemberInput{
		{ID: seeker, Kind: encounter.KindPlayer, Position: seekerCell},
		{ID: buddy, Kind: encounter.KindPlayer, Position: buddyCell},
		{ID: loner, Kind: encounter.KindPlayer, Position: lonerCell},
	}
	if inVault {
		members = append(members, encounter.MemberInput{ID: lurker, Kind: encounter.KindPlayer, Position: lurkerCell})
	}
	return members
}

func (s *ConcealSuite) open(resolver encounter.CheckResolver, inVault bool) *encounter.Encounter {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{}, Announcer: quietAnnouncer{},
		CheckResolver: resolver, Witness: s.witness,
		Field:   concealField(),
		Members: partyMembers(inVault),
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err)
	return enc
}

// beatsFor reads one member's story down to the named beat kind, decoded —
// recipient scoping asserted through the same read a client makes.
func (s *ConcealSuite) beatsFor(enc *encounter.Encounter, member core.EntityID, kind string) []map[string]any {
	story, err := enc.Story(&encounter.StoryInput{Audience: member})
	s.Require().NoError(err)
	beats := make([]map[string]any, 0)
	for _, entry := range story {
		var beat map[string]any
		s.Require().NoError(json.Unmarshal(entry.Payload, &beat))
		if beat["beat"] == kind {
			beats = append(beats, beat)
		}
	}
	return beats
}

// hasBoundary reports whether the atlas draws a boundary in the crossing
// between two authored [col,row] pairs, and returns it.
func hasBoundary(atlas encounter.Atlas, a, b spatial.Position) (encounter.AtlasBoundary, bool) {
	ca, cb := cellAt(int(a.X), int(a.Y)), cellAt(int(b.X), int(b.Y))
	for _, bd := range atlas.Boundaries {
		if (bd.From == ca && bd.To == cb) || (bd.From == cb && bd.To == ca) {
			return bd, true
		}
	}
	return encounter.AtlasBoundary{}, false
}

// hasDoorway reports whether the atlas lists any doorway for a door.
func hasDoorway(atlas encounter.Atlas, door encounter.DoorID) bool {
	for _, dw := range atlas.Doorways {
		if dw.Door == door {
			return true
		}
	}
	return false
}

// doorsListed reports whether a member-scoped door list carries a door.
func doorsListed(doors []encounter.Door, id encounter.DoorID) bool {
	for _, d := range doors {
		if d.ID == id {
			return true
		}
	}
	return false
}

// TestAConcealedFieldRefusesConstructionWithoutItsCapabilities: the two
// concealment capabilities are supplied-never-defaulted, refused at Setup
// AND Load exactly when the field carries concealed structure.
func (s *ConcealSuite) TestAConcealedFieldRefusesConstructionWithoutItsCapabilities() {
	setup := func(resolver encounter.CheckResolver, witness encounter.Witness) error {
		_, err := encounter.NewEncounter(&encounter.SetupInput{
			Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
			TurnDriver: passDriver{}, Striker: passStriker{}, Announcer: quietAnnouncer{},
			CheckResolver: resolver, Witness: witness,
			Field:   concealField(),
			Members: partyMembers(false),
			Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
		})
		return err
	}

	s.Run("setup without a resolver", func() {
		s.Require().ErrorIs(setup(nil, s.witness), encounter.ErrNoCheckResolver)
	})
	s.Run("setup without a witness", func() {
		s.Require().ErrorIs(setup(findsNothing{}, nil), encounter.ErrNoWitness)
	})

	data := s.open(findsNothing{}, false).ToData()
	load := func(resolver encounter.CheckResolver, witness encounter.Witness) error {
		_, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
			Data:  data,
			Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
			TurnDriver: passDriver{}, Striker: passStriker{}, Announcer: quietAnnouncer{},
			CheckResolver: resolver, Witness: witness,
		})
		return err
	}
	s.Run("load without a resolver", func() {
		s.Require().ErrorIs(load(nil, s.witness), encounter.ErrNoCheckResolver)
	})
	s.Run("load without a witness", func() {
		s.Require().ErrorIs(load(findsNothing{}, nil), encounter.ErrNoWitness)
	})
}

// TestAPlainFieldNeedsNoCapabilitiesAndBuildsNoWorld: zero behavior change
// for a dungeon without concealment — construction accepts nil capabilities,
// the member-scoped reads ARE the unscoped ones, the blob writes no world
// key (the exact bytes every pre-concealment blob already has), and a
// search answers without machinery.
func (s *ConcealSuite) TestAPlainFieldNeedsNoCapabilitiesAndBuildsNoWorld() {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{}, Announcer: quietAnnouncer{},
		Field: doorField(3, encounter.DoorIsClosed(), "plain-door", 1),
		Members: []encounter.MemberInput{
			{ID: nessa, Kind: encounter.KindPlayer, Position: nessaCell},
			{ID: orin, Kind: encounter.KindPlayer, Position: orinCell},
		},
		Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
	})
	s.Require().NoError(err, "nil capabilities are legal when nothing is concealed")

	full, err := enc.Atlas()
	s.Require().NoError(err)
	scoped, err := enc.AtlasFor(nessa)
	s.Require().NoError(err)
	s.Equal(full, scoped, "the member-scoped atlas IS the atlas")

	doors, err := enc.DoorsFor(nessa)
	s.Require().NoError(err)
	s.Equal(enc.Doors(), doors, "the member-scoped door list IS the door list")

	blob, err := json.Marshal(enc.ToData())
	s.Require().NoError(err)
	s.NotContains(string(blob), `"world"`, "a plain dungeon writes no world key at all")

	out, err := enc.Search(&encounter.SearchInput{Member: nessa, Region: doorWest})
	s.Require().NoError(err, "searching a plain dungeon is legal — refusing would answer the question")
	s.Equal(&encounter.SearchOutput{}, out)
}

// TestSearchRevealsTheDoorToTheSearcherAlone: success writes the fact with
// audience = the searcher, and every read agrees — the beat is on the
// searcher's story only, the door appears in the searcher's lists only, and
// a party-mate's atlas still wears the mask. Finding sweeps only the
// searched region: the vault-door touches no hall cell and stays unfound.
func (s *ConcealSuite) TestSearchRevealsTheDoorToTheSearcherAlone() {
	enc := s.open(findsEverything{}, false)

	_, err := enc.Search(&encounter.SearchInput{Member: seeker, Region: hallRegion})
	s.Require().NoError(err)

	revealed := s.beatsFor(enc, seeker, "door_revealed")
	s.Require().Len(revealed, 1, "one door entered the searcher's knowledge")
	s.Equal(veilDoor, revealed[0]["door"])
	s.Equal("closed", revealed[0]["state"], "the beat carries the door's live state")
	s.Empty(s.beatsFor(enc, buddy, "door_revealed"), "the party-mate heard nothing")
	s.Empty(s.beatsFor(enc, seeker, "region_revealed"), "finding a CLOSED door reveals no region")

	seekerDoors, err := enc.DoorsFor(seeker)
	s.Require().NoError(err)
	s.True(doorsListed(seekerDoors, veilDoor), "the searcher's door list gains it")
	s.False(doorsListed(seekerDoors, vaultDoor), "the unsearched seam stays secret")

	buddyDoors, err := enc.DoorsFor(buddy)
	s.Require().NoError(err)
	s.False(doorsListed(buddyDoors, veilDoor), "the party-mate's list does not")

	seekerAtlas, err := enc.AtlasFor(seeker)
	s.Require().NoError(err)
	s.True(hasDoorway(seekerAtlas, veilDoor), "the doorways arrive")
	if _, masked := hasBoundary(seekerAtlas, spatial.Position{X: 4, Y: concealRow}, spatial.Position{X: 5, Y: concealRow}); masked {
		s.Fail("the mask must come off for the finder")
	}

	buddyAtlas, err := enc.AtlasFor(buddy)
	s.Require().NoError(err)
	s.False(hasDoorway(buddyAtlas, veilDoor), "the party-mate's atlas still withholds the doorways")
	if _, masked := hasBoundary(buddyAtlas, spatial.Position{X: 4, Y: concealRow}, spatial.Position{X: 5, Y: concealRow}); !masked {
		s.Fail("and still wears the mask")
	}
}

// TestSearchNeverSaysWhetherThereWasAnythingToFind: a failed check and an
// empty region answer with the same bytes everywhere a caller can look —
// the output, every member's story, and the persisted blob.
func (s *ConcealSuite) TestSearchNeverSaysWhetherThereWasAnythingToFind() {
	enc := s.open(findsNothing{}, false)

	before, err := json.Marshal(enc.ToData())
	s.Require().NoError(err)

	failed, err := enc.Search(&encounter.SearchInput{Member: seeker, Region: hallRegion})
	s.Require().NoError(err, "a failed search is an outcome, not an error")
	afterFailed, err := json.Marshal(enc.ToData())
	s.Require().NoError(err)

	empty, err := enc.Search(&encounter.SearchInput{Member: loner, Region: cellarRgn})
	s.Require().NoError(err)
	afterEmpty, err := json.Marshal(enc.ToData())
	s.Require().NoError(err)

	s.Equal(failed, empty, "the two outputs are the same bytes")
	s.Equal(string(before), string(afterFailed), "a failed check leaves no trace in the blob")
	s.Equal(string(before), string(afterEmpty), "an empty region leaves the same none")
}

// TestSearchAsksTheResolverOncePerUnfoundDeclaration: the sweep covers
// exactly the region's concealed doors, hands each one's whole authored
// approach list over, and never re-rolls a door already found. The annex
// touches BOTH doors — the veil's far cell is annex floor — so its sweep is
// the two-declaration case.
func (s *ConcealSuite) TestSearchAsksTheResolverOncePerUnfoundDeclaration() {
	recorder := &recordingResolver{inner: findsEverything{}}
	s.witness.perceivers = map[encounter.DoorID][]encounter.MemberID{}
	enc := s.open(recorder, false)

	_, err := enc.Search(&encounter.SearchInput{Member: buddy, Region: annexRegion})
	s.Require().NoError(err)

	s.Require().Len(recorder.asked, 2, "one roll per concealed declaration in the region")
	s.Equal(vaultFind(), recorder.asked[0].Approaches, "sorted door order: vault-door first")
	s.Equal(veilFind(), recorder.asked[1].Approaches, "the whole authored list, verbatim")
	s.Equal(buddy, core.EntityID(recorder.asked[0].Member))

	_, err = enc.Search(&encounter.SearchInput{Member: buddy, Region: annexRegion})
	s.Require().NoError(err)
	s.Len(recorder.asked, 2, "a found door is never re-rolled")
}

// TestSearchRefusesARegionTheSearcherIsNotIn: v1's presence rule — and a
// region that does not exist refuses with the same bytes, so a guessed ID
// cannot probe for hidden rooms.
func (s *ConcealSuite) TestSearchRefusesARegionTheSearcherIsNotIn() {
	enc := s.open(findsNothing{}, false)

	_, elsewhere := enc.Search(&encounter.SearchInput{Member: seeker, Region: annexRegion})
	s.Require().ErrorIs(elsewhere, encounter.ErrElsewhere)

	_, hidden := enc.Search(&encounter.SearchInput{Member: seeker, Region: vaultRegion})
	s.Require().ErrorIs(hidden, encounter.ErrElsewhere)

	_, ghost := enc.Search(&encounter.SearchInput{Member: seeker, Region: "no-such-region"})
	s.Require().ErrorIs(ghost, encounter.ErrElsewhere)

	s.Equal(
		strings.ReplaceAll(ghost.Error(), "no-such-region", vaultRegion), hidden.Error(),
		"a concealed region refuses byte-identically to one that does not exist")
}

// TestFindingADoorRevealsTheDoorAndNeverTheRegion: the two knowledge
// moments. Found closed, the vault-door's doorways arrive — including the
// one cell of hidden floor per entrance, the accepted disclosure — and the
// vault itself stays byte-identical to never-authored: no cells, no region
// entry, no boundaries, and no REGION_REVEALED beat.
func (s *ConcealSuite) TestFindingADoorRevealsTheDoorAndNeverTheRegion() {
	enc := s.open(findsEverything{}, false)

	_, err := enc.Search(&encounter.SearchInput{Member: buddy, Region: annexRegion})
	s.Require().NoError(err)

	s.Empty(s.beatsFor(enc, buddy, "region_revealed"), "finding is not seeing what is behind")

	atlas, err := enc.AtlasFor(buddy)
	s.Require().NoError(err)
	s.True(hasDoorway(atlas, vaultDoor), "the found door's doorways arrive")

	vaultCell := cellAt(10, concealRow)
	for _, c := range atlas.Cells {
		s.Require().NotEqual(vaultCell, c, "no vault floor in the atlas")
	}
	for _, r := range atlas.Regions {
		s.Require().NotEqual(encounter.RegionID(vaultRegion), r.ID, "no vault region entry")
	}
}

// TestOpeningInPresenceRevealsToPerceivers: a knower opens the concealed
// door; the door's own state beat goes to its knowers alone, and every
// perceiver the witness names gets their recipient-scoped reveals — the
// door if it is news, and the region behind it, carried as the atlas slice
// the never-authored answer withheld.
func (s *ConcealSuite) TestOpeningInPresenceRevealsToPerceivers() {
	enc := s.open(findsEverything{}, false)

	_, err := enc.Search(&encounter.SearchInput{Member: buddy, Region: annexRegion})
	s.Require().NoError(err)

	s.witness.perceivers[vaultDoor] = []encounter.MemberID{buddy}
	_, err = enc.OpenDoor(&encounter.OpenDoorInput{Door: vaultDoor, Actor: buddy})
	s.Require().NoError(err)

	doorBeats := s.beatsFor(enc, buddy, "door")
	s.Require().NotEmpty(doorBeats, "the knower hears the door move")
	s.Empty(s.beatsFor(enc, seeker, "door"), "a non-knower never hears a concealed door's state beat")

	regionBeats := s.beatsFor(enc, buddy, "region_revealed")
	s.Require().Len(regionBeats, 1, "perceiving the door OPEN reveals the region")
	region, ok := regionBeats[0]["region"].(map[string]any)
	s.Require().True(ok)
	s.Equal(vaultRegion, region["id"])
	s.Len(region["cells"], 24, "the whole 3x8 slice rides the beat")
	s.NotEmpty(regionBeats[0]["boundaries"], "with every boundary touching its cells")

	s.Empty(s.beatsFor(enc, seeker, "region_revealed"), "nobody else perceived it")

	atlas, err := enc.AtlasFor(buddy)
	s.Require().NoError(err)
	found := false
	for _, r := range atlas.Regions {
		if r.ID == vaultRegion {
			found = true
		}
	}
	s.True(found, "the perceiver's atlas now carries the vault")
}

// TestALatePerceiverGetsTheirRevealOnArrival: the enumerated causes are
// examples, not a closed set — a member who was elsewhere when the door
// opened perceives present state on any later sight refresh and gets both
// reveals then.
func (s *ConcealSuite) TestALatePerceiverGetsTheirRevealOnArrival() {
	enc := s.open(findsEverything{}, false)

	_, err := enc.Search(&encounter.SearchInput{Member: buddy, Region: annexRegion})
	s.Require().NoError(err)
	s.witness.perceivers[vaultDoor] = []encounter.MemberID{buddy}
	_, err = enc.OpenDoor(&encounter.OpenDoorInput{Door: vaultDoor, Actor: buddy})
	s.Require().NoError(err)
	s.Empty(s.beatsFor(enc, seeker, "door_revealed"), "the seeker was not there")

	// The seeker walks up: the witness's answer changes, and the next
	// refresh notices present state.
	s.witness.perceivers[vaultDoor] = []encounter.MemberID{buddy, seeker}
	_, err = enc.Step(&encounter.StepInput{Member: seeker, To: cellAt(3, concealRow)})
	s.Require().NoError(err)

	doorReveals := s.beatsFor(enc, seeker, "door_revealed")
	s.Require().Len(doorReveals, 1, "the late perceiver gets the door")
	s.Equal(vaultDoor, doorReveals[0]["door"])
	s.Equal("open", doorReveals[0]["state"], "carrying its LIVE state")
	s.Len(s.beatsFor(enc, seeker, "region_revealed"), 1, "and the room behind it")
}

// TestCrossingAnUnknownOpenDoorTeachesIt: walking through a door is
// perceiving it, whatever the witness would have said — the crossing writes
// the fact and the reveal beats before the step's own refresh runs.
func (s *ConcealSuite) TestCrossingAnUnknownOpenDoorTeachesIt() {
	enc := s.open(findsEverything{}, false)

	_, err := enc.Search(&encounter.SearchInput{Member: seeker, Region: hallRegion})
	s.Require().NoError(err)
	_, err = enc.OpenDoor(&encounter.OpenDoorInput{Door: veilDoor, Actor: seeker})
	s.Require().NoError(err)
	s.Empty(s.beatsFor(enc, buddy, "door_revealed"), "the witness scripts nobody perceiving")

	_, err = enc.Step(&encounter.StepInput{Member: buddy, To: cellAt(5, concealRow)})
	s.Require().NoError(err)
	_, err = enc.Step(&encounter.StepInput{Member: buddy, To: cellAt(4, concealRow)})
	s.Require().NoError(err, "the open veil-door is crossable")

	reveals := s.beatsFor(enc, buddy, "door_revealed")
	s.Require().Len(reveals, 1, "crossing taught the crosser")
	s.Equal(veilDoor, reveals[0]["door"])

	doors, err := enc.DoorsFor(buddy)
	s.Require().NoError(err)
	s.True(doorsListed(doors, veilDoor))

	// The shared moved beat said nothing: one payload cannot name a secret
	// to knowers without naming it to everyone, so a concealed door never
	// rides it — the loner, who knows nothing, heard a plain step.
	for _, beat := range s.beatsFor(enc, loner, "moved") {
		if named, ok := beat["doors"]; ok {
			s.NotContains(named, veilDoor, "a concealed door never rides the shared moved beat")
		}
	}
}

// TestKnowledgeRidesTheBlob: load-act-save — the facts persist under the
// world key, a reload folds to the same knowledge, and an unchanged
// encounter round-trips byte-identically.
func (s *ConcealSuite) TestKnowledgeRidesTheBlob() {
	enc := s.open(findsEverything{}, false)

	_, err := enc.Search(&encounter.SearchInput{Member: seeker, Region: hallRegion})
	s.Require().NoError(err)

	data := enc.ToData()
	blob, err := json.Marshal(data)
	s.Require().NoError(err)
	s.Contains(string(blob), `"world"`, "a concealed field persists its world")
	s.Contains(string(blob), `"known:door:veil-door"`, "with the searcher's fact")

	back, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data:  data,
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{}, Announcer: quietAnnouncer{},
		CheckResolver: findsEverything{}, Witness: s.witness,
	})
	s.Require().NoError(err)

	doors, err := back.DoorsFor(seeker)
	s.Require().NoError(err)
	s.True(doorsListed(doors, veilDoor), "the finder still knows")
	buddyDoors, err := back.DoorsFor(buddy)
	s.Require().NoError(err)
	s.False(doorsListed(buddyDoors, veilDoor), "the party-mate still does not")

	reblob, err := json.Marshal(back.ToData())
	s.Require().NoError(err)
	s.Equal(string(blob), string(reblob), "an unchanged encounter round-trips byte-identically")
}

// TestABlobWorldMustMatchItsField: the trust boundary. This composition
// writes ONE fact shape — a minted kind, its entity as subject, an
// ever-member as actor, audienced to that actor alone — so a world on a
// field with nothing concealed, and every other fact shape, means the blob
// was edited and refuses by name (PR #1373 review, Minors 2 and 3).
func (s *ConcealSuite) TestABlobWorldMustMatchItsField() {
	load := func(data encounter.EncounterData) error {
		_, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
			Data:  data,
			Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
			TurnDriver: passDriver{}, Striker: passStriker{}, Announcer: quietAnnouncer{},
			CheckResolver: findsNothing{}, Witness: s.witness,
		})
		return err
	}
	// A well-formed veil-door fact to corrupt one field at a time.
	honest := func() encounter.FactData {
		return encounter.FactData{
			Kind:     "known:door:veil-door",
			Subject:  "door:veil-door",
			Actor:    string(seeker),
			Audience: []string{string(seeker)},
		}
	}
	withFact := func(f encounter.FactData) encounter.EncounterData {
		data := s.open(findsNothing{}, false).ToData()
		data.World.Facts = append(data.World.Facts, f)
		return data
	}

	s.Run("the honest shape loads", func() {
		s.Require().NoError(load(withFact(honest())))
	})
	s.Run("a kind this field does not mint", func() {
		f := honest()
		f.Kind = "known:door:some-other-dungeons-door"
		err := load(withFact(f))
		s.Require().ErrorIs(err, encounter.ErrInvalidData)
		s.Contains(err.Error(), "does not mint")
	})
	s.Run("a subject that does not match its kind", func() {
		f := honest()
		f.Subject = "door:vault-door"
		err := load(withFact(f))
		s.Require().ErrorIs(err, encounter.ErrInvalidData)
		s.Contains(err.Error(), "does not match its kind")
	})
	s.Run("an actor this encounter never had", func() {
		f := honest()
		f.Actor, f.Audience = "stranger", []string{"stranger"}
		err := load(withFact(f))
		s.Require().ErrorIs(err, encounter.ErrInvalidData)
		s.Contains(err.Error(), "no member this encounter has ever had")
	})
	s.Run("an audience that is not exactly its actor", func() {
		f := honest()
		f.Audience = []string{string(seeker), string(buddy)}
		err := load(withFact(f))
		s.Require().ErrorIs(err, encounter.ErrInvalidData)
		s.Contains(err.Error(), "audienced to exactly its actor")
	})
	s.Run("a world on a field with nothing concealed", func() {
		plain, err := encounter.NewEncounter(&encounter.SetupInput{
			Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
			TurnDriver: passDriver{}, Striker: passStriker{}, Announcer: quietAnnouncer{},
			Field: doorField(3, encounter.DoorIsClosed(), "plain-door", 1),
			Members: []encounter.MemberInput{
				{ID: nessa, Kind: encounter.KindPlayer, Position: nessaCell},
				{ID: orin, Kind: encounter.KindPlayer, Position: orinCell},
			},
			Endings: []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
		})
		s.Require().NoError(err)
		data := plain.ToData()
		data.World = &encounter.WorldData{Facts: []encounter.FactData{}}
		lerr := load(data)
		s.Require().ErrorIs(lerr, encounter.ErrInvalidData)
		s.Contains(lerr.Error(), "no concealed structure")
	})
}

// TestAnOldBlobsOccupantIsPiercedAtLoad — PR #1373 review, Minor 4: a blob
// saved between v0.41.0's carried concealment and the world existing holds
// an occupant of a concealed region with no occupancy fact. Presence
// pierces from frame one applies to loads too: the occupant knows the
// floor under their feet BEFORE any verb runs, the fact is minted for the
// next save, and their reveal beat is on their story — while a blob this
// build wrote reloads with nothing to do (the byte-identity pin in
// TestKnowledgeRidesTheBlob).
func (s *ConcealSuite) TestAnOldBlobsOccupantIsPiercedAtLoad() {
	live := s.open(findsNothing{}, true)
	data := live.ToData()
	data.World = nil // the v0.41.0-era dialect: concealment authored, no world key

	// The simulation strips the FACTS but the story blob keeps Setup's own
	// reveal beat (a true old blob would carry neither); knowledge is the
	// facts, so the load must pierce again — measure the beats it ADDS.
	beatsBefore := len(s.beatsFor(live, lurker, "region_revealed"))

	back, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data:  data,
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{}, Announcer: quietAnnouncer{},
		CheckResolver: findsNothing{}, Witness: s.witness,
	})
	s.Require().NoError(err)

	atlas, err := back.AtlasFor(lurker)
	s.Require().NoError(err)
	holdsVault := false
	for _, r := range atlas.Regions {
		holdsVault = holdsVault || r.ID == vaultRegion
	}
	s.True(holdsVault, "the occupant knows the floor under their feet before any verb runs")

	s.Len(s.beatsFor(back, lurker, "region_revealed"), beatsBefore+1, "with their reveal minted onto their own story")
	s.Empty(s.beatsFor(back, seeker, "region_revealed"), "and nobody else's")

	found := false
	for _, f := range back.ToData().World.Facts {
		if f.Kind == "known:region:"+vaultRegion && f.Actor == string(lurker) {
			found = true
		}
	}
	s.True(found, "the next save persists the minted occupancy fact")
}
