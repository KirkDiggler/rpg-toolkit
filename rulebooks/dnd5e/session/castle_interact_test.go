// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

// castle_interact_test.go is something done to a thing, at the session door
// (rpg-project#959): the castle's kitchen passage played through the
// session's own verbs. Alice buys the way from the scullion with the purse,
// and she and the cleric, who caught the answer, come to know the grate:
// its things to do appear on their Speak rows and nobody else's. Alice lifts
// the grate and goes up to the chapel. The cleric, shown the grate, sees
// her climb down; bob, who came in after and was never told, sees only that
// she did something there and left; dan, out of sight, is sent nothing. In
// the chapel she examines the altar, and she and the cleric watching learn
// what it shows.
//
// THE VERBS ARE PLAYED ONCE, IN [TestMain], and each test asserts on what
// they published and left: the castle's launch and verbs are heavy, and the
// CI budget is judged per test.

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

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// The kitchen passage, as these tests name it: cells are dungeon-absolute.
var (
	// besideThePurseCell is kitchen floor beside the purse and in speaking
	// reach of the scullion.
	besideThePurseCell = spatial.Position{X: -17, Y: -10}
	// byTheGrate is kitchen floor beside the grate, where the cleric stands.
	byTheGrate = spatial.Position{X: -13, Y: -14}
	// atTheGrate is where alice does the grate's thing, within reach.
	atTheGrate = spatial.Position{X: -11, Y: -14}
	// acrossTheKitchen is where bob stands, in sight of the grate and out of
	// its reach, having come in after the telling.
	acrossTheKitchen = spatial.Position{X: -14, Y: -11}
	// beforeTheAltar is chapel floor within reach of the altar.
	beforeTheAltar = spatial.Position{X: 25, Y: -12}
	// besideTheAltar is chapel floor in sight of the altar.
	besideTheAltar = spatial.Position{X: 24, Y: -10}
)

// passagePlayTests are the tests that read [passagePlay].
var passagePlayTests = []string{
	"TestCastleTheToldAreOfferedTheGrate", "TestCastleTheGrateAtEachTheirOwn", "TestCastleTheGrateAtSharedView",
	"TestCastleAnUntoldWitnessSeesOnlyAPlace", "TestCastleAPropsAnswerIsNarratedToWhomItWasShown",
	"TestCastleTheAltarExaminedTeachesItsWatchers", "TestCastleAThingIsNotSaidAndTheUntoldAreRefused",
}

// passagePlay is the kitchen passage, played once in [TestMain].
var passagePlay struct {
	once sync.Once
	err  error
	// rows are each player's interact rows at the grate, after the telling.
	rows map[string][]session.InteractOptionRow
	// from and to are where alice stood and was set down.
	from, to spatial.Position
	// grate is the grate done, at each their own: what it published, and
	// every player's catch-up stream after, folded at each their own and at
	// shared view.
	grate  beat
	own    map[string][]session.Act
	view   map[string][]session.Act
	altar  beat
	learnt map[string]bool
	// telling is the scullion's answer to alice's coin; altarView is bob's
	// catch-up after the altar, at shared view.
	telling   beat
	altarView []session.Act
	// untold is bob doing the grate's thing; said is alice SAYING it.
	untold, said error
}

// buildThePassagePlay plays it all. It takes no *testing.T because
// [TestMain] runs it.
func buildThePassagePlay() { passagePlay.err = playThePassage() }

