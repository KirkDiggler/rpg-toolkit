// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package castletest

// castle_king_test.go is THE MAD KING'S CASTLE, PLAYED (rpg-project#605):
// the one studio document, compiled, opened the way a host opens it, and
// driven one route at a time. The routes live beside this file
// (castle_king_*_test.go); this file is the host they share.
//
// THE COMPILED CONTENT IS WHAT PLAYS. Every creature enters with the table,
// options, arrival, temper, holdings and facing the file compiled for it. No
// test swaps a table or adds a line: a route that needs one the file does not
// carry is a finding, not a fixture edit.
//
// THE HOST IS DELIBERATELY PLAIN. Everyone sees sixty feet and hears with a
// passive Perception of 10; everyone walks thirty feet and swings a spear
// at five. The world's die always rolls 1, so the first row a table offers is
// the one that fires and the first answer a line has is the one given. A
// temper is a soldier whatever word was dealt: what a temper means is the
// rulebook's, and no route here is about one.

import (
	"context"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

const (
	kingFixture = "../dungeonspec/testdata/world-builder-v4-castle.yaml"

	// hero is the party's one player.
	hero = encounter.MemberID("hero")
)

// The castle's cast, by the ids the file gives them.
const (
	kingCaptain = encounter.MemberID("captain")
	kingGate1   = encounter.MemberID("gate-1")
	kingGate2   = encounter.MemberID("gate-2")
	kingHall1   = encounter.MemberID("hall-1")
	kingCook    = encounter.MemberID("cook")
	kingKeep1   = encounter.MemberID("keep-1")
	kingKeep2   = encounter.MemberID("keep-2")
	theKing     = encounter.MemberID("king")
	kingReserve = encounter.MemberID("reserve-1")
)

// The castle's factions, as the file names them.
const (
	garrison  = encounter.FactionID("garrison")
	loyalists = encounter.FactionID("loyalists")
)

// The castle's doors, by the compiled ids of the studio's wall doors. The
// file shuts every one, and locks two: the cell door, and the kitchen's door
// into the great hall (sneaking S1).
const (
	// kingGateDoor is the gatehouse's one way into the courtyard.
	kingGateDoor = encounter.DoorID("the-mad-king/gate-wall-door")

	// kingHallDoor is the courtyard's door into the great hall.
	kingHallDoor = encounter.DoorID("the-mad-king/hall-wall-door")

	// kingKitchenDoor, kingChapelDoor and kingThroneDoor are the great
	// hall's doors into the kitchen, the chapel and the throne room.
	kingKitchenDoor = encounter.DoorID("the-mad-king/kitchen-wall-door")
	kingChapelDoor  = encounter.DoorID("the-mad-king/chapel-wall-door")
	kingThroneDoor  = encounter.DoorID("the-mad-king/throne-wall-door")
)

// castleCell is the dungeon-absolute cell of an offset [col, row] on the
// studio's floor: the frame the keep fixture places its rooms in, and the
// frame [encounter.Encounter.Members] answers positions in.
func castleCell(col, row int) spatial.Position {
	return encounter.HexCellAt(encounter.HexesArePointyTop(), col, row)
}

// Where routes begin, as offset cells for [encounter.MemberInput.Position].
var (
	// besideTheCook is the kitchen floor beside the cook, where every route
	// that starts with the kitchen's gossip begins.
	besideTheCook = spatial.Position{X: -19, Y: -12}

	// besideTheKing is the throne room floor beside the king.
	besideTheKing = spatial.Position{X: -5, Y: -17}
)

// kingSpear is the one weapon everybody in this host swings.
var kingSpear = core.Ref{Module: "dnd5e", Type: "weapons", ID: "spear"}

// kingScene is one run of the castle and what the host recorded of it.
type kingScene struct {
	t        testing.TB
	enc      *encounter.Encounter
	compiled dungeonspec.Compiled
	strikes  *[]cellsStrike
	standing *castleStanding

	// host is how the castle was opened, kept so [kingScene.reload] can load
	// the persisted encounter back with the same capabilities.
	host castleHost
}

// openTheCastle compiles the file and opens it with the hero at a cell and
// every creature where the file posts it, carrying what the file compiled.
func openTheCastle(t testing.TB, at spatial.Position) *kingScene {
	t.Helper()

	return openTheCastleWithDoors(t, at, nil)
}

// openTheCastleWithDoors is [openTheCastle] with some doors starting in a
// state the file does not give them, so a test can stand a room's door shut
// with every other door open without walking a hero to each.
func openTheCastleWithDoors(t testing.TB, at spatial.Position, doors map[encounter.DoorID]encounter.DoorState) *kingScene {
	t.Helper()

	return openTheCastleWithEars(t, at, doors, nil)
}

// openTheCastleWithEars is [openTheCastleWithDoors] with some members'
// passive Perception other than the host's 10, so a test can find the DC a
// sound reaches a room at.
func openTheCastleWithEars(
	t testing.TB, at spatial.Position, doors map[encounter.DoorID]encounter.DoorState, passive map[encounter.MemberID]int,
) *kingScene {
	t.Helper()

	return openTheCastleAs(t, castleHost{at: at, doors: doors, passive: passive})
}

// castleHost is everything a route may set about how the castle is opened:
// where the hero starts, doors standing in a state the file does not give
// them, passive Perception other than 10, more players beside the hero, the
// check resolver (the host's nothingIsEverFound when nil), and watch arcs for
// some members (none — all round — for everybody else), how far everybody
// sees (the host's sixty feet when zero), and walking speeds other than the
// host's thirty feet for some members.
type castleHost struct {
	at       spatial.Position
	doors    map[encounter.DoorID]encounter.DoorState
	passive  map[encounter.MemberID]int
	players  []encounter.MemberInput
	resolver encounter.CheckResolver
	arcs     map[encounter.MemberID]int
	sightFt  int
	speedFt  map[encounter.MemberID]int

	// heroFacing is the hero's stated facing, or nil for the unstated one.
	heroFacing *float64

	// noReload opts a route out of [kingScene.reload]: the encounter is never
	// saved and loaded back between turns.
	noReload bool

	// without names endings of the file a route opens without, so a run the
	// ending would close plays on past it: the fall route keeps playing after
	// the king is down, where `king-slain` would settle it.
	without []string
}

// openTheCastleAs compiles the file and opens it the way the host says.
func openTheCastleAs(t testing.TB, host castleHost) *kingScene {
	t.Helper()
	compiled := loadContent(t, kingFixture)
	for i := range compiled.Field.Doors {
		if state, ok := host.doors[compiled.Field.Doors[i].ID]; ok {
			compiled.Field.Doors[i].State = state
		}
	}

	members := []encounter.MemberInput{{ID: hero, Kind: encounter.KindPlayer, Position: host.at, Facing: host.heroFacing}}
	members = append(members, host.players...)
	for _, m := range compiled.Monsters {
		var holds []encounter.IntelID
		for _, h := range m.Holds {
			holds = append(holds, encounter.IntelID(h))
		}
		members = append(members, encounter.MemberInput{
			ID:       encounter.MemberID(m.MemberID),
			Kind:     encounter.KindMonster,
			Position: m.At,
			Faction:  encounter.FactionID(m.Faction),
			Table:    m.Table,
			Options:  m.Options,
			Arrives:  m.Arrives,
			Temper:   soldierProfiles(m.Temper),
			Holds:    holds,
			Facing:   m.FacingDegrees,
		})
	}

	strikes := &[]cellsStrike{}
	standing := &castleStanding{down: map[encounter.MemberID]bool{}}
	endings := []encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}}
	for _, e := range compiled.Endings {
		if !slices.Contains(host.without, e.Key) {
			endings = append(endings, e)
		}
	}
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field:   compiled.Field,
		Members: members,
		Endings: endings,
		// Unbounded, so a route can read a creature's whole story back.
		Retention:    encounter.RetentionUnbounded,
		Capabilities: host.capabilities(strikes, standing),
	})
	require.NoError(t, err)

	return &kingScene{t: t, enc: enc, compiled: compiled, strikes: strikes, standing: standing, host: host}
}

