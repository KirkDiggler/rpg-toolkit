// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package replaytest_test

// host_test.go is the castle's plain host (castletest's castle_king_test.go),
// copied because test helpers do not cross a package boundary, with ONE
// difference: the world's die is the host's to hand in. A run is played on
// a real die (crypto, unseeded: R6) and replayed on the record's own rolls.
//
// Everyone sees sixty feet and hears with a passive Perception of 10 unless
// the host names another; everyone walks thirty feet and swings a spear at
// five. A temper is a soldier whatever word was dealt.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/dice"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
)

// lowered is one fixture compiled once for the package.
type lowered struct {
	once     sync.Once
	compiled dungeonspec.Compiled
	err      error
}

var (
	loweringsMu sync.Mutex
	lowerings   = map[string]*lowered{}
)

// compiledOnce is the package's one lowering of a document under a key, with
// a door list of the caller's own (a host rewrites doors before it opens).
func compiledOnce(t testing.TB, key string, source func() ([]byte, error)) dungeonspec.Compiled {
	t.Helper()
	loweringsMu.Lock()
	l, ok := lowerings[key]
	if !ok {
		l = &lowered{}
		lowerings[key] = l
	}
	loweringsMu.Unlock()

	l.once.Do(func() {
		raw, err := source()
		if err != nil {
			l.err = err
			return
		}
		l.compiled, l.err = dungeonspec.Load(raw)
	})
	require.NoError(t, l.err, "%s compiles", key)

	compiled := l.compiled
	compiled.Field.Doors = slices.Clone(compiled.Field.Doors)

	return compiled
}

// loadFile is [compiledOnce] of a file on disk.
func loadFile(t testing.TB, path string) dungeonspec.Compiled {
	t.Helper()

	return compiledOnce(t, path, func() ([]byte, error) { return os.ReadFile(path) }) //nolint:gosec // a test reading its own fixture
}

// countingDie is the played run's die: crypto, unseeded, and every face it
// threw kept in order, so the test can hold the record's rolls against the
// rolls the world actually made.
type countingDie struct {
	mu    sync.Mutex
	inner dice.Roller
	faces []thrown
}

// thrown is one face a [countingDie] threw and the die's size.
type thrown struct{ face, of int }

func newCountingDie() *countingDie { return &countingDie{inner: dice.NewRoller()} }

func (d *countingDie) Roll(ctx context.Context, size int) (int, error) {
	face, err := d.inner.Roll(ctx, size)
	if err != nil {
		return 0, err
	}
	d.mu.Lock()
	d.faces = append(d.faces, thrown{face, size})
	d.mu.Unlock()

	return face, nil
}

func (d *countingDie) RollN(ctx context.Context, count, size int) ([]int, error) {
	out := make([]int, 0, count)
	for i := 0; i < count; i++ {
		face, err := d.Roll(ctx, size)
		if err != nil {
			return nil, err
		}
		out = append(out, face)
	}

	return out, nil
}

// thrownFaces is every face the die threw, in order.
func (d *countingDie) thrownFaces() []thrown {
	d.mu.Lock()
	defer d.mu.Unlock()

	return slices.Clone(d.faces)
}

// kingSpear is the one weapon everybody in this host swings.
var kingSpear = core.Ref{Module: "dnd5e", Type: "weapons", ID: "spear"}

// soldierProfiles completes a compiled temper the way a host does, with the
// profile every dealt word means — here, a soldier's for every word.
func soldierProfiles(temper encounter.Temper) encounter.Temper {
	if len(temper.Mix) == 0 {
		return temper
	}
	temper.Profiles = make(map[string]encounter.TemperProfile, len(temper.Mix))
	for word := range temper.Mix {
		temper.Profiles[word] = encounter.TemperProfile{}
	}

	return temper
}

// plainSight is a torchlit floor: everybody sees the host's feet.
type plainSight struct{ feet int }

func (s plainSight) Sight(members []encounter.MemberID) (map[encounter.MemberID]int, error) {
	out := make(map[encounter.MemberID]int, len(members))
	for _, id := range members {
		out[id] = encounter.CellsFromFeet(s.feet)
	}

	return out, nil
}

// plainSheets is everybody's sheet: thirty feet, a spear at five, a passive
// Perception of 10 unless passive names another, and the watch arc arcs
// names, if any.
type plainSheets struct {
	passive map[encounter.MemberID]int
	arcs    map[encounter.MemberID]int
}

func (c plainSheets) Sheets(members []encounter.MemberID) (map[encounter.MemberID]encounter.SheetFacts, error) {
	out := make(map[encounter.MemberID]encounter.SheetFacts, len(members))
	for _, id := range members {
		passive, ok := c.passive[id]
		if !ok {
			passive = 10
		}
		out[id] = encounter.SheetFacts{
			SpeedFeet:         30,
			Actions:           []encounter.ActionView{{Ref: kingSpear, RangeFeet: 5}},
			PassivePerception: passive,
			WatchArc:          c.arcs[id],
		}
	}

	return out, nil
}

