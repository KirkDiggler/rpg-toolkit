// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

// bothways_test.go is dispositions turning BOTH ways, at the seam
// (rpg-project#493, `ideas/living-world/disposition/both-ways.md`). The
// composition's own laws — the `until` widening, the aggression law, the fold
// that decides a stance, the settled fact that survives a save — are pinned in
// the encounter suite (bothways_test.go there). What is pinned HERE is that
// they reach a host:
//
//   - a swing at a placed NPC is refused BY NAME, with this package's own
//     sentinel, before any sheet or world is written (R4);
//   - a player attacking a neutral camp turns the camp: the next read says
//     hostile, the camp is in the fight, and every recipient's stream carries
//     the stance turn with the CAUSE that made it (R3);
//   - "the guards are civil until midnight" turns a pair with nobody
//     stepping: the round arrives on a clock that is already running, the
//     stance beat says which round did it, and the guards are in the fight
//     on the next read (R2, `{ round }`).
//
// # Why every scene here starts inside a fight
//
// The design's headline is a camp the party can provoke in free roam, and
// this seam cannot drive that today: Attack has no world-clock offer, so a
// swing outside a fight is ErrStaleDeclaration before any of this is reached
// (see attack.go's clock gate, and Cast's identical one). The skeleton in these
// fixtures is what gives alice a turn to swing on. What the scenes prove is
// the law; that a party in free roam has no verb to provoke a camp with is a
// gap in the seam's offers, not in the ruling, and it is reported rather than
// papered over here.

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

const (
	bwSession  = "sess"
	bwWorld    = "world"
	bwGoblins  = "goblins"
	bwUndead   = "undead"
	bwChief    = "chief"
	bwWarrior  = "warrior"
	bwSkeleton = "skeleton"
)

// BothWaysSuite is the seam's half of rpg-project#493.
type BothWaysSuite struct {
	suite.Suite

	mgr    *session.Manager
	stream *fakeStream
}

func TestBothWaysSuite(t *testing.T) { suite.Run(t, new(BothWaysSuite)) }

