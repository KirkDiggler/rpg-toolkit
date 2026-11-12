// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

// castle_gate_hail_test.go is THE GUARD OPENS THE CONVERSATION at the session
// (rpg-project#923, Gate S3 of #769): a carrier of the front gate who sees a
// character come inside its approach distance hails her. The session drops
// the rest of her walk, puts the address on her own Speak row and nobody
// else's, and the hail reaches whoever witnesses it as the guard's answer.
//
// The castle the encounter module ships carries the hail: the front gate's
// `approach: { within: 2 }`, and an `on: approached` row and line for the
// captain, gate-1 and gate-2. The captain is the gate's nearest carrier to a
// party walking up the road, so he is the one who hails.

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/character"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// theCaptainsHail is the captain's hail line in the castle.
const theCaptainsHail = "You there. Nobody passes this gate without my say. Speak."

// The hail on the road to the gate: the cell alice is hailed on walking up
// from the party start toward the gate, and the cells back down the road,
// the last of them beyond Speak reach of the captain.
var (
	theHailCell     = spatial.Position{X: -6, Y: 10}
	backDownTheRoad = []spatial.Position{{X: -6, Y: 11}, {X: -6, Y: 12}, {X: -6, Y: 13}}
	// watchingTheRoad is where the cleric stands to watch: in sight of the
	// road and the captain, beyond the gate's approach distance.
	watchingTheRoad = spatial.Position{X: -7, Y: 13}
	// besideTheHailCell is a cell beside where the hail stops alice, in the
	// room she stands in.
	besideTheHailCell = spatial.Position{X: -7, Y: 11}
)

// captainParty is the captain as an act names him.
var captainParty = session.Party{Kind: session.PartyMember, ID: "captain"}

// theFrontGateAddress is the captain addressing alice on the front gate.
var theFrontGateAddress = &session.Address{Guard: "captain", Interaction: "front-gate"}

// castleWithApproach is the castle with the front gate's approach at within:
// the castle's own `approach: { within: 2 }`, its distance replaced.
func castleWithApproach(t *testing.T, within int) []byte {
	t.Helper()
	raw := string(castleSource(t))
	const authored = "approach: { within: 2 }"
	require.Equal(t, 1, strings.Count(raw, authored), "the castle authors the front gate's approach once")
	return []byte(strings.Replace(raw, authored, fmt.Sprintf("approach: { within: %d }", within), 1))
}

// openTheHailingCastle is the castle opened for alice and the cleric, the
// cleric watching the road.
func openTheHailingCastle(t *testing.T) *freeRoamCastle {
	t.Helper()
	c := openTheCastleFor(t, []*character.Data{armedFighter("alice"), healingCleric()}, 18)
	c.placeInTheRecord(t, "cleric", watchingTheRoad)
	return c
}

// walkUpToTheGate is alice's walk from the party start to the cell beside
// the gate, played.
func (c *freeRoamCastle) walkUpToTheGate(t *testing.T) ([]spatial.Position, *session.MoveOutput, beat) {
	t.Helper()
	path := c.walkInto(t, besideTheCastleGate)
	require.Contains(t, path, theHailCell, "precondition: the road passes the hail cell")
	var out *session.MoveOutput
	played := c.play(t, "alice walks up to the gate", func() error {
		var err error
		out, err = c.mgr.Move(context.Background(), &session.MoveInput{Session: "sess", Member: "alice", Path: path})
		return err
	})
	return path, out, played
}

// speakRow is the member's one world-clock Speak row.
func speakRow(t *testing.T, out *session.AffordOutput) session.Declaration {
	t.Helper()
	var rows []session.Declaration
	for _, d := range out.Declarations {
		if d.Verb == session.VerbSpeak {
			rows = append(rows, d)
		}
	}
	require.Len(t, rows, 1, "one Speak row on the world clock")
	return rows[0]
}

// frontGateProgress is the front gate's progress as the record holds it.
func frontGateProgress(t *testing.T, enc *encounter.Encounter) string {
	t.Helper()
	for _, in := range enc.ToData().Interactions {
		if in.ID == "front-gate" {
			raw, err := json.Marshal(in.Blob)
			require.NoError(t, err)
			return string(raw)
		}
	}
	require.FailNow(t, "the run holds no front gate")
	return ""
}

