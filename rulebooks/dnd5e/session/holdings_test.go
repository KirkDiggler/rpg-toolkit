// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

// holdings_test.go drives the seam's half of living-world SLICE 2
// (rpg-toolkit#1496; ruled on rpg-project#368): Loot, Hold, the atlas facts
// the wire needs, the three new beats, and the ending that fires when
// somebody walks out of a bound exit carrying the artifact.
//
// conceal_test.go's shape, and for its reason. The composition's own laws —
// the probe law, "a body with nothing gives the same bytes", the transfer
// routine, the drop — are pinned in the encounter suite; what is pinned HERE
// is that they SURVIVE THE SEAM: same secrecy through per-recipient delivery,
// same bytes through translate, with the session's real standing seam
// answering out of real sheets instead of a scripted stand-in.
//
// # The fixture
//
// concealedWorld's geometry with the slice's cast added, everything the
// scenes touch laid out on ONE AUTHORED ROW so adjacency is not a question
// the reader has to answer: in an offset frame, (c,r) and (c±1,r) are
// neighbours whatever the parity of r.
//
//	row 1:  (1) alice   (2) captain   (3) heirloom   (4) bob   (5) chalice
//
// alice and bob are players. The CAPTAIN is a monster who knows the veil —
// the concealed door between the hall and the vault — and who is DOWN,
// because the session's own standing seam reads its stored sheet and the
// sheet says zero. The RELIC and the URN stand inside the concealed vault,
// where the probe-law scenes need them: one holdable, one not, and neither
// visible to anybody standing in the hall.
//
// FRONT-GATE is the authored exit at alice's own cell, and the one the
// scenario binds; SIDE-DOOR is a second authored exit at bob's cell that
// NOTHING binds — the sharper version of "anywhere but the exit" (R9's
// reason is the hole, not the cell).

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

const (
	heirloomID = "heirloom"
	chaliceID  = "chalice"
	relicID    = "relic"
	urnID      = "urn"
	pillarID   = "pillar"
	frontGate  = "front-gate"
	sideDoor   = "side-door"
	recovered  = "recovered"
)

var (
	aliceCell    = cell(1, 1)
	captainCell  = cell(2, 1)
	heirloomCell = cell(3, 1)
	bobCell      = cell(4, 1)
	chaliceCell  = cell(5, 1)

	// Inside the concealed vault: nobody in the hall is shown these cells.
	relicCell = cell(8, 2)
	urnCell   = cell(9, 2)

	// In the hall corner, where everybody can see it: the prop that says
	// nothing about being holdable, which is what every prop was before this
	// slice, and the one target ErrNotHoldable is reachable through.
	pillarCell = cell(0, 5)
)

func noBlocking() *bool  { v := false; return &v }
func yesBlocking() *bool { v := true; return &v }

// holdable is a prop somebody can pick up: named, in nobody's way, and saying
// both blocking answers out loud as every prop must.
func holdable(id, ref string, at spatial.Position) encounter.PropInput {
	return encounter.PropInput{
		ID: encounter.PropID(id), Holdable: true, Ref: ref, At: at,
		BlocksMovement: noBlocking(), BlocksLineOfSight: noBlocking(),
	}
}

// scenery is a prop nobody declared holdable — which is every prop that
// existed before this slice.
func scenery(id, ref string, at spatial.Position) encounter.PropInput {
	return encounter.PropInput{
		ID: encounter.PropID(id), Ref: ref, At: at,
		BlocksMovement: yesBlocking(), BlocksLineOfSight: yesBlocking(),
	}
}

