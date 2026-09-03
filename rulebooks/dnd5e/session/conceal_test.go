// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

// conceal_test.go drives the seam's half of living-world slice 1
// (rpg-toolkit#1375; ruled on rpg-project#350 and #351): the two supplied
// capabilities, the per-member reads, the Search verb, the applied unlock
// route, the reveal beats on the event surface, and per-recipient dense
// numbering. The composition's own laws (never-authored yardstick, probe law,
// move law, failed-equals-empty) are pinned in the encounter suite; what is
// pinned HERE is that they survive the seam — same bytes through translate,
// same secrecy through delivery — with the real resolver and the real
// witness in the loop instead of scripted stand-ins.
//
// Every positional fixture is the session's own pointy-top AxialHexGrid
// canvas (grid law), and expectations are computed from the authored shape.

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/races"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/skills"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// cell is an AUTHORED position: regions, walls, doors and member starts are
// written in the authoring frame (odd rows shoved east) and converted once by
// the field. Everything a VERB takes — paths, spawn and join cells — speaks
// the wire's dungeon-absolute axial frame instead, so those use hexCell,
// which is the same conversion the field applies (the staggered-corridor
// lesson: say which frame you mean and convert exactly once).
func cell(x, y int) spatial.Position { return spatial.Position{X: float64(x), Y: float64(y)} }

// axialWall is one blocking crossing between two axial cells.
func axialWall(from, to spatial.Position) encounter.WallInput {
	return encounter.WallInput{Boundary: spatial.Boundary{
		From: from, To: to, BlocksMovement: true, BlocksLineOfSight: true,
	}}
}

// axialSeam walls every crossing between column 5 and column 6 of the two
// 6x6 fixture rooms. Under the canvas's pointy adjacency (odd rows shoved
// east), every row has the straight (5,y)-(6,y) crossing and ODD rows also
// reach (6,y∓1) diagonally — those are all of them, and the builder was
// checked against the field's own adjacency refusals rather than assumed.
// gapRow leaves that row's straight crossing open for the door; -1 seals the
// seam whole.
func axialSeam(gapRow int) []encounter.WallInput {
	var out []encounter.WallInput
	for y := 0; y < 6; y++ {
		if y != gapRow {
			out = append(out, axialWall(cell(5, y), cell(6, y)))
		}
		if y%2 == 1 {
			out = append(out, axialWall(cell(5, y), cell(6, y-1)))
			if y+1 < 6 {
				out = append(out, axialWall(cell(5, y), cell(6, y+1)))
			}
		}
	}
	return out
}

// encNeverResolves and encNeverWitnesses satisfy Setup's capability
// requirement while a concealed FIXTURE world is being authored — nothing
// searches and no concealed door stands open during authoring, so a consult
// during the build is the fixture's own bug.
type encNeverResolves struct{}

func (encNeverResolves) ResolveCheck(*encounter.ResolveCheckInput) (*encounter.ResolveCheckOutput, error) {
	panic("a fixture build resolved a check")
}

type encNeverWitnesses struct{}

func (encNeverWitnesses) Perceivers(*encounter.PerceiversInput) ([]encounter.MemberID, error) {
	return nil, nil
}

// findDCPerception and findDCInvestigation price the veil's two find routes.
// Sharp-eyed alice (+4 perception) beats 12 on testDice's flat 10; dull bob
// (+0 everything) beats neither.
const (
	findDCPerception    = 12
	findDCInvestigation = 14
)

// veilFind is the veil door's find check: spotted or reasoned out, each
// route priced separately (the multi-approach ruling).
func veilFind() []encounter.CheckApproach {
	return []encounter.CheckApproach{
		{Ability: "perception", DC: findDCPerception},
		{Ability: "investigation", DC: findDCInvestigation},
	}
}

// concealedWorld is the minimal honest dungeon, at session scale: a visible
// 6x6 hall, a concealed 6x6 vault, and one concealed door — the veil — as
// the whole frontier between them. alice and bob stand in the hall with
// ordinary sight; carol stands in the far corner seeing five feet (one
// cell), the fixture's way of authoring a member the witness honestly
// answers "no" about without a second sight model.
//
//	hall (visible)          vault (concealed)
//	carol . . . . .  |  . . . . . .
//	  .   . . . . .  |  . . . . . .
//	  .   . a b . veil  . . . . . .      a=alice(1,1) b=bob(2,1)
//	                 ^ (5,0)-(6,0), closed, find: veilFind()
func concealedWorld(t fataler, doorState encounter.DoorState) *encounter.EncounterData {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Striker: encounter.RefusingStriker{}, Announcer: encQuietAnnouncer{}, Sight: encEveryoneSees{},
		Initiative: encOrderAsGiven{}, TurnDriver: encPassDriver{},
		Standing:      encEveryoneStanding{},
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
				State:     doorState,
				Concealed: veilFind(),
			}},
		},
		Members: []encounter.MemberInput{
			{ID: "alice", Kind: encounter.KindPlayer, Position: cell(1, 1)},
			{ID: "bob", Kind: encounter.KindPlayer, Position: cell(2, 1)},
			{ID: "carol", Kind: encounter.KindPlayer, Position: cell(0, 5), SightFeet: 5},
		},
		Endings: []encounter.EndingInput{
			{Key: "out", Trigger: encounter.TriggerExternal{}},
		},
	})
	if err != nil {
		t.Fatalf("building concealed world: %v", err)
	}
	data := enc.ToData()
	return &data
}

// concealRegion marks an authored region as hidden space.
func concealRegion(r encounter.RegionInput) encounter.RegionInput {
	r.Concealed = true
	return r
}

// walledTwinWorld is concealedWorld with the secret replaced by honesty:
// both rooms authored visible, a solid seam wall where the veil hides — the
// move law's comparison world.
func walledTwinWorld(t fataler) *encounter.EncounterData {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Striker: encounter.RefusingStriker{}, Announcer: encQuietAnnouncer{}, Sight: encEveryoneSees{},
		Initiative: encOrderAsGiven{}, TurnDriver: encPassDriver{},
		Standing: encEveryoneStanding{},
		Field: encounter.FieldInput{Canvas: pointyCanvas(),
			Regions: []encounter.RegionInput{
				rectRegion("hall", 0, 0, 6, 6),
				rectRegion("vault", 6, 0, 6, 6),
			},
			Walls: axialSeam(-1),
		},
		Members: []encounter.MemberInput{
			{ID: "alice", Kind: encounter.KindPlayer, Position: cell(1, 1)},
			{ID: "bob", Kind: encounter.KindPlayer, Position: cell(2, 1)},
			{ID: "carol", Kind: encounter.KindPlayer, Position: cell(0, 5), SightFeet: 5},
		},
		Endings: []encounter.EndingInput{
			{Key: "out", Trigger: encounter.TriggerExternal{}},
		},
	})
	if err != nil {
		t.Fatalf("building walled twin: %v", err)
	}
	data := enc.ToData()
	return &data
}