// playThePassage is the scene, played in order.
//
//nolint:gocyclo // one scene, played in order
func playThePassage() error {
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
	c := &freeRoamCastle{
		live: &actCapture{}, dice: &castleDice{d20s: []int{18}},
		characters: newFakeCharacters(armedFighter("alice"), healingCleric(), armedFighter("bob"), armedFighter("dan")),
		encounters: newFakeEncounters(), sessions: newFakeSessions(), seats: newFakeSeats(),
	}
	c.mgr, err = session.NewManager(&session.Config{Record: session.DiscardRecord{}, Acts: c.live, Seats: c.seats, CharacterConfig: newFakeCharacterConfig(),
		PresentationIDs: testPresentationIDs{}, Dice: c.dice, TurnDriver: session.Driver(),
		Sessions: c.sessions, Encounters: c.encounters, Characters: c.characters,
	})
	if err != nil {
		return err
	}
	if _, err := c.mgr.Launch(ctx, &session.LaunchInput{Session: "sess", Dungeon: &compiled, Party: []string{"alice", "cleric", "bob", "dan"}}); err != nil {
		return fmt.Errorf("launch: %w", err)
	}
	c.encounters.byID["sess"].Sharing = encounter.SharingOwn
	// THE TELLING: alice beside the purse, the cleric by the grate, bob and
	// dan down the road where nothing in the kitchen reaches them.
	standIn(c, "alice", spatial.Position{X: besideThePurseCell.X - 1, Y: besideThePurseCell.Y})
	standIn(c, "cleric", byTheGrate)
	standIn(c, "bob", downTheRoad)
	standIn(c, "dan", onTheRoadUnseen)
	if _, err := c.mgr.Move(ctx, &session.MoveInput{Session: "sess", Member: "alice", Path: []spatial.Position{besideThePurseCell}}); err != nil {
		return fmt.Errorf("alice steps beside the purse: %w", err)
	}
	if _, err := c.mgr.Hold(ctx, &session.HoldInput{Session: "sess", Member: "alice", Target: "purse"}); err != nil {
		return fmt.Errorf("alice takes the purse: %w", err)
	}
	if passagePlay.telling, err = capture(c, "alice buys the way", func() error {
		_, err := speakAsOffered(ctx, c.mgr, &session.SpeakInput{Session: "sess", Member: "alice", Target: "scullion", OptionID: "favour"})
		return err
	}); err != nil {
		return fmt.Errorf("alice buys the way: %w", err)
	}
	// BOB COMES IN AFTER, across the kitchen; alice steps up to the grate.
	standIn(c, "bob", acrossTheKitchen)
	standIn(c, "alice", spatial.Position{X: atTheGrate.X + 1, Y: atTheGrate.Y})
	if _, err := c.mgr.Move(ctx, &session.MoveInput{Session: "sess", Member: "alice", Path: []spatial.Position{atTheGrate}}); err != nil {
		return fmt.Errorf("alice steps up to the grate: %w", err)
	}
	passagePlay.rows = map[string][]session.InteractOptionRow{}
	for _, m := range []string{"alice", "cleric", "bob", "dan"} {
		panel, err := c.mgr.Afford(ctx, &session.AffordInput{Session: "sess", Member: m})
		if err != nil {
			return err
		}
		for _, d := range panel.Declarations {
			passagePlay.rows[m] = append(passagePlay.rows[m], d.InteractOptions...)
		}
	}
	revision := uint64(0)
	for _, r := range passagePlay.rows["alice"] {
		if r.Prop == "grate" && r.OptionID == "to-chapel" {
			revision = r.Revision
		}
	}
	_, passagePlay.said = c.mgr.Speak(ctx, &session.SpeakInput{Session: "sess", Member: "alice", Prop: "grate", OptionID: "to-chapel", Revision: revision})
	_, passagePlay.untold = c.mgr.Interact(ctx, &session.InteractInput{Session: "sess", Actor: "bob", Prop: "grate", OptionID: "to-chapel"})

	// THE GRATE, at each their own.
	passagePlay.from = placedAt(c, "alice")
	if passagePlay.grate, err = capture(c, "alice lifts the grate", func() error {
		_, err := c.mgr.Interact(ctx, &session.InteractInput{Session: "sess", Actor: "alice", Prop: "grate", OptionID: "to-chapel", Revision: revision})
		return err
	}); err != nil {
		return fmt.Errorf("alice lifts the grate: %w", err)
	}
	passagePlay.to = placedAt(c, "alice")
	after := storesOf(c)
	passagePlay.own, passagePlay.view = map[string][]session.Act{}, map[string][]session.Act{}
	// THE CATCH-UP READS THE TESTS USE, and no more: each is a fold of the
	// castle's whole story.
	for sharing, reads := range map[encounter.Sharing][]string{encounter.SharingOwn: {"bob", "dan"}, encounter.SharingView: {"alice", "bob"}} {
		into := passagePlay.own
		if sharing == encounter.SharingView {
			into = passagePlay.view
		}
		h, err := newJailHost(after, sharing, 18)
		if err != nil {
			return err
		}
		for _, m := range reads {
			read, err := h.mgr.Acts(ctx, &session.ActsInput{Session: "sess", Member: m})
			if err != nil {
				return err
			}
			into[m] = read.Acts
		}
	}

	// THE ALTAR: alice before it, the cleric beside it.
	standIn(c, "cleric", besideTheAltar)
	standIn(c, "alice", spatial.Position{X: beforeTheAltar.X - 1, Y: beforeTheAltar.Y})
	if _, err := c.mgr.Move(ctx, &session.MoveInput{Session: "sess", Member: "alice", Path: []spatial.Position{beforeTheAltar}}); err != nil {
		return fmt.Errorf("alice steps up to the altar: %w", err)
	}
	if passagePlay.altar, err = capture(c, "alice examines the altar", func() error {
		_, err := c.mgr.Interact(ctx, &session.InteractInput{Session: "sess", Actor: "alice", Prop: "altar", OptionID: "examine"})
		return err
	}); err != nil {
		return fmt.Errorf("alice examines the altar: %w", err)
	}
	view, err := newJailHost(storesOf(c), encounter.SharingView, 18)
	if err != nil {
		return err
	}
	read, err := view.mgr.Acts(ctx, &session.ActsInput{Session: "sess", Member: "bob"})
	if err != nil {
		return err
	}
	passagePlay.altarView = read.Acts
	passagePlay.learnt = map[string]bool{}
	if journal := c.encounters.byID["sess"].Journal; journal != nil {
		raw, err := json.Marshal(journal)
		if err != nil {
			return err
		}
		for _, m := range []string{"alice", "cleric", "bob", "dan"} {
			passagePlay.learnt[m] = strings.Contains(string(raw), `"kind":"known:fact:king-is-cultist","actor":"`+m+`"`)
		}
	}
	return nil
}