// TestCastleTheHailStopsTheWalk: alice walks up the road toward the gate.
// On the cell where the captain sees her come inside the gate's approach,
// he hails her: the Move answers stopped with Addressed naming him and the
// front gate, Steps ends on that cell, the cells after it were never
// walked, and she stands there. His hail line is under her Move's act.
func TestCastleTheHailStopsTheWalk(t *testing.T) {
	c := openTheHailingCastle(t)

	path, out, played := c.walkUpToTheGate(t)
	require.Equal(t, session.MovementStopped, out.Status, "the hail stops the walk")
	require.Equal(t, theFrontGateAddress, out.Addressed, "the captain addresses her on the front gate")
	require.Nil(t, out.Barred, "a hail is not a bump")
	require.Nil(t, out.Formed, "a hail is not a fight")
	require.NotEmpty(t, out.Steps)
	require.Equal(t, theHailCell, out.Steps[len(out.Steps)-1].Position, "the last step taken is the hail cell")
	require.Less(t, len(out.Steps), len(path), "the rest of the walk is dropped")
	for _, s := range out.Steps {
		require.NotEqual(t, besideTheCastleGate, s.Position, "the walk never reached the gate")
	}
	require.Equal(t, theHailCell, c.where(t, "alice"), "she stands where the hail stopped her")

	hails := where[session.TableAnswer](played.to("alice"), func(a session.TableAnswer) bool { return a.Who == captainParty && a.Say != "" })
	require.Len(t, hails, 1, "the captain says one line")
	require.Equal(t, theCaptainsHail, hails[0].body.Say, "and it is his hail")
	require.Equal(t, "move", hails[0].act.Verb, "under her Move")
	require.Equal(t, aliceParty, hails[0].body.To, "and spoken to her")

	// NOTHING RESUMES: the dropped cells are not walked by a later read.
	c.bar(t, "alice")
	require.Equal(t, theHailCell, c.where(t, "alice"), "still where the hail stopped her")
}

// TestCastleTheAddressRidesTheSpeakRow: once hailed, alice's own Speak row
// carries the address, and lists the captain's lines; it is free roam, no
// fight, nobody active. The cleric's Speak row carries none, and no other
// verb's row does. Two reads answer the same bytes.
func TestCastleTheAddressRidesTheSpeakRow(t *testing.T) {
	c := openTheHailingCastle(t)
	_, out, _ := c.walkUpToTheGate(t)
	require.NotNil(t, out.Addressed, "precondition: she was hailed")

	bar := c.bar(t, "alice")
	require.Equal(t, session.ClockWorld, bar.Clock, "free roam: the world clock, nobody active")
	speak := speakRow(t, bar)
	require.Equal(t, theFrontGateAddress, speak.Address, "her Speak row carries the address")
	require.True(t, speak.Available, "and is available: %+v", speak.Why)
	var hisLines []string
	for _, row := range speak.SpeakOptions {
		if row.Target == "captain" {
			hisLines = append(hisLines, row.OptionID)
		}
	}
	require.Contains(t, hisLines, "guest", "the row lists the captain's front-gate claim")
	for _, d := range bar.Declarations {
		if d.Verb != session.VerbSpeak {
			require.Nil(t, d.Address, "%s carries no address", d.Verb)
		}
	}

	require.Nil(t, speakRow(t, c.bar(t, "cleric")).Address, "the cleric is not addressed")

	first, err := json.Marshal(c.bar(t, "alice"))
	require.NoError(t, err)
	second, err := json.Marshal(c.bar(t, "alice"))
	require.NoError(t, err)
	require.Equal(t, string(first), string(second), "two reads answer the same bytes")
}

// TestCastleTheAddressSurvivesAReload: the address is the run's, saved with
// it. A second Manager over the same repositories, the host reloading the
// session, still answers it on alice's Speak row.
func TestCastleTheAddressSurvivesAReload(t *testing.T) {
	c := openTheHailingCastle(t)
	_, out, _ := c.walkUpToTheGate(t)
	require.NotNil(t, out.Addressed, "precondition: she was hailed")

	reloaded, err := c.manager(t).Afford(context.Background(), &session.AffordInput{Session: "sess", Member: "alice"})
	require.NoError(t, err)
	require.Equal(t, theFrontGateAddress, speakRow(t, reloaded).Address, "the reloaded session still addresses her")
}