// plainHallWorld is the hall alone — no vault, no walls, no secret; the map
// edge is its own solid mass. The fixture for scenes about worlds with
// nothing concealed in them.
func plainHallWorld(t fataler) *encounter.EncounterData {
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Striker: encounter.RefusingStriker{}, Announcer: encQuietAnnouncer{}, Sight: encEveryoneSees{},
		Initiative: encOrderAsGiven{}, TurnDriver: encPassDriver{},
		Standing: encEveryoneStanding{},
		Field: encounter.FieldInput{Canvas: pointyCanvas(),
			Regions: []encounter.RegionInput{rectRegion("hall", 0, 0, 6, 6)},
		},
		Members: []encounter.MemberInput{
			{ID: "alice", Kind: encounter.KindPlayer, Position: cell(1, 1)},
			{ID: "bob", Kind: encounter.KindPlayer, Position: cell(2, 1)},
			{ID: "carol", Kind: encounter.KindPlayer, Position: cell(0, 5), SightFeet: 5},
		},
		Endings: []encounter.EndingInput{
			{Key: "out", Trigger: encounter.TriggerExternal{}},
		},
	})
	if err != nil {
		t.Fatalf("building plain hall: %v", err)
	}
	data := enc.ToData()
	return &data
}

// sharpEyed is a searcher whose proficient perception (+2 WIS, +2 prof)
// beats the veil's DC 12 on a flat 10; investigation stays +0.
func sharpEyed(id string) *character.Data {
	return &character.Data{
		ID: id, PlayerID: "player-" + id, Name: "Sharp", Level: 3,
		ProficiencyBonus: 2, RaceID: races.Dwarf, ClassID: classes.Rogue,
		HitPoints: 20, MaxHitPoints: 20, ArmorClass: 14,
		AbilityScores: shared.AbilityScores{abilities.WIS: 14, abilities.DEX: 14},
		Skills:        map[skills.Skill]shared.ProficiencyLevel{skills.Perception: shared.Proficient},
	}
}

// dullEyed rolls +0 on every listed route: flat 10 beats neither find DC.
func dullEyed(id string) *character.Data {
	return &character.Data{
		ID: id, PlayerID: "player-" + id, Name: "Dull", Level: 1,
		ProficiencyBonus: 2, RaceID: races.Human, ClassID: classes.Fighter,
		HitPoints: 10, MaxHitPoints: 10, ArmorClass: 10,
		AbilityScores: shared.AbilityScores{},
	}
}

// armedSearcher is a finder who can also fight: attack_test.go's armedFighter
// — longsword, martial proficiency, STR 16 — given proficient perception, so
// ONE member can find the veil and then swing inside the fight it does not go
// away for. WIS 12 (+1) and proficiency (+2) put testDice's flat 10 at 13,
// one over the veil's DC 12; investigation stays +0 against DC 14, so she
// finds it by exactly the route and margin sharpEyed does.
func armedSearcher(id string) *character.Data {
	sheet := armedFighter(id)
	sheet.Skills = map[skills.Skill]shared.ProficiencyLevel{skills.Perception: shared.Proficient}
	return sheet
}

type ConcealSuite struct {
	suite.Suite

	stream     *fakeStream
	sessions   session.SessionRepository
	encounters session.EncounterRepository
	characters session.CharacterRepository
	mgr        *session.Manager
}

func TestConcealSuite(t *testing.T) {
	suite.Run(t, new(ConcealSuite))
}

// startWith wires a fresh manager around the given world and cast. Monster
// turns pass: every scene here but the fought-in one is about what a member
// reads and is told, never about what a monster does on its own turn.
func (s *ConcealSuite) startWith(world *encounter.EncounterData, cast ...*character.Data) {
	s.startDriven(session.Pass{}, world, cast...)
}

// startDriven is startWith with the monster TurnDriver named, so one scene can
// let the real behaviour take a monster's turn while the rest stay quiet.
func (s *ConcealSuite) startDriven(
	driver session.TurnDriver, world *encounter.EncounterData, cast ...*character.Data,
) {
	s.stream = &fakeStream{}
	s.sessions, s.encounters = newFakeSessions(), newFakeEncounters()
	s.characters = newFakeCharacters(cast...)
	mgr, err := session.NewManager(&session.Config{
		Dice: testDice{}, TurnDriver: driver,
		Sessions: s.sessions, Encounters: s.encounters,
		Characters: s.characters, Events: s.stream,
	})
	s.Require().NoError(err)
	s.mgr = mgr

	_, err = mgr.StartSession(context.Background(), &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: world,
	})
	s.Require().NoError(err)
}

// eventsFor filters the published stream down to one recipient, in order.
func eventsFor(published []session.Event, recipient string) []session.Event {
	var out []session.Event
	for _, e := range published {
		if e.Recipient == recipient {
			out = append(out, e)
		}
	}
	return out
}

// assertDense fails unless the recipient's numbers are consecutive — the
// gap-oracle kill, stated as one check on the values.
func (s *ConcealSuite) assertDense(events []session.Event, who string) {
	s.T().Helper()
	for i := 1; i < len(events); i++ {
		s.Require().Equal(events[i-1].Seq+1, events[i].Seq,
			"%s's own stream must be dense: seq %d follows %d", who, events[i].Seq, events[i-1].Seq)
	}
}

// TestSearchRevealsToTheSearcherAlone is the slice's headline scene: alice
// searches the hall, the veil appears for her alone, and nothing about the
// verb — response, delivery, or any other member's stream — tells the table
// a question was even asked.
func (s *ConcealSuite) TestSearchRevealsToTheSearcherAlone() {
	ctx := context.Background()
	s.startWith(concealedWorld(s.T(), encounter.DoorIsClosed()), sharpEyed("alice"))

	out, err := s.mgr.Search(ctx, &session.SearchInput{
		Session: "sess", Member: "alice", Region: "hall"})
	s.Require().NoError(err)
	s.Equal([]string{"encounter:world", "session:sess"}, out.Saved.Written,
		"the found fact rides the world; the advanced stream cursors ride the session")

	// The searcher's own stream carries the reveal, typed.
	alice := eventsFor(s.stream.published, "alice")
	s.Require().Len(alice, 1, "one door_revealed for the finder")
	s.Equal(session.EventDoorRevealed, alice[0].Kind)
	body, ok := alice[0].Body.(session.DoorRevealedBody)
	s.Require().True(ok, "a reveal event carries its typed body")
	s.Equal("veil", body.Door)
	s.Equal("closed", body.State, "found is not opened — concealment looks like a shut door")
	s.Require().Len(body.Doorways, 1)
	s.Equal(session.AtlasDoorway{Door: "veil", From: cell(5, 0), To: cell(6, 0)}, body.Doorways[0],
		"the body is the patch for the cached atlas's doorway list")
	s.Empty(body.Approaches, "an unlocked door carries no lock routes")

	// Nobody else hears anything — no beat, and (per-recipient numbering)
	// no readable hole either.
	s.Empty(eventsFor(s.stream.published, "bob"))
	s.Empty(eventsFor(s.stream.published, "carol"))

	// The per-member reads agree with the streams.
	doors, err := s.mgr.Doors(ctx, &session.DoorsInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)
	s.Require().Len(doors.Doors, 1, "the finder's door list holds the veil now")
	s.Equal("veil", doors.Doors[0].ID)

	unfound, err := s.mgr.Doors(ctx, &session.DoorsInput{Session: "sess", Member: "bob"})
	s.Require().NoError(err)
	s.Empty(unfound.Doors, "for bob the veil is still nowhere")

	// Finding the door is not seeing behind it: both atlases still hold the
	// hall alone — 36 cells of a 6x6 region — and the vault stays
	// never-authored even for the finder.
	found, err := s.mgr.Atlas(ctx, &session.AtlasInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)
	s.Len(found.Cells, 36, "the vault does not exist for alice yet — she knows a DOOR")
	s.Require().Len(found.Doorways, 1, "but the doorway is on her map")

	blind, err := s.mgr.Atlas(ctx, &session.AtlasInput{Session: "sess", Member: "bob"})
	s.Require().NoError(err)
	s.Len(blind.Cells, 36)
	s.Empty(blind.Doorways, "bob's map shows no doorway")
	s.Len(blind.Boundaries, len(found.Boundaries)+1,
		"encounter v0.45.1's boundary rule masks the unfound veil even where it borders "+
			"hidden space: floor that ends without a wall but still refuses movement is itself "+
			"a tell; alice's found doorway replaces exactly that one masquerade wall")
}

