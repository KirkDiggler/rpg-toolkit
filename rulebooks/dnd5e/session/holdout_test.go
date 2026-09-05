// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

// holdout_test.go is the hold-out, STEP A, at the SEAM (rpg-project#375, the
// hold-out design §5 and §9): sides and knowledge, driven through the verbs a
// host uses and read off the streams a client reads.
//
// The composition's own laws — who is opposed, the flip in the graph, the
// fight that loses its sides, the stance that is never stored — are pinned in
// the encounter suite (holdout_test.go there). What is pinned HERE is that
// they SURVIVE THE SEAM:
//
//   - a monster's faction reaches the composition through Spawn, the one
//     verb every monster actually enters a run by (design §3 "Spawn");
//   - the roster row says whose side everyone is on;
//   - the `stance` beat crosses as a typed EventStanceChanged in every
//     recipient's own dense numbering, followed by a FIGHT_ENDED whose cause
//     is the stance and an ENDED naming the hold-out;
//   - a verb after the flip loads the stored world back and still answers
//     neutral, with no stance in the blob (A9);
//   - a faction the dungeon does not declare is refused by name.
//
// # The fixture
//
// The raider camp itself — the file the encounter suite plays, read from the
// PINNED encounter module rather than a local checkout or a fourth copy
// (the plan keeps three, byte-identical by test). The party sits at the gate
// with the Wiseman's letter on the ground beside it; the scout stands in the
// yard and the chief, the camp's MIND, in the hut. The camp is spawned
// through Spawn — id, ref, cell, faction and holdings straight off the
// compiled placements, the way rpg-api spawns it.

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/scenarios"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

const (
	campSession = "camp"
	campWorldID = "camp-world"
	campModule  = "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	campFile    = "dungeonspec/testdata/reference-raider-camp.yaml"
	campKey     = "reference-raider-camp"
	campFaction = "raiders"
	campLetter  = "letter"
	campChief   = "chief"
	campScout   = "scout"
	gateYard    = campKey + "/gate-yard"
	yardHut     = campKey + "/yard-hut"

	// feetPerCell is the composition's own scale, spelled once for the walk.
	feetPerCell = 5
)

