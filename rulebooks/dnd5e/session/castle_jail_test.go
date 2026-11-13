// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

// castle_jail_test.go is the castle's arrest played to the cells through the
// session's own verbs (rpg-project#925): both claims fail and the gate
// arrests alice and the cleric; bob, who came up to the gate after, watches.
// A held member's walk is refused; alice says "We'll come quietly." and the
// taken are set down in the cells, each viewer shown the move as they
// perceived it; jailed, alice picks the cell door and walks out; seen out of
// the cells by an arresting guard, she turns the garrison and the fight is
// published.
//
// THE VERBS ARE PLAYED ONCE, IN [TestMain], and each test asserts on what
// they published and left: the jail and the lock are the castle's two
// heaviest verbs, and the CI budget is judged per test.

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// The castle's arrest, as these tests name it.
const (
	comeQuietly    = "come-quietly"
	cellsDoor      = "the-mad-king/cells-wall-door"
	theCellsRegion = "region-cells"
	jailWords      = "The guards march you to the cells."
	jailCause      = "dnd5e:consequences:arrest"
)

// jailCastleTests are the tests that read [jailCastle].
var jailCastleTests = []string{
	"TestCastleComingQuietlyAtEachTheirOwn", "TestCastleComingQuietlyAtSharedView",
	"TestCastleTheJailedPickTheLockAndWalkOut", "TestCastleAnEscapeSeenTurnsTheGarrison",
	"TestCastleComingQuietlyIsTheTakensLine", "TestCastleAnAbsentTakenIsWantedByTheGuardsAlone",
}

// quietly is "We'll come quietly." played from the arrest at one stop of
// the dial: the held walk's refusal, what the line published, and where
// alice was and is.
type quietly struct {
	walkErr  error
	played   beat
	from, to spatial.Position
	toRegion encounter.RegionID
	gate     string
}

// jailCastle is the castle's arrest, played once in [TestMain].
var jailCastle struct {
	once     sync.Once
	err      error
	arrested castleStores
	// quietly is the line at each their own and at shared view.
	quietly map[encounter.Sharing]quietly
	// notHis is bob saying the line he is not offered, and the gate after.
	notHis     error
	notHisGate string
	// picked is alice, jailed, at the cells door from inside: the door
	// after the lock, and her walk out.
	picked  session.Door
	walkOut *session.MoveOutput
	// escape is alice out of the cells stepping into the captain's sight.
	escape      beat
	escapeClock session.ClockKind
	// absent is the line said with the cleric gone: whom the guards want,
	// and alice's and bob's whole streams, live and on catch-up, as JSON.
	absentWanted  []encounter.MemberID
	absentStreams map[string]string
}

// newJailHost is a castle over the given stores, with its manager, and
// dice that throw d20 on every check.
func newJailHost(stores castleStores, sharing encounter.Sharing, d20 int) (*freeRoamCastle, error) {
	c := &freeRoamCastle{
		live: &actCapture{}, dice: &castleDice{d20s: []int{d20}},
		sessions: newFakeSessions(), encounters: newFakeEncounters(),
		characters: newFakeCharacters(), seats: newFakeSeats(),
	}
	if err := restoreInto(c, stores); err != nil {
		return nil, err
	}
	if sharing != "" {
		c.encounters.byID["sess"].Sharing = sharing
	}
	var err error
	c.mgr, err = session.NewManager(&session.Config{Record: session.DiscardRecord{}, Acts: c.live, Seats: c.seats, CharacterConfig: newFakeCharacterConfig(),
		PresentationIDs: testPresentationIDs{}, Dice: c.dice, TurnDriver: session.Driver(),
		Sessions: c.sessions, Encounters: c.encounters, Characters: c.characters,
	})
	return c, err
}

// capture runs one verb and returns what it published.
func capture(c *freeRoamCastle, label string, verb func() error) (beat, error) {
	c.live.mu.Lock()
	c.live.acts = nil
	c.live.mu.Unlock()
	err := verb()
	c.live.mu.Lock()
	defer c.live.mu.Unlock()
	return beat{label: label, acts: append([]session.Act(nil), c.live.acts...)}, err
}