// capabilities is everything the host supplies the encounter, built in one
// place so a route opened and a route reloaded play against the same host.
func (host castleHost) capabilities(strikes *[]cellsStrike, standing *castleStanding) encounter.Capabilities {
	var resolver encounter.CheckResolver = nothingIsEverFound{}
	if host.resolver != nil {
		resolver = host.resolver
	}

	return encounter.Capabilities{
		Sight:         castleSight{feet: host.sightFt},
		Equipment:     encounter.UnobservedEquipment{},
		Conditions:    encounter.UnobservedEquipment{},
		Sheets:        castleSheets{passive: host.passive, arcs: host.arcs, speed: host.speedFt},
		Participation: standing,
		Initiative:    orderAsGiven{},
		Driver:        encounter.TableDriver{Roller: rollsOne{}},
		Roller:        rollsOne{},
		CheckResolver: resolver,
		Witness:       nobodyPerceivesAnything{},
		Actors: encounter.Actors{
			Striker:   cellsStriker{calls: strikes},
			Mover:     quietMover{},
			Announcer: quietAnnouncer{},
		},
	}
}

// castleReload is the package's switch for [kingScene.reload]: on unless a
// run says `-castle.reload=false`.
var castleReload = flag.Bool("castle.reload", true,
	"save and load back the castle encounter at every EndTurn and world-round boundary")