// campSource reads the shipped camp from the pinned encounter module, resolved
// from go.mod by go list (the precedent rpg-api set for reading a pinned
// module's own files): what these scenes meet is exactly the file the
// composition this build runs against compiled and pictured.
func campSource(t *testing.T) string {
	t.Helper()
	out, err := exec.CommandContext(t.Context(), "go", "list", "-m", "-f", "{{.Dir}}", campModule).Output()
	if err != nil {
		t.Fatalf("resolve the pinned encounter module's directory via go list: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(strings.TrimSpace(string(out)), campFile))
	if err != nil {
		t.Fatalf("read the camp fixture: %v", err)
	}
	return string(raw)
}

// compileCamp compiles one source of the camp.
func compileCamp(t *testing.T, source string) dungeonspec.Compiled {
	t.Helper()
	compiled, err := dungeonspec.Load([]byte(source))
	if err != nil {
		t.Fatalf("the camp compiles: %v", err)
	}
	return compiled
}

// The step-B lines of the shipped fixture, spelled once: the letter's
// predicate and the three reinforcements — the encounter suite's own split.
const (
	letterArrives     = `, arrives: { round: 6 } }`
	reinforcementLine = `  - { id: reinforcement-%d, ref: "dnd5e:monsters:zombie", at: %s, faction: raiders, arrives: { down: chief } }` + "\n"
)

var reinforcementCells = []string{"[1,4]", "[2,4]", "[1,5]"}

// stepASource is the shipped camp with step B taken back out — the letter
// lying at the gate from the first frame and no reinforcements — refused
// when a line to remove is not there exactly once, so an edit to the fixture
// cannot silently turn this into the unstripped file.
func stepASource(t *testing.T, source string) string {
	t.Helper()
	if n := strings.Count(source, letterArrives); n != 1 {
		t.Fatalf("the letter's arrives appears %d times, not once", n)
	}
	source = strings.Replace(source, letterArrives, " }", 1)
	for i, at := range reinforcementCells {
		line := fmt.Sprintf(reinforcementLine, i+1, at)
		if n := strings.Count(source, line); n != 1 {
			t.Fatalf("reinforcement %d appears %d times, not once", i+1, n)
		}
		source = strings.Replace(source, line, "", 1)
	}
	return source
}

// arrivalOf spells a compiled placement's predicate in this package's own
// words — the switch a host writes to hand the composition's Trigger to
// Spawn, one arm per form of the grammar. Nil stays nil.
func arrivalOf(t *testing.T, trigger encounter.Trigger) session.Arrival {
	t.Helper()
	switch p := trigger.(type) {
	case nil:
		return nil
	case encounter.TriggerRound:
		return session.ArrivesAtRound{Round: p.Round}
	case encounter.TriggerMemberDown:
		return session.ArrivesOnFall{Member: string(p.Member)}
	case encounter.TriggerFact:
		return session.ArrivesOnFact{Fact: p.Fact}
	case encounter.TriggerStance:
		return session.ArrivesOnStance{Between: [2]string{p.Between[0], p.Between[1]}, Stance: string(p.Stance)}
	default:
		t.Fatalf("no arrival form for %T", trigger)
		return nil
	}
}

// campWorld is the authored world a session starts in: the compiled field,
// the party at the gate, and — when the scene wants the run to be able to
// END on the flip — the hold-out scenario's own ending, read from the file's
// binding through the scenario package, the same path a host takes. Without
// it the run stays open after the camp turns, which is what A9 needs.
//
// NO MONSTERS ARE AUTHORED. The world is built empty of them and each is
// brought in through Spawn, because that is how every monster enters a live
// run — the whole reason Spawn carries a faction at all.
func campWorld(t *testing.T, compiled dungeonspec.Compiled, withEnding bool) *encounter.EncounterData {
	t.Helper()
	endings := []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}}
	if withEnding {
		scenario, ok := scenarios.Lookup(scenarios.HoldOutID)
		if !ok {
			t.Fatalf("no %s scenario", scenarios.HoldOutID)
		}
		declared, err := scenario.New(compiled.Scenarios[scenarios.HoldOutID], scenarios.FactsFrom(compiled.Field))
		if err != nil {
			t.Fatalf("binding the hold-out: %v", err)
		}
		endings = append(endings, declared.Endings...)
	}

	seats := compiled.PartyStart
	if len(seats) < 2 {
		t.Fatalf("the camp seats %d, and the party is two", len(seats))
	}
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Striker: encounter.RefusingStriker{}, Announcer: encQuietAnnouncer{}, Sight: encEveryoneSees{},
		Initiative: encOrderAsGiven{}, TurnDriver: encPassDriver{}, Standing: encEveryoneStanding{},
		CheckResolver: encNeverResolves{}, Witness: encNeverWitnesses{},
		Field: compiled.Field,
		Members: []encounter.MemberInput{
			{ID: "alice", Kind: encounter.KindPlayer, Position: seats[0].At},
			{ID: "bob", Kind: encounter.KindPlayer, Position: seats[1].At},
		},
		Endings:   endings,
		Retention: encounter.RetentionUnbounded,
	})
	if err != nil {
		t.Fatalf("building the camp: %v", err)
	}
	data := enc.ToData()
	return &data
}

type HoldOutSessionSuite struct {
	suite.Suite

	// compiled is the camp AS STEP A HAD IT — the shipped file with every
	// `arrives` stripped (stepASource): the letter on the ground at the gate,
	// no reinforcements. The step-A scenes play on it unchanged. canonical is
	// the shipped file itself, arrivals and all; the step-B scenes in
	// holdout_reserve_test.go play on that one. camp is whichever the running
	// scene opened.
	compiled  dungeonspec.Compiled
	canonical dungeonspec.Compiled
	camp      dungeonspec.Compiled

	stream     *fakeStream
	sessions   *fakeSessions
	encounters *fakeEncounters
	characters *fakeCharacters
	mgr        *session.Manager
}