// TestAFailedSearchAndNothingToFindAreIdentical carries failed-equals-empty
// across the seam: same response, same save report, same silent stream.
func (s *ConcealSuite) TestAFailedSearchAndNothingToFindAreIdentical() {
	ctx := context.Background()

	s.startWith(concealedWorld(s.T(), encounter.DoorIsClosed()), dullEyed("bob"))
	failed, err := s.mgr.Search(ctx, &session.SearchInput{
		Session: "sess", Member: "bob", Region: "hall"})
	s.Require().NoError(err)
	failedEvents := len(s.stream.published)

	s.startWith(plainHallWorld(s.T()), dullEyed("bob"))
	empty, err := s.mgr.Search(ctx, &session.SearchInput{
		Session: "sess", Member: "bob", Region: "hall"})
	s.Require().NoError(err)

	s.Equal(empty, failed, "the whole output, compared as a value: a failed roll and a "+
		"hall with nothing in it answer with the same bytes")
	s.Zero(failedEvents, "and neither says anything to anybody")
	s.Empty(s.stream.published)
}

// TestSearchIsPresenceEnforced: the region searched is the one the searcher
// stands in, and a region that does not exist refuses IDENTICALLY — a
// distinct answer would let a guessed ID probe for hidden rooms.
func (s *ConcealSuite) TestSearchIsPresenceEnforced() {
	ctx := context.Background()
	s.startWith(concealedWorld(s.T(), encounter.DoorIsClosed()), sharpEyed("alice"))

	_, elsewhere := s.mgr.Search(ctx, &session.SearchInput{
		Session: "sess", Member: "alice", Region: "vault"})
	s.Require().ErrorIs(elsewhere, session.ErrElsewhere)

	_, phantom := s.mgr.Search(ctx, &session.SearchInput{
		Session: "sess", Member: "alice", Region: "no-such-place"})
	s.Require().ErrorIs(phantom, session.ErrElsewhere)
}

// TestASearcherWithNoSheetIsRefusedBeforeTheRoomIsLookedAt: characters are
// the only searchers in v1, and the refusal must not vary with what the
// region holds — a monster refused only in rooms with something to find
// would be an oracle. Same member, a room with a secret and a room without:
// byte-identical refusals.
func (s *ConcealSuite) TestASearcherWithNoSheetIsRefusedBeforeTheRoomIsLookedAt() {
	ctx := context.Background()

	spawnSkeleton := func(at spatial.Position) {
		_, err := s.mgr.Spawn(ctx, &session.SpawnInput{
			Session: "sess", ID: "skel-1", Ref: refs.Monsters.Skeleton().String(),
			Position: at,
		})
		s.Require().NoError(err)
	}

	// Spawned INSIDE the vault: out of everyone's sight, so no fight forms.
	s.startWith(concealedWorld(s.T(), encounter.DoorIsClosed()))
	spawnSkeleton(hexCell(9, 4))
	_, secret := s.mgr.Search(ctx, &session.SearchInput{
		Session: "sess", Member: "skel-1", Region: "hall"})
	s.Require().ErrorIs(secret, session.ErrNoCharacter)

	s.startWith(plainHallWorld(s.T()))
	spawnSkeleton(hexCell(4, 4))
	_, plain := s.mgr.Search(ctx, &session.SearchInput{
		Session: "sess", Member: "skel-1", Region: "hall"})
	s.Require().ErrorIs(plain, session.ErrNoCharacter)

	s.Equal(plain.Error(), secret.Error(),
		"the refusal is decided by the sheet, never by the room's contents")
}

// TestAFightFormsOnAConcealedDungeon is the tripwire's promised replacement
// (rpg-toolkit#1378): TestAFightOnAConcealedDungeonFailsClosedForNow pinned
// that fight formation refused while resolution reloaded the world without
// the concealment capabilities, and documented its own deletion the day
// resolution grew them. It has. Every Resolve site now hands down the same
// checkSeam and witnessSeam the seam's own verbs bind, so the exact path
// that refused — fight formation, the announcer, resolution.Resolve — is
// the one this scene drives to a formed fight.
//
// The second half is the law the tripwire existed to protect while the path
// was closed: COMBAT IS NOT A REVEAL. The fight forms on a world already
// carrying a real found fact (alice searched first, so the reload replays
// history, not a virgin blob), and mid-fight each member still gets exactly
// their own answer — the finder keeps her door, and the non-knower's
// projection is the never-authored one, byte-identical to what he was told
// before anything spawned: the hall's 36 cells and not a doorway on it.
//
// Formation is where this scene stops. What happens INSIDE the formed fight —
// the three mid-fight Resolve sites this one never reaches — belongs to
// TestTheFightOnAConcealedDungeonIsFoughtIn below (rpg-toolkit#1398).
func (s *ConcealSuite) TestAFightFormsOnAConcealedDungeon() {
	ctx := context.Background()
	s.startWith(concealedWorld(s.T(), encounter.DoorIsClosed()), sharpEyed("alice"), dullEyed("bob"))

	// alice finds the veil before contact: the world under the fight holds
	// a detection fact only she may be shown.
	_, err := s.mgr.Search(ctx, &session.SearchInput{
		Session: "sess", Member: "alice", Region: "hall"})
	s.Require().NoError(err)

	before, err := s.mgr.Atlas(ctx, &session.AtlasInput{Session: "sess", Member: "bob"})
	s.Require().NoError(err)

	// In plain view of the hall: contact on arrival, and the fight FORMS.
	_, err = s.mgr.Spawn(ctx, &session.SpawnInput{
		Session: "sess", ID: "skel-1", Ref: refs.Monsters.Skeleton().String(),
		Position: hexCell(4, 4),
	})
	s.Require().NoError(err,
		"the fight forms: resolution carries CheckResolver and Witness down to its reload now")

	var started []session.Event
	for _, e := range s.stream.published {
		if e.Kind == session.EventFightStarted {
			started = append(started, e)
		}
	}
	s.Require().NotEmpty(started, "the spawn pulled the hall into one bubble")
	body, ok := started[0].Body.(session.FightStartedBody)
	s.Require().True(ok, "got %T", started[0].Body)
	s.Contains(body.Members, "skel-1")
	s.Contains(body.Members, "alice")

	// Concealment law, knower's side: the fight did not take her door away.
	doors, err := s.mgr.Doors(ctx, &session.DoorsInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)
	s.Require().Len(doors.Doors, 1, "the finder keeps the veil mid-fight")
	s.Equal("veil", doors.Doors[0].ID)

	// Non-knower's side: bob's mid-fight projection is the never-authored
	// answer — and not merely the right shape, the SAME BYTES the seam gave
	// him before the fight existed.
	during, err := s.mgr.Atlas(ctx, &session.AtlasInput{Session: "sess", Member: "bob"})
	s.Require().NoError(err)
	s.Len(during.Cells, 36, "the vault still does not exist for bob")
	s.Empty(during.Doorways, "and no doorway leads to it")
	s.Equal(before, during, "combat taught a non-knower nothing: projection unchanged, byte for byte")
}