// reload saves the encounter the way a host does — ToData, through JSON — and
// plays on from what loads back, at the boundaries a host reloads at: every
// EndTurn and every world round (rpg-project#667). A fact the blob does not
// carry is then lost here exactly as it is lost on a deployed stack, which is
// how the castle routes catch it: a `search` row that came back wordless
// (rpg-toolkit#2114) failed the S1 route under this and passed without it.
//
// The host's own values (the strikes it recorded, who is down) are the
// host's, not the blob's, and carry over as a real host's store does.
func (s *kingScene) reload() {
	s.t.Helper()
	if !*castleReload || s.host.noReload {
		return
	}
	blob, err := json.Marshal(s.enc.ToData())
	require.NoError(s.t, err)
	var data encounter.EncounterData
	require.NoError(s.t, json.Unmarshal(blob, &data))
	enc, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{
		Data: data, Capabilities: s.host.capabilities(s.strikes, s.standing),
	})
	require.NoError(s.t, err, "the castle loads back from its own blob")
	s.enc = enc
}

// reloadOnRound reloads when the world clock has moved past round: a world
// round ended under the step just taken, which is a boundary a host reloads
// at.
func (s *kingScene) reloadOnRound(round int) {
	s.t.Helper()
	if s.enc.ToData().Clock.HighWater != round {
		s.reload()
	}
}

// soldierProfiles completes a compiled temper the way a host does, with the
// profile every dealt word means — here, a soldier's for every word.
func soldierProfiles(temper encounter.Temper) encounter.Temper {
	if len(temper.Mix) == 0 {
		return temper
	}
	temper.Profiles = make(map[string]encounter.TemperProfile, len(temper.Mix))
	for word := range temper.Mix {
		temper.Profiles[word] = encounter.TemperProfile{}
	}

	return temper
}

// speak says one of a creature's lines with the hero's check already rolled.
func (s *kingScene) speak(target encounter.MemberID, option, skill string, beaten bool) (*encounter.SpeakOutput, error) {
	s.t.Helper()

	return s.enc.Speak(context.Background(), &encounter.SpeakInput{
		Actor: hero, Target: target, OptionID: option, Skill: skill,
		Beaten: beaten, DC: 15, Total: map[bool]int{true: 20, false: 3}[beaten], Roller: rollsOne{},
	})
}