func TestHoldOutSessionSuite(t *testing.T) { suite.Run(t, new(HoldOutSessionSuite)) }

func (s *HoldOutSessionSuite) SetupSuite() {
	source := campSource(s.T())
	s.canonical = compileCamp(s.T(), source)
	s.compiled = compileCamp(s.T(), stepASource(s.T(), source))
}

// campOptions is how a scene opens the camp: the shipped file or the step-A
// variant, whether the hold-out ending is bound, who drives the monsters, who
// the party is, and which authored placements are spawned at the start (nil
// means all of them).
type campOptions struct {
	shipped    bool
	withEnding bool
	driver     session.TurnDriver
	cast       []*character.Data
	spawn      []string
}

// start wires a fresh manager around the camp and spawns it: the letter on
// the ground, nobody knowing anything, the monsters passing their turns, the
// stream cleared so a scene reads only what its own verbs caused.
func (s *HoldOutSessionSuite) start(withEnding bool) {
	s.startWith(campOptions{withEnding: withEnding})
}

// startWith is start with the scene's own choices; every zero value is the
// default start makes.
func (s *HoldOutSessionSuite) startWith(opts campOptions) {
	if opts.driver == nil {
		opts.driver = session.Pass{}
	}
	if opts.cast == nil {
		opts.cast = []*character.Data{sharpEyed("alice"), dullEyed("bob")}
	}
	s.camp = s.compiled
	if opts.shipped {
		s.camp = s.canonical
	}
	s.stream = &fakeStream{}
	s.sessions, s.encounters = newFakeSessions(), newFakeEncounters()
	s.characters = newFakeCharacters(opts.cast...)

	mgr, err := session.NewManager(&session.Config{PresentationIDs: testPresentationIDs{},
		Dice: testDice{}, TurnDriver: opts.driver,
		Sessions: s.sessions, Encounters: s.encounters,
		Characters: s.characters, Events: s.stream,
	})
	s.Require().NoError(err)
	s.mgr = mgr

	_, err = mgr.StartSession(context.Background(), &session.StartSessionInput{
		Session: campSession, Encounter: campWorldID, World: campWorld(s.T(), s.camp, opts.withEnding),
	})
	s.Require().NoError(err)

	for _, m := range s.camp.Monsters {
		if opts.spawn == nil || slices.Contains(opts.spawn, m.ID) {
			s.spawn(m)
		}
	}
	s.stream.published = nil
}

// placement is one authored placement by id.
func (s *HoldOutSessionSuite) placement(id string) dungeonspec.MonsterPlacement {
	s.T().Helper()
	for _, m := range s.camp.Monsters {
		if m.ID == id {
			return m
		}
	}
	s.Require().Failf("no placement", "%q is not placed in the camp", id)
	return dungeonspec.MonsterPlacement{}
}

// spawn brings one authored placement into the run through the host's verb:
// id, ref, cell, faction, holdings and arrival straight off the compiled
// placement.
func (s *HoldOutSessionSuite) spawn(m dungeonspec.MonsterPlacement) *session.SpawnOutput {
	s.T().Helper()
	out, err := s.mgr.Spawn(context.Background(), &session.SpawnInput{
		Session: campSession, ID: m.ID, Ref: m.Ref, Position: absolute(m.At),
		Holds: m.Holds, Faction: m.Faction, Arrives: arrivalOf(s.T(), m.Arrives),
	})
	s.Require().NoError(err, "spawning %s", m.ID)
	return out
}

