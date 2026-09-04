// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

// holdings_test.go is living-world SLICE 2 (rpg-project#368,
// recover-the-artifact): Loot, Take, holdings, and the ending that fires
// when somebody walks out of a bound exit carrying the artifact.
//
// One scene per row of the design's §8 acceptance table that this module
// proves, plus the two secrecy scenes design P3 turns on.
//
// # The fixture
//
// The tomb in miniature, laid out so every rule has a cell to be true at:
//
//	x: 0..3 hall | 4..7 tomb | 8..10 vault (CONCEALED)
//
// The hall|tomb seam is walled except an open gap at row 5, so a member
// walks between them through that one crossing. The tomb|vault seam is
// walled except row 2, where the concealed VAULT DOOR stands — the only way
// in, and shut.
//
// Four props. The HEIRLOOM and the CHALICE are takeable and stand in the
// TOMB, where the scenes about carrying things can reach them without first
// having to find a secret. The RELIC is takeable and stands inside the
// concealed vault, which is what the probe-law scene is about. The PILLAR
// says nothing about being takeable, which is what every prop was before
// this slice.
//
// The party: RAIDER and PARTNER in the hall, the CAPTAIN standing in the
// tomb knowing the vault door. FRONT-GATE is the authored exit at the cell
// the raider starts on, and SIDE-DOOR is a second authored exit no scenario
// binds.

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

const (
	vaultSeamRow = 2
	hallGapRow   = 5
	tombVault    = "tomb-vault-door"
	heirloom     = "heirloom"
	chalice      = "chalice"
	relic        = "relic"
	pillar       = "pillar"
	frontGate    = "front-gate"
	sideDoor     = "side-door"
	recovered    = "recovered"
)

var (
	raider  = core.EntityID("raider")
	partner = core.EntityID("partner")
	captain = core.EntityID("captain")
	sentry  = core.EntityID("sentry")

	raiderCell   = spatial.Position{X: 1, Y: 3}
	partnerCell  = spatial.Position{X: 2, Y: 3}
	captainCell  = spatial.Position{X: 5, Y: 3}
	sentryCell   = spatial.Position{X: 5, Y: 4}
	heirloomCell = spatial.Position{X: 6, Y: 3}
	chaliceCell  = spatial.Position{X: 6, Y: 4}
	relicCell    = spatial.Position{X: 9, Y: 3}
	pillarCell   = spatial.Position{X: 6, Y: 6}
	sideDoorCell = spatial.Position{X: 2, Y: hallGapRow}

	// hallSideOfGap and tombSideOfGap are the one open crossing between the
	// two rooms — the waypoints every walk between them goes through.
	hallSideOfGap = spatial.Position{X: 3, Y: hallGapRow}
	tombSideOfGap = spatial.Position{X: 4, Y: hallGapRow}
)

func vaultFindCheck() []encounter.CheckApproach {
	return []encounter.CheckApproach{{Ability: "perception", DC: 15}}
}

func no() *bool  { v := false; return &v }
func yes() *bool { v := true; return &v }

// takeableProp is a prop somebody can pick up: named, in nobody's way, and
// saying both blocking answers out loud as every prop must.
func takeableProp(id encounter.PropID, ref string, at spatial.Position) encounter.PropInput {
	return encounter.PropInput{
		ID: id, Takeable: true, Ref: ref, At: at,
		BlocksMovement: no(), BlocksLineOfSight: no(),
	}
}

// heirloomField is the fixture dungeon described in the file comment.
func heirloomField() encounter.FieldInput {
	return encounter.FieldInput{
		Canvas: pointyCanvas(),
		Regions: []encounter.RegionInput{
			rectRegion("hall", 0, 0, 4, 8),
			rectRegion("tomb", 4, 0, 4, 8),
			func() encounter.RegionInput {
				r := rectRegion("vault", 8, 0, 3, 8)
				r.Concealed = true
				return r
			}(),
		},
		Walls: append(
			seamWallExcept(3, 8, hallGapRow),
			seamWallExcept(7, 8, vaultSeamRow)...),
		Doors: []encounter.DoorInput{{
			ID: tombVault, Edges: doorEdgesAcross(7, vaultSeamRow),
			State: encounter.DoorIsClosed(), Concealed: vaultFindCheck(),
		}},
		Props: []encounter.PropInput{
			takeableProp(heirloom, "dnd5e:props:reliquary", heirloomCell),
			takeableProp(chalice, "dnd5e:props:chalice", chaliceCell),
			takeableProp(relic, "dnd5e:props:relic", relicCell),
			// A prop that said nothing about being takeable, which is every
			// prop that existed before this slice.
			{ID: pillar, Ref: "dnd5e:props:pillar", At: pillarCell,
				BlocksMovement: yes(), BlocksLineOfSight: yes()},
		},
		Exits: []encounter.FieldExit{
			{ID: frontGate, At: raiderCell},
			{ID: sideDoor, At: sideDoorCell},
		},
	}
}