// TestTheFightOnAConcealedDungeonIsFoughtIn carries the scene above past the
// moment of formation: the fight that forms on a concealed world is then
// FOUGHT IN — one honest round of it — and every mid-fight verb reloads that
// world through resolution's door on the way.
//
// # Why a second scene exists (rpg-toolkit#1398)
//
// Formation drives exactly one of session's four resolution.Resolve sites,
// the announcer's. PR #1397 supplied CheckResolver and Witness at all four,
// and its review MEASURED the other three by mutation: dropping the pair at
// striker.go, attack.go or activate.go each left the whole suite green. On a
// concealed world a dropped pair is not a subtle regression — it is
// encounter's load door refusing to reconstruct — so a refactor could ship
// green and the first swing inside a concealed dungeon would fail mid-fight,
// which is the exact failure #1378 existed to close. One round of combat is
// the whole pin:
//
//   - alice, the finder, swings her longsword at the zombie — attack.go
//   - bob, who never found the veil, Dodges on his own turn — activate.go
//   - the zombie takes its turn and slams back — striker.go
//
// Measured the same way once it was written — drop the pair at one site, run
// the whole suite, put the file back with cp — and all three mutants that
// survived before die HERE, this scene the only failure in every round:
//
//	attack.go    attack: resolution: load world: load encounter: encounter: no check resolver capability
//	activate.go  activate: … the same refusal, from Dodge's own reload
//	striker.go   endturn: end turn "carol": drive monster turns "zed-1": execute: strike: strike: … the same refusal
//
// announcer.go's mutant still fails TestAFightFormsOnAConcealedDungeon, and
// fails this scene too now — the one site whose coverage rested on a single
// test has two.
//
// # And the law the tripwire existed to protect
//
// COMBAT IS NOT A REVEAL, now through a FOUGHT round rather than a formed
// one. bob watches a swing go past, spends his own turn dodging, and shares
// the bubble with a monster whose own turn reloaded the same concealed world;
// his Atlas afterwards is byte-identical to the never-authored answer he was
// handed before the zombie existed.
func (s *ConcealSuite) TestTheFightOnAConcealedDungeonIsFoughtIn() {
	ctx := context.Background()
	// The real monster behaviour, not Pass: the monster's own turn is what
	// reaches strikerSeam.Strike, and a passing driver never gets there.
	s.startDriven(session.Behavior(), concealedWorld(s.T(), encounter.DoorIsClosed()),
		armedSearcher("alice"), armedFighter("bob"), dullEyed("carol"))

	// alice finds the veil before contact, so every reload below replays a
	// world carrying a real detection fact rather than a virgin blob.
	_, err := s.mgr.Search(ctx, &session.SearchInput{
		Session: "sess", Member: "alice", Region: "hall"})
	s.Require().NoError(err)

	before, err := s.mgr.Atlas(ctx, &session.AtlasInput{Session: "sess", Member: "bob"})
	s.Require().NoError(err)

	// Contact in the hall, one cell from alice: the fight forms, and she
	// leads the order.
	//
	// A ZOMBIE RATHER THAN THE FORMATION SCENE'S SKELETON, for two fixture
	// reasons this scene needs and that one does not: 22 hit points survive
	// alice's longsword (testDice's flat d8 plus STR makes 13, and a
	// 13-hit-point skeleton dies on the spot, ending the fight before anyone
	// else can act), and slam is its only attack, so the monster whose turn
	// drives the striker swings at the member standing next to it rather than
	// choosing a bow target across the hall.
	spawned, err := s.mgr.Spawn(ctx, &session.SpawnInput{
		Session: "sess", ID: "zed-1", Ref: refs.Monsters.Zombie().String(),
		Position: hexCell(1, 2),
	})
	s.Require().NoError(err)
	s.Require().NotNil(spawned.Formed, "a monster in plain view and in reach forms the fight on the spot")
	s.Equal([]string{"alice", "bob", "carol", "zed-1"}, spawned.Formed.Order)

	// THE SWING — attack.go's own reload of the concealed world. Plain dice
	// on purpose: nothing about this gate is about the faces.
	swing, err := s.mgr.Attack(ctx, &session.AttackInput{
		Session: "sess", Attacker: "alice", Target: "zed-1",
		DeclarationID: currentAttackID(s.T(), s.mgr, "sess", "alice"),
	})
	s.Require().NoError(err,
		"a swing inside a concealed dungeon reconstructs the world through resolution's load door")
	s.Equal(10, swing.Roll, "testDice's flat d20")
	s.Equal(15, swing.Total, "10 + STR(+3) + proficiency(+2) — the longsword compiles, it is not waved through")
	s.Equal(8, swing.Against, "the zombie's own armour class")
	s.True(swing.Hit, "15 beats 8")
	s.Equal(11, swing.Damage, "a lawful d8 at its face plus STR(+3) — not enough to drop a 22-hit-point zombie")

	endTurn := func(who string) *session.EndTurnOutput {
		out, err := s.mgr.EndTurn(ctx, &session.EndTurnInput{
			Session: "sess", Member: who,
			DeclarationID: currentEndTurnID(s.T(), s.mgr, "sess", who)})
		s.Require().NoError(err)
		return out
	}
	s.Equal("bob", endTurn("alice").Next)

	// THE ACTIVATION — activate.go's reload, on the turn of the member who
	// knows nothing about the veil. Dodge is the ability every character
	// already carries, so this needs no fixture of its own.
	dodged, err := s.mgr.Activate(ctx, &session.ActivateInput{
		Session: "sess", Member: "bob",
		DeclarationID: activationSelector(s.T(), s.mgr, "bob", "dnd5e:combat_abilities:dodge"),
	})
	s.Require().NoError(err,
		"activating inside a concealed dungeon reconstructs the world through the same door")
	s.Equal("dnd5e:combat_abilities:dodge", dodged.Ability)

	s.Equal("carol", endTurn("bob").Next)

	// THE MONSTER'S TURN — striker.go's reload, driven inside carol's EndTurn
	// because the zombie is next in the order and no player ends its turn.
	s.stream.published = nil
	wrapped := endTurn("carol")
	s.True(wrapped.RoundWrapped, "the zombie closed the round")
	s.Equal("alice", wrapped.Next, "which wraps back to whoever led it")

	swungBack := false
	for _, e := range eventsFor(s.stream.published, "alice") {
		switch body := e.Body.(type) {
		case session.StruckBody:
			swungBack = swungBack || body.Attacker == "zed-1"
		case session.MissedBody:
			swungBack = swungBack || body.Attacker == "zed-1"
		}
	}
	s.True(swungBack, "the zombie's own turn resolved a real strike through the striker seam")

	// Knower's side: a fought round did not take her door away.
	doors, err := s.mgr.Doors(ctx, &session.DoorsInput{Session: "sess", Member: "alice"})
	s.Require().NoError(err)
	s.Require().Len(doors.Doors, 1, "the finder still holds the veil after the round")
	s.Equal("veil", doors.Doors[0].ID)

	// Non-knower's side: the never-authored answer, and the SAME BYTES he was
	// given before any of this happened.
	after, err := s.mgr.Atlas(ctx, &session.AtlasInput{Session: "sess", Member: "bob"})
	s.Require().NoError(err)
	s.Len(after.Cells, 36, "the vault still does not exist for bob")
	s.Empty(after.Doorways, "and no doorway leads to it")
	s.Equal(before, after,
		"a fought round taught a non-knower nothing: projection unchanged, byte for byte")
}