// placedAt is where a member stands in a host's stored world.
func placedAt(c *freeRoamCastle, id encounter.MemberID) spatial.Position {
	for _, m := range c.encounters.byID["sess"].Members {
		if m.ID == id && m.Cell != nil {
			return spatial.Position{X: m.Cell.X, Y: m.Cell.Y}
		}
	}
	return spatial.Position{}
}

// standIn puts a member on a cell in a host's stored world.
func standIn(c *freeRoamCastle, id encounter.MemberID, cell spatial.Position) {
	for i, m := range c.encounters.byID["sess"].Members {
		if m.ID == id {
			c.encounters.byID["sess"].Members[i].Cell = &encounter.PositionData{X: cell.X, Y: cell.Y}
		}
	}
}

// gateState is the front gate's progress in a host's stored world.
func gateState(c *freeRoamCastle) string {
	for _, in := range c.encounters.byID["sess"].Interactions {
		if in.ID == "front-gate" {
			return in.Blob.State
		}
	}
	return ""
}

// buildTheJailCastle plays it all. It takes no *testing.T because
// [TestMain] runs it; every precondition it checks is an error.
func buildTheJailCastle() { jailCastle.err = playTheJailCastle() }

// playTheJailCastle is the scene, played in order.
//
//nolint:gocyclo // one scene, played in order
func playTheJailCastle() error {
	ctx := context.Background()
	dir, err := exec.Command("go", "list", "-m", "-f", "{{.Dir}}", encounterModule).Output()
	if err != nil {
		return fmt.Errorf("find the encounter module: %w", err)
	}
	raw, err := os.ReadFile(filepath.Join(strings.TrimSpace(string(dir)), castleFixture))
	if err != nil {
		return err
	}
	compiled, err := dungeonspec.Load(raw)
	if err != nil {
		return err
	}
	// 12 + 3 beats the stranger's 5; every claim after misses.
	c := &freeRoamCastle{
		live: &actCapture{}, dice: &castleDice{d20s: []int{12, 1}},
		characters: newFakeCharacters(armedFighter("alice"), healingCleric(), armedFighter("bob")),
		encounters: newFakeEncounters(), sessions: newFakeSessions(), seats: newFakeSeats(),
	}
	c.mgr, err = session.NewManager(&session.Config{Record: session.DiscardRecord{}, Acts: c.live, Seats: c.seats, CharacterConfig: newFakeCharacterConfig(),
		PresentationIDs: testPresentationIDs{}, Dice: c.dice, TurnDriver: session.Driver(),
		Sessions: c.sessions, Encounters: c.encounters, Characters: c.characters,
	})
	if err != nil {
		return err
	}
	if _, err := c.mgr.Launch(ctx, &session.LaunchInput{Session: "sess", Dungeon: &compiled, Party: []string{"alice", "cleric"}}); err != nil {
		return fmt.Errorf("launch: %w", err)
	}
	if _, err := speakAsOffered(ctx, c.mgr, &session.SpeakInput{Session: "sess", Member: "alice", Target: "pedlar", OptionID: "tonight"}); err != nil {
		return fmt.Errorf("the stranger tells alice of the feast: %w", err)
	}
	standIn(c, "alice", backDownTheRoad[0])
	standIn(c, "cleric", watchingTheRoad)
	if _, err := c.mgr.Move(ctx, &session.MoveInput{Session: "sess", Member: "alice", Path: []spatial.Position{theHailCell}}); err != nil {
		return fmt.Errorf("the step to the hail cell: %w", err)
	}
	for _, claim := range []string{"troupe", "guest"} {
		if _, err := speakAsOffered(ctx, c.mgr, &session.SpeakInput{Session: "sess", Member: "alice", Target: "captain", OptionID: claim}); err != nil {
			return fmt.Errorf("alice's %s claim: %w", claim, err)
		}
	}
	// BOB COMES UP AFTER: beside alice on the road, not taken, watching.
	if _, err := c.mgr.Join(ctx, &session.JoinInput{Session: "sess", Member: "bob", Position: besideAlice}); err != nil {
		return fmt.Errorf("bob joins: %w", err)
	}
	enc, err := c.mgr.LoadWorldForTest(ctx, "sess")
	if err != nil {
		return err
	}
	if made, ok := enc.Arrests()["front-gate"]; !ok || len(made.Taken) != 2 {
		return fmt.Errorf("precondition: the gate arrests alice and the cleric: %+v", made)
	}
	jailCastle.arrested = storesOf(c)

	// "WE'LL COME QUIETLY", at each stop of the dial.
	jailCastle.quietly = map[encounter.Sharing]quietly{}
	var jailed castleStores
	for _, sharing := range []encounter.Sharing{encounter.SharingOwn, encounter.SharingView} {
		h, err := newJailHost(jailCastle.arrested, sharing, 1)
		if err != nil {
			return err
		}
		var q quietly
		_, q.walkErr = h.mgr.Move(ctx, &session.MoveInput{Session: "sess", Member: "alice", Path: []spatial.Position{backDownTheRoad[0]}})
		q.from = placedAt(h, "alice")
		if q.played, err = capture(h, "alice comes quietly", func() error {
			_, err := speakAsOffered(ctx, h.mgr, &session.SpeakInput{Session: "sess", Member: "alice", Target: "captain", OptionID: comeQuietly})
			return err
		}); err != nil {
			return fmt.Errorf("alice comes quietly at %s: %w", sharing, err)
		}
		q.to, q.gate = placedAt(h, "alice"), gateState(h)
		after, err := h.mgr.LoadWorldForTest(ctx, "sess")
		if err != nil {
			return err
		}
		q.toRegion, _ = after.RegionAt(q.to)
		jailCastle.quietly[sharing] = q
		jailed = storesOf(h)
	}

	// BOB SAYS THE LINE HE IS NOT OFFERED.
	h, err := newJailHost(jailCastle.arrested, encounter.SharingOwn, 1)
	if err != nil {
		return err
	}
	_, jailCastle.notHis = h.mgr.Speak(ctx, &session.SpeakInput{Session: "sess", Member: "bob", Target: "captain", OptionID: comeQuietly})
	jailCastle.notHisGate = gateState(h)

	// JAILED, ALICE PICKS THE CELL DOOR AND WALKS OUT.
	if h, err = newJailHost(jailed, encounter.SharingOwn, 20); err != nil {
		return err
	}
	atlas, err := h.mgr.Atlas(ctx, &session.AtlasInput{Session: "sess", Member: "alice"})
	if err != nil {
		return err
	}
	if enc, err = h.mgr.LoadWorldForTest(ctx, "sess"); err != nil {
		return err
	}
	var inside, outside spatial.Position
	for _, d := range atlas.Doorways {
		if d.Door != cellsDoor {
			continue
		}
		if r, _ := enc.RegionAt(d.From); r == theCellsRegion {
			inside, outside = d.From, d.To
		} else if r, _ := enc.RegionAt(d.To); r == theCellsRegion {
			inside, outside = d.To, d.From
		}
	}
	if inside == outside {
		return fmt.Errorf("precondition: the cells door has a crossing into the cells")
	}
	standIn(h, "alice", inside) // across her cell to the door: not what this proves
	picked, err := h.mgr.Unlock(ctx, &session.UnlockInput{Session: "sess", Member: "alice", Door: cellsDoor})
	if err != nil {
		return fmt.Errorf("alice picks the lock: %w", err)
	}
	jailCastle.picked = picked.Door
	if jailCastle.walkOut, err = h.mgr.Move(ctx, &session.MoveInput{Session: "sess", Member: "alice", Path: []spatial.Position{outside}}); err != nil {
		return fmt.Errorf("alice walks out: %w", err)
	}

	// SEEN OUTSIDE BY THE CAPTAIN WHO ARRESTED HER.
	if h, err = newJailHost(jailed, encounter.SharingOwn, 10); err != nil {
		return err
	}
	standIn(h, "alice", backDownTheRoad[0]) // out of the cells: the walk there is not what this proves
	if jailCastle.escape, err = capture(h, "alice steps into the captain's sight", func() error {
		_, err := h.mgr.Move(ctx, &session.MoveInput{Session: "sess", Member: "alice", Path: []spatial.Position{theHailCell}})
		return err
	}); err != nil {
		return fmt.Errorf("the escape: %w", err)
	}
	turn, err := h.mgr.Turn(ctx, &session.TurnInput{Session: "sess", Member: "alice"})
	if err != nil {
		return err
	}
	jailCastle.escapeClock = turn.Clock

	// THE CLERIC GONE WHEN ALICE COMES QUIETLY.
	if h, err = newJailHost(jailCastle.arrested, encounter.SharingView, 1); err != nil {
		return err
	}
	if _, err := h.mgr.Exit(ctx, &session.ExitInput{Session: "sess", Member: "cleric"}); err != nil {
		return fmt.Errorf("the cleric leaves: %w", err)
	}
	absent, err := capture(h, "alice comes quietly", func() error {
		_, err := speakAsOffered(ctx, h.mgr, &session.SpeakInput{Session: "sess", Member: "alice", Target: "captain", OptionID: comeQuietly})
		return err
	})
	if err != nil {
		return fmt.Errorf("alice comes quietly without the cleric: %w", err)
	}
	if enc, err = h.mgr.LoadWorldForTest(ctx, "sess"); err != nil {
		return err
	}
	for _, w := range enc.Arrests()["front-gate"].Wanted {
		jailCastle.absentWanted = append(jailCastle.absentWanted, w.ID)
	}
	jailCastle.absentStreams = map[string]string{}
	for _, member := range []string{"alice", "bob"} {
		read, err := h.mgr.Acts(ctx, &session.ActsInput{Session: "sess", Member: member})
		if err != nil {
			return err
		}
		raw, err := json.Marshal(append(absent.to(member), read.Acts...))
		if err != nil {
			return err
		}
		jailCastle.absentStreams[member] = string(raw)
	}
	return nil
}

