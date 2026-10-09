// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/damage"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
)

// LaunchSuite covers slice 4's launch done-when: one load-act-save that
// refuses before anything is written, stands the whole board, seats and
// rests the party, forms the fight last, and returns one report.
type LaunchSuite struct {
	suite.Suite

	sessions   *fakeSessions
	encounters *fakeEncounters
	characters *fakeCharacters
	seats      *fakeSeats
	stream     *fakeStream
	mgr        *session.Manager
}

func TestLaunchSuite(t *testing.T) { suite.Run(t, new(LaunchSuite)) }

func (s *LaunchSuite) SetupTest() {
	tired := func(id string) *character.Data {
		data := withHitDice(armedFighter(id), 3)
		data.Resources[resources.HitDice] = character.RecoverableResourceData{Current: 0, Maximum: 3, ResetType: coreResources.ResetLongRest}
		data.HitPoints = 5
		return data
	}
	s.sessions, s.encounters = newFakeSessions(), newFakeEncounters()
	s.characters = newFakeCharacters(tired("alice"), tired("bob"), tired("carol"))
	s.seats = newFakeSeats()
	s.stream = &fakeStream{}
	mgr, err := session.NewManager(&session.Config{Seats: s.seats, PresentationIDs: testPresentationIDs{},
		Dice: testDice{}, TurnDriver: session.Pass{}, Sessions: s.sessions, Encounters: s.encounters,
		Characters: s.characters, Events: s.stream,
	})
	s.Require().NoError(err)
	s.mgr = mgr
}

func (s *LaunchSuite) camp() *dungeonspec.Compiled {
	compiled := compileCamp(s.T(), campSource(s.T()))
	return &compiled
}

func (s *LaunchSuite) launch(dungeon *dungeonspec.Compiled, party ...string) (*session.LaunchOutput, error) {
	return s.mgr.Launch(context.Background(), &session.LaunchInput{
		Session: "run", DungeonKey: "reference-raider-camp", Dungeon: dungeon, Party: party,
	})
}

func (s *LaunchSuite) assertNothingWritten() {
	s.T().Helper()
	s.Zero(s.characters.saves, "no character is written")
	s.Zero(s.encounters.saves, "no world is written")
	s.Zero(s.seats.saves, "no seat is written")
	_, err := s.sessions.GetSession(context.Background(), "run")
	s.ErrorIs(err, session.ErrNotFound, "no session is written")
	s.Empty(s.stream.published)
}

func (s *LaunchSuite) TestALaunchSeatsAndRestsEveryPartyMemberAndReportsOnce() {
	out, err := s.launch(s.camp(), "alice", "bob")
	s.Require().NoError(err)

	for _, id := range []string{"alice", "bob"} {
		stored, getErr := s.characters.GetCharacter(context.Background(), id)
		s.Require().NoError(getErr)
		s.Equal(stored.MaxHitPoints, stored.HitPoints, "%s is rested", id)
		s.Equal("run", s.seats.seatOf(id), "%s is seated", id)
		s.Contains(out.Saved.Written, "character:"+id)
		s.Contains(out.Saved.Written, "seat:"+id)
	}
	s.Contains(out.Saved.Written, "encounter:run")
	s.Contains(out.Saved.Written, "session:run")
	s.Less(positionOf(out.Saved.Written, "seat:bob"), positionOf(out.Saved.Written, "encounter:run"),
		"the party is rested and seated before the run that holds them")

	data, err := s.sessions.GetSession(context.Background(), "run")
	s.Require().NoError(err)
	s.Equal("reference-raider-camp", data.Dungeon)

	ids := map[string]bool{}
	for _, member := range out.Members {
		ids[member.ID] = true
	}
	for _, want := range []string{"alice", "bob", "chief", "scout"} {
		s.True(ids[want], "%s is on the board", want)
	}
	s.False(ids["reinforcement-1"], "a reserved placement is not on the board")

	status, err := s.mgr.Status(context.Background(), &session.StatusInput{Session: "run"})
	s.Require().NoError(err)
	s.NotNil(status)
}