// said is speak that must be accepted.
func (s *kingScene) said(target encounter.MemberID, option, skill string, beaten bool) {
	s.t.Helper()
	_, err := s.speak(target, option, skill, beaten)
	require.NoError(s.t, err, "%s offers %q", target, option)
}

// walkTo walks the hero one cell at a time along the pathfinder's own route
// toward a cell, until the hero stands beside it or a fight takes the walk.
func (s *kingScene) walkTo(to spatial.Position) {
	s.t.Helper()
	route, err := s.enc.Route(encounter.RouteInput{Mover: hero, Policy: encounter.MoveToward, Anchor: to, Budget: 200})
	require.NoError(s.t, err)
	require.Empty(s.t, route.StoppedBy, "the hero has a way to %v", to)
	for i, cell := range route.Path {
		round := s.enc.ToData().Clock.HighWater
		_, err := s.enc.Step(&encounter.StepInput{Member: hero, To: cell, EndWalk: i == len(route.Path)-1})
		require.NoError(s.t, err)
		s.reloadOnRound(round)
		if clockKind(s.t, s.enc, hero) == encounter.ClockTurn {
			return
		}
	}
}

// open opens a door the hero is standing within reach of.
func (s *kingScene) open(door encounter.DoorID) {
	s.t.Helper()
	_, err := s.enc.OpenDoor(&encounter.OpenDoorInput{Door: door, Actor: hero})
	require.NoError(s.t, err, "the hero reaches %s", door)
}

// outOfTheKitchen brings the hero from the kitchen to the yard by the only
// way there is: the kitchen door into the great hall — locked, so the hero
// picks it — across the hall, and out by the hall door, which it leaves open.
func (s *kingScene) outOfTheKitchen() {
	s.t.Helper()
	// Toward the hall cell past the shut kitchen door: the walk ends inside
	// the kitchen, beside the door, in reach.
	s.walkTo(castleCell(-14, -12))
	s.pickTheKitchenDoor()
	// Toward the yard cell past the shut hall door: the walk ends inside it.
	s.walkTo(castleCell(0, -6))
	s.open(kingHallDoor)
}

// kitchenDoorPick is the kitchen door's thieves'-tools route, as the file
// authors it.
var kitchenDoorPick = encounter.CheckApproach{Ability: "dex", Tool: "dnd5e:item:thieves-tools", DC: 13}

// kitchenDoorForce is the kitchen door's Strength route, as the file authors
// it: a beaten one is a door broken in, and it crashes.
var kitchenDoorForce = encounter.CheckApproach{Ability: "str", DC: 15}

// pickTheKitchenDoor picks the locked kitchen door with the hero's
// thieves' tools, already rolled and beaten: the lock beaten opens it,
// silently.
func (s *kingScene) pickTheKitchenDoor() {
	s.t.Helper()
	_, err := s.enc.Unlock(&encounter.UnlockInput{
		Door: kingKitchenDoor, Actor: hero, Beaten: true, Total: 17, Applied: kitchenDoorPick,
	})
	require.NoError(s.t, err, "the hero picks the kitchen door")
}

// throughTheGate brings the hero from the courtyard to the gate door, opens
// it, and walks on to a member in the gatehouse.
func (s *kingScene) throughTheGate(to encounter.MemberID) {
	s.t.Helper()
	s.walkTo(castleCell(0, 6))
	s.open(kingGateDoor)
	s.walkTo(s.at(to))
}

// partyStart is the file's own party start, as an offset cell.
func partyStart(t *testing.T) spatial.Position {
	t.Helper()
	compiled := loadContent(t, kingFixture)
	require.NotEmpty(t, compiled.PartyStart)

	return compiled.PartyStart[0].At
}

// at is where a member stands now.
func (s *kingScene) at(id encounter.MemberID) spatial.Position {
	s.t.Helper()
	members, err := s.enc.Members()
	require.NoError(s.t, err)
	for _, m := range members {
		if m.ID == id {
			return m.Position
		}
	}
	require.FailNowf(s.t, "not in the run", "%s", id)

	return spatial.Position{}
}