// TestTheResolverAppliesTheBestListedApproach shows the selection rule
// holding THROUGH THE SEAM, end to end: best = the best chance of success —
// modifier against the route's own DC — never the biggest modifier. mira's
// +5 perception loses to DC 16 while her bare +0 investigation beats DC 10;
// a resolver that grabbed the big number would fail this search. The rule
// itself lives behind resolution's door now (resolution v0.27.0's MakeCheck,
// toolkit#1380 — its own suite pins the selection); what this scene owns is
// that the seam hands the whole authored list down and the verdict comes
// back through Search unchanged.
func (s *ConcealSuite) TestTheResolverAppliesTheBestListedApproach() {
	ctx := context.Background()

	world := func() *encounter.EncounterData {
		enc, err := encounter.NewEncounter(&encounter.SetupInput{
			Striker: encounter.RefusingStriker{}, Announcer: encQuietAnnouncer{}, Sight: encEveryoneSees{},
			Initiative: encOrderAsGiven{}, TurnDriver: encPassDriver{},
			Standing:      encEveryoneStanding{},
			CheckResolver: encNeverResolves{},
			Witness:       encNeverWitnesses{},
			Field: encounter.FieldInput{Canvas: pointyCanvas(),
				Regions: []encounter.RegionInput{
					rectRegion("hall", 0, 0, 6, 6),
					concealRegion(rectRegion("vault", 6, 0, 6, 6)),
				},
				Walls: axialSeam(0),
				Doors: []encounter.DoorInput{{
					ID:    "veil",
					Edges: []encounter.DoorEdge{{From: cell(5, 0), To: cell(6, 0)}},
					State: encounter.DoorIsClosed(),
					Concealed: []encounter.CheckApproach{
						{Ability: "perception", DC: 16},
						{Ability: "investigation", DC: 10},
					},
				}},
			},
			Members: []encounter.MemberInput{
				{ID: "mira", Kind: encounter.KindPlayer, Position: spatial.Position{X: 1, Y: 1}},
			},
			Endings: []encounter.EndingInput{{Key: "out", Trigger: encounter.TriggerExternal{}}},
		})
		if err != nil {
			s.T().Fatalf("building margin world: %v", err)
		}
		data := enc.ToData()
		return &data
	}()

	mira := &character.Data{
		ID: "mira", PlayerID: "player-mira", Name: "Mira", Level: 3,
		ProficiencyBonus: 2, RaceID: races.Elf, ClassID: classes.Ranger,
		HitPoints: 20, MaxHitPoints: 20, ArmorClass: 14,
		AbilityScores: shared.AbilityScores{abilities.WIS: 16, abilities.INT: 10},
		Skills:        map[skills.Skill]shared.ProficiencyLevel{skills.Perception: shared.Proficient},
	}
	s.startWith(world, mira)

	_, err := s.mgr.Search(ctx, &session.SearchInput{
		Session: "sess", Member: "mira", Region: "hall"})
	s.Require().NoError(err)

	events := eventsFor(s.stream.published, "mira")
	s.Require().Len(events, 1,
		"flat 10 + 0 investigation meets its DC 10; flat 10 + 5 perception misses its DC 16 — "+
			"only best-by-margin finds this door")
	s.Equal(session.EventDoorRevealed, events[0].Kind)
}

// TestUnlockPicksTheRouteAndReportsItsDC is the applied-route contract on the
// verb the caller can see: the session resolves the member's best listed
// approach (brawn's +2 STR against DC 12 beats his +0 DEX against DC 14),
// fills the composition's Applied, and the answer carries THAT route's DC —
// with a bare ability ref rolling the raw ability, no skill involved.
func (s *ConcealSuite) TestUnlockPicksTheRouteAndReportsItsDC() {
	ctx := context.Background()

	world := func() *encounter.EncounterData {
		enc, err := encounter.NewEncounter(&encounter.SetupInput{
			Striker: encounter.RefusingStriker{}, Announcer: encQuietAnnouncer{}, Sight: encEveryoneSees{},
			Initiative: encOrderAsGiven{}, TurnDriver: encPassDriver{},
			Standing: encEveryoneStanding{},
			Field: encounter.FieldInput{Canvas: pointyCanvas(),
				Regions: []encounter.RegionInput{
					rectRegion("hall", 0, 0, 6, 6),
					rectRegion("store", 6, 0, 6, 6),
				},
				Walls: axialSeam(0),
				Doors: []encounter.DoorInput{{
					ID:    "gate",
					Edges: []encounter.DoorEdge{{From: cell(5, 0), To: cell(6, 0)}},
					State: encounter.DoorIsLocked(encounter.Lock{Approaches: []encounter.CheckApproach{
						{Ability: "str", DC: 12},
						{Ability: "dex", Tool: "dnd5e:item:thieves-tools", DC: 14},
					}}),
				}},
			},
			Members: []encounter.MemberInput{
				{ID: "brawn", Kind: encounter.KindPlayer, Position: spatial.Position{X: 5, Y: 0}},
			},
			Endings: []encounter.EndingInput{{Key: "out", Trigger: encounter.TriggerExternal{}}},
		})
		if err != nil {
			s.T().Fatalf("building lock world: %v", err)
		}
		data := enc.ToData()
		return &data
	}()

	brawn := &character.Data{
		ID: "brawn", PlayerID: "player-brawn", Name: "Brawn", Level: 1,
		ProficiencyBonus: 2, RaceID: races.HalfOrc, ClassID: classes.Barbarian,
		HitPoints: 14, MaxHitPoints: 14, ArmorClass: 12,
		AbilityScores: shared.AbilityScores{abilities.STR: 14},
	}
	s.startWith(world, brawn)

	out, err := s.mgr.Unlock(ctx, &session.UnlockInput{
		Session: "sess", Member: "brawn", Door: "gate"})
	s.Require().NoError(err)
	s.True(out.Beaten, "flat 10 + 2 STR meets the forced route's DC 12")
	s.Equal(12, out.Total)
	s.Equal(12, out.DC, "the DC reported is the APPLIED route's — never the other listed route's 14")
	s.Equal(session.DoorApproach{Ability: "str", DC: 12}, out.Applied)
	s.Equal("open", out.Door.State)
}