// recoverEnding is the scenario's declared ending: leave through the front
// gate holding the heirloom.
func recoverEnding() encounter.EndingInput {
	return encounter.EndingInput{
		Key:     recovered,
		Trigger: encounter.TriggerExitedHolding{Exit: frontGate, Item: heirloom},
	}
}

// nobodyIsInContact is the participation capability these scenes install:
// members may be DOWN, but nobody is ever in contact, so no fight ever
// forms and no bubble puts a turn clock between a member and a verb.
//
// Deliberate and stated out loud (rpg-toolkit#1033). These scenes are about
// LOOT, TAKE and the ending — what a fight does to those is one rule, tested
// on its own in TestTheTurnClockGatesBothVerbs with a capability that does
// report contact. Leaving contact on everywhere else would make every scene
// here quietly also a scene about initiative.
type nobodyIsInContact struct {
	down []encounter.MemberID
}

func (n *nobodyIsInContact) Standing(members []encounter.MemberID) ([]encounter.MemberID, error) {
	return n.reported(members), nil
}

func (n *nobodyIsInContact) Assess(members []encounter.MemberID) (*encounter.ParticipationAssessment, error) {
	down := map[encounter.MemberID]bool{}
	for _, id := range n.reported(members) {
		down[id] = true
	}
	assessment := &encounter.ParticipationAssessment{}
	for _, id := range members {
		member := encounter.MemberParticipation{
			Member: id, Contact: false, Conscious: true, Turn: encounter.TurnParticipationWait,
		}
		if down[id] {
			member.Down = true
			member.Conscious = false
			member.Turn = encounter.TurnParticipationRemove
		}
		assessment.Members = append(assessment.Members, member)
	}
	return assessment, nil
}

func (n *nobodyIsInContact) reported(members []encounter.MemberID) []encounter.MemberID {
	asked := map[encounter.MemberID]bool{}
	for _, id := range members {
		asked[id] = true
	}
	var out []encounter.MemberID
	for _, id := range n.down {
		if asked[id] {
			out = append(out, id)
		}
	}
	return out
}

type HoldingsSuite struct {
	suite.Suite

	witness  *scriptedWitness
	standing *nobodyIsInContact
}

func TestHoldingsSuite(t *testing.T) {
	suite.Run(t, new(HoldingsSuite))
}

func (s *HoldingsSuite) SetupTest() {
	s.witness = &scriptedWitness{perceivers: map[encounter.DoorID][]encounter.MemberID{}}
	s.standing = &nobodyIsInContact{}
}

// captainKnows is the cast: raider and partner in the hall, and the captain
// in the tomb carrying the way to the vault when knows is true.
func (s *HoldingsSuite) cast(knows bool) []encounter.MemberInput {
	var carried []encounter.DoorID
	if knows {
		carried = []encounter.DoorID{tombVault}
	}
	return []encounter.MemberInput{
		{ID: raider, Kind: encounter.KindPlayer, Position: raiderCell},
		{ID: partner, Kind: encounter.KindPlayer, Position: partnerCell},
		{ID: captain, Kind: encounter.KindMonster, Position: captainCell, Knows: carried},
	}
}

func (s *HoldingsSuite) open(knows bool, endings ...encounter.EndingInput) *encounter.Encounter {
	if len(endings) == 0 {
		endings = []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}}
	}
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: s.standing, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{}, Announcer: quietAnnouncer{},
		CheckResolver: findsNothing{}, Witness: s.witness,
		Field:     heirloomField(),
		Members:   s.cast(knows),
		Endings:   endings,
		Retention: encounter.RetentionUnbounded,
	})
	s.Require().NoError(err)
	return enc
}