// TestCastleSteppingOutOfReachEndsTheAddress: alice walks back down the road
// past Speak reach of the captain. The walk completes (an address ending
// stops nothing), the address is gone from her Speak row, and the front
// gate's progress is what it was. Walking back up, she is hailed again.
func TestCastleSteppingOutOfReachEndsTheAddress(t *testing.T) {
	c := openTheHailingCastle(t)
	_, out, _ := c.walkUpToTheGate(t)
	require.NotNil(t, out.Addressed, "precondition: she was hailed")
	progress := frontGateProgress(t, c.world(t))

	// ONE CELL BACK IS STILL SPEAK REACH, though past the approach: the
	// address is measured at the reach Speak is, so it stands.
	near := c.move(t, backDownTheRoad[:1])
	require.Nil(t, near.Addressed, "no second hail while hers stands")
	enc := c.world(t)
	stepped := enc.Distance(c.where(t, "alice"), memberOf(t, enc, "captain").Position)
	require.Greater(t, stepped, float64(2), "precondition: past the approach distance")
	require.LessOrEqual(t, stepped, float64(session.InteractionReach), "precondition: within Speak reach")
	require.Equal(t, theFrontGateAddress, speakRow(t, c.bar(t, "alice")).Address, "inside Speak reach the address stands")

	away := c.move(t, backDownTheRoad[1:])
	require.Equal(t, session.MovementCompleted, away.Status, "walking away is not stopped: %s", away.StopReason)
	require.Nil(t, away.Addressed, "nobody hails her walking away")
	enc = c.world(t)
	require.Greater(t, enc.Distance(c.where(t, "alice"), memberOf(t, enc, "captain").Position), float64(session.InteractionReach),
		"precondition: she stands beyond Speak reach of the captain")
	require.Nil(t, speakRow(t, c.bar(t, "alice")).Address, "the address ended")
	require.Equal(t, progress, frontGateProgress(t, c.world(t)), "the front gate's progress is untouched")

	back := c.move(t, []spatial.Position{backDownTheRoad[1], backDownTheRoad[0], theHailCell})
	require.Equal(t, theFrontGateAddress, back.Addressed, "coming back, she is hailed again")
	require.Equal(t, theFrontGateAddress, speakRow(t, c.bar(t, "alice")).Address)
}

// TestCastleABystanderListensIn: the cleric watches the road from down
// below the gate. She is not addressed, and she stands a room beyond the
// gate where the captain hails alice and alice answers him, so she sees each
// of them speak and reads none of their words (rpg-project#933: a spoken line
// reaches the speaker's room and whom it is said to). Alice, hailed, reads
// his hail whole.
func TestCastleABystanderListensIn(t *testing.T) {
	c := openTheHailingCastle(t)
	_, out, hail := c.walkUpToTheGate(t)
	require.NotNil(t, out.Addressed, "precondition: she was hailed")

	seen := where[session.TableAnswer](hail.to("cleric"), func(a session.TableAnswer) bool {
		return a.Who == captainParty && a.Key == "behavior/6:policy/approached"
	})
	require.Len(t, seen, 1, "the cleric sees the captain answer alice's approach")
	require.Empty(t, seen[0].body.Say, "out of earshot, she reads none of his words")
	require.Equal(t, session.Party{}, seen[0].body.To, "nor whom they were said to")
	heard := where[session.TableAnswer](hail.to("alice"), func(a session.TableAnswer) bool { return a.Who == captainParty && a.Say != "" })
	require.Len(t, heard, 1, "alice, whom he hails, reads his words")
	require.Equal(t, theCaptainsHail, heard[0].body.Say)
	require.Equal(t, aliceParty, heard[0].body.To)

	greeted := c.play(t, "alice greets the captain", func() error {
		_, err := speakAsOffered(context.Background(), c.mgr, &session.SpeakInput{Session: "sess", Member: "alice", Target: "captain", OptionID: "greet"})
		return err
	})
	require.Empty(t, where[session.SpeakAct](greeted.to("cleric"), func(session.SpeakAct) bool { return true }),
		"the cleric reads no line of alice's: she was seen to speak, out of earshot")
	var versions []witnessed
	for _, b := range beatsNamed(storyBeats(t, c.mgr, "sess", "cleric"), encounter.BeatSpeech) {
		if b.Fields["speaker"] == "alice" {
			versions = append(versions, b)
		}
	}
	require.Len(t, versions, 1, "her story holds her version of alice's line")

	// ACROSS HER WHOLE STORY, not only that version: no beat of any kind
	// tells her the words said at the gate, which line alice said, its
	// check, how it went or what it taught.
	lines := where[session.SpeakAct](greeted.to("alice"), func(s session.SpeakAct) bool { return s.By == aliceParty })
	require.Len(t, lines, 1, "precondition: alice's own act of her line")
	said := []string{theCaptainsHail, lines[0].body.Line}
	if lines[0].body.Answer != nil && lines[0].body.Answer.Say != "" {
		said = append(said, lines[0].body.Answer.Say)
	}
	for _, b := range storyBeats(t, c.mgr, "sess", "cleric") {
		for _, key := range []string{"words", "say", "line"} {
			if w, ok := b.Fields[key].(string); ok {
				require.NotContains(t, said, w, "seq %d %s: she is told words said at the gate", b.Seq, b.Beat)
			}
		}
		require.NotEqual(t, "greet", b.Fields["option"], "seq %d %s: she is told which line it was", b.Seq, b.Beat)
		if b.Fields["speaker"] == "alice" || b.Fields["actor"] == "alice" || b.Beat == encounter.BeatAnswered {
			for _, key := range []string{"check", "beaten", "fact"} {
				require.NotContains(t, b.Fields, key, "seq %d %s: she is told the line's %s", b.Seq, b.Beat, key)
			}
			key, _ := b.Fields["key"].(string)
			require.NotContains(t, []string{string(encounter.AnswerSuccess), string(encounter.AnswerFailure)}, key,
				"seq %d %s: she is told how the line went", b.Seq, b.Beat)
		}
	}
	for _, a := range where[session.TableAnswer](greeted.to("cleric"), func(a session.TableAnswer) bool { return a.Who == captainParty }) {
		require.Empty(t, a.body.Say, "her acts carry none of his words")
		require.Empty(t, a.body.Option, "nor the line he answered")
		require.NotContains(t, []string{string(encounter.AnswerSuccess), string(encounter.AnswerFailure)}, a.body.Key, "nor how it went")
	}
}