// yard opens a hall with two players, a neutral goblin camp beside them and a
// skeleton across the room, and returns with the skeleton's fight already on.
//
// THE CAMP IS SPAWNED, not placed at construction, because Spawn is the verb
// every monster really enters a run by and the only one that records a sheet
// — and a camp with no sheet cannot be swung at. The undead are declared and
// say nothing about the party, which by the composition's own default makes
// them hostile; that is the whole of their job.
func (s *BothWaysSuite) yard(until encounter.Trigger) {
	alice := armedFighter("alice")
	bob := armedFighter("bob")
	sessions, encounters := newFakeSessions(), newFakeEncounters()
	characters := newFakeCharacters(alice, bob)
	s.stream = &fakeStream{}

	// THE SWING MISSES, ON PURPOSE. The ruling is that the camp turns, not
	// that the goblin dies, and a struck goblin at these hit points falls —
	// which would take the one member the scene is about out of the fight it
	// is claiming they joined. A miss lands the same deed through the same
	// path ([Encounter.landAttack] runs for struck AND missed), so the law is
	// exercised with nothing else moving. Three tens open the skeleton's
	// initiative (ties, so the order is the alphabetical tie-break and alice
	// acts first), then the 1 is alice's d20.
	rolls := append([]int{10, 10, 10, 1}, ordinaryRolls(40)...)

	mgr, err := session.NewManager(&session.Config{
		PresentationIDs: testPresentationIDs{}, Dice: &sequenceDice{rolls: rolls}, TurnDriver: session.Pass{},
		Sessions: sessions, Encounters: encounters,
		Characters: characters, Events: s.stream,
	})
	s.Require().NoError(err)
	s.mgr = mgr

	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Striker: encounter.RefusingStriker{}, Mover: encounter.RefusingMover{},
		Announcer: encQuietAnnouncer{}, Sight: encEveryoneSees{}, Equipment: encNoHandsObserved{},
		Initiative: encOrderAsGiven{}, TurnDriver: encPassDriver{}, Standing: encEveryoneStanding{},
		Field: encounter.FieldInput{
			Canvas:   pointyCanvas(),
			Regions:  []encounter.RegionInput{rectRegion("hall", 0, 0, 8, 8)},
			Factions: []encounter.FactionInput{{ID: bwGoblins}, {ID: bwUndead}},
			Dispositions: []encounter.DispositionInput{{
				Between: [2]encounter.FactionID{bwGoblins, encounter.FactionParty},
				Stance:  encounter.StanceNeutral,
				Until:   until,
			}},
		},
		Members: []encounter.MemberInput{
			{ID: "alice", Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}},
			{ID: "bob", Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 3}},
		},
		Endings:   []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}},
		Retention: encounter.RetentionUnbounded,
	})
	s.Require().NoError(err)
	data := enc.ToData()

	ctx := context.Background()
	_, err = s.mgr.StartSession(ctx, &session.StartSessionInput{
		Session: bwSession, Encounter: bwWorld, World: &data,
	})
	s.Require().NoError(err)

	// The camp first, civil, in plain sight of both players: no fight forms,
	// which is the precondition the whole file is about.
	for _, spawn := range []struct {
		id string
		at spatial.Position
	}{
		{bwChief, spatial.Position{X: 2, Y: 1}},
		{bwWarrior, spatial.Position{X: 2, Y: 2}},
	} {
		out, err := s.mgr.Spawn(ctx, &session.SpawnInput{
			Session: bwSession, ID: spawn.id, Ref: refs.Monsters.Goblin().String(),
			Position: spawn.at, Faction: bwGoblins,
		})
		s.Require().NoError(err)
		s.Require().Nil(out.Formed, "a neutral camp in plain sight starts no fight — that is the gap #493 closes")
	}

	// And then the thing that is actually hostile, which is what gives the
	// party a turn to act on at all.
	formed, err := s.mgr.Spawn(ctx, &session.SpawnInput{
		Session: bwSession, ID: bwSkeleton, Ref: refs.Monsters.Skeleton().String(),
		Position: spatial.Position{X: 5, Y: 5}, Faction: bwUndead,
	})
	s.Require().NoError(err)
	s.Require().NotNil(formed.Formed, "precondition: the skeleton's fight is on")
	s.Require().Equal(session.ClockTurn, s.clockOf("alice"), "precondition: alice has a turn to swing on")
	s.Require().Equal(session.ClockWorld, s.clockOf(bwChief), "precondition: the camp is not in it")

	s.stream.published = nil
}

// ordinaryRolls is a tail of unremarkable faces for the dice a scene does not
// care about — the goblins' own initiative when they join, and anything a
// later verb asks for. Ten is the face testDice hands every die, so a scene
// that stops scripting reads the same as every fixture that never started.
func ordinaryRolls(n int) []int {
	out := make([]int, n)
	for i := range out {
		out[i] = 10
	}
	return out
}

// clockOf is which clock a member is on right now — the seam's read for "are
// they in a fight".
func (s *BothWaysSuite) clockOf(member string) session.ClockKind {
	s.T().Helper()
	turn, err := s.mgr.Turn(context.Background(), &session.TurnInput{Session: bwSession, Member: member})
	s.Require().NoError(err)
	return turn.Clock
}

// believedStance is what one member's own View says about a subject's stance
// toward them. This seam publishes no stance read, so a pair's turn is read
// here the way a client reads it: off a sighting (rpg-project#458).
func (s *BothWaysSuite) believedStance(viewer, subject string) string {
	s.T().Helper()
	sightings, err := s.mgr.View(context.Background(), &session.ViewInput{Session: bwSession, Member: viewer})
	s.Require().NoError(err)
	for _, sighting := range sightings {
		if sighting.Subject == subject {
			return sighting.Stance
		}
	}
	s.Require().Fail("no sighting", "%s does not see %s", viewer, subject)
	return ""
}

// stanceTurn is the one stance beat on a recipient's stream, typed.
func (s *BothWaysSuite) stanceTurn(recipient string) session.StanceChangedBody {
	s.T().Helper()
	events := eventsOfKind(s.stream.published, recipient, session.EventStanceChanged)
	s.Require().Len(events, 1, "%s heard the pair turn exactly once", recipient)
	body, ok := events[0].Body.(session.StanceChangedBody)
	s.Require().True(ok, "the turn crosses as its typed body, not as bytes a client has to parse")
	return body
}