func (s *HoldOutSessionSuite) hold(member, prop string) {
	s.T().Helper()
	_, err := s.mgr.Hold(context.Background(), &session.HoldInput{
		Session: campSession, Member: member, Target: prop, Range: 2})
	s.Require().NoError(err)
}

func (s *HoldOutSessionSuite) atlas(member string) *session.Atlas {
	s.T().Helper()
	atlas, err := s.mgr.Atlas(context.Background(), &session.AtlasInput{Session: campSession, Member: member})
	s.Require().NoError(err)
	return atlas
}

func (s *HoldOutSessionSuite) where(member string) spatial.Position {
	s.T().Helper()
	out, err := s.mgr.Where(context.Background(), &session.WhereInput{Session: campSession, Member: member})
	s.Require().NoError(err)
	return out.Position
}

func (s *HoldOutSessionSuite) turn(member string) *session.TurnOutput {
	s.T().Helper()
	out, err := s.mgr.Turn(context.Background(), &session.TurnInput{Session: campSession, Member: member})
	s.Require().NoError(err)
	return out
}

func (s *HoldOutSessionSuite) status() *session.Status {
	s.T().Helper()
	out, err := s.mgr.Status(context.Background(), &session.StatusInput{Session: campSession})
	s.Require().NoError(err)
	return out
}

func (s *HoldOutSessionSuite) roster() map[string]session.PublicMember {
	s.T().Helper()
	out, err := s.mgr.Roster(context.Background(), &session.RosterInput{Session: campSession, Player: "player-alice"})
	s.Require().NoError(err)
	rows := make(map[string]session.PublicMember, len(out.Members))
	for _, row := range out.Members {
		rows[row.ID] = row
	}
	return rows
}

func (s *HoldOutSessionSuite) endTurn(member string) {
	s.T().Helper()
	_, err := s.mgr.EndTurn(context.Background(), &session.EndTurnInput{
		Session: campSession, Member: member,
		DeclarationID: currentEndTurnID(s.T(), s.mgr, campSession, member),
	})
	s.Require().NoError(err)
}

// regionOf is which region owns a cell, read off the atlas — structure on
// the truth grain, the same for everyone.
func regionOf(atlas *session.Atlas, at spatial.Position) string {
	for _, r := range atlas.Regions {
		for _, c := range r.Cells {
			if c == at {
				return r.ID
			}
		}
	}
	return ""
}

// doorway is the two cells of a named doorway, the one in near first.
func (s *HoldOutSessionSuite) doorway(door, near string) (nearCell, farCell spatial.Position) {
	s.T().Helper()
	atlas := s.atlas("alice")
	for _, dw := range atlas.Doorways {
		if dw.Door != door {
			continue
		}
		if regionOf(atlas, dw.From) == near {
			return dw.From, dw.To
		}
		return dw.To, dw.From
	}
	s.Require().Failf("no doorway", "%s is not in the atlas", door)
	return spatial.Position{}, spatial.Position{}
}

