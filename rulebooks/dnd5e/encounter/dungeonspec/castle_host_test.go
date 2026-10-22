// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec_test

// castle_host_test.go is the part of the castle's host the studio-site
// scenarios in this package stand on: the floor's frame, the castle's sheets
// and standing, the recorded striker, the table die, and the two readings a
// site asks of a run. The castle itself is played in ../castletest, which
// carries its own copy, because test helpers do not cross a package
// boundary.

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// castleCell is the dungeon-absolute cell of an offset [col, row] on the
// studio's floor: the frame the sites place their rooms in, and the frame
// [encounter.Encounter.Members] answers positions in.
func castleCell(col, row int) spatial.Position {
	return encounter.HexCellAt(encounter.HexesArePointyTop(), col, row)
}

// kingSpear is the one weapon everybody in this host swings.
var kingSpear = core.Ref{Module: "dnd5e", Type: "weapons", ID: "spear"}

// castleSpeedFeet is everybody's walking speed in this host.
const castleSpeedFeet = 30

// castleSheets is everybody's sheet in this host: thirty feet, a spear at
// five, a passive Perception of 10 unless passive names another, and the
// watch arc arcs names, if any.
type castleSheets struct {
	passive map[encounter.MemberID]int
	arcs    map[encounter.MemberID]int
}

func (c castleSheets) Sheets(members []encounter.MemberID) (map[encounter.MemberID]encounter.SheetFacts, error) {
	out := make(map[encounter.MemberID]encounter.SheetFacts, len(members))
	for _, id := range members {
		passive, ok := c.passive[id]
		if !ok {
			passive = 10
		}
		out[id] = encounter.SheetFacts{
			SpeedFeet:         castleSpeedFeet,
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

// cellsStrike is one swing a driven creature took.
type cellsStrike struct{ attacker, target encounter.MemberID }

// cellsStriker records every swing and lands it as a miss, so the scene is
// about who swings at whom and nobody falls.
type cellsStriker struct{ calls *[]cellsStrike }

func (s cellsStriker) Strike(
	_ context.Context, enc *encounter.Encounter, attacker, target encounter.MemberID, action core.Ref,
) error {
	*s.calls = append(*s.calls, cellsStrike{attacker, target})
	_, err := enc.RecordTrain(&encounter.RecordTrainInput{Units: []encounter.TrainUnit{{Outcome: &encounter.RecordInput{
		Kind: encounter.OutcomeMissed, Actor: attacker, Targets: []encounter.MemberID{target},
		Attack: &encounter.AttackIdentity{Ref: action.String(), Name: "Club", DamageType: "bludgeoning"},
	}}}})
	return err
}

// rollsOne is the table die: the first eligible row always fires.
type rollsOne struct{}

func (rollsOne) Roll(context.Context, int) (int, error) { return 1, nil }

func (rollsOne) RollN(_ context.Context, count, _ int) ([]int, error) {
	out := make([]int, count)
	for i := range out {
		out[i] = 1
	}
	return out, nil
}

// clockKind is the clock one member is on.
func clockKind(t testing.TB, enc *encounter.Encounter, id encounter.MemberID) encounter.ClockKind {
	t.Helper()
	on, err := enc.ClockOf(&encounter.ClockOfInput{Member: id})
	require.NoError(t, err)

	return on.Kind
}

// offArc is how far a bearing lies from a facing, 0 to 180.
func offArc(facing, bearing float64) float64 {
	off := bearing - facing
	for off > 180 {
		off -= 360
	}
	for off < -180 {
		off += 360
	}
	if off < 0 {
		off = -off
	}

	return off
}