// learners is every member who has learned a fact, by the journal's subject.
func (s *kingScene) learners(fact string) map[encounter.MemberID]bool {
	s.t.Helper()
	out := map[encounter.MemberID]bool{}
	for _, f := range s.enc.ToData().Journal.Facts {
		if f.Kind == "known:fact:"+fact {
			out[encounter.MemberID(f.Subject)] = true
		}
	}

	return out
}

// answeredWith is every creature the audience saw answer with a row that
// teaches a fact, in story order: who SAID it, which the journal of who
// learned it does not record.
func (s *kingScene) answeredWith(fact string, audience encounter.MemberID) []encounter.MemberID {
	s.t.Helper()
	story, err := s.enc.Story(&encounter.StoryInput{Audience: audience})
	require.NoError(s.t, err)
	var out []encounter.MemberID
	for _, beat := range story {
		var body struct {
			Beat     string `json:"beat"`
			Creature string `json:"creature"`
			Fact     string `json:"fact"`
		}
		require.NoError(s.t, json.Unmarshal(beat.Payload, &body))
		if body.Beat == encounter.BeatAnswered && body.Fact == fact {
			out = append(out, encounter.MemberID(body.Creature))
		}
	}

	return out
}

// factionOf is the faction the file places a member in; the hero's is the
// party.
func (s *kingScene) factionOf(id encounter.MemberID) encounter.FactionID {
	s.t.Helper()
	if id == hero {
		return encounter.FactionParty
	}
	for _, m := range s.compiled.Monsters {
		if encounter.MemberID(m.MemberID) == id {
			return encounter.FactionID(m.Faction)
		}
	}
	require.FailNowf(s.t, "not in the file", "%s", id)

	return ""
}

// garrisonMembers is every member the file places in the garrison.
func (s *kingScene) garrisonMembers() []encounter.MemberID {
	var out []encounter.MemberID
	for _, m := range s.compiled.Monsters {
		if encounter.FactionID(m.Faction) == garrison {
			out = append(out, encounter.MemberID(m.MemberID))
		}
	}

	return out
}

// stance is the declared-plus-known stance between two factions now.
func (s *kingScene) stance(a, b encounter.FactionID) encounter.Stance {
	s.t.Helper()
	out, err := s.enc.Stance(a, b)
	require.NoError(s.t, err)

	return out
}

// hostile is whether the graph holds two members enemies now.
func (s *kingScene) hostile(a, b encounter.MemberID) bool {
	s.t.Helper()
	out, known := s.enc.IsHostile(a, b)
	require.True(s.t, known, "%s and %s are both in the run", a, b)

	return out
}

// ending is the run's ending, or "" while it is open.
func (s *kingScene) ending() string {
	s.t.Helper()
	status, err := s.enc.Status()
	require.NoError(s.t, err)
	if status.Open || status.Outcome == nil {
		return ""
	}

	return status.Outcome.Ending
}

// struck lands one swing of the hero's: a hit, and a fall when it drops.
func (s *kingScene) struck(target encounter.MemberID, drops bool) {
	s.t.Helper()
	if drops {
		s.standing.down[target] = true
	}
	_, err := s.enc.RecordTrain(&encounter.RecordTrainInput{Units: []encounter.TrainUnit{{Outcome: &encounter.RecordInput{
		Kind: encounter.OutcomeStruck, Actor: hero, Targets: []encounter.MemberID{target},
		Attack: &encounter.AttackIdentity{Ref: kingSpear.String(), Name: "Spear", DamageType: "piercing"},
		Values: map[encounter.OutcomeValue]int{encounter.ValueAmount: 6},
	}}}})
	require.NoError(s.t, err)
}

// endTurn ends the hero's turn in a fight, and every driven turn after it
// runs before this returns.
func (s *kingScene) endTurn() {
	s.t.Helper()
	s.reload()
	_, err := s.enc.EndTurn(&encounter.EndTurnInput{Member: hero})
	require.NoError(s.t, err)
}