// storesOf is a copy of a castle's stores as they stand.
func storesOf(c *freeRoamCastle) castleStores {
	out := castleStores{
		sessions: map[string]*session.SessionData{}, encounters: map[string]*encounter.EncounterData{},
		characters: map[string]*character.Data{}, seats: map[string]session.SeatData{},
	}
	for id, d := range c.sessions.byID {
		copied, _ := copyOf(d)
		out.sessions[id] = copied
	}
	for id, d := range c.encounters.byID {
		copied, _ := copyOf(d)
		out.encounters[id] = copied
	}
	for id, d := range c.characters.byID {
		copied, _ := copyOf(d)
		out.characters[id] = copied
	}
	for id, d := range c.seats.byID {
		out.seats[id] = d
	}
	return out
}

// theJail is [jailCastle], built if this run's TestMain did not.
func theJail(t *testing.T) {
	t.Helper()
	jailCastle.once.Do(buildTheJailCastle)
	require.NoError(t, jailCastle.err, "the jail castle")
}

// moverIs keeps a RelocateAct moving member.
func moverIs(member string) func(session.RelocateAct) bool {
	return func(r session.RelocateAct) bool { return r.Mover.ID == member }
}

// requireCameQuietly is what every stop of the dial agrees on: the held
// walk refused, the gate jailed, alice in the cells and shown both ends with
// her words, the cleric shown her own move, and no `wanted` in any stream.
func requireCameQuietly(t *testing.T, q quietly) {
	t.Helper()
	requireHeldYou(t, q.walkErr, "a held member's walk")
	require.Equal(t, "jailed", q.gate, "the gate is jailed")
	require.Equal(t, encounter.RegionID(theCellsRegion), q.toRegion, "alice is in the cells")
	from, to := q.from, q.to
	mine := where[session.RelocateAct](q.played.to("alice"), moverIs("alice"))
	require.Len(t, mine, 1, "alice is shown her move")
	require.Equal(t, session.RelocateAct{
		Mover: aliceParty, From: &from, To: &to, Cause: jailCause, Words: jailWords, End: session.RelocateBoth,
	}, mine[0].body)
	clerics := where[session.RelocateAct](q.played.to("cleric"), moverIs("cleric"))
	require.Len(t, clerics, 1, "the cleric, taken too, is shown her own move")
	require.Equal(t, jailWords, clerics[0].body.Words)
	for _, member := range []string{"alice", "cleric", "bob"} {
		raw, err := json.Marshal(q.played.to(member))
		require.NoError(t, err)
		require.NotContains(t, strings.ToLower(string(raw)), "wanted", "%s: no wanted after the jail", member)
	}
}