// standing is the host's answer to who is standing: everybody, until a
// swing the host scored as a fall puts somebody down.
type standing struct{ down map[encounter.MemberID]bool }

func (c *standing) Standing(members []encounter.MemberID) ([]encounter.MemberID, error) {
	var out []encounter.MemberID
	for _, id := range members {
		if c.down[id] {
			out = append(out, id)
		}
	}

	return out, nil
}

func (c *standing) Assess(members []encounter.MemberID) (*encounter.ParticipationAssessment, error) {
	assessment := &encounter.ParticipationAssessment{}
	for _, id := range members {
		assessment.Members = append(assessment.Members, encounter.MemberParticipation{
			Member: id, Contact: !c.down[id], Conscious: !c.down[id], Down: c.down[id],
			Turn: encounter.TurnParticipationWait,
		})
	}

	return assessment, nil
}

// shuffledInitiative is the played run's initiative: the host's own roll,
// on the host's die (never the world's, so the world's rolls stay the
// record's `tempered` and `answered` beats alone). The order it lands is on
// the record as the `bubble-formed` beat's `order`.
type shuffledInitiative struct{ die dice.Roller }

func (s shuffledInitiative) RollInitiative(members []encounter.MemberID) ([]encounter.MemberID, error) {
	out := slices.Clone(members)
	for i := len(out) - 1; i > 0; i-- {
		j, err := s.die.Roll(context.Background(), i+1)
		if err != nil {
			return nil, err
		}
		out[i], out[j-1] = out[j-1], out[i]
	}

	return out, nil
}

// rollingStriker is the played run's driven swing: the host's d20 against
// an 11, a d6 on a hit, on the host's die. Nobody falls to it — who is
// down is the host's own scoring, and this host scores falls only for the
// player's outcomes. What it lands is on the record as the swing's
// act-tagged `struck` or `missed` beat.
type rollingStriker struct{ die dice.Roller }

func (s rollingStriker) Strike(
	ctx context.Context, enc *encounter.Encounter, attacker, target encounter.MemberID, action core.Ref,
) error {
	d20, err := s.die.Roll(ctx, 20)
	if err != nil {
		return err
	}
	outcome := encounter.RecordInput{
		Kind: encounter.OutcomeMissed, Actor: attacker, Targets: []encounter.MemberID{target},
		Attack: &encounter.AttackIdentity{Ref: action.String(), Name: "Spear", DamageType: "piercing"},
	}
	if d20 >= 11 {
		d6, err := s.die.Roll(ctx, 6)
		if err != nil {
			return err
		}
		outcome.Kind, outcome.Values = encounter.OutcomeStruck, map[encounter.OutcomeValue]int{encounter.ValueAmount: d6}
	}
	_, err = enc.RecordTrain(&encounter.RecordTrainInput{Units: []encounter.TrainUnit{{Outcome: &outcome}}})

	return err
}

// quietAnnouncer hears every boundary and does nothing.
type quietAnnouncer struct{}

func (quietAnnouncer) Announce(context.Context, *encounter.Encounter, []encounter.Boundary) error {
	return nil
}

// quietMover hears every step and does nothing.
type quietMover struct{}

func (quietMover) Move(context.Context, *encounter.Encounter, encounter.MoveStep) error {
	return nil
}

// errNotAsked is a host collaborator asked what these routes never ask.
var errNotAsked = errors.New("replaytest: this host is never asked that")

// errNotOnRecord is a replay's host asked for an answer the record does
// not hold.
var errNotOnRecord = errors.New("replaytest: the record holds no such answer")

// asksNoCheck is the check resolver of a route that rolls no check: asked
// for one, it refuses, so a check these routes make — and whether the
// record carries its verdict — could not pass unseen.
type asksNoCheck struct{}

func (asksNoCheck) ResolveCheck(in *encounter.ResolveCheckInput) (*encounter.ResolveCheckOutput, error) {
	return nil, fmt.Errorf("check for %s: %w", in.Member, errNotAsked)
}

func (asksNoCheck) ResolveDiscoveryCheck(in *encounter.ResolveCheckInput) (*encounter.ResolveCheckOutput, error) {
	return nil, fmt.Errorf("discovery check for %s: %w", in.Member, errNotAsked)
}

// perceivesNothingAsked is the witness of a route that opens no concealed
// door: asked who perceives one, it refuses, for [asksNoCheck]'s reason.
type perceivesNothingAsked struct{}

func (perceivesNothingAsked) Perceivers(*encounter.PerceiversInput) ([]encounter.MemberID, error) {
	return nil, fmt.Errorf("perceivers: %w", errNotAsked)
}

// beatOf is a log entry's beat kind.
func beatOf(t testing.TB, payload []byte) string {
	t.Helper()
	var b struct {
		Beat string `json:"beat"`
	}
	require.NoError(t, json.Unmarshal(payload, &b))

	return b.Beat
}