func (s *LaunchSuite) TestALaunchWhoseThirdMonsterCannotBeResolvedWritesNothing() {
	dungeon := s.camp()
	s.Require().GreaterOrEqual(len(dungeon.Monsters), 3)
	dungeon.Monsters[2].Ref = "dnd5e:monsters:no-such-monster"

	_, err := s.launch(dungeon, "alice", "bob")
	s.Require().ErrorIs(err, session.ErrUnknownContent)
	s.assertNothingWritten()
}

func (s *LaunchSuite) TestAPartyBiggerThanTheSeatsWritesNothing() {
	dungeon := s.camp()
	dungeon.PartyStart = dungeon.PartyStart[:1]

	_, err := s.launch(dungeon, "alice", "bob")
	s.Require().ErrorIs(err, session.ErrInvalidWorld)
	s.assertNothingWritten()
}

func (s *LaunchSuite) TestAnIDClaimedTwiceWritesNothing() {
	dungeon := s.camp()
	s.characters.byID["scout"] = armedFighter("scout")

	_, err := s.launch(dungeon, "alice", "scout")
	s.Require().ErrorIs(err, session.ErrDuplicateMember)
	s.assertNothingWritten()
}

func (s *LaunchSuite) TestAnUnresolvableSheetWritesNothing() {
	_, err := s.launch(s.camp(), "alice", "nobody")
	s.Require().ErrorIs(err, session.ErrNoCharacter)
	s.assertNothingWritten()
}

func (s *LaunchSuite) TestAFactionTheDungeonDoesNotDeclareWritesNothing() {
	dungeon := s.camp()
	dungeon.Monsters[1].Faction = "strangers"

	_, err := s.launch(dungeon, "alice", "bob")
	s.Require().Error(err)
	s.assertNothingWritten()
}

func (s *LaunchSuite) TestARefusalOnTheThirdMemberWritesNothing() {
	dungeon := s.camp()
	s.Require().GreaterOrEqual(len(dungeon.Monsters), 3)
	dungeon.Monsters[2].Faction = "strangers"

	_, err := s.launch(dungeon, "alice", "bob")
	s.Require().ErrorIs(err, session.ErrNoFaction, "the board refuses the third member by name")
	s.assertNothingWritten()
}

func (s *LaunchSuite) TestACharacterAnotherRunHoldsWritesNothing() {
	s.Require().NoError(s.seats.SaveSeat(context.Background(), &session.SeatData{Character: "bob", Session: "elsewhere"}))
	saves := s.seats.saves

	_, err := s.launch(s.camp(), "alice", "bob")
	s.Require().ErrorIs(err, session.ErrSeatedElsewhere)
	s.Equal(saves, s.seats.saves)
	s.Zero(s.characters.saves)
	s.Zero(s.encounters.saves)
}

func (s *LaunchSuite) TestALaunchOverAnExistingSessionIsRefused() {
	_, err := s.launch(s.camp(), "alice")
	s.Require().NoError(err)

	_, err = s.mgr.Launch(context.Background(), &session.LaunchInput{
		Session: "run", Dungeon: s.camp(), Party: []string{"carol"},
	})
	s.Require().ErrorIs(err, session.ErrSessionExists)
	s.Empty(s.seats.seatOf("carol"))
}

// truceSource is two authored factions hostile to EACH OTHER, standing in
// sight of one another, and a party seat far from both behind a wall.
const truceSource = `
version: 2
key: launch-two-sides
name: Two Sides
orientation: pointy
void: opaque
regions:
  - id: hall
    name: The Hall
    archetype: crypt
    lighting: { intensity: 0.8 }
    cells:
      - [[0,0],[1,0],[2,0],[3,0],[4,0],[5,0]]
      - [[0,1],[1,1],[2,1],[3,1],[4,1],[5,1]]
      - [[0,2],[1,2],[2,2],[3,2],[4,2],[5,2]]
start: { at: [0,0], facing: e }
factions:
  - { id: wolves }
  - { id: raiders }
dispositions:
  - { between: [wolves, raiders], stance: hostile }
place:
  - { id: wolf-a,  ref: "dnd5e:monsters:zombie",   at: [4,1], faction: wolves }
  - { id: raider-a, ref: "dnd5e:monsters:skeleton", at: [5,2], faction: raiders }
  - { id: raider-b, ref: "dnd5e:monsters:skeleton", at: [5,0], faction: raiders }
`