// TestTheProbeLawHoldsAtTheSeam: everywhere a door id is spoken, a concealed
// unfound door answers byte-identically to a door that does not exist — and
// because Unlock reads the lock through the member's own knowledge, not even
// a die is rolled against the hidden lock.
func (s *ConcealSuite) TestTheProbeLawHoldsAtTheSeam() {
	ctx := context.Background()
	world := concealedWorld(s.T(),
		encounter.DoorIsLocked(encounter.Lock{Approaches: []encounter.CheckApproach{{Ability: "dex", DC: 12}}}))

	rolled := 0
	s.stream = &fakeStream{}
	s.sessions, s.encounters = newFakeSessions(), newFakeEncounters()
	s.characters = newFakeCharacters(sharpEyed("alice"), dullEyed("bob"))
	mgr, err := session.NewManager(&session.Config{
		Dice: testDice{calls: &rolled}, TurnDriver: session.Pass{},
		Sessions: s.sessions, Encounters: s.encounters,
		Characters: s.characters, Events: s.stream,
	})
	s.Require().NoError(err)
	s.mgr = mgr
	_, err = mgr.StartSession(ctx, &session.StartSessionInput{
		Session: "sess", Encounter: "world", World: world})
	s.Require().NoError(err)

	_, hidden := s.mgr.Unlock(ctx, &session.UnlockInput{Session: "sess", Member: "bob", Door: "veil"})
	s.Require().ErrorIs(hidden, session.ErrNoConnection)
	s.Zero(rolled, "no check rolls against a lock the member has not found — "+
		"the lock is read through DoorsFor, so for bob there is no lock")

	_, phantom := s.mgr.Unlock(ctx, &session.UnlockInput{Session: "sess", Member: "bob", Door: "phantom"})
	s.Require().ErrorIs(phantom, session.ErrNoConnection)
	s.Equal(phantom.Error(), hidden.Error(),
		"unlocking the veil and unlocking a door that was never authored are the same bytes")

	_, hiddenOpen := s.mgr.OpenDoor(ctx, &session.OpenDoorInput{Session: "sess", Member: "bob", Door: "veil"})
	_, phantomOpen := s.mgr.OpenDoor(ctx, &session.OpenDoorInput{Session: "sess", Member: "bob", Door: "phantom"})
	s.Require().Error(hiddenOpen)
	s.Equal(phantomOpen.Error(), hiddenOpen.Error(), "and OpenDoor answers the same way")

	// The knower's answer is the real door: alice finds it, and the same
	// verb that refused bob now names the lock's own refusal path for her.
	_, err = s.mgr.Search(ctx, &session.SearchInput{Session: "sess", Member: "alice", Region: "hall"})
	s.Require().NoError(err)
	out, err := s.mgr.Unlock(ctx, &session.UnlockInput{Session: "sess", Member: "alice", Door: "veil"})
	s.Require().NoError(err)
	s.True(out.Beaten, "flat 10 + 2 DEX meets DC 12 — the finder rolls the real lock")
}

// TestTheMoveLawHoldsAtTheSeam: a walk into a concealed unfound crossing is
// refused byte-identically to the honest twin's authored wall.
func (s *ConcealSuite) TestTheMoveLawHoldsAtTheSeam() {
	ctx := context.Background()
	walkAt := func(world *encounter.EncounterData) error {
		s.startWith(world, dullEyed("bob"))
		// bob steps to the doorway cell, then tries the crossing itself.
		_, err := s.mgr.Move(ctx, &session.MoveInput{
			Session: "sess", Member: "bob",
			Path: []spatial.Position{cell(3, 1), cell(4, 0), cell(5, 0), cell(6, 0)}})
		return err
	}

	veiled := walkAt(concealedWorld(s.T(), encounter.DoorIsClosed()))
	s.Require().Error(veiled)

	walled := walkAt(walledTwinWorld(s.T()))
	s.Require().Error(walled)

	s.Equal(walled.Error(), veiled.Error(),
		"walking into the veil and walking into the twin's wall are the same refusal, byte for byte")
}

// TestOpeningRevealsToThePerceiversThroughTheOneSeam is the witness scene:
// the door opens, and who learns is decided by the SAME sight answers the
// percept machinery uses — alice (finder, in range) gets the room, bob
// (never searched, in range) gets door and room by seeing it open, carol
// (five feet of sight, far corner) gets nothing until she walks up.
func (s *ConcealSuite) TestOpeningRevealsToThePerceiversThroughTheOneSeam() {
	ctx := context.Background()
	s.startWith(concealedWorld(s.T(), encounter.DoorIsClosed()),
		sharpEyed("alice"), dullEyed("bob"), dullEyed("carol"))

	_, err := s.mgr.Search(ctx, &session.SearchInput{Session: "sess", Member: "alice", Region: "hall"})
	s.Require().NoError(err)

	s.stream.published = nil
	out, err := s.mgr.OpenDoor(ctx, &session.OpenDoorInput{Session: "sess", Member: "alice", Door: "veil"})
	s.Require().NoError(err)
	s.Equal("open", out.Door.State)

	// alice knew the door; perceiving it OPEN hands her the room.
	alice := eventsFor(s.stream.published, "alice")
	kinds := func(events []session.Event) []session.EventKind {
		out := make([]session.EventKind, 0, len(events))
		for _, e := range events {
			out = append(out, e.Kind)
		}
		return out
	}
	s.Contains(kinds(alice), session.EventDoor, "the state change reaches the knower")
	s.Contains(kinds(alice), session.EventRegionRevealed)
	s.assertDense(alice, "alice")

	// bob never searched: watching the wall open IS his reveal — door and
	// room together, and deliberately no door-state beat for the door he
	// did not know a moment ago.
	bob := eventsFor(s.stream.published, "bob")
	s.Equal([]session.EventKind{session.EventDoorRevealed, session.EventRegionRevealed}, kinds(bob),
		"perceiving an open concealed door reveals the door and the room behind it")
	s.assertDense(bob, "bob")

	region, ok := bob[1].Body.(session.RegionRevealedBody)
	s.Require().True(ok)
	s.Equal("vault", region.Region.ID)
	s.Len(region.Region.Cells, 36, "the patch carries the region's whole authored slice — 6x6")
	s.NotEmpty(region.Boundaries, "with every boundary touching its cells, border walls included")

	// carol sees five feet from the far corner: the one sight seam answers
	// "no" for her exactly as her own percepts would, so she learns nothing.
	s.Empty(eventsFor(s.stream.published, "carol"))

	blind, err := s.mgr.Atlas(ctx, &session.AtlasInput{Session: "sess", Member: "carol"})
	s.Require().NoError(err)
	s.Len(blind.Cells, 36, "carol's map still holds the hall alone")

	// She walks up; her own verb's sight refresh is where present state
	// reaches her — the late perceiver needs no special case.
	s.stream.published = nil
	_, err = s.mgr.Move(ctx, &session.MoveInput{
		Session: "sess", Member: "carol",
		Path: []spatial.Position{
			hexCell(1, 5), hexCell(2, 4), hexCell(2, 3),
			hexCell(3, 2), hexCell(3, 1), hexCell(4, 0)}})
	s.Require().NoError(err)

	carol := eventsFor(s.stream.published, "carol")
	s.Contains(kinds(carol), session.EventDoorRevealed, "walking up to an open door reveals it")
	s.Contains(kinds(carol), session.EventRegionRevealed)
	s.assertDense(carol, "carol")

	after, err := s.mgr.Atlas(ctx, &session.AtlasInput{Session: "sess", Member: "carol"})
	s.Require().NoError(err)
	s.Len(after.Cells, 72, "and her map holds both rooms now")
}