// pathTo is the shortest legal walk from where a member stands to a cell: a
// breadth-first search over the atlas, stepping to an axial neighbour within
// one region or across a doorway — the rule the composition refuses a step
// by (ErrNoCrossing) — and around the cells the rest of the cast stands on.
// The path is DERIVED from the map rather than spelled in a scene, so a
// scene says where somebody walks and the map says how.
func (s *HoldOutSessionSuite) pathTo(member string, to spatial.Position) []spatial.Position {
	s.T().Helper()
	atlas := s.atlas(member)
	from := s.where(member)

	floor := make(map[spatial.Position]bool, len(atlas.Cells))
	for _, c := range atlas.Cells {
		floor[c] = true
	}
	for other := range s.roster() {
		if other != member {
			delete(floor, s.where(other))
		}
	}
	crossing := make(map[[2]spatial.Position]bool, 2*len(atlas.Doorways))
	for _, dw := range atlas.Doorways {
		crossing[[2]spatial.Position{dw.From, dw.To}] = true
		crossing[[2]spatial.Position{dw.To, dw.From}] = true
	}
	legal := func(a, b spatial.Position) bool {
		return floor[b] && (regionOf(atlas, a) == regionOf(atlas, b) || crossing[[2]spatial.Position{a, b}])
	}

	axial := []spatial.Position{{X: 1, Y: 0}, {X: -1, Y: 0}, {X: 0, Y: 1}, {X: 0, Y: -1}, {X: 1, Y: -1}, {X: -1, Y: 1}}
	before := map[spatial.Position]spatial.Position{from: from}
	queue := []spatial.Position{from}
	for len(queue) > 0 && before[to] == (spatial.Position{}) && to != from {
		at := queue[0]
		queue = queue[1:]
		for _, d := range axial {
			next := spatial.Position{X: at.X + d.X, Y: at.Y + d.Y}
			if _, seen := before[next]; seen || !legal(at, next) {
				continue
			}
			before[next] = at
			queue = append(queue, next)
		}
	}
	if _, reached := before[to]; !reached {
		s.Require().Failf("no path", "%s cannot walk from %v to %v", member, from, to)
	}
	var path []spatial.Position
	for at := to; at != from; at = before[at] {
		path = append([]spatial.Position{at}, path...)
	}
	return path
}

// freeNeighbour is a cell beside a member they can step onto: an axial
// neighbour in their own region that nobody stands on — for the scenes that
// need "a step, anywhere" as the verb that loads the world back.
func (s *HoldOutSessionSuite) freeNeighbour(member string) spatial.Position {
	s.T().Helper()
	atlas := s.atlas(member)
	from := s.where(member)
	taken := map[spatial.Position]bool{}
	for other := range s.roster() {
		taken[s.where(other)] = true
	}
	floor := map[spatial.Position]bool{}
	for _, c := range atlas.Cells {
		floor[c] = true
	}
	for _, d := range []spatial.Position{{X: 1, Y: 0}, {X: -1, Y: 0}, {X: 0, Y: 1}, {X: 0, Y: -1}, {X: 1, Y: -1}, {X: -1, Y: 1}} {
		next := spatial.Position{X: from.X + d.X, Y: from.Y + d.Y}
		if floor[next] && !taken[next] && regionOf(atlas, next) == regionOf(atlas, from) {
			return next
		}
	}
	s.Require().Failf("no free neighbour", "%s at %v has nowhere to step", member, from)
	return spatial.Position{}
}

// walk moves a member to a cell on whichever clock they are on. On the world
// clock a walk is free. On the turn clock it waits for the member's turn,
// spends what Afford says is left, ends the turn, and lets the table come
// back around — the monsters passing (session.Pass), the other player
// ending theirs. Returns the last Move's answer; stops when the run ends.
func (s *HoldOutSessionSuite) walk(member string, to spatial.Position) *session.MoveOutput {
	s.T().Helper()
	var last *session.MoveOutput
	for {
		path := s.pathTo(member, to)
		if len(path) == 0 {
			return last
		}
		in := &session.MoveInput{Session: campSession, Member: member, Path: path}
		if s.turn(member).Clock == session.ClockTurn {
			s.untilTurnOf(member)
			decl := currentDeclaration(s.T(), s.mgr, campSession, member, session.VerbMove)
			cells := len(path)
			if decl.Remaining != nil && *decl.Remaining/feetPerCell < cells {
				cells = *decl.Remaining / feetPerCell
			}
			if !decl.Available || cells == 0 {
				s.endTurn(member)
				continue
			}
			in.DeclarationID = decl.ID
			in.Path = path[:cells]
		}
		out, err := s.mgr.Move(context.Background(), in)
		s.Require().NoError(err, "%s walking to %v", member, to)
		s.Require().NotEmpty(out.Steps, "%s walked nowhere", member)
		last = out
		if out.Outcome != nil {
			return out
		}
	}
}