// heirloomWorld is the fixture described at the top of this file. knows says
// whether the captain was authored knowing the veil — the ONE difference the
// secrecy scenes vary, so that everything else about the two worlds is
// identical by construction rather than by inspection.
func heirloomWorld(t fataler, knows bool) *encounter.EncounterData {
	captain := encounter.MemberInput{
		ID: "captain", Kind: encounter.KindMonster, Position: captainCell,
	}
	if knows {
		captain.Knows = []encounter.DoorID{"veil"}
	}

	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Striker: encounter.RefusingStriker{}, Announcer: encQuietAnnouncer{}, Sight: encEveryoneSees{},
		Initiative: encOrderAsGiven{}, TurnDriver: encPassDriver{},
		Standing:      encCaptainIsDown{},
		CheckResolver: encNeverResolves{},
		Witness:       encNeverWitnesses{},
		Field: encounter.FieldInput{Canvas: pointyCanvas(),
			Regions: []encounter.RegionInput{
				rectRegion("hall", 0, 0, 6, 6),
				concealRegion(rectRegion("vault", 6, 0, 6, 6)),
			},
			Walls: axialSeam(0),
			Doors: []encounter.DoorInput{{
				ID:        "veil",
				Edges:     []encounter.DoorEdge{{From: cell(5, 0), To: cell(6, 0)}},
				State:     encounter.DoorIsClosed(),
				Concealed: veilFind(),
			}},
			Props: []encounter.PropInput{
				holdable(heirloomID, "dnd5e:props:reliquary", heirloomCell),
				holdable(chaliceID, "dnd5e:props:chalice", chaliceCell),
				holdable(relicID, "dnd5e:props:relic", relicCell),
				scenery(urnID, "dnd5e:props:urn", urnCell),
				scenery(pillarID, "dnd5e:props:pillar", pillarCell),
			},
			Exits: []encounter.FieldExit{
				{ID: frontGate, At: aliceCell},
				{ID: sideDoor, At: bobCell},
			},
		},
		Members: []encounter.MemberInput{
			{ID: "alice", Kind: encounter.KindPlayer, Position: aliceCell},
			{ID: "bob", Kind: encounter.KindPlayer, Position: bobCell},
			captain,
		},
		Endings: []encounter.EndingInput{
			{Key: "out", Trigger: encounter.TriggerExternal{}},
			{Key: recovered, Trigger: encounter.TriggerExitedHolding{
				Exit: frontGate, Item: heirloomID,
			}},
		},
		Retention: encounter.RetentionUnbounded,
	})
	if err != nil {
		t.Fatalf("building heirloom world: %v", err)
	}
	data := enc.ToData()
	return &data
}

// encCaptainIsDown is the composition's Standing capability WHILE THE FIXTURE
// IS BEING AUTHORED, and it exists so that no fight has already formed by the
// time a session loads the blob: the captain is a monster standing beside two
// players, and a captain reported up at authoring time would put all three in
// a bubble before any scene had run.
//
// It answers for the fixture build ALONE. Once the session owns the world,
// the real standingSeam answers out of the captain's stored sheet — which is
// why every suite below seeds that sheet at zero hit points before the first
// verb, and why TestTheBodyIsDownBecauseItsSheetSaysSo pins that it is the
// SHEET doing the work rather than this stand-in.
type encCaptainIsDown struct{}

func (encCaptainIsDown) Standing(_ []encounter.MemberID) ([]encounter.MemberID, error) {
	return []encounter.MemberID{"captain"}, nil
}

func (encCaptainIsDown) Assess(members []encounter.MemberID) (*encounter.ParticipationAssessment, error) {
	assessment := &encounter.ParticipationAssessment{}
	for _, id := range members {
		participation := encounter.MemberParticipation{
			Member: id, Contact: false, Conscious: true, Turn: encounter.TurnParticipationWait,
		}
		if id == "captain" {
			participation.Down = true
			participation.Conscious = false
			participation.Turn = encounter.TurnParticipationRemove
		}
		assessment.Members = append(assessment.Members, participation)
	}
	return assessment, nil
}

type HoldingsSuite struct {
	suite.Suite

	stream     *fakeStream
	sessions   *fakeSessions
	encounters *fakeEncounters
	characters *fakeCharacters
	mgr        *session.Manager
}

func TestHoldingsSuite(t *testing.T) { suite.Run(t, new(HoldingsSuite)) }