// TestAJoinIntoHiddenSpaceStaysHidden covers the first-crosser branch the
// engine review seeded (PR #1373: a mover included in a frontier-scoped beat
// before their own fold answers), on the session verb that can actually
// reach it with the honest witness in play: a member PLACED inside a
// concealed region. Their own join beat is delivered to them before
// presence-pierce writes their region fact — and to nobody else, because a
// non-knower cannot hear someone appear inside a room that does not exist
// for them.
//
// (The walk form of the branch is unreachable through this seam's honest
// witness: standing at an open concealed door's own edge IS perceiving it,
// so a real walker is always revealed by the sweep before the crossing
// step. The engine's scripted-witness scenes hold that form.)
func (s *ConcealSuite) TestAJoinIntoHiddenSpaceStaysHidden() {
	ctx := context.Background()
	s.startWith(concealedWorld(s.T(), encounter.DoorIsClosed()), armedFighter("david"))

	s.stream.published = nil
	out, err := s.mgr.Join(ctx, &session.JoinInput{
		Session: "sess", Member: "david", Position: hexCell(9, 3)})
	s.Require().NoError(err)

	// david's own stream: his join, then the room presence pierced —
	// numbered densely from 1, because nothing was ever delivered to him
	// before.
	david := eventsFor(s.stream.published, "david")
	s.Require().Len(david, 2)
	s.Equal(session.EventJoined, david[0].Kind)
	s.Equal(uint64(1), david[0].Seq, "the joiner's stream starts at 1 — their own numbering, not the record's")
	s.Equal(session.EventRegionRevealed, david[1].Kind)
	s.Equal(uint64(2), david[1].Seq)
	s.Equal(uint64(1), out.Seq, "the verb's own Seq speaks the actor's numbering too")

	// The hall members hear that DAVID JOINED — membership is a table-level
	// fact and every non-detection beat stays full-data until v1.0 (the
	// standing ruling) — but the beat carries no position, and nothing
	// tells them WHERE he appeared: no reveal, no discovery, no hole.
	for _, name := range []string{"alice", "bob"} {
		heard := eventsFor(s.stream.published, name)
		s.Require().Len(heard, 1, "%s hears the membership beat and nothing else", name)
		s.Equal(session.EventJoined, heard[0].Kind)
		body, ok := heard[0].Body.(session.JoinedBody)
		s.Require().True(ok)
		s.Equal(session.JoinedBody{Member: "david"}, body,
			"who joined, never where — the joined body carries no cell")
	}

	// Presence pierces the ROOM, not its door: david's map holds the whole
	// authored floor — the hall was never a secret — plus the vault he
	// stands in, and still no veil doorway (occupant knows the room, not
	// its door; the composition's own pin, visible through the seam).
	occupant, err := s.mgr.Atlas(ctx, &session.AtlasInput{Session: "sess", Member: "david"})
	s.Require().NoError(err)
	s.Len(occupant.Cells, 72)
	s.Empty(occupant.Doorways, "the veil stays unfound even for the room's occupant")

	stranger, err := s.mgr.Atlas(ctx, &session.AtlasInput{Session: "sess", Member: "bob"})
	s.Require().NoError(err)
	s.Len(stranger.Cells, 36, "the hall members' maps are unchanged")
}

// TestPerRecipientNumberingSurvivesLoadAndTrim is the numbering design's
// whole claim in one run: an audience-of-one beat desynchronises the
// members' counters, a fresh Manager over the same repositories continues
// every stream without a restart or a skip, the retention window trims the
// early story, and every stream stays dense throughout — with Story
// answering catch-up byte-equal to what was delivered live, and refusing a
// resume point the window no longer holds.
func (s *ConcealSuite) TestPerRecipientNumberingSurvivesLoadAndTrim() {
	ctx := context.Background()
	s.startWith(concealedWorld(s.T(), encounter.DoorIsClosed()), sharpEyed("alice"), dullEyed("bob"))

	// The desynchroniser: alice's search delivers to alice alone, putting
	// her counter one ahead of bob's for the rest of the session.
	_, err := s.mgr.Search(ctx, &session.SearchInput{Session: "sess", Member: "alice", Region: "hall"})
	s.Require().NoError(err)

	shuttle := func(mgr *session.Manager, times int) {
		for i := 0; i < times; i++ {
			to := cell(3, 1)
			if i%2 == 1 {
				to = cell(2, 1)
			}
			_, err := mgr.Move(ctx, &session.MoveInput{
				Session: "sess", Member: "bob", Path: []spatial.Position{to}})
			s.Require().NoError(err)
		}
	}
	shuttle(s.mgr, 6)

	// A different process picks the session up: same repositories, fresh
	// Manager, fresh stream. Numbering must continue, not restart.
	live := &fakeStream{}
	mgr2, err := session.NewManager(&session.Config{
		Dice: testDice{}, TurnDriver: session.Pass{},
		Sessions: s.sessions, Encounters: s.encounters,
		Characters: s.characters, Events: live,
	})
	s.Require().NoError(err)

	firstBatch := eventsFor(s.stream.published, "bob")
	s.Require().NotEmpty(firstBatch)
	handoff := firstBatch[len(firstBatch)-1].Seq

	shuttle(mgr2, 40) // well past DefaultRetention 32: the early story trims away

	second := eventsFor(live.published, "bob")
	s.Require().NotEmpty(second)
	s.Equal(handoff+1, second[0].Seq,
		"the new process continues bob's stream exactly where the old one left off")
	s.assertDense(append(append([]session.Event{}, firstBatch...), second...), "bob")

	alice := append(eventsFor(s.stream.published, "alice"), eventsFor(live.published, "alice")...)
	s.assertDense(alice, "alice")
	s.Equal(alice[0].Seq+uint64(len(alice))-1, alice[len(alice)-1].Seq)

	// Catch-up equals live delivery for whatever the window still holds —
	// same numbers, same payloads, one projection (rpg-api-protos#239's law
	// carried into the new numbering).
	tail := second[len(second)-8:]
	replay, err := mgr2.Story(ctx, &session.StoryInput{
		Session: "sess", Member: "bob", FromSeq: tail[0].Seq})
	s.Require().NoError(err)
	s.Require().Len(replay, len(tail))
	for i := range tail {
		s.Equal(tail[i], replay[i], "live and catch-up must be byte-equal for the same seq")
	}

	// And the resume point that aged out refuses by name, in the member's
	// own numbering.
	_, err = mgr2.Story(ctx, &session.StoryInput{Session: "sess", Member: "bob", FromSeq: 1})
	s.Require().ErrorIs(err, session.ErrStoryTrimmed)
}