// TestTwoHostileFactionsFormOneFightAfterEveryMemberIsPlaced is the board
// law: the whole board stands before any fight forms, so the fight that forms
// holds everyone it should and starts after the last member arrives.
func (s *LaunchSuite) TestTwoHostileFactionsFormOneFightAfterEveryMemberIsPlaced() {
	compiled, err := dungeonspec.Load([]byte(truceSource))
	s.Require().NoError(err)

	_, err = s.mgr.Launch(context.Background(), &session.LaunchInput{
		Session: "run", Dungeon: &compiled, Party: []string{"alice"},
	})
	s.Require().NoError(err)

	// Every member's own stream: exactly one fight started, and it started
	// after every arrival that member was told about — the monsters saw each
	// other while the board stood, and nothing formed until the party came.
	type account struct {
		lastJoined, fight uint64
		fights            int
	}
	streams := map[string]*account{}
	for _, event := range s.stream.published {
		acct := streams[event.Recipient]
		if acct == nil {
			acct = &account{}
			streams[event.Recipient] = acct
		}
		switch event.Kind {
		case session.EventJoined:
			acct.lastJoined = event.Seq
		case session.EventFightStarted:
			acct.fights++
			acct.fight = event.Seq
		}
	}
	for _, member := range []string{"alice", "wolf-a", "raider-a", "raider-b"} {
		acct := streams[member]
		s.Require().NotNil(acct, "%s was told the story", member)
		s.Equal(1, acct.fights, "%s: one contact, one fight", member)
		s.Greater(acct.fight, acct.lastJoined, "%s: the fight forms after every member is placed", member)
	}
}

// twoCampsSource is a hall with two camps, each hostile to the party and to
// nobody else, standing apart.
const twoCampsSource = `
version: 2
key: launch-two-camps
name: Two Camps
orientation: pointy
void: opaque
regions:
  - id: hall
    name: The Hall
    archetype: crypt
    lighting: { intensity: 0.8 }
    cells:
      - [[0,0],[1,0],[2,0],[3,0],[4,0],[5,0]]
      - [[0,1],[1,1],[2,1],[3,1],[4,1],[5,1]]
      - [[0,2],[1,2],[2,2],[3,2],[4,2],[5,2]]
start: { at: [0,0], facing: e }
factions:
  - { id: wolves }
  - { id: raiders }
dispositions:
  - { between: [wolves, party], stance: hostile }
  - { between: [raiders, party], stance: hostile }
place:
  - { id: wolf-a,   ref: "dnd5e:monsters:zombie",   at: [5,0], faction: wolves }
  - { id: raider-a, ref: "dnd5e:monsters:skeleton", at: [5,2], faction: raiders }
`

// TestAPartyOfTwoArrivesIntoOneFightHoldingBoth is the one-look law with a
// party of two: the fight forms after both party members stand, and holds
// both of them and both camps — never formed on the first arrival with the
// second still off the board.
func (s *LaunchSuite) TestAPartyOfTwoArrivesIntoOneFightHoldingBoth() {
	compiled, err := dungeonspec.Load([]byte(twoCampsSource))
	s.Require().NoError(err)

	out, err := s.mgr.Launch(context.Background(), &session.LaunchInput{
		Session: "run", Dungeon: &compiled, Party: []string{"alice", "bob"},
	})
	s.Require().NoError(err)

	s.Require().Len(out.Formed, 1)
	s.ElementsMatch([]string{"alice", "bob", "wolf-a", "raider-a"}, out.Formed[0].Order)
	for _, member := range []string{"alice", "bob"} {
		var lastJoined, fight uint64
		fights := 0
		for _, event := range s.stream.published {
			if event.Recipient != member {
				continue
			}
			switch event.Kind {
			case session.EventJoined:
				lastJoined = event.Seq
			case session.EventFightStarted:
				fights++
				fight = event.Seq
			}
		}
		s.Equal(1, fights, "%s: one fight", member)
		s.Greater(fight, lastJoined, "%s: the fight forms after both party members stand", member)
	}
}

