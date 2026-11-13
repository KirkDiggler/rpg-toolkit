// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// castle_sharing_test.go is the party's sharing dial at the session door
// (rpg-project#935): the castle says nothing, so it runs at shared view, and
// each player is sent what any party friend perceived, marked via them.

// shoutedTonight is the castle with the stranger's checked line shouted.
func shoutedTonight(raw []byte) ([]byte, error) {
	line := []byte("          - id: tonight\n            say: What’s all the bustle?\n")
	if bytes.Count(raw, line) != 1 {
		return nil, fmt.Errorf("precondition: the stranger's line, once")
	}
	return bytes.Replace(raw, line, append(append([]byte(nil), line...), []byte("            loudness: shout\n")...), 1), nil
}

// roadCastleTests are the tests that open [roadCastles].
var roadCastleTests = []string{
	"TestCastleAFriendsShoutReachesTheGreatHallViaHer", "TestCastleEachTheirOwnTheGreatHallHearsNothing",
	"TestCastleAFriendsSightingIsSharedViaHer", "TestCastleAFriendWhoLeftKeepsWhatHeShared",
	"TestCastleCatchUpHoldsAFriendWhoLeftWhileSheWasAway", "TestCastleCatchUpHoldsAFriendWhoLeftFirst",
	"TestCastleCatchUpHoldsWhatWasSharedWithTwoAway",
}

// roadCastles is the castle with the stranger's line shouted, launched once
// for alice and the cleric at each stop of the dial and kept as stores, so
// each test restores a launched run rather than paying the launch.
var roadCastles struct {
	once             sync.Once
	err              error
	view, own, three castleStores
}

// buildTheRoadCastles launches both. It takes no *testing.T because
// [TestMain] runs it; every precondition it checks is an error.
func buildTheRoadCastles() {
	dir, err := exec.Command("go", "list", "-m", "-f", "{{.Dir}}", encounterModule).Output()
	if err != nil {
		roadCastles.err = fmt.Errorf("find the encounter module: %w", err)
		return
	}
	raw, err := os.ReadFile(filepath.Join(strings.TrimSpace(string(dir)), castleFixture))
	if err != nil {
		roadCastles.err = err
		return
	}
	shouted, err := shoutedTonight(raw)
	if err != nil {
		roadCastles.err = err
		return
	}
	key := []byte("\nkey: the-mad-king\n")
	own := bytes.Replace(shouted, key, append(append([]byte(nil), key...), []byte("party: { sharing: own }\n")...), 1)
	if roadCastles.view, err = launchTheRoad(shouted); err != nil {
		roadCastles.err = err
		return
	}
	if roadCastles.three, err = launchTheRoad(shouted, armedFighter("bob")); err != nil {
		roadCastles.err = err
		return
	}
	roadCastles.own, roadCastles.err = launchTheRoad(own)
}

// launchTheRoad launches source for alice, the cleric and anyone more, and
// keeps its stores.
func launchTheRoad(source []byte, more ...*character.Data) (castleStores, error) {
	compiled, err := dungeonspec.Load(source)
	if err != nil {
		return castleStores{}, err
	}
	alice := armedFighter("alice")
	alice.AbilityScores[abilities.CHA] = 16
	c := &freeRoamCastle{
		live: &actCapture{}, dice: &castleDice{d20s: []int{18}}, characters: newFakeCharacters(append([]*character.Data{alice, healingCleric()}, more...)...),
		encounters: newFakeEncounters(), sessions: newFakeSessions(), seats: newFakeSeats(),
	}
	c.mgr, err = session.NewManager(&session.Config{Record: session.DiscardRecord{}, Acts: c.live, Seats: c.seats, CharacterConfig: newFakeCharacterConfig(),
		PresentationIDs: testPresentationIDs{}, Dice: c.dice, TurnDriver: session.Driver(),
		Sessions: c.sessions, Encounters: c.encounters, Characters: c.characters,
	})
	if err != nil {
		return castleStores{}, err
	}
	party := []string{"alice", "cleric"}
	for _, sheet := range more {
		party = append(party, sheet.ID)
	}
	if _, err := c.mgr.Launch(context.Background(), &session.LaunchInput{Session: "sess", Dungeon: &compiled, Party: party}); err != nil {
		return castleStores{}, fmt.Errorf("launch: %w", err)
	}
	return castleStores{sessions: c.sessions.byID, encounters: c.encounters.byID, characters: c.characters.byID, seats: c.seats.byID}, nil
}