// recipients is everybody any event was addressed to.
func (s *BothWaysSuite) recipients() []string {
	seen := map[string]bool{}
	var out []string
	for _, e := range s.stream.published {
		if !seen[e.Recipient] {
			seen[e.Recipient] = true
			out = append(out, e.Recipient)
		}
	}
	return out
}

// swing is alice's attack on a target, through the offer she actually holds.
func (s *BothWaysSuite) swing(target string) error {
	s.T().Helper()
	_, err := s.mgr.Attack(context.Background(), &session.AttackInput{
		Session: bwSession, Attacker: "alice", Target: target,
		DeclarationID: currentAttackID(s.T(), s.mgr, bwSession, "alice"),
	})
	return err
}

// TestAttackingANeutralCampTurnsItAndPutsItInTheFight is R3 at the seam, and
// it is the design's opening paragraph answered: before this slice the swing
// landed, no verb read opposition, and the camp's own faction read
// `enemy: none` — one aggrieved goblin with no initiative and no friends.
//
// Now the pair is hostile the moment the swing lands, both goblins are in the
// fight, and every recipient is told WHY.
func (s *BothWaysSuite) TestAttackingANeutralCampTurnsItAndPutsItInTheFight() {
	s.yard(nil)
	s.Require().Equal(string(encounter.StanceNeutral), s.believedStance("alice", bwChief),
		"precondition: alice is looking at a creature she is not at war with")

	s.Require().NoError(s.swing(bwChief))

	// EVERY READ BELOW IS A RELOAD. This seam holds no session process: each
	// verb loads the stored world, acts, and saves it (S4). So "the next read
	// says hostile" is also the save-and-load claim — a stance is derived and
	// never stored, and if the fact that turned the pair had not gone into the
	// blob, the very next line would find a civil camp.
	s.Run("the pair is hostile on the next read", func() {
		s.Equal(string(encounter.StanceHostile), s.believedStance("alice", bwChief))
		s.Equal(string(encounter.StanceHostile), s.believedStance("bob", bwWarrior),
			"a pair is a pair: bob never swung at anyone and is at war with the camp too")
		s.Equal(string(encounter.StanceHostile), s.believedStance(bwWarrior, "alice"),
			"and the goblin who was never touched reads the party as an enemy")
	})

	s.Run("the camp is in the fight, not just the one who was hit", func() {
		s.Equal(session.ClockTurn, s.clockOf(bwChief))
		s.Equal(session.ClockTurn, s.clockOf(bwWarrior),
			"the one that was merely WATCHING is engaged — that is the camp, and it is the whole ruling")
		s.Equal(session.ClockTurn, s.clockOf("alice"))
	})

	s.Run("every recipient is told the pair turned, and what caused it", func() {
		s.Require().NotEmpty(s.recipients())
		for _, who := range s.recipients() {
			turn := s.stanceTurn(who)
			s.ElementsMatch([]string{bwGoblins, encounter.FactionParty}, turn.Between,
				"the pair is a set and carries no direction")
			s.Equal(string(encounter.StanceHostile), turn.Stance)
			s.Equal("attacked by alice", turn.Cause,
				"a streamer is told why the camp turned rather than left to infer it from the swing before it")
		}
	})
}