// TestAPartyLeftDefeatedArrivesRestedIntoTheFight is the rest-before-the-
// board law: a party that ended its last run at zero hit points is rested and
// saved before the board's one look, so the look reads them standing and the
// fight forms around them — rather than the run opening on a defeated party.
func (s *LaunchSuite) TestAPartyLeftDefeatedArrivesRestedIntoTheFight() {
	for _, id := range []string{"alice", "bob"} {
		s.characters.byID[id].HitPoints = 0
	}
	compiled, err := dungeonspec.Load([]byte(twoCampsSource))
	s.Require().NoError(err)

	out, err := s.mgr.Launch(context.Background(), &session.LaunchInput{
		Session: "run", Dungeon: &compiled, Party: []string{"alice", "bob"},
	})
	s.Require().NoError(err)

	s.Require().Len(out.Formed, 1, "the rested party is in contact, so the fight forms")
	s.Contains(out.Formed[0].Order, "alice")
	s.Contains(out.Formed[0].Order, "bob")
	status, err := s.mgr.Status(context.Background(), &session.StatusInput{Session: "run"})
	s.Require().NoError(err)
	s.True(status.Open, "the run did not open on a defeated party")
}

// TestALaunchCannotPlaceAMonsterHoldingARecordThatIsNotThere is the
// fail-closed half of an authored `holds:` at the seam: an undeclared record
// is refused by name rather than arriving ignorant, crosses as this package's
// own sentinel, and leaves nothing behind.
//
// This is the failure a host hits by forwarding the AUTHOR's raw record id
// instead of the compiled `<key>/<id>` dungeonspec mints, which is the one
// mistake worth making loud.
func (s *LaunchSuite) TestALaunchCannotPlaceAMonsterHoldingARecordThatIsNotThere() {
	dungeon := s.camp()
	dungeon.Monsters[0].Holds = []string{"no-such-record"}

	_, err := s.launch(dungeon, "alice", "bob")
	s.Require().ErrorIs(err, session.ErrNoIntel,
		"a record this dungeon does not declare — not ErrNoConnection, which is about geometry")
	s.assertNothingWritten()
}

// launchHoldingLatecomer launches the heirloom hall with nobody authored
// knowing the way in except a skeleton placed beside alice carrying holds,
// then puts it on the floor by zeroing its stored sheet — the same way the
// holdings suite makes a body, because it is the same mechanism: the
// composition is TOLD who is down.
//
// This is the live shape (rpg-project#368 P1, carried to the intel record by
// rpg-project#372): a host brings every monster onto the board through the
// placements of the dungeon it launches, so a placement's `holds:` is the
// only way an authored record reaches a live monster.
func launchHoldingLatecomer(t *testing.T, holds []string) (*session.Manager, *fakeStream) {
	t.Helper()
	sessions, stream := newFakeSessions(), &fakeStream{}
	mgr, err := session.NewManager(&session.Config{Seats: newFakeSeats(), PresentationIDs: testPresentationIDs{},
		Dice: testDice{}, TurnDriver: session.Pass{},
		Sessions: sessions, Encounters: newFakeEncounters(),
		Characters: newFakeCharacters(sharpEyed("alice"), dullEyed("bob")), Events: stream,
	})
	require.NoError(t, err)

	sc := heirloomWorld(false) // nobody ELSE was authored knowing anything
	latecomer := monsterAt("latecomer", refs.Monsters.Skeleton().String(), 0, 1)
	latecomer.Holds = holds
	sc.Monsters = append(sc.Monsters, latecomer)
	launchScene(t, mgr, sc)

	stored := sessions.byID[testSession]
	downed := false
	for i := range stored.NPCs {
		if stored.NPCs[i].ID == "latecomer" {
			stored.NPCs[i].HitPoints = 0
			downed = true
		}
	}
	require.True(t, downed, "the launch recorded no sheet to put on the floor")
	stream.published = nil
	return mgr, stream
}