// theRoadAndTheHall restores the launched road castle — at shared view, or
// at each their own — with the cleric beside the stranger on the road and
// alice in the great hall, behind closed doors, where no shout from the road
// reaches her.
func theRoadAndTheHall(t *testing.T, eachOwn bool) *freeRoamCastle {
	t.Helper()
	roadCastles.once.Do(buildTheRoadCastles)
	require.NoError(t, roadCastles.err, "the road castles")
	stores := roadCastles.view
	if eachOwn {
		stores = roadCastles.own
	}
	return restoreTheRoad(t, stores)
}

// restoreTheRoad restores stores with the cleric beside the stranger and
// alice in the great hall.
func restoreTheRoad(t *testing.T, stores castleStores) *freeRoamCastle {
	t.Helper()
	c := &freeRoamCastle{
		live: &actCapture{}, dice: &castleDice{d20s: []int{18, 18, 18, 18}},
		sessions: newFakeSessions(), encounters: newFakeEncounters(),
		characters: newFakeCharacters(), seats: newFakeSeats(),
	}
	require.NoError(t, restoreInto(c, stores))
	c.mgr = c.manager(t)
	stranger := c.where(t, "pedlar")
	c.placeInTheRecord(t, "cleric", spatial.Position{X: stranger.X + 1, Y: stranger.Y})
	c.placeInTheRecord(t, "alice", inTheGreatHall)
	return c
}

// lastActID is the id of the last act member holds.
func (c *freeRoamCastle) lastActID(t *testing.T, member string) string {
	t.Helper()
	out, err := c.mgr.Acts(context.Background(), &session.ActsInput{Session: "sess", Member: member})
	require.NoError(t, err)
	if len(out.Acts) == 0 {
		return ""
	}
	return out.Acts[len(out.Acts)-1].ID
}

// TestCastleAFriendsShoutReachesTheGreatHallViaHer is the operator's
// scenario: the cleric shouts her checked line at the stranger on the road;
// alice, in the great hall, perceives nothing of it — and at shared view is
// sent the words, who said them, to whom, and the roll, via the cleric. The
// stranger's answer reaches her as one pick, the cleric's, via the cleric.
// Catch-up after the act she held before is exactly what she was sent live.
func TestCastleAFriendsShoutReachesTheGreatHallViaHer(t *testing.T) {
	c := theRoadAndTheHall(t, false)
	held := c.lastActID(t, "alice")
	beforeSpeech := len(beatsNamed(storyBeats(t, c.mgr, "sess", "alice"), encounter.BeatSpeech))
	played := c.clericSays(t, "tonight")
	cleric := []session.Party{clericParty()}

	lines := where[session.SpeechAct](played.to("alice"), func(s session.SpeechAct) bool { return s.Option == "tonight" })
	require.Len(t, lines, 1, "one Speech act for the shout")
	s := lines[0].body
	require.NotNil(t, s.Words)
	require.Equal(t, "What’s all the bustle?", *s.Words, "the words")
	require.Equal(t, clericParty(), s.Speaker, "who said them")
	require.Equal(t, strangerParty(), s.SpokenTo, "to whom")
	require.NotNil(t, s.Roll, "and the roll")
	require.Equal(t, "persuasion", s.Skill)
	require.Nil(t, s.Hops, "a speaker named is no distance")
	require.Equal(t, &session.Perceived{Senses: []session.Sense{session.SenseSight, session.SenseHearing}, Via: cleric},
		lines[0].act.Perceived, "all of it through the cleric")

	picks := where[session.TableAnswer](played.to("alice"), func(a session.TableAnswer) bool { return a.Who == strangerParty() })
	require.Len(t, picks, 1, "one pick for his one answer")
	require.NotNil(t, picks[0].body.Roll, "the fullest: the cleric caught the line")
	require.Equal(t, cleric, picks[0].act.Perceived.Via)

	// The record told her nothing of it: the merge is what she is shown.
	require.Len(t, beatsNamed(storyBeats(t, c.mgr, "sess", "alice"), encounter.BeatSpeech), beforeSpeech,
		"her own story holds no version of the shout")

	caught, err := c.mgr.Acts(context.Background(), &session.ActsInput{Session: "sess", Member: "alice", AfterID: held})
	require.NoError(t, err)
	require.Equal(t, played.to("alice"), caught.Acts, "catch-up is what she was sent live")
}

