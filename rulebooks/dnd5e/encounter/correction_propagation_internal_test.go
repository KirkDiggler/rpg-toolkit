// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/play/intel"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

const (
	propagationActive  MemberID = "active-player"
	propagationGoblin  MemberID = "driven-goblin"
	propagationSubject MemberID = "remembered-player"
	propagationCaller  MemberID = "world-player"
	propagationDoor    DoorID   = "propagation-door"
)

var propagationArrival = spatial.Position{X: 2, Y: 1}

type propagationDriver struct {
	intents []TurnIntent
	calls   int
}

func (d *propagationDriver) Act(MonsterView) (TurnIntent, error) {
	i := d.calls
	d.calls++
	if i < len(d.intents) {
		return d.intents[i], nil
	}
	return Pass{}, nil
}

type propagationSight struct{}

func (propagationSight) Sight(members []MemberID) (map[MemberID]int, error) {
	out := make(map[MemberID]int, len(members))
	for _, id := range members {
		out[id] = 0
	}
	return out, nil
}

type propagationStanding struct {
	down []MemberID
}

func (s *propagationStanding) Standing(members []MemberID) ([]MemberID, error) {
	asked := make(map[MemberID]bool, len(members))
	for _, id := range members {
		asked[id] = true
	}

	var out []MemberID
	for _, id := range s.down {
		if asked[id] {
			out = append(out, id)
		}
	}
	return out, nil
}

// newCorrectionPropagationEncounter builds the one real driven-arrival state
// every propagation surface below needs. Setup first creates lawful holdings
// without forming a fight by giving every seed member the same kind; the saved
// roster is then assigned its runtime kinds and the goblin's current sight
// testimony is changed into held testimony at the arrival cell before load.
func newCorrectionPropagationEncounter(
	t *testing.T,
	withBubble bool,
) (*Encounter, *propagationStanding) {
	t.Helper()

	base, err := NewEncounter(&SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{},
		TurnDriver: passDriver{}, Striker: passStriker{}, Announcer: quietAnnouncer{},
		Field: FieldInput{
			Canvas:  openAir(),
			Regions: []RegionInput{rectRegion("propagation-field", 0, 0, 10, 10)},
			Doors: []DoorInput{{
				ID: propagationDoor,
				Edges: []DoorEdge{{
					From: spatial.Position{X: 4, Y: 4},
					To:   spatial.Position{X: 5, Y: 4},
				}},
				State: DoorIsClosed(),
			}},
		},
		Members: []MemberInput{
			{ID: propagationActive, Kind: KindMonster, Position: spatial.Position{X: 0, Y: 1}},
			{ID: propagationGoblin, Kind: KindMonster, Position: spatial.Position{X: 1, Y: 1}, SpeedFeet: 30},
			{ID: propagationSubject, Kind: KindMonster, Position: spatial.Position{X: 8, Y: 8}},
			{ID: propagationCaller, Kind: KindMonster, Position: spatial.Position{X: 0, Y: 3}},
		},
		Endings: []EndingInput{{Key: "called", Trigger: TriggerExternal{}}},
	})
	require.NoError(t, err)

	data := base.ToData()
	for i := range data.Members {
		switch data.Members[i].ID {
		case propagationGoblin:
			data.Members[i].Kind = KindMonster
		default:
			data.Members[i].Kind = KindPlayer
		}
	}

	known, err := EncodeLocationPayload(LocationKnowledge{
		State: LocationKnown, Position: propagationArrival,
	})
	require.NoError(t, err)
	holding, present := data.Intel.Holdings[propagationGoblin][intel.Subject(propagationSubject)]
	require.True(t, present, "seed must give the driven goblin sight testimony")
	holding.Payload = known
	holding.CurrentVia = nil
	data.Intel.Holdings[propagationGoblin][intel.Subject(propagationSubject)] = holding

	standing := &propagationStanding{}
	driver := &propagationDriver{intents: []TurnIntent{
		Move{Path: []spatial.Position{propagationArrival}},
		Pass{},
	}}
	enc, err := LoadEncounter(&LoadEncounterInput{
		Data: data, Sight: propagationSight{}, Standing: standing, Initiative: orderAsGiven{},
		TurnDriver: driver, Striker: passStriker{}, Announcer: quietAnnouncer{},
	})
	require.NoError(t, err)

	if withBubble {
		_, err = enc.form(&FormInput{Order: []MemberID{
			propagationActive, propagationGoblin, propagationSubject,
		}})
		require.NoError(t, err)
		require.Zero(t, driver.calls, "the player starts the fixture bubble")
	}

	return enc, standing
}