// TestALaunchedMonsterCarriesTheRecordsItWasPlacedWith is the scene: a
// placement with Holds, loot, and the looter alone learns the way in.
func TestALaunchedMonsterCarriesTheRecordsItWasPlacedWith(t *testing.T) {
	ctx := context.Background()
	mgr, stream := launchHoldingLatecomer(t, []string{veilMap})

	_, err := mgr.Loot(ctx, &session.LootInput{
		Session: testSession, Member: "alice", Target: "latecomer", Range: 2})
	require.NoError(t, err)

	t.Run("the looter alone is told about the secret", func(t *testing.T) {
		require.Equal(t, []session.EventKind{session.EventLooted, session.EventConcealmentRevealed, session.EventSighted},
			recipientKinds(stream.published, "alice"))
		var revealed []session.Event
		for _, e := range eventsFor(stream.published, "alice") {
			if e.Kind == session.EventConcealmentRevealed {
				revealed = append(revealed, e)
			}
		}
		require.Len(t, revealed, 1)
		body, ok := revealed[0].Body.(session.ConcealmentRevealedBody)
		require.True(t, ok)
		require.Equal(t, vaultSecret, body.Concealment, "the way in came off the body that was placed carrying it")
	})

	t.Run("the bystander hears the beat and learns nothing", func(t *testing.T) {
		require.Equal(t, []session.EventKind{session.EventLooted}, recipientKinds(stream.published, "bob"))
		blind, err := mgr.Doors(ctx, &session.DoorsInput{Session: testSession, Member: "bob"})
		require.NoError(t, err)
		require.Empty(t, blind.Doors)
	})
}

// TestALaunchedMonsterHoldingNothingRevealsNothing is the negative that makes
// the scene above a claim about Holds rather than about placing.
//
// Same verb, same body, same loot — the ONE difference is the field — and the
// bystander's bytes are identical either way, which is design P3 asked of the
// live path.
func TestALaunchedMonsterHoldingNothingRevealsNothing(t *testing.T) {
	ctx := context.Background()

	loot := func(holds []string) ([]session.EventKind, string) {
		mgr, stream := launchHoldingLatecomer(t, holds)
		_, err := mgr.Loot(ctx, &session.LootInput{
			Session: testSession, Member: "alice", Target: "latecomer", Range: 2})
		require.NoError(t, err)
		story, err := mgr.Story(ctx, &session.StoryInput{Session: testSession, Member: "bob"})
		require.NoError(t, err)
		raw, err := json.Marshal(story)
		require.NoError(t, err)
		return recipientKinds(stream.published, "bob"), string(raw)
	}

	richKinds, richStory := loot([]string{veilMap})
	poorKinds, poorStory := loot(nil)

	require.Equal(t, []session.EventKind{session.EventLooted}, poorKinds)
	require.Equal(t, poorKinds, richKinds)
	require.Equal(t, poorStory, richStory,
		"a monster placed knowing nothing and one placed knowing the run's "+
			"only secret are indistinguishable to everybody but the looter")
}

// LaunchActionsSuite is the driving case of rpg-project#448 at this seam,
// folded here from the retired Spawn verb's own file: a dungeon places goblin
// archers — one with a scimitar as backup, one with nothing but the bow —
// without touching Go, and the launch arms each from its placement.
//
// Everything below is chosen so it cannot pass on a value the caller supplied.
// The placement carries weapon REFS. What is asserted is +4 to hit for 1d6+2
// piercing at 80/320 feet, which exists only because the shortbow was
// assembled against a goblin's DEX 14 and its CR-based +2.
type LaunchActionsSuite struct {
	suite.Suite

	sessions *fakeSessions
	mgr      *session.Manager
}

func TestLaunchActionsSuite(t *testing.T) { suite.Run(t, new(LaunchActionsSuite)) }