// TestTheGuardsTurnAtMidnightWithNobodyStepping is R2's `{ round }` form at
// the seam, and the second half of the correction the build forced: fight
// formation reads FIRST CONTACT, and the party was already looking at these
// guards — so without the stance site forming the fight itself, a camp that
// turned on a round would have stood there being hostile at nobody.
//
// NOBODY STEPS. The only verb in this scene is EndTurn.
func (s *BothWaysSuite) TestTheGuardsTurnAtMidnightWithNobodyStepping() {
	s.yard(encounter.TriggerRound{Round: 3})

	s.Run("round two: still civil", func() {
		s.endRound()
		s.Equal(string(encounter.StanceNeutral), s.believedStance("alice", bwChief))
		s.Equal(session.ClockWorld, s.clockOf(bwChief), "and still out of the fight")
		s.Empty(eventsOfKind(s.stream.published, "alice", session.EventStanceChanged),
			"nothing turned, so nothing was announced")
	})

	s.stream.published = nil

	s.Run("round three: the guards turn, and the beat says which round did it", func() {
		s.endRound()
		s.Equal(string(encounter.StanceHostile), s.believedStance("alice", bwChief))
		turn := s.stanceTurn("alice")
		s.ElementsMatch([]string{bwGoblins, encounter.FactionParty}, turn.Between)
		s.Equal(string(encounter.StanceHostile), turn.Stance)
		s.Equal("round 3 started", turn.Cause)
	})

	s.Run("and the fight formed without anybody walking anywhere", func() {
		s.Equal(session.ClockTurn, s.clockOf(bwChief))
		s.Equal(session.ClockTurn, s.clockOf(bwWarrior))
	})
}

// endRound takes the fight all the way round to alice again. The skeleton has no
// player, so ending the turn before it drives its turn through in the same
// call (ADR-0043) and there is no explicit EndTurn for it.
func (s *BothWaysSuite) endRound() {
	s.T().Helper()
	ctx := context.Background()
	for _, who := range []string{"alice", "bob"} {
		if s.clockOf(who) != session.ClockTurn {
			continue
		}
		_, err := s.mgr.EndTurn(ctx, &session.EndTurnInput{
			Session: bwSession, Member: who,
			DeclarationID: currentEndTurnID(s.T(), s.mgr, bwSession, who),
		})
		s.Require().NoError(err, "ending %s's turn", who)
	}
}

// TestSwingingAtAPlacedNPCIsRefusedByNameAndWritesNothing is R4 at the seam.
//
// The composition refuses this too, and translate carries its sentinel — but
// the refusal a host actually meets is this one, one layer earlier, because
// every candidate universe here already drops KindWorld members. What that
// used to produce was ErrStaleDeclaration, which is a lie about a permanent
// fact: re-reading the offers answers the same thing forever, and the only
// thing that makes a merchant attackable is authoring it as a monster.
//
// NOTHING IS WRITTEN, which is what "refused at the verb, fail closed" means:
// no sheet, no world, no story. A caller that retries has nothing to undo.
type BothWaysNPCSuite struct {
	suite.Suite
}

func TestBothWaysNPCSuite(t *testing.T) { suite.Run(t, new(BothWaysNPCSuite)) }

func (s *BothWaysNPCSuite) TestSwingingAtAPlacedNPCIsRefusedByNameAndWritesNothing() {
	alice := armedFighter("alice")
	mgr, _, encounters, characters := aFight(s.T(), alice, nil)
	ctx := context.Background()

	// A merchant, placed the way a world NPC is placed, standing right beside
	// alice — so the refusal cannot be an accident of distance or sight.
	_, err := mgr.PlaceNPC(ctx, &session.PlaceNPCInput{
		Session: "sess", Member: "vendor", Position: spatial.Position{X: 1, Y: 2}, NPC: merchantData(),
	})
	s.Require().NoError(err)

	worldSaves, sheetSaves := encounters.saves, characters.saves

	out, err := mgr.Attack(ctx, &session.AttackInput{
		Session: "sess", Attacker: "alice", Target: "vendor",
		DeclarationID: currentAttackID(s.T(), mgr, "sess", "alice"),
	})

	s.Require().Error(err)
	s.Nil(out)
	s.ErrorIs(err, session.ErrNotATarget,
		"the host is told the one thing that would change the answer: author it as a monster")
	s.NotErrorIs(err, session.ErrStaleDeclaration,
		"NOT stale — stale means re-read the offers, and re-reading says this forever")
	s.NotErrorIs(err, encounter.ErrNotATarget,
		"and never the composition's own sentinel: matching on it would couple the host to a module we intend to replace")
	s.Contains(err.Error(), "author it as a monster",
		"the sentence a host surfaces has to name the fix, not just the refusal")

	s.Equal(worldSaves, encounters.saves, "a refused swing writes no world")
	s.Equal(sheetSaves, characters.saves, "and no sheet")
}