// TestOneVerbBiggerThanTheRetentionWindowCommitsWhole is the review's C1
// probe (PR #1377 review; ruled: RETENTION IS STORAGE-ONLY and may never
// affect delivery or numbering), buildable only on encounter v0.43.0's
// storage-boundary retention (#1381) and pure WorldView (#1385). One Move
// whose path is DefaultRetention+8 cells appends more beats than the window
// holds: under per-append trimming this destroyed its own early beats before
// commit could number them — a permanent, opaque refusal of an ordinary
// walk. Now the whole delta is numbered and DELIVERED before the save-point
// ToData trims what storage keeps; nothing a member was owed is lost to a
// policy about disk.
//
// This is also the single-verb variant of
// TestPerRecipientNumberingSurvivesLoadAndTrim: the trim happens inside ONE
// verb here (that scene's forty beats arrive one verb at a time), and the
// reload at the end proves the cursors carried the un-restartable count
// across it.
func (s *ConcealSuite) TestOneVerbBiggerThanTheRetentionWindowCommitsWhole() {
	ctx := context.Background()
	s.startWith(plainHallWorld(s.T()))

	// A short walk first: cursors exist and are non-zero, the reviewer's
	// own reproduction precondition (a fresh session's zero cursors would
	// dodge the refusal by seeding).
	_, err := s.mgr.Move(ctx, &session.MoveInput{
		Session: "sess", Member: "bob", Path: []spatial.Position{cell(3, 1)}})
	s.Require().NoError(err)
	warmup := map[string][]session.Event{}
	for _, name := range []string{"alice", "bob", "carol"} {
		warmup[name] = eventsFor(s.stream.published, name)
	}

	// THE PROBE: one verb, DefaultRetention+8 beats. It must commit, and
	// every member must be delivered every beat.
	steps := encounter.DefaultRetention + 8
	path := make([]spatial.Position, 0, steps)
	for i := 0; i < steps; i++ {
		to := cell(2, 1)
		if i%2 == 1 {
			to = cell(3, 1)
		}
		path = append(path, to)
	}
	s.stream.published = nil
	out, err := s.mgr.Move(ctx, &session.MoveInput{Session: "sess", Member: "bob", Path: path})
	s.Require().NoError(err, "a walk longer than the retention window is an ordinary player action")
	s.Require().Len(out.Steps, steps, "the whole walk happened")

	for _, name := range []string{"alice", "bob", "carol"} {
		events := eventsFor(s.stream.published, name)
		s.Require().Len(events, steps,
			"%s is delivered every beat of the big verb — retention govern storage, not delivery", name)
		whole := append(append([]session.Event{}, warmup[name]...), events...)
		s.assertDense(whole, name)
	}

	// The mover's own output speaks their numbering for every step, densely.
	for i := 1; i < len(out.Steps); i++ {
		s.Equal(out.Steps[i-1].Seq+1, out.Steps[i].Seq)
	}

	// Storage kept only the window — retention did its one job, at the one
	// boundary. Computed, not echoed: the blob's floor is NextSeq minus the
	// window.
	world, err := s.encounters.GetEncounter(ctx, "world")
	s.Require().NoError(err)
	s.Require().Len(world.Log.Entries, encounter.DefaultRetention,
		"the persisted log holds exactly the window")
	s.Equal(world.Log.NextSeq-uint64(encounter.DefaultRetention), world.Log.Entries[0].Seq)

	// Catch-up below the floor refuses honestly, in bob's own numbering; at
	// the oldest surviving number it answers, byte-equal to what was
	// delivered live.
	_, err = s.mgr.Story(ctx, &session.StoryInput{Session: "sess", Member: "bob", FromSeq: 1})
	s.Require().ErrorIs(err, session.ErrStoryTrimmed)

	bob := eventsFor(s.stream.published, "bob")
	tail := bob[len(bob)-4:]
	replay, err := s.mgr.Story(ctx, &session.StoryInput{
		Session: "sess", Member: "bob", FromSeq: tail[0].Seq})
	s.Require().NoError(err)
	s.Require().Len(replay, len(tail))
	for i := range tail {
		s.Equal(tail[i], replay[i])
	}

	// And the count survives a load that happened entirely after the
	// single-verb trim: a fresh Manager continues bob's stream exactly.
	live := &fakeStream{}
	mgr2, err := session.NewManager(&session.Config{
		Dice: testDice{}, TurnDriver: session.Pass{},
		Sessions: s.sessions, Encounters: s.encounters,
		Characters: s.characters, Events: live,
	})
	s.Require().NoError(err)
	_, err = mgr2.Move(ctx, &session.MoveInput{
		Session: "sess", Member: "bob", Path: []spatial.Position{cell(2, 1)}})
	s.Require().NoError(err)
	next := eventsFor(live.published, "bob")
	s.Require().Len(next, 1)
	s.Equal(bob[len(bob)-1].Seq+1, next[0].Seq,
		"the numbering neither restarted nor skipped across the in-verb trim and the reload")
}

// TestPlainDungeonNumberingIsIdentityForAFoundingMember shows exactly which
// half of the numbering change touches a world with no concealment: none,
// for a member whose story is the whole story — their numbers ARE the
// record's — while a late joiner's stream is dense from 1 by construction
// rather than starting mid-count.
func (s *ConcealSuite) TestPlainDungeonNumberingIsIdentityForAFoundingMember() {
	ctx := context.Background()
	s.startWith(plainHallWorld(s.T()), armedFighter("david"))

	for i, to := range []spatial.Position{cell(3, 1), cell(2, 1), cell(3, 1)} {
		_ = i
		_, err := s.mgr.Move(ctx, &session.MoveInput{
			Session: "sess", Member: "bob", Path: []spatial.Position{to}})
		s.Require().NoError(err)
	}

	// The record's own global numbering, read from the persisted blob.
	world, err := s.encounters.GetEncounter(ctx, "world")
	s.Require().NoError(err)

	bob := eventsFor(s.stream.published, "bob")
	s.Require().NotEmpty(bob)
	wantSeqs := make(map[uint64]bool, len(world.Log.Entries))
	for _, entry := range world.Log.Entries {
		wantSeqs[entry.Seq] = true
	}
	for _, e := range bob {
		s.True(wantSeqs[e.Seq],
			"a founding member of an all-shared story keeps the record's own numbers — identity, provably")
	}

	// A late joiner starts their own count at 1, however far the record has
	// moved on.
	s.stream.published = nil
	out, err := s.mgr.Join(ctx, &session.JoinInput{
		Session: "sess", Member: "david", Position: hexCell(4, 4)})
	s.Require().NoError(err)
	s.Equal(uint64(1), out.Seq, "the joiner's first delivered beat is their number 1")
	s.Greater(world.Log.NextSeq, uint64(2), "while the record is well past it")
}