// untilTurnOf ends the other players' turns until the clock rests on member.
// A monster's turn never rests: the TurnDriver drives it at the boundary.
func (s *HoldOutSessionSuite) untilTurnOf(member string) {
	s.T().Helper()
	for {
		turn := s.turn(member)
		s.Require().Equal(session.ClockTurn, turn.Clock)
		if turn.Active == member {
			return
		}
		s.Require().Contains([]string{"alice", "bob"}, turn.Active,
			"the clock rests on a player; a monster's turn is driven, never waited on")
		s.endTurn(turn.Active)
	}
}

func (s *HoldOutSessionSuite) events(recipient string) []session.Event {
	return eventsFor(s.stream.published, recipient)
}

func (s *HoldOutSessionSuite) kinds(recipient string) []session.EventKind {
	var out []session.EventKind
	for _, e := range s.events(recipient) {
		out = append(out, e.Kind)
	}
	return out
}

// bodyOf fetches the one event of a kind on a recipient's stream, failing if
// there is not exactly one.
func (s *HoldOutSessionSuite) bodyOf(recipient string, kind session.EventKind) session.EventBody {
	s.T().Helper()
	var found []session.Event
	for _, e := range s.events(recipient) {
		if e.Kind == kind {
			found = append(found, e)
		}
	}
	s.Require().Len(found, 1, "%s should have exactly one %s: %v", recipient, kind, s.kinds(recipient))
	return found[0].Body
}