// start wires a fresh manager around the fixture and seeds the captain's
// sheet at zero, which is what makes the body a body.
func (s *HoldingsSuite) start(knows bool, cast ...*character.Data) {
	if len(cast) == 0 {
		cast = []*character.Data{sharpEyed("alice"), dullEyed("bob")}
	}
	s.stream = &fakeStream{}
	s.sessions, s.encounters = newFakeSessions(), newFakeEncounters()
	s.characters = newFakeCharacters(cast...)

	mgr, err := session.NewManager(&session.Config{PresentationIDs: testPresentationIDs{},
		Dice: testDice{}, TurnDriver: session.Pass{},
		Sessions: s.sessions, Encounters: s.encounters,
		Characters: s.characters, Events: s.stream,
	})
	s.Require().NoError(err)
	s.mgr = mgr

	_, err = mgr.StartSession(context.Background(), &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: heirloomWorld(s.T(), knows),
	})
	s.Require().NoError(err)

	// THE BODY IS A BODY BECAUSE ITS SHEET SAYS SO. The session's standing
	// seam reads this record, so a captain with no sheet would be reported
	// UP — and Loot would refuse with ErrNotDown before any scene got going.
	stored := s.sessions.byID["sess"]
	stored.NPCs = append(stored.NPCs, monster.Data{
		ID: "captain", Name: "Skeleton Captain", HitPoints: 0, MaxHitPoints: 22, ArmorClass: 15,
	})

	s.stream.published = nil
}

// events returns everything published to one recipient, in order.
func (s *HoldingsSuite) events(recipient string) []session.Event {
	return eventsFor(s.stream.published, recipient)
}

// kinds names the kinds one recipient was told about, in order — the shape
// most of these scenes assert on, since WHAT reached a member and what did
// not is the whole claim.
func (s *HoldingsSuite) kinds(recipient string) []session.EventKind {
	var out []session.EventKind
	for _, e := range s.events(recipient) {
		out = append(out, e.Kind)
	}
	return out
}

// atlasOf is one member's own map.
func (s *HoldingsSuite) atlasOf(member string) *session.Atlas {
	s.T().Helper()
	atlas, err := s.mgr.Atlas(context.Background(), &session.AtlasInput{Session: "sess", Member: member})
	s.Require().NoError(err)
	return atlas
}

// propIDs names the props on a member's map, in the order the map lists them.
func propIDs(atlas *session.Atlas) []string {
	out := make([]string, 0, len(atlas.Props))
	for _, p := range atlas.Props {
		out = append(out, p.ID)
	}
	return out
}

// atlasBytes is one member's whole map as bytes — the comparison the secrecy
// scenes make, because "these two are equal" is a claim about every field,
// including ones added after this test was written.
func (s *HoldingsSuite) atlasBytes(member string) string {
	s.T().Helper()
	raw, err := json.Marshal(s.atlasOf(member))
	s.Require().NoError(err)
	return string(raw)
}

// bodyOf fetches the one event of a kind on a recipient's stream, failing if
// there is not exactly one.
func (s *HoldingsSuite) bodyOf(recipient string, kind session.EventKind) session.EventBody {
	s.T().Helper()
	var found []session.Event
	for _, e := range s.events(recipient) {
		if e.Kind == kind {
			found = append(found, e)
		}
	}
	s.Require().Len(found, 1, "%s should have exactly one %s", recipient, kind)
	return found[0].Body
}

// absolute converts an AUTHORED cell to the dungeon-absolute one every read
// and every beat speaks — the field's own conversion, applied exactly once,
// so a scene's expectation is DERIVED from the fixture rather than echoed
// back from whatever the code produced.
func absolute(p spatial.Position) spatial.Position { return hexCell(int(p.X), int(p.Y)) }

func aliceCellAbsolute() spatial.Position { return absolute(aliceCell) }
func bobCellAbsolute() spatial.Position   { return absolute(bobCell) }

// storyBytes is one member's whole delivered story, as bytes. A stronger
// claim than the live stream for the secrecy scenes: it includes the
// numbering, and a HOLE where a reveal went would itself be an oracle.
func (s *HoldingsSuite) storyBytes(member string) string {
	s.T().Helper()
	story, err := s.mgr.Story(context.Background(), &session.StoryInput{
		Session: "sess", Member: member})
	s.Require().NoError(err)
	raw, err := json.Marshal(story)
	s.Require().NoError(err)
	return string(raw)
}

// assertDenseFor fails unless a recipient's delivered numbers are
// consecutive — the gap oracle, killed as one check on the values.
func (s *HoldingsSuite) assertDenseFor(who string) {
	s.T().Helper()
	events := s.events(who)
	for i := 1; i < len(events); i++ {
		s.Require().Equal(events[i-1].Seq+1, events[i].Seq,
			"%s's own stream must be dense: seq %d follows %d", who, events[i].Seq, events[i-1].Seq)
	}
}