// TestCastleEachTheirOwnTheGreatHallHearsNothing is the same shout at each
// their own: alice is sent nothing of it.
func TestCastleEachTheirOwnTheGreatHallHearsNothing(t *testing.T) {
	c := theRoadAndTheHall(t, true)
	played := c.clericSays(t, "tonight")
	require.Empty(t, where[session.SpeechAct](played.to("alice"), func(session.SpeechAct) bool { return true }))
	require.Empty(t, where[session.TableAnswer](played.to("alice"), func(a session.TableAnswer) bool { return a.Who == strangerParty() }))
	for _, a := range played.to("alice") {
		if a.Perceived != nil {
			require.Empty(t, a.Perceived.Via, "nothing comes through a friend")
		}
	}
}

// TestCastleAFriendsSightingIsSharedViaHer: alice walks up the road and
// sees the gate's guards; the cleric, in the great hall, sees none of them
// and is sent alice's sightings via alice — while her own sight is unchanged.
func TestCastleAFriendsSightingIsSharedViaHer(t *testing.T) {
	c := theRoadAndTheHall(t, false)
	walked := c.play(t, "alice steps into the great hall", func() error {
		_, err := walkOn(context.Background(), c.mgr, &session.MoveInput{
			Session: "sess", Member: "alice", Path: []spatial.Position{{X: inTheGreatHall.X + 1, Y: inTheGreatHall.Y}},
		})
		return err
	})

	alice := []session.Party{{Kind: session.PartyMember, ID: "alice"}}
	var gained []string
	for _, p := range where[session.Perception](walked.to("cleric"), func(p session.Perception) bool { return p.Sense == session.SenseSight }) {
		if len(p.act.Perceived.Via) == 0 {
			continue // the cleric's own sight, on the road
		}
		require.Equal(t, alice, p.act.Perceived.Via, "a sighting through a friend is alice's")
		require.Equal(t, alice[0], p.body.Observer, "and names her as the one who saw — the cleric only reads it")
		require.Equal(t, "cleric", p.act.Recipient)
		gained = append(gained, p.body.Sighted.Gained...)
	}
	require.NotEmpty(t, gained, "alice saw the great hall's people, and the cleric is told")

	var own []string
	for _, p := range where[session.Perception](walked.to("alice"), func(p session.Perception) bool { return p.Sense == session.SenseSight }) {
		if len(p.act.Perceived.Via) == 0 {
			own = append(own, p.body.Sighted.Gained...)
		}
	}
	require.ElementsMatch(t, own, gained, "what the cleric is told is what alice saw")
}

