// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

// castle_gate_hail_test.go is THE GUARD OPENS THE CONVERSATION at the session
// (rpg-project#923, Gate S3 of #769): a carrier of the front gate who sees a
// character come inside its approach distance hails her. The session drops
// the rest of her walk, puts the address on her own Speak row and nobody
// else's, and the hail reaches whoever witnesses it as the guard's answer.
//
// The castle the encounter module ships carries no approach yet, so these
// tests open it with the hail written in ([hailingCastleEdits]): the front
// gate's approach and the captain's hail line. The captain is the gate's
// nearest carrier to a party walking up the road, so he is the one who hails.

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

// theCaptainsHail is the captain's hail line on the hailing castle.
const theCaptainsHail = "Hold up there, friend. What brings you to the gate?"

// hailingCastleEdits are the castle's own text with the hail added: the
// front gate's approach (its distance a %d), and the captain's
// `on: approached` row and line. Each edit names text the castle must hold
// exactly once.
var hailingCastleEdits = [][2]string{
	{"interactions:\n  front-gate:\n    recipe:\n", "interactions:\n  front-gate:\n    approach: { within: %d }\n    recipe:\n"},
	{
		"      ref: site:behaviors:captain-policy\n      initial: ready\n      states:\n        ready:\n          transitions:\n",
		"      ref: site:behaviors:captain-policy\n      initial: ready\n      states:\n        ready:\n          transitions:\n            - on: approached\n              do:\n                - pick: approached\n",
	},
	{
		"    table:\n      time:\n        # Hand edit (smarter monsters S2",
		"    table:\n      approached:\n        - say: " + theCaptainsHail + "\n      time:\n        # Hand edit (smarter monsters S2",
	},
}

// The hail on the road to the gate: the cell alice is hailed on walking up
// from the party start toward the gate, and the cells back down the road,
// the last of them beyond Speak reach of the captain.
var (
	theHailCell     = spatial.Position{X: -6, Y: 10}
	backDownTheRoad = []spatial.Position{{X: -6, Y: 11}, {X: -6, Y: 12}, {X: -6, Y: 13}}
	// watchingTheRoad is where the cleric stands to watch: in sight of the
	// road and the captain, beyond the gate's approach distance.
	watchingTheRoad = spatial.Position{X: -7, Y: 13}
)

// captainParty is the captain as an act names him.
var captainParty = session.Party{Kind: session.PartyMember, ID: "captain"}

// theFrontGateAddress is the captain addressing alice on the front gate.
var theFrontGateAddress = &session.Address{Guard: "captain", Interaction: "front-gate"}

// hailingCastleSource is the castle with [hailingCastleEdits] applied and the
// front gate's approach at within.
func hailingCastleSource(t *testing.T, within int) []byte {
	t.Helper()
	raw := string(castleSource(t))
	for i, edit := range hailingCastleEdits {
		require.Equal(t, 1, strings.Count(raw, edit[0]), "the castle holds the edited text once: %q", edit[0])
		with := edit[1]
		if i == 0 {
			with = fmt.Sprintf(with, within)
		}
		raw = strings.Replace(raw, edit[0], with, 1)
	}
	return []byte(raw)
}

// openTheHailingCastle is the hailing castle opened for alice and the cleric,
// the cleric watching the road.
func openTheHailingCastle(t *testing.T) *freeRoamCastle {
	t.Helper()
	c := openTheCastleFrom(t, hailingCastleSource(t, 2), []*character.Data{armedFighter("alice"), healingCleric()}, 18)
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
		out = c.move(t, path)
		return nil
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

// TestCastleABystanderListensIn: the cleric watches the road. She is not
// addressed, but she reads the captain's hail, and then alice's line to
// him, in her own acts.
func TestCastleABystanderListensIn(t *testing.T) {
	c := openTheHailingCastle(t)
	_, out, hail := c.walkUpToTheGate(t)
	require.NotNil(t, out.Addressed, "precondition: she was hailed")

	heard := where[session.TableAnswer](hail.to("cleric"), func(a session.TableAnswer) bool { return a.Who == captainParty && a.Say != "" })
	require.Len(t, heard, 1, "the cleric reads the captain's hail")
	require.Equal(t, theCaptainsHail, heard[0].body.Say)

	greeted := c.play(t, "alice greets the captain", func() error {
		_, err := c.mgr.Speak(context.Background(), &session.SpeakInput{Session: "sess", Member: "alice", Target: "captain", OptionID: "greet"})
		return err
	})
	spoken := where[session.SpeakAct](greeted.to("cleric"), func(s session.SpeakAct) bool { return s.By == aliceParty && s.To == captainParty })
	require.Len(t, spoken, 1, "the cleric reads alice's line to the captain")
	require.Equal(t, "greet", spoken[0].body.Option)
}

// TestLaunchRefusesAnApproachBeyondSpeakReach: an interaction whose approach
// is farther than [session.InteractionReach] is refused at launch, so a
// hailed character can always answer from where the hail stopped her. An
// approach of exactly the reach launches.
func TestLaunchRefusesAnApproachBeyondSpeakReach(t *testing.T) {
	launch := func(within int) error {
		compiled, err := dungeonspec.Load(hailingCastleSource(t, within))
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