// TestCastleACompanionBesideHerHearsEveryWord (rpg-project#933): the cleric
// stands beside alice when she greets the captain, in the same room, so she
// reads alice's line whole — whom it was said to included — and the
// captain's answer to it with its words, as alice does.
func TestCastleACompanionBesideHerHearsEveryWord(t *testing.T) {
	c := openTheCastleFor(t, []*character.Data{armedFighter("alice"), healingCleric()}, 18)
	c.placeInTheRecord(t, "cleric", besideTheHailCell)
	c.walkUpToTheGate(t)
	greeted := c.play(t, "alice greets the captain", func() error {
		_, err := speakAsOffered(context.Background(), c.mgr, &session.SpeakInput{Session: "sess", Member: "alice", Target: "captain", OptionID: "greet"})
		return err
	})
	for _, who := range []string{"alice", "cleric"} {
		lines := where[session.SpeakAct](greeted.to(who), func(s session.SpeakAct) bool { return s.By == aliceParty })
		require.Len(t, lines, 1, "%s reads alice's line", who)
		line := lines[0].body
		require.Equal(t, captainParty, line.To, "%s sees whom it was said to", who)
		require.Equal(t, "greet", line.Option)
		require.NotEmpty(t, line.Line)
		require.NotNil(t, line.Answer, "%s reads the captain's answer", who)
		require.NotEmpty(t, line.Answer.Say, "%s caught his words", who)
		require.Equal(t, aliceParty, line.Answer.To, "said to alice")
		require.Equal(t, "greet", line.Answer.Option, "%s is told the line he answered", who)
		require.Equal(t, string(encounter.AnswerSuccess), line.Answer.Key, "%s is told how it went: a plain line lands", who)
	}
}

// TestLaunchRefusesAnApproachBeyondSpeakReach: an interaction whose approach
// is farther than [session.InteractionReach] is refused at launch, so a
// hailed character can always answer from where the hail stopped her. An
// approach of exactly the reach launches.
func TestLaunchRefusesAnApproachBeyondSpeakReach(t *testing.T) {
	launch := func(within int) error {
		compiled, err := dungeonspec.Load(castleWithApproach(t, within))
		require.NoError(t, err)
		mgr, err := session.NewManager(&session.Config{Record: session.DiscardRecord{}, Acts: &actCapture{}, Seats: newFakeSeats(), CharacterConfig: newFakeCharacterConfig(),
			PresentationIDs: testPresentationIDs{}, Dice: &castleDice{d20s: []int{18}}, TurnDriver: session.Driver(),
			Sessions: newFakeSessions(), Encounters: newFakeEncounters(), Characters: newFakeCharacters(armedFighter("alice")),
		})
		require.NoError(t, err)
		_, err = mgr.Launch(context.Background(), &session.LaunchInput{Session: "sess", Dungeon: &compiled, Party: []string{"alice"}})
		return err
	}

	err := launch(session.InteractionReach + 1)
	require.ErrorIs(t, err, session.ErrInvalidWorld, "an approach beyond Speak reach is refused")
	require.ErrorContains(t, err, "front-gate", "and the refusal names the interaction")
	require.NoError(t, launch(session.InteractionReach), "an approach at Speak reach launches")
}

// walkOn is a walk clicked again each time a hail stops it short
// (rpg-project#923): the castle's gate hails a party walking up the road, and
// a test about anything else walks on through it, as a player does. A walk
// already addressed draws no second hail, so the walk on completes or stops
// for its own reason. The answer is the last Move's, with every step taken.
func walkOn(ctx context.Context, mgr *session.Manager, in *session.MoveInput) (*session.MoveOutput, error) {
	var steps []session.Step
	path := in.Path
	for {
		out, err := mgr.Move(ctx, &session.MoveInput{Session: in.Session, Member: in.Member, DeclarationID: in.DeclarationID, Path: path})
		if err != nil {
			return nil, err
		}
		steps = append(steps, out.Steps...)
		if out.Addressed == nil || out.Status != session.MovementStopped || len(out.Steps) == 0 {
			out.Steps = steps
			return out, nil
		}
		path = path[len(out.Steps):]
	}
}