// TestCastleAFriendWhoLeftKeepsWhatHeShared: membership is a fact of the
// entry's time. The cleric shouts while alice is in the great hall, and
// alice is sent it via the cleric; then the cleric leaves the run. Alice's
// catch-up still holds what came through the cleric before the exit, exactly
// as it was sent live; alice's line afterwards comes through nobody; and the
// cleric, gone, is shown nothing after her exit.
func TestCastleAFriendWhoLeftKeepsWhatHeShared(t *testing.T) {
	c := theRoadAndTheHall(t, false)
	held := c.lastActID(t, "alice")
	shout := c.clericSays(t, "tonight")
	require.NotEmpty(t, where[session.SpeechAct](shout.to("alice"), func(s session.SpeechAct) bool { return s.Option == "tonight" }),
		"precondition: alice was sent the shout through the cleric")

	clericHeld := c.lastActID(t, "cleric")
	_, err := c.mgr.Exit(context.Background(), &session.ExitInput{Session: "sess", Member: "cleric"})
	require.NoError(t, err)

	caught, err := c.mgr.Acts(context.Background(), &session.ActsInput{Session: "sess", Member: "alice", AfterID: held})
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(caught.Acts), len(shout.to("alice")))
	require.Equal(t, shout.to("alice"), caught.Acts[:len(shout.to("alice"))],
		"after the cleric left, alice's catch-up still holds what he shared, as sent live")

	stranger := c.where(t, "pedlar")
	c.placeInTheRecord(t, "alice", spatial.Position{X: stranger.X - 1, Y: stranger.Y})
	played := c.play(t, "alice says tonight", func() error {
		_, err := speakAsOffered(context.Background(), c.mgr, &session.SpeakInput{
			Session: "sess", Member: "alice", Target: "pedlar", OptionID: "tonight",
		})
		return err
	})
	require.NotEmpty(t, where[session.SpeechAct](played.to("alice"), func(session.SpeechAct) bool { return true }),
		"precondition: alice spoke")
	for _, a := range played.to("alice") {
		if a.Perceived != nil {
			require.Empty(t, a.Perceived.Via, "nothing reaches alice through a player who left")
		}
	}

	after, err := c.mgr.Acts(context.Background(), &session.ActsInput{Session: "sess", Member: "cleric", AfterID: clericHeld})
	require.NoError(t, err)
	require.Empty(t, where[session.SpeechAct](after.Acts, func(session.SpeechAct) bool { return true }),
		"the cleric, gone, is shown nothing of alice's line")
	for _, a := range after.Acts {
		if a.Perceived != nil {
			require.Empty(t, a.Perceived.Via, "a player who left is shared nothing: %T", a.Body)
		}
	}
}

// TestCastleSharedKnowledgeIsPreviewedButNeverLaunched: a castle authored
// with the third stop is a valid dungeon — AtlasOf, which the host asks on
// every validate and save, projects it — and only Launch, which starts a run
// for players, refuses it: with the encounter's sentence, as a precondition
// (ErrSharingNotBuilt), never as a broken world. No run exists, so no act is
// ever folded at it.
func TestCastleSharedKnowledgeIsPreviewedButNeverLaunched(t *testing.T) {
	source := castleSource(t)
	key := []byte("\nkey: the-mad-king\n")
	knowledge := bytes.Replace(source, key, append(append([]byte(nil), key...), []byte("party: { sharing: knowledge }\n")...), 1)
	compiled, err := dungeonspec.Load(knowledge)
	require.NoError(t, err, "the compiler takes the word")
	c := &freeRoamCastle{live: &actCapture{}, characters: newFakeCharacters(armedFighter("alice")), encounters: newFakeEncounters(),
		dice: &castleDice{d20s: []int{18}}, sessions: newFakeSessions(), seats: newFakeSeats()}
	c.mgr = c.manager(t)

	atlas, err := c.mgr.AtlasOf(context.Background(), &session.AtlasOfInput{Dungeon: &compiled, DungeonKey: "the-mad-king"})
	require.NoError(t, err, "previewed and saved like any dungeon")
	require.NotEmpty(t, atlas.Cells)

	_, err = c.mgr.Launch(context.Background(), &session.LaunchInput{Session: "sess", Dungeon: &compiled, Party: []string{"alice"}})
	require.ErrorIs(t, err, session.ErrSharingNotBuilt)
	require.NotErrorIs(t, err, session.ErrInvalidWorld, "a precondition, not a broken world")
	require.NotErrorIs(t, err, encounter.ErrSharedKnowledgeNotBuilt, "no inner sentinel crosses the boundary")
	require.ErrorContains(t, err, "the party's sharing is set to shared knowledge, and shared knowledge is not built yet (rpg-project#937)")
	require.Empty(t, c.sessions.byID, "no run was written")
}