// recipients is everyone anything was delivered to, sorted.
func (s *HoldOutSessionSuite) recipients() []string {
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

// intoTheYard walks a member from the gate through the palisade's doorway.
func (s *HoldOutSessionSuite) intoTheYard(member string) *session.MoveOutput {
	s.T().Helper()
	_, far := s.doorway(gateYard, "gate")
	return s.walk(member, far)
}

// intoTheHut walks a member from the yard through the hut's doorway.
func (s *HoldOutSessionSuite) intoTheHut(member string) *session.MoveOutput {
	s.T().Helper()
	_, far := s.doorway(yardHut, "yard")
	return s.walk(member, far)
}

// TestSpawnedMonstersCarryTheirFactionAndAFightFormsBySide is A1 through the
// seam: the camp arrived through Spawn on its authored side, the roster says
// so, and — nobody knowing anything — a player walking into the yard starts
// a fight with the scout, by faction rather than by kind.
func (s *HoldOutSessionSuite) TestSpawnedMonstersCarryTheirFactionAndAFightFormsBySide() {
	s.start(false)

	s.Run("the roster says whose side everyone is on", func() {
		rows := s.roster()
		s.Require().Len(rows, 4)
		s.Equal("party", rows["alice"].Faction, "a player named no faction and is on the players' side")
		s.Equal("party", rows["bob"].Faction)
		s.Equal(campFaction, rows[campChief].Faction, "the side it was spawned with")
		s.Equal(campFaction, rows[campScout].Faction)
	})

	out := s.intoTheYard("alice")
	s.Run("a fight forms on sight", func() {
		s.Require().NotNil(out.Formed, "the scout and alice saw each other")
		s.Contains(out.Formed.Order, "alice")
		s.Contains(out.Formed.Order, campScout)
		s.Equal(session.ClockTurn, s.turn("alice").Clock)
		s.Equal(session.ClockTurn, s.turn(campScout).Clock)
		s.Contains(s.kinds("bob"), session.EventFightStarted, "a fight is not secret")
	})
}

// TestAMonsterSpawnedWithNoFactionIsInMonstersAndFightsAsItAlwaysDid is A7's
// live half on a dungeon that DOES declare factions: a spawn naming none is
// in the reserved `monsters`, hostile to the party as every monster always
// was — the composition's default, not this seam's — and a fight forms the
// moment it stands in the party's sight.
func (s *HoldOutSessionSuite) TestAMonsterSpawnedWithNoFactionIsInMonstersAndFightsAsItAlwaysDid() {
	s.start(false)

	// Two cells down the gate's own column from alice: in plain sight.
	at := s.compiled.PartyStart[0].At
	out, err := s.mgr.Spawn(context.Background(), &session.SpawnInput{
		Session: campSession, ID: "stray", Ref: refs.Monsters.Zombie().String(),
		Position: absolute(spatial.Position{X: at.X, Y: at.Y + 2}),
	})
	s.Require().NoError(err)
	s.Require().NotNil(out.Formed, "arriving in plain sight starts a fight, on the default table")
	s.Contains(out.Formed.Order, "stray")
	s.Equal("monsters", s.roster()["stray"].Faction, "the kind's default, decided once, in the composition")
}

// TestTheLetterCarriedToTheChiefMidFightTurnsTheCampOnTheWire is A2 at the
// seam, and the `ended` beat naming the hold-out: alice picks up the letter,
// walks into a fight in the yard, carries it into the hut on her turns —
// and every recipient's own stream reads stance, then the fight ending BY
// STANCE, then the run ending, each typed, each in dense numbering.
func (s *HoldOutSessionSuite) TestTheLetterCarriedToTheChiefMidFightTurnsTheCampOnTheWire() {
	s.start(true)
	s.hold("alice", campLetter)
	formed := s.intoTheYard("alice")
	s.Require().NotNil(formed.Formed, "precondition: the fight is on")
	s.Require().Equal(session.ClockTurn, s.turn("alice").Clock, "and the hut is walked to on her turns")
	s.stream.published = nil

	out := s.intoTheHut("alice")

	s.Run("the hold-out ended the run", func() {
		s.Require().NotNil(out.Outcome)
		s.Equal(scenarios.HoldOutID, out.Outcome.Ending)
		status := s.status()
		s.False(status.Open)
		s.Require().NotNil(status.Outcome)
		s.Equal(scenarios.HoldOutID, status.Outcome.Ending)
	})

	for _, who := range []string{"alice", "bob"} {
		s.Run(who+" reads the flip, the fight ending by stance, and the run ending, in that order", func() {
			// The pair is a SET: a disposition has no direction, and the
			// composition writes it in its own normalized order. A reader
			// matches the two ids, never a first and a second.
			turned, ok := s.bodyOf(who, session.EventStanceChanged).(session.StanceChangedBody)
			s.Require().True(ok, "the flip crosses as its typed body")
			s.ElementsMatch([]string{campFaction, encounter.FactionParty}, turned.Between)
			s.Equal(string(encounter.StanceNeutral), turned.Stance, "the author's word for what the pair now is")
			s.Equal(session.FightEndedBody{Cause: session.DissolveByStance}, s.bodyOf(who, session.EventFightEnded),
				"the fight lost its sides — not a decision, not a defeat")
			s.Equal(session.EndedBody{Ending: scenarios.HoldOutID}, s.bodyOf(who, session.EventEnded),
				"the scenario's ending, named as any declared ending is")

			var stanceAt, dissolvedAt, endedAt int
			for i, kind := range s.kinds(who) {
				switch kind {
				case session.EventStanceChanged:
					stanceAt = i
				case session.EventFightEnded:
					dissolvedAt = i
				case session.EventEnded:
					endedAt = i
				}
			}
			s.Less(stanceAt, dissolvedAt, "the flip is the cause, the dissolution its effect")
			s.Less(dissolvedAt, endedAt, "and the ending is the last word")
		})
	}

	s.Run("every recipient heard the stance turn exactly once, in their own dense numbering", func() {
		for _, who := range s.recipients() {
			s.bodyOf(who, session.EventStanceChanged)
			events := s.events(who)
			for i := 1; i < len(events); i++ {
				s.Require().Equal(events[i-1].Seq+1, events[i].Seq,
					"%s's own stream must be dense: seq %d follows %d", who, events[i].Seq, events[i-1].Seq)
			}
		}
	})

	s.Run("no beat reached a client as unknown", func() {
		for _, e := range s.stream.published {
			s.NotEqual(session.EventUnknown, e.Kind, "an armless beat narrates nothing")
			s.NotNil(e.Body, "and every kind this build names decodes its typed body: %s", e.Kind)
		}
	})
}

// TestTheStanceSurvivesSaveAndLoadAsAFoldNotAField is A9 through the seam.
// The session reloads the stored world on every verb, so the verb AFTER the
// flip is the load: bob walks into the scout's sight and no fight forms; the
// roster still says raiders; and nothing in the stored world but the story's
// own beat says "neutral" — the declared disposition still reads hostile.
func (s *HoldOutSessionSuite) TestTheStanceSurvivesSaveAndLoadAsAFoldNotAField() {
	s.start(false) // no ending bound: the run stays open once the camp turns
	s.hold("alice", campLetter)
	s.Require().NotNil(s.intoTheYard("alice").Formed)
	out := s.intoTheHut("alice")
	s.Require().Nil(out.Outcome, "nothing was declared to end on the flip")
	s.bodyOf("bob", session.EventStanceChanged)
	s.Require().Equal(session.ClockWorld, s.turn("alice").Clock, "the fight dissolved")

	s.Run("the declaration is untouched and no stance is written", func() {
		stored, ok := s.encounters.byID[campWorldID]
		s.Require().True(ok)
		s.Require().Len(stored.Field.Dispositions, 1)
		s.Equal(string(encounter.StanceHostile), stored.Field.Dispositions[0].Stance)
		structure, err := json.Marshal(struct {
			Field    encounter.FieldData
			Members  []encounter.MemberData
			World    *encounter.WorldData
			Holdings *encounter.HoldingsData
		}{stored.Field, stored.Members, stored.World, stored.Holdings})
		s.Require().NoError(err)
		s.NotContains(string(structure), `"neutral"`, "the only place neutral appears is the story's own beat")
	})

	s.stream.published = nil
	s.Run("the next verb loads the world back, and the camp is still turned", func() {
		// Beside the scout, in the yard, in plain sight of it.
		scout := s.where(campScout)
		out := s.walk("bob", spatial.Position{X: scout.X + 1, Y: scout.Y})
		s.Nil(out.Formed, "a raider in sight of a player, and no fight")
		s.Equal(session.ClockWorld, s.turn("bob").Clock)
		s.Equal(session.ClockWorld, s.turn(campScout).Clock)
		s.NotContains(s.kinds("alice"), session.EventFightStarted)
		s.Equal(campFaction, s.roster()[campScout].Faction, "membership is declaration; the stance is the fold")
	})
}

// TestASpawnCannotJoinAFactionTheDungeonDoesNotDeclare is the fail-closed
// half at the seam: a faction the file never declared is refused by name,
// as this package's own sentinel, and nothing is left behind — the mistake a
// host makes by forwarding a word the dungeon does not have, made loud
// rather than arriving on the wrong side.
func (s *HoldOutSessionSuite) TestASpawnCannotJoinAFactionTheDungeonDoesNotDeclare() {
	s.start(false)
	at := s.compiled.PartyStart[0].At

	_, err := s.mgr.Spawn(context.Background(), &session.SpawnInput{
		Session: campSession, ID: "stray", Ref: refs.Monsters.Zombie().String(),
		Position: absolute(spatial.Position{X: at.X, Y: at.Y + 2}), Faction: "kobolds",
	})
	s.Require().ErrorIs(err, session.ErrNoFaction)
	s.Require().NotErrorIs(err, session.ErrNoIntel, "a faction is not a record")

	var placed bool
	for _, npc := range s.sessions.byID[campSession].NPCs {
		if npc.ID == "stray" {
			placed = true
		}
	}
	s.False(placed, "the refusal left nothing behind")
	s.Empty(s.stream.published, "and told nobody")
	s.NotContains(s.roster(), "stray")
}