// TestCastleComingQuietlyAtEachTheirOwn: bob, who came up after the arrest
// and stands beside the hail cell, saw alice leave: he is shown the leaving,
// where from, never where to, and none of her words.
func TestCastleComingQuietlyAtEachTheirOwn(t *testing.T) {
	theJail(t)
	q := jailCastle.quietly[encounter.SharingOwn]
	requireCameQuietly(t, q)
	seen := where[session.RelocateAct](q.played.to("bob"), moverIs("alice"))
	require.Len(t, seen, 1, "bob saw alice go")
	require.Equal(t, session.RelocateLeft, seen[0].body.End)
	require.NotNil(t, seen[0].body.From)
	require.Nil(t, seen[0].body.To, "never where to")
	require.Empty(t, seen[0].body.Words, "and none of her words")
}

// TestCastleComingQuietlyAtSharedView: bob is shown alice's move once, her
// own version through her: both ends and the words.
func TestCastleComingQuietlyAtSharedView(t *testing.T) {
	theJail(t)
	q := jailCastle.quietly[encounter.SharingView]
	requireCameQuietly(t, q)
	seen := where[session.RelocateAct](q.played.to("bob"), moverIs("alice"))
	require.Len(t, seen, 1, "one act for one move")
	require.Equal(t, session.RelocateBoth, seen[0].body.End)
	require.NotNil(t, seen[0].body.To, "the destination through alice")
	require.Equal(t, jailWords, seen[0].body.Words, "and her words")
	require.NotNil(t, seen[0].act.Perceived)
	require.Contains(t, seen[0].act.Perceived.Via, aliceParty, "through alice")
}