// leaving is the castle's exits and returns, for the catch-up tests below.
type leaving struct{ c *freeRoamCastle }

func (l leaving) exit(t *testing.T, who string) {
	t.Helper()
	_, err := l.c.mgr.Exit(context.Background(), &session.ExitInput{Session: "sess", Member: who})
	require.NoError(t, err, "%s leaves", who)
}

func (l leaving) rejoin(t *testing.T, who string) {
	t.Helper()
	_, err := l.c.mgr.Join(context.Background(), &session.JoinInput{Session: "sess", Member: who, Position: inTheGreatHall})
	require.NoError(t, err, "%s comes back", who)
}

// holds asserts who's catch-up after held opens with exactly what they were
// sent live, the cleric's shout among it.
func (l leaving) holds(t *testing.T, who, held string, live []session.Act) {
	t.Helper()
	require.NotEmpty(t, where[session.SpeechAct](live, func(s session.SpeechAct) bool { return s.Option == "tonight" }),
		"precondition: %s was sent the shout through the cleric", who)
	caught, err := l.c.mgr.Acts(context.Background(), &session.ActsInput{Session: "sess", Member: who, AfterID: held})
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(caught.Acts), len(live), "%s's catch-up", who)
	require.Equal(t, live, caught.Acts[:len(live)], "%s's catch-up opens with what was sent live", who)
}

// sharedThenLeft plays the cleric's shout to alice in the great hall, then
// the two leave in the order given and alice comes back, and asserts alice's
// catch-up still opens with what the cleric shared.
func sharedThenLeft(t *testing.T, first, second string) {
	c := theRoadAndTheHall(t, false)
	l := leaving{c}
	held := c.lastActID(t, "alice")
	live := c.clericSays(t, "tonight").to("alice")
	l.exit(t, first)
	l.exit(t, second)
	l.rejoin(t, "alice")
	l.holds(t, "alice", held, live)
}

// TestCastleCatchUpHoldsAFriendWhoLeftWhileSheWasAway is the reviewer's
// probe: alice leaves, the cleric leaves while she is away — so her own story
// never holds his exit — and alice comes back. Her catch-up still holds what
// the cleric shared with her before either left.
func TestCastleCatchUpHoldsAFriendWhoLeftWhileSheWasAway(t *testing.T) {
	sharedThenLeft(t, "alice", "cleric")
}

// TestCastleCatchUpHoldsAFriendWhoLeftFirst: the other order — the cleric
// leaves, then alice, then alice comes back.
func TestCastleCatchUpHoldsAFriendWhoLeftFirst(t *testing.T) {
	sharedThenLeft(t, "cleric", "alice")
}

// TestCastleCatchUpHoldsWhatWasSharedWithTwoAway: three players; the cleric
// shouts to alice and bob in the great hall; alice and the cleric leave while
// bob stays, and alice comes back. Both catch-ups hold what the cleric shared.
func TestCastleCatchUpHoldsWhatWasSharedWithTwoAway(t *testing.T) {
	roadCastles.once.Do(buildTheRoadCastles)
	require.NoError(t, roadCastles.err)
	c := restoreTheRoad(t, roadCastles.three)
	l := leaving{c}
	c.placeInTheRecord(t, "bob", inTheGreatHall)
	heldAlice, heldBob := c.lastActID(t, "alice"), c.lastActID(t, "bob")
	shout := c.clericSays(t, "tonight")
	l.exit(t, "alice")
	l.exit(t, "cleric")
	l.holds(t, "bob", heldBob, shout.to("bob"))
	l.rejoin(t, "alice")
	l.holds(t, "alice", heldAlice, shout.to("alice"))
}