// thePassage is [passagePlay], built if this run's TestMain did not.
func thePassage(t *testing.T) {
	t.Helper()
	passagePlay.once.Do(buildThePassagePlay)
	require.NoError(t, passagePlay.err, "the passage play")
}

// grateRows is the member's interact rows for the grate, by option id.
func grateRows(member string) []string {
	var out []string
	for _, r := range passagePlay.rows[member] {
		if r.Prop == "grate" {
			out = append(out, r.OptionID)
		}
	}
	return out
}

// interactsBy is every InteractAct in acts done by actor.
func interactsBy(acts []session.Act, actor string) []found[session.InteractAct] {
	return where[session.InteractAct](acts, func(a session.InteractAct) bool { return a.Actor.ID == actor })
}

// TestCastleTheToldAreOfferedTheGrate: the grate's things to do are on the
// Speak row of alice and the cleric, who caught the scullion's answer, as
// interact rows and not as lines; bob, who came in after, and dan are
// offered nothing at the grate.
func TestCastleTheToldAreOfferedTheGrate(t *testing.T) {
	thePassage(t)
	require.ElementsMatch(t, []string{"to-chapel", "to-cells"}, grateRows("alice"), "alice was told")
	require.ElementsMatch(t, []string{"to-chapel", "to-cells"}, grateRows("cleric"), "the cleric caught it too")
	require.Empty(t, grateRows("bob"), "bob was never told")
	require.Empty(t, grateRows("dan"), "nor dan")
	for _, r := range passagePlay.rows["alice"] {
		if r.OptionID == "to-chapel" {
			require.Equal(t, "You lift the grate and take the way up.", r.Do)
			require.True(t, r.Available)
		}
	}
}