// TestCastleComingQuietlyIsTheTakensLine: bob, never arrested, is not
// offered the line, and saying it is refused as a line not offered.
func TestCastleComingQuietlyIsTheTakensLine(t *testing.T) {
	theJail(t)
	require.ErrorIs(t, jailCastle.notHis, session.ErrNoOption, "not his to say")
	require.Equal(t, "arrested", jailCastle.notHisGate, "nothing moved")
}

// TestCastleTheJailedPickTheLockAndWalkOut: jailed holds nothing by itself.
// Alice, at the cells door from inside, picks its lock — a picked lock opens
// the door — and walks out.
func TestCastleTheJailedPickTheLockAndWalkOut(t *testing.T) {
	theJail(t)
	require.Equal(t, "open", jailCastle.picked.State, "she picks the lock and the door opens")
	require.Equal(t, session.MovementCompleted, jailCastle.walkOut.Status, "she walks out: %s", jailCastle.walkOut.StopReason)
}

// TestCastleAnEscapeSeenTurnsTheGarrison: alice, jailed, out of the cells
// and seen by the captain who arrested her: the garrison turns and the fight
// is published to her.
//
// FINDING, NOT PINNED: the walk's own output does not report this fight
// (MoveOutput.Formed is nil): the encounter's step reports a fight formed by
// contact, and this one formed when the garrison turned at the step's sight
// refresh. The record carries it, and the stream shows it.
func TestCastleAnEscapeSeenTurnsTheGarrison(t *testing.T) {
	theJail(t)
	turned := where[session.StateChange](jailCastle.escape.to("alice"), func(s session.StateChange) bool { return s.Stance != nil })
	require.NotEmpty(t, turned, "the garrison turns")
	formed := where[session.ClockEntry](jailCastle.escape.to("alice"), func(e session.ClockEntry) bool { return e.Mark == session.ClockFightFormed })
	require.NotEmpty(t, formed, "the fight is published to her")
	require.Equal(t, session.ClockTurn, jailCastle.escapeClock, "and she is in it")
}

// TestCastleAnAbsentTakenIsWantedByTheGuardsAlone: the cleric, taken, has
// left the run when alice comes quietly. The guards come to want her; no
// player's stream, live or on catch-up, carries it.
func TestCastleAnAbsentTakenIsWantedByTheGuardsAlone(t *testing.T) {
	theJail(t)
	require.Contains(t, jailCastle.absentWanted, encounter.MemberID("cleric"), "precondition: the guards want the absent cleric")
	for member, stream := range jailCastle.absentStreams {
		require.NotContains(t, strings.ToLower(stream), "wanted", "%s: the guards' wanted is theirs alone", member)
	}
}