// beats reads one member's whole story, decoded, in order — the same read a
// client makes, so recipient scoping is asserted through the projection
// rather than around it.
func (s *HoldingsSuite) beats(enc *encounter.Encounter, member core.EntityID) []map[string]any {
	story, err := enc.Story(&encounter.StoryInput{Audience: member})
	s.Require().NoError(err)
	out := make([]map[string]any, 0, len(story))
	for _, entry := range story {
		var beat map[string]any
		s.Require().NoError(json.Unmarshal(entry.Payload, &beat))
		out = append(out, beat)
	}
	return out
}

// beatsOfKind filters one member's story to a beat kind.
func (s *HoldingsSuite) beatsOfKind(enc *encounter.Encounter, member core.EntityID, kind string) []map[string]any {
	out := make([]map[string]any, 0)
	for _, beat := range s.beats(enc, member) {
		if beat["beat"] == kind {
			out = append(out, beat)
		}
	}
	return out
}

// propInAtlas returns the atlas's entry for a prop id, and whether it is
// there at all.
func propInAtlas(atlas encounter.Atlas, id encounter.PropID) (encounter.AtlasProp, bool) {
	for _, p := range atlas.Props {
		if p.ID == id {
			return p, true
		}
	}
	return encounter.AtlasProp{}, false
}

// atlasBytes is one member's atlas rendered to JSON — the comparison every
// secrecy scene here makes, because "the same bytes" is the claim design P3
// actually makes and a field-by-field assertion is a weaker one.
func (s *HoldingsSuite) atlasBytes(enc *encounter.Encounter, member core.EntityID) string {
	atlas, err := enc.AtlasFor(member)
	s.Require().NoError(err)
	raw, err := json.Marshal(atlas)
	s.Require().NoError(err)
	return string(raw)
}

// storyBytes is one member's whole story rendered to JSON — every beat, in
// order, payload and all.
func (s *HoldingsSuite) storyBytes(enc *encounter.Encounter, member core.EntityID) string {
	raw, err := json.Marshal(s.beats(enc, member))
	s.Require().NoError(err)
	return string(raw)
}

// drop puts the captain down, which is what makes them lootable.
func (s *HoldingsSuite) drop(enc *encounter.Encounter) {
	s.standing.down = []encounter.MemberID{captain}
	// The composition notices a body on the next sight refresh, the same way
	// the game does. Pump is the cheapest verb that runs one.
	_, err := enc.Pump(&encounter.PumpInput{})
	s.Require().NoError(err)
}

// walkTo moves a member to a cell, through the seam gap when the two are in
// different rooms.
//
// Step is a PLACEMENT question and does not check adjacency ([StepOutput]'s
// own doc), so a move inside one room is one step however far it is — but a
// move BETWEEN rooms still cannot cross a wall, so it goes through the one
// open crossing. Two waypoints, named, rather than a pathfinder: the fixture
// has exactly one way through and a reader should be able to see it.
func (s *HoldingsSuite) walkTo(enc *encounter.Encounter, member core.EntityID, to spatial.Position) {
	from, ok := enc.RegionAt(s.cellOf(enc, member))
	s.Require().True(ok, "the member is standing somewhere")
	target, ok := enc.RegionAt(cellAt(int(to.X), int(to.Y)))
	s.Require().True(ok, "[%g,%g] is floor somebody owns", to.X, to.Y)

	if from != target {
		s.Require().NotEqual("vault", target, "the vault is shut; no scene here walks in")
		gate := []spatial.Position{hallSideOfGap, tombSideOfGap}
		if from == "tomb" {
			gate = []spatial.Position{tombSideOfGap, hallSideOfGap}
		}
		for _, w := range gate {
			s.step(enc, member, w)
		}
	}
	s.step(enc, member, to)
}

// step is one placement, in the fixture's own authored coordinates.
func (s *HoldingsSuite) step(enc *encounter.Encounter, member core.EntityID, to spatial.Position) {
	_, err := enc.Step(&encounter.StepInput{Member: member, To: cellAt(int(to.X), int(to.Y))})
	s.Require().NoError(err, "step to [%g,%g]", to.X, to.Y)
}

// cellOf is a member's current absolute cell, read the way a client reads it.
func (s *HoldingsSuite) cellOf(enc *encounter.Encounter, member core.EntityID) spatial.Position {
	members, err := enc.Members()
	s.Require().NoError(err)
	for _, m := range members {
		if m.ID == member {
			return m.Position
		}
	}
	s.Require().Fail("no such member", string(member))
	return spatial.Position{}
}