func requireSurfacedPropagationCorrection(
	t *testing.T,
	enc *Encounter,
	deltas map[MemberID]*IntelDelta,
) {
	t.Helper()
	delta := deltas[propagationGoblin]
	require.NotNil(t, delta, "enclosing output must surface the nested driven correction")
	require.Contains(t, delta.Corrected, intel.Subject(propagationSubject))

	holdings, err := enc.View(&ViewInput{Member: propagationGoblin})
	require.NoError(t, err)
	for _, holding := range holdings {
		if holding.Subject != intel.Subject(propagationSubject) {
			continue
		}
		require.Equal(t, intel.Held, holding.Status)
		location, ok := DecodeLocationPayload(holding.Payload)
		require.True(t, ok)
		require.Equal(t, LocationUnknown, location.State)
		return
	}
	require.Fail(t, "corrected holding not found")
}

// TestDrivenArrivalCorrectionPropagationMatrix pins every distinct caller or
// output category that can enclose a driven turn. Removing one output merge
// must fail that row even though the underlying holding still mutates.
func TestDrivenArrivalCorrectionPropagationMatrix(t *testing.T) {
	tests := []struct {
		name       string
		withBubble bool
		noticeDown bool
		act        func(*testing.T, *Encounter) map[MemberID]*IntelDelta
	}{
		{
			name: "direct form",
			act: func(t *testing.T, enc *Encounter) map[MemberID]*IntelDelta {
				out, err := enc.form(&FormInput{Order: []MemberID{
					propagationGoblin, propagationActive, propagationSubject,
				}})
				require.NoError(t, err)
				return out.IntelDeltas
			},
		},
		{
			name:       "direct transfer",
			withBubble: true,
			act: func(t *testing.T, enc *Encounter) map[MemberID]*IntelDelta {
				out, err := enc.Transfer(&TransferInput{Member: propagationActive, To: ClockWorld})
				require.NoError(t, err)
				return out.IntelDeltas
			},
		},
		{
			name:       "step refresh",
			withBubble: true,
			noticeDown: true,
			act: func(t *testing.T, enc *Encounter) map[MemberID]*IntelDelta {
				out, err := enc.Step(&StepInput{
					Member: propagationCaller,
					To:     spatial.Position{X: 0, Y: 4},
				})
				require.NoError(t, err)
				return out.IntelDeltas
			},
		},
		{
			name:       "pump refresh",
			withBubble: true,
			noticeDown: true,
			act: func(t *testing.T, enc *Encounter) map[MemberID]*IntelDelta {
				out, err := enc.Pump(&PumpInput{})
				require.NoError(t, err)
				return out.IntelDeltas
			},
		},
		{
			name:       "join refresh",
			withBubble: true,
			noticeDown: true,
			act: func(t *testing.T, enc *Encounter) map[MemberID]*IntelDelta {
				out, err := enc.Join(&JoinInput{
					Member: "joining-player", Kind: KindPlayer,
					Cell: spatial.Position{X: 3, Y: 3},
				})
				require.NoError(t, err)
				return out.IntelDeltas
			},
		},
		{
			name:       "exit refresh",
			withBubble: true,
			noticeDown: true,
			act: func(t *testing.T, enc *Encounter) map[MemberID]*IntelDelta {
				out, err := enc.Exit(&ExitInput{Member: propagationCaller})
				require.NoError(t, err)
				return out.IntelDeltas
			},
		},
		{
			name:       "door refresh",
			withBubble: true,
			noticeDown: true,
			act: func(t *testing.T, enc *Encounter) map[MemberID]*IntelDelta {
				out, err := enc.OpenDoor(&OpenDoorInput{Door: propagationDoor, Actor: propagationCaller})
				require.NoError(t, err)
				return out.IntelDeltas
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			enc, standing := newCorrectionPropagationEncounter(t, test.withBubble)
			if test.noticeDown {
				standing.down = []MemberID{propagationActive}
			}

			deltas := test.act(t, enc)
			requireSurfacedPropagationCorrection(t, enc, deltas)
		})
	}
}