// castleSpeedFeet is everybody's walking speed in this host.
const castleSpeedFeet = 30

// sessionSightFeet is what the session sees with: the rulebook's stated
// default for a sheet that states no senses (combat.DefaultSightFeet), which
// is every sheet the castle seats. This composition may not import the
// rulebook (C1), so the number is mirrored here and
// [TestSessionSightFeetIsTheRulebooks] reads the rulebook's own line to keep
// the two the same.
const sessionSightFeet = 120

// castleSight is a torchlit castle: everybody sees sixty feet, or what the
// host says ([castleHost.sightFt]).
type castleSight struct{ feet int }

func (s castleSight) Sight(members []encounter.MemberID) (map[encounter.MemberID]int, error) {
	feet := s.feet
	if feet == 0 {
		feet = 60
	}
	out := make(map[encounter.MemberID]int, len(members))
	for _, id := range members {
		out[id] = encounter.CellsFromFeet(feet)
	}

	return out, nil
}

// castleSheets is everybody's sheet in this host: thirty feet unless speed
// names another, a spear at five, a passive Perception of 10 unless passive
// names another, and the watch arc arcs names, if any.
type castleSheets struct {
	passive map[encounter.MemberID]int
	arcs    map[encounter.MemberID]int
	speed   map[encounter.MemberID]int
}

func (c castleSheets) Sheets(members []encounter.MemberID) (map[encounter.MemberID]encounter.SheetFacts, error) {
	out := make(map[encounter.MemberID]encounter.SheetFacts, len(members))
	for _, id := range members {
		passive, ok := c.passive[id]
		if !ok {
			passive = 10
		}
		speed, ok := c.speed[id]
		if !ok {
			speed = castleSpeedFeet
		}
		out[id] = encounter.SheetFacts{
			SpeedFeet:         speed,
			Actions:           []encounter.ActionView{{Ref: kingSpear, RangeFeet: 5}},
			PassivePerception: passive,
			WatchArc:          c.arcs[id],
		}
	}

	return out, nil
}

// castleStanding is the host's answer to who is standing: everybody, until
// a swing the host scored as a fall puts somebody down.
type castleStanding struct{ down map[encounter.MemberID]bool }

func (c *castleStanding) Standing(members []encounter.MemberID) ([]encounter.MemberID, error) {
	var out []encounter.MemberID
	for _, id := range members {
		if c.down[id] {
			out = append(out, id)
		}
	}

	return out, nil
}

func (c *castleStanding) Assess(members []encounter.MemberID) (*encounter.ParticipationAssessment, error) {
	assessment := &encounter.ParticipationAssessment{}
	for _, id := range members {
		assessment.Members = append(assessment.Members, encounter.MemberParticipation{
			Member: id, Contact: !c.down[id], Conscious: !c.down[id], Down: c.down[id],
			Turn: encounter.TurnParticipationWait,
		})
	}

	return assessment, nil
}

// rulebookSightLine is the rulebook's own declaration of its default sight,
// read as text: this module may not import it (C1).
var rulebookSightLine = regexp.MustCompile(`(?m)^const DefaultSightFeet = (\d+)$`)

// TestSessionSightFeetIsTheRulebooks pins [sessionSightFeet] to the line that
// declares combat.DefaultSightFeet, so a route run at the session's sight is
// run at the sight the session actually has.
func TestSessionSightFeetIsTheRulebooks(t *testing.T) {
	t.Parallel()
	src, err := os.ReadFile(filepath.Join("..", "..", "combat", "sight.go"))
	require.NoError(t, err, "the rulebook's sight is declared beside this module")
	m := rulebookSightLine.FindSubmatch(src)
	require.NotNil(t, m, "combat/sight.go declares DefaultSightFeet")
	feet, err := strconv.Atoi(string(m[1]))
	require.NoError(t, err)
	require.Equal(t, feet, sessionSightFeet, "the castle's session sight is the rulebook's default")
}