func (s *LaunchActionsSuite) SetupTest() {
	s.sessions = newFakeSessions()
	mgr, err := session.NewManager(&session.Config{Seats: newFakeSeats(),
		PresentationIDs: testPresentationIDs{}, Dice: testDice{}, TurnDriver: session.Pass{},
		Sessions: s.sessions, Encounters: newFakeEncounters(), Characters: testCharacters(),
		Events: session.DiscardEvents{},
	})
	s.Require().NoError(err)
	s.mgr = mgr
}

func (s *LaunchActionsSuite) SetupSubTest() { s.SetupTest() }

// armedGoblinAt is a goblin placement in the vault carrying the author's
// action list.
func armedGoblinAt(id string, col int, actions []string) dungeonspec.MonsterPlacement {
	placement := monsterAt(id, refs.Monsters.Goblin().String(), col, 2)
	placement.Actions = actions
	return placement
}

// launchArmed launches hexWorld with the given goblins on its board.
func (s *LaunchActionsSuite) launchArmed(goblins ...dungeonspec.MonsterPlacement) error {
	sc := hexWorld()
	sc.Monsters = goblins
	_, err := s.mgr.Launch(context.Background(), sceneInput(sc))
	return err
}

// storedActions returns the actions on a monster's STORED sheet — the sheet a
// rehydrated run reads, not a projection built for this call.
func (s *LaunchActionsSuite) storedActions(id string) []combatActions.Definition {
	s.T().Helper()
	for _, npc := range s.sessions.byID[testSession].NPCs {
		if npc.ID == id {
			return npc.Actions
		}
	}
	s.Require().Failf("no sheet", "the launch recorded no sheet for %q", id)
	return nil
}

// TestAnArcherWithNothingButTheBow is the second of the two goblins: the
// author armed it with one weapon, and one weapon is all it has.
func (s *LaunchActionsSuite) TestAnArcherWithNothingButTheBow() {
	s.Require().NoError(s.launchArmed(armedGoblinAt("coward", 8, []string{refs.Weapons.Shortbow().String()})))
	actions := s.storedActions("coward")

	s.Require().Len(actions, 1, "the author said bow, so the scimitar its stat block gives it is gone")
	bow := actions[0]
	s.Equal(refs.Weapons.Shortbow().String(), bow.Ref.String(),
		"the action's ref is the weapon's, as it is for a character")
	s.Equal("Shortbow", bow.Name)

	s.Require().NotNil(bow.Attack)
	s.Equal(4, bow.Attack.AttackBonus, "DEX 14 and the goblin's own +2, which the caller never passed")
	s.Require().NotNil(bow.Attack.Ability)
	s.Equal(2, bow.Attack.Ability.Modifier, "the +2 on 1d6+2")
	s.Require().Len(bow.Attack.Damage, 1)
	s.Equal("1d6", bow.Attack.Damage[0].Dice)
	s.Equal(damage.Piercing, bow.Attack.Damage[0].Type)
	s.Equal(&combatActions.RangedDelivery{NormalFeet: 80, LongFeet: 320}, bow.Attack.Delivery.Ranged)
	s.Nil(bow.Attack.Delivery.Melee, "there is no blade on this one")
}

// TestAnArcherWithABladeForWhenYouGetClose is the first goblin, and the
// ORDER is the whole of what makes it different.
func (s *LaunchActionsSuite) TestAnArcherWithABladeForWhenYouGetClose() {
	s.Require().NoError(s.launchArmed(armedGoblinAt("backup", 9, []string{
		refs.Weapons.Scimitar().String(), refs.Weapons.Shortbow().String(),
	})))
	actions := s.storedActions("backup")

	s.Require().Len(actions, 2)
	s.Equal(refs.Weapons.Scimitar().String(), actions[0].Ref.String(),
		"the author listed the blade first, and the driver takes the first action in reach")
	s.Equal(refs.Weapons.Shortbow().String(), actions[1].Ref.String())
	s.Equal(&combatActions.MeleeDelivery{ReachFeet: 5}, actions[0].Attack.Delivery.Melee)
}