// TestCastleTheGrateAtEachTheirOwn: alice is shown what she did and where
// she went; the cleric, shown the grate, sees her climb down and leave; dan,
// out of sight, is sent nothing. Bob, in sight and never told, is
// [TestCastleAnUntoldWitnessSeesOnlyAPlace].
func TestCastleTheGrateAtEachTheirOwn(t *testing.T) {
	thePassage(t)
	from, to := passagePlay.from, passagePlay.to
	live := passagePlay.grate

	mine := interactsBy(live.to("alice"), "alice")
	require.Len(t, mine, 1, "alice is shown what she did")
	require.Equal(t, session.Party{Kind: session.PartyProp, ID: "grate"}, mine[0].body.Prop)
	require.Equal(t, "to-chapel", mine[0].body.Option)
	require.Equal(t, "You lift the grate and take the way up.", mine[0].body.Words)
	require.NotNil(t, mine[0].body.Beaten)
	require.True(t, *mine[0].body.Beaten, "a plain thing lands")
	moved := where[session.RelocateAct](live.to("alice"), moverIs("alice"))
	require.Len(t, moved, 1)
	require.Equal(t, session.RelocateBoth, moved[0].body.End)
	require.Equal(t, &from, moved[0].body.From)
	require.Equal(t, &to, moved[0].body.To)

	seen := interactsBy(live.to("cleric"), "alice")
	require.Len(t, seen, 1, "the cleric saw it")
	require.Equal(t, session.InteractAct{
		Actor: aliceParty, Prop: session.Party{Kind: session.PartyProp, ID: "grate"},
		Words: "lifts the grate by the hearth and climbs down into the dark",
	}, seen[0].body, "the seen words, never the doer's own")

	require.Empty(t, interactsBy(append(live.to("dan"), passagePlay.own["dan"]...), "alice"), "dan saw nothing")
	require.Empty(t, where[session.RelocateAct](append(live.to("dan"), passagePlay.own["dan"]...), moverIs("alice")))
}

// TestCastleAnUntoldWitnessSeesOnlyAPlace: bob, in sight of alice and
// never told of the grate, at each their own, sees only that she did
// something where she stood and left — no prop named anywhere on his
// stream, live or on catch-up.
//
// THE ONE CASE THE OPERATOR'S "if you saw it, you know it" FLIPS: at the
// encounter that rules it, bob comes to know the grate by sight, is shown
// the `thing` version, and is offered the grate's things to do. Every
// assertion of this case is here.
func TestCastleAnUntoldWitnessSeesOnlyAPlace(t *testing.T) {
	thePassage(t)
	from, live := passagePlay.from, passagePlay.grate
	placed := interactsBy(live.to("bob"), "alice")
	require.Len(t, placed, 1, "bob saw her do something")
	require.Equal(t, session.InteractAct{Actor: aliceParty, Place: &from}, placed[0].body, "where, and nothing it was")
	left := where[session.RelocateAct](live.to("bob"), moverIs("alice"))
	require.Len(t, left, 1)
	require.Equal(t, session.RelocateLeft, left[0].body.End)
	raw, err := json.Marshal(append(live.to("bob"), passagePlay.own["bob"]...))
	require.NoError(t, err)
	require.NotContains(t, string(raw), "grate", "no prop named on bob's stream, live or on catch-up")
}

// TestCastleTheGrateAtSharedView: bob is shown alice's own version through
// her — the grate, what she did, her words — one act for one deed, marked
// via alice; shown, never known: his own offers stay without the grate.
func TestCastleTheGrateAtSharedView(t *testing.T) {
	thePassage(t)
	through := interactsBy(passagePlay.view["bob"], "alice")
	require.Len(t, through, 1, "one act for one deed")
	require.Equal(t, session.Party{Kind: session.PartyProp, ID: "grate"}, through[0].body.Prop, "the grate, through alice")
	require.Equal(t, "to-chapel", through[0].body.Option)
	require.Equal(t, "You lift the grate and take the way up.", through[0].body.Words)
	require.Nil(t, through[0].body.Place, "a named thing needs no place")
	require.NotNil(t, through[0].act.Perceived)
	require.Equal(t, []session.Party{aliceParty}, through[0].act.Perceived.Via)
	require.Empty(t, grateRows("bob"), "shown, never known: bob is offered nothing at the grate")

	mine := interactsBy(passagePlay.view["alice"], "alice")
	require.Len(t, mine, 1)
	require.Nil(t, mine[0].act.Perceived, "alice's own is hers")
}

