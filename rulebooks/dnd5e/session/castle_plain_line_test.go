// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

// castle_plain_line_test.go is rpg-project#785 R5 on the castle, in free
// roam: "the check is the action… speaking with no check would be a free
// action". Alice walks up to the captain and greets him (a plain line): no
// round of the world passes and nothing is lent her sheet. Then she tells him
// to stand aside (Intimidation): the world is paid its round.

import (
	"context"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
)

func TestCastleGreetingTheCaptainIsFreeAndAThreatPaysTheRound(t *testing.T) {
	ctx := context.Background()
	c := openTheCastleForTwo(t, 18)

	c.play(t, "alice walks up to the captain", func() error {
		enc, err := c.mgr.LoadWorldForTest(ctx, "sess")
		if err != nil {
			return err
		}
		route, err := enc.Route(encounter.RouteInput{Mover: "alice", Policy: encounter.MoveToward, Anchor: standingAt(enc, "captain"), Budget: 200})
		if err != nil {
			return err
		}
		_, err = walkOn(ctx, c.mgr, &session.MoveInput{Session: "sess", Member: "alice", Path: route.Path})
		return err
	})

	// ONE ROW ON THE WORLD CLOCK, both lines on it: the world clock has no
	// slot to split by, and the round is the composition's to charge.
	out := c.bar(t, "alice")
	require.Equal(t, session.ClockWorld, out.Clock, "precondition: free roam")
	var speaks []session.Declaration
	for _, d := range out.Declarations {
		if d.Verb == session.VerbSpeak {
			speaks = append(speaks, d)
		}
	}
	require.Len(t, speaks, 1, "one Speak row in free roam")
	assert.Equal(t, session.SlotNone, speaks[0].Slot)
	var captainLines []string
	for _, o := range speaks[0].SpeakOptions {
		if o.Target == "captain" {
			captainLines = append(captainLines, o.OptionID)
		}
	}
	assert.True(t, slices.Contains(captainLines, "greet"), "the captain's greeting is offered: %v", captainLines)
	assert.True(t, slices.Contains(captainLines, "stand-aside"), "and the threat: %v", captainLines)

	ticksIn := func(b beat) int {
		return len(where[session.ClockEntry](b.to("alice"), func(e session.ClockEntry) bool { return e.Mark == session.ClockTick }))
	}

	greet := c.play(t, "alice greets the captain", func() error {
		_, err := c.mgr.Speak(ctx, &session.SpeakInput{Session: "sess", Member: "alice", Target: "captain", OptionID: "greet"})
		return err
	})
	line := only[session.SpeakAct](t, greet.to("alice"))
	assert.Equal(t, "greet", line.body.Option)
	assert.Empty(t, line.body.Skill, "a plain line names no skill")
	assert.Nil(t, line.body.Roll, "and rolled nothing, so the act carries no roll")
	require.NotNil(t, line.body.Answer, "the captain answers a greeting")
	assert.Zero(t, ticksIn(greet), "a plain line pays the world no round")
	assert.Nil(t, storedData(t, c.characters, "alice").ActionEconomy, "nothing was lent her sheet")

	threat := c.play(t, "alice tells the captain to stand aside", func() error {
		_, err := c.mgr.Speak(ctx, &session.SpeakInput{Session: "sess", Member: "alice", Target: "captain", OptionID: "stand-aside"})
		return err
	})
	told := only[session.SpeakAct](t, threat.to("alice"))
	assert.Equal(t, "intimidation", told.body.Skill)
	assert.NotNil(t, told.body.Roll, "the threat rolled its check")
	assert.Equal(t, 1, ticksIn(threat), "a checked line is the action, and the world is paid its round")
	assert.Nil(t, storedData(t, c.characters, "alice").ActionEconomy, "the speaker's sheet is still out of combat")
}