// TestTheAuthorsOrderIsNotNormalised is the mutant that made the two tests
// above worth writing.
//
// Drop the ordering from the launch — sort the list, deduplicate it, or
// forward it in any order but the one written — and the two goblins stop being
// different creatures: the one meant to swing when cornered shoots point blank
// instead. So the same two weapons are placed the other way round here, and
// the bow has to be first.
func (s *LaunchActionsSuite) TestTheAuthorsOrderIsNotNormalised() {
	s.Require().NoError(s.launchArmed(
		armedGoblinAt("blade-first", 8, []string{
			refs.Weapons.Scimitar().String(), refs.Weapons.Shortbow().String(),
		}),
		armedGoblinAt("bow-first", 9, []string{
			refs.Weapons.Shortbow().String(), refs.Weapons.Scimitar().String(),
		}),
	))

	blade := s.storedActions("blade-first")
	s.Equal(refs.Weapons.Scimitar().String(), blade[0].Ref.String())

	bow := s.storedActions("bow-first")
	s.Equal(refs.Weapons.Shortbow().String(), bow[0].Ref.String(),
		"the same two weapons the other way round stay the other way round")
	s.Equal(refs.Weapons.Scimitar().String(), bow[1].Ref.String())
}

// TestAnUnarmedPlacementKeepsTheStatBlocksOwnArms is the negative that makes
// the tests above claims about Actions rather than about placing.
func (s *LaunchActionsSuite) TestAnUnarmedPlacementKeepsTheStatBlocksOwnArms() {
	s.Require().NoError(s.launchArmed(armedGoblinAt("default", 8, nil)))
	actions := s.storedActions("default")

	s.Require().Len(actions, 2, "a goblin's own scimitar and shortbow")
	s.Equal(refs.Weapons.Scimitar().String(), actions[0].Ref.String())
	s.Equal(refs.Weapons.Shortbow().String(), actions[1].Ref.String())
}

// TestALaunchRefusesAWeaponNothingCanBuild is decision 5 at this seam: it
// fails here, reading the dungeon, not at a turn.
func (s *LaunchActionsSuite) TestALaunchRefusesAWeaponNothingCanBuild() {
	for _, tc := range []struct {
		name   string
		action string
		is     error
	}{
		{"a weapon the catalog does not have", "dnd5e:weapons:trebuchet", session.ErrUnknownContent},
		{"a ref that is not a weapon", "dnd5e:monster_actions:wolf-bite", session.ErrUnknownContent},
		// The row that makes the TYPE check load-bearing rather than
		// decorative. Deleting the module/type test leaves the two rows
		// above passing — nothing answers to "wolf-bite" in the weapons
		// catalog either — but `dnd5e:monster_actions:mace` has a weapon's
		// id in a namespace that is not the weapons catalog, and only the
		// type check refuses it. Found by running that mutant.
		{"an authored action whose id collides with a weapon's",
			"dnd5e:monster_actions:mace", session.ErrUnknownContent},
		{"another module's weapon", "homebrew:weapons:shortbow", session.ErrUnknownContent},
		{"a bare weapon id", "shortbow", session.ErrBadRef},
	} {
		s.Run(tc.name, func() {
			err := s.launchArmed(armedGoblinAt("doomed", 8, []string{tc.action}))
			s.Require().Error(err)
			s.ErrorIs(err, tc.is)
			s.ErrorContains(err, tc.action, "the refusal names the ref the author wrote")
			s.NotContains(s.sessions.byID, testSession, "a refused launch stores nothing")
		})
	}
}

// TestABadWeaponLateInTheListStillRefusesTheWholeLaunch: the monster is armed
// all at once, so it cannot arrive holding the half of the list that parsed.
func (s *LaunchActionsSuite) TestABadWeaponLateInTheListStillRefusesTheWholeLaunch() {
	err := s.launchArmed(armedGoblinAt("doomed", 8,
		[]string{refs.Weapons.Shortbow().String(), "dnd5e:weapons:trebuchet"}))
	s.Require().Error(err)
	s.ErrorIs(err, session.ErrUnknownContent)
	s.NotContains(s.sessions.byID, testSession)
}