// TestCastleTheAltarExaminedTeachesItsWatchers: alice examines the altar
// (checked, and beaten); she is shown her check, the cleric beside it the
// seen words, and both come to know what it shows.
func TestCastleTheAltarExaminedTeachesItsWatchers(t *testing.T) {
	thePassage(t)
	mine := interactsBy(passagePlay.altar.to("alice"), "alice")
	require.Len(t, mine, 1)
	require.Equal(t, "examine", mine[0].body.Option)
	require.Equal(t, "religion", mine[0].body.Skill, "her check")
	require.NotNil(t, mine[0].body.Roll)
	seen := interactsBy(passagePlay.altar.to("cleric"), "alice")
	require.Len(t, seen, 1)
	require.Equal(t, "looks the altar over", seen[0].body.Words)
	require.Nil(t, seen[0].body.Roll, "the check is the doer's")
	require.True(t, passagePlay.learnt["alice"], "alice learns what it shows")
	require.True(t, passagePlay.learnt["cleric"], "and so does the cleric, watching")
	require.False(t, passagePlay.learnt["bob"], "bob, back in the kitchen, does not")
}

// TestCastleAThingIsNotSaidAndTheUntoldAreRefused: alice saying the grate's
// thing is refused as a line not offered; bob, never told, doing it is
// refused in the second person as a thing he does not see.
func TestCastleAThingIsNotSaidAndTheUntoldAreRefused(t *testing.T) {
	thePassage(t)
	require.ErrorIs(t, passagePlay.said, session.ErrNoOption, "a thing to do is not said")
	var refusal *session.ThingRefusal
	require.ErrorAs(t, passagePlay.untold, &refusal)
	require.ErrorIs(t, passagePlay.untold, session.ErrNoProp)
	require.Equal(t, "You see no such thing here.", refusal.Sentence)
}

// altarSaid is the altar's answer to a beaten examination.
const altarSaid = "The sigils are the Pale One’s. The king keeps a shrine to it."

// narrated is every TableAnswer in acts that carries narration, by whom.
func narrated(acts []session.Act, who string) []found[session.TableAnswer] {
	return where[session.TableAnswer](acts, func(a session.TableAnswer) bool { return a.Who.ID == who && a.Narration != "" })
}

// TestCastleAPropsAnswerIsNarratedToWhomItWasShown: the altar's answer
// reaches alice, who examined it, and the cleric, shown it, as narration on
// the pick; bob and dan, who saw none of it, get none at each their own; at
// shared view bob gets it through alice. The scullion's answer — a
// creature's — carries no narration: its words rode speech.
func TestCastleAPropsAnswerIsNarratedToWhomItWasShown(t *testing.T) {
	thePassage(t)
	for _, member := range []string{"alice", "cleric"} {
		got := narrated(passagePlay.altar.to(member), "altar")
		require.Len(t, got, 1, "%s is told what the altar shows", member)
		require.Equal(t, altarSaid, got[0].body.Narration, member)
		require.Equal(t, session.Party{Kind: session.PartyProp, ID: "altar"}, got[0].body.Who)
	}
	for _, member := range []string{"bob", "dan"} {
		require.Empty(t, narrated(passagePlay.altar.to(member), "altar"), "%s saw none of it", member)
	}
	through := narrated(passagePlay.altarView, "altar")
	require.Len(t, through, 1, "at shared view bob is told it through alice")
	require.Equal(t, altarSaid, through[0].body.Narration)
	require.NotNil(t, through[0].act.Perceived)
	require.Contains(t, through[0].act.Perceived.Via, aliceParty)

	scullion := where[session.TableAnswer](passagePlay.telling.to("alice"), func(a session.TableAnswer) bool { return a.Who.ID == "scullion" })
	require.NotEmpty(t, scullion, "precondition: alice sees the scullion's pick")
	for _, a := range scullion {
		require.Empty(t, a.body.Narration, "a creature's answer is never narration")
	}
	words := where[session.SpeechAct](passagePlay.telling.to("alice"), func(s session.SpeechAct) bool {
		return s.Speaker.ID == "scullion" && s.Words != nil
	})
	require.NotEmpty(t, words, "its words rode speech")
}
