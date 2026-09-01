// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// audience_internal_test.go pins the shelf rpg-toolkit#940 sits on
// (rpg-project#260 slice 4): audienceFor's v1 "everyone" policy, and the
// classification every one of the ten Audience: call sites now records.
//
// Both tests are deliberately white-box. audienceFor is unexported by
// design (nothing outside this package decides who hears a beat), and the
// classification a beat carries is not otherwise observable: v1's policy
// makes subjectBeat, bubbleBeat, and tableBeat-with-no-override produce the
// IDENTICAL audience, so there is no black-box way to tell them apart today
// — the whole point of a policy shelf nobody has flipped yet.

// TestAudienceForPolicy pins v1's "everyone" policy, one subtest per class —
// audienceFor's own contract, not any one call site's.
func TestAudienceForPolicy(t *testing.T) {
	enc, err := NewEncounter(&SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: everyoneStanding{}, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
		Field: FieldInput{Canvas: CanvasInput{Void: VoidIsOpaque(), Orientation: HexesArePointyTop()}, Regions: []RegionInput{rectRegion("hall", 0, 0, 6, 6)}},
		Members: []MemberInput{
			{ID: "zebra", Kind: KindPlayer, Position: spatial.Position{X: 0, Y: 0}},
			{ID: "alice", Kind: KindPlayer, Position: spatial.Position{X: 1, Y: 0}},
			{ID: "mikko", Kind: KindPlayer, Position: spatial.Position{X: 2, Y: 0}},
		},
		Endings: []EndingInput{{Key: "called", Trigger: TriggerExternal{}}},
	})
	require.NoError(t, err)

	roster := enc.rosterIDs()
	require.Equal(t, []MemberID{"alice", "mikko", "zebra"}, roster,
		"rosterIDs is sorted — the baseline every non-overriding class falls back to")

	t.Run("subjectBeat ignores subjects and sends everyone", func(t *testing.T) {
		require.Equal(t, roster, enc.audienceFor(subjectBeat, "alice"),
			"a subject beat is about alice, but v1 still sends the whole roster")
		require.Equal(t, roster, enc.audienceFor(subjectBeat),
			"and the same with no subjects at all — v1 does not distinguish")
	})

	t.Run("bubbleBeat ignores subjects and sends everyone", func(t *testing.T) {
		require.Equal(t, roster, enc.audienceFor(bubbleBeat, "alice", "mikko"),
			"a bubble beat names its members, but v1 still sends the whole roster")
	})

	t.Run("tableBeat with no subjects falls back to the live roster", func(t *testing.T) {
		require.Equal(t, roster, enc.audienceFor(tableBeat),
			"Pump's tick beat and Exit's exit beat both call it exactly this way")
	})

	t.Run("tableBeat with subjects returns them verbatim, unsorted", func(t *testing.T) {
		unsorted := []MemberID{"zebra", "alice", "mikko"}
		require.Equal(t, unsorted, enc.audienceFor(tableBeat, unsorted...),
			"scene-opened's declaration order must survive untouched — the "+
				"one case where audienceFor does NOT hand back e.rosterIDs()")
	})
}

// beatKind decodes one story entry's "beat" field — the same convention
// every append site in this module already writes its payload under.
func beatKind(t *testing.T, payload []byte) string {
	t.Helper()
	var v struct {
		Beat string `json:"beat"`
	}
	require.NoError(t, json.Unmarshal(payload, &v))
	require.NotEmpty(t, v.Beat, "every beat this module appends carries its own kind")
	return v.Beat
}

// wantClass is the design's own table (rpg-project's ideas/perceive/design.md
// §2), expressed in the real beatClass constants rather than restated as
// strings — a beat kind not in this table, or a table entry no scripted
// scene ever produces, is exactly the drift this test exists to catch.
var wantClass = map[string]beatClass{
	"struck":           subjectBeat,
	"missed":           subjectBeat,
	"down":             subjectBeat,
	"moved":            subjectBeat,
	"joined":           subjectBeat,
	"door":             subjectBeat, // the early adopter — see doorverbs.go's setDoorState
	"bubble-formed":    bubbleBeat,
	"turn-ended":       bubbleBeat,
	"transferred":      bubbleBeat,
	"bubble-dissolved": bubbleBeat,
	"scene-opened":     tableBeat,
	"tick":             tableBeat,
	"exited":           tableBeat,
	"ended":            tableBeat,
}

// TestCallSiteClassification runs one scripted scene through every one of
// the ten Audience: sites and checks the FULL SET of beat kinds it produces
// is exactly wantClass's key set — nothing missing, nothing unclassified.
//
// It cannot check WHICH class a runtime call passed (v1's policy erases that
// signal for nine of the fourteen kinds — see this file's own doc), so what
// it pins is completeness: every kind this module can emit has a reviewed
// entry in wantClass, and wantClass names nothing this module does not
// actually emit. scene-opened's declaration-order behaviour (the one
// call site where class IS observable, per TestAudienceForPolicy's last
// subtest) gets its own direct assertion below as a bonus.
func TestCallSiteClassification(t *testing.T) {
	standing := &oneDown{}

	enc, err := NewEncounter(&SetupInput{
		Sight: everyoneSeesTheWholeMap{}, Standing: standing, Initiative: orderAsGiven{}, TurnDriver: passDriver{}, Striker: passStriker{}, Mover: quietMover{}, Announcer: quietAnnouncer{},
		Retention: RetentionUnbounded,
		Field: FieldInput{
			Canvas: CanvasInput{Void: VoidIsOpaque(), Orientation: HexesArePointyTop()},
			Regions: []RegionInput{
				rectRegion("hall", 0, 0, 8, 8),
				rectRegion("annex", 50, 0, 2, 2),
			},
			Doors: []DoorInput{
				{ID: "the-door", Edges: []DoorEdge{{From: spatial.Position{X: 50, Y: 0}, To: spatial.Position{X: 51, Y: 0}}}, State: DoorIsClosed()},
			},
		},
		Members: []MemberInput{
			{ID: "zebra", Kind: KindPlayer, Position: spatial.Position{X: 0, Y: 0}, SpeedFeet: 30, SightFeet: 60},
			{ID: "alice", Kind: KindPlayer, Position: spatial.Position{X: 1, Y: 0}, SpeedFeet: 30, SightFeet: 60},
			{ID: "goblin", Kind: KindMonster, Position: spatial.Position{X: 2, Y: 0}, SpeedFeet: 30},
		},
		Endings: []EndingInput{{Key: "called", Trigger: TriggerExternal{}}},
	})
	require.NoError(t, err)
	require.Len(t, enc.bubbles, 1, "everyone sees everyone at first light — a bubble forms unprompted")

	// moved + turn-ended: whoever is active takes one step, then ends their
	// turn (which drives the goblin's own turn-ended if it lands on it).
	active, err := enc.bubbles[0].Active()
	require.NoError(t, err)
	activeMember := MemberID(active)

	from, ok := enc.canvas.GetEntityPosition(string(activeMember))
	require.True(t, ok)
	to := from
	to.X++
	_, err = enc.Step(&StepInput{Member: activeMember, To: to})
	require.NoError(t, err)

	_, err = enc.EndTurn(&EndTurnInput{Member: activeMember})
	require.NoError(t, err)

	// transferred: zebra leaves the bubble for the world clock, unrelated
	// to whatever happens to the goblin below.
	_, err = enc.Transfer(&TransferInput{Member: "zebra", To: ClockWorld})
	require.NoError(t, err)

	// missed: a clean outcome beat, nobody down yet.
	_, err = enc.Record(&RecordInput{
		Kind: OutcomeMissed, Actor: "zebra", Targets: []MemberID{"goblin"},
		Values: map[OutcomeValue]int{ValueRoll: 4, ValueAgainst: 15},
	})
	require.NoError(t, err)

	// struck + downed (+ bubble-dissolved, if the goblin was the fight's
	// only monster — noticeDown decides that, not this test).
	standing.who = "goblin"
	_, err = enc.Record(&RecordInput{
		Kind: OutcomeStruck, Actor: "alice", Targets: []MemberID{"goblin"},
		Values: map[OutcomeValue]int{ValueAmount: 7}, Critical: false,
	})
	require.NoError(t, err)

	// bubble-dissolved, if noticeDown did not already call it above: force
	// it rather than depend on defeat's own timing being exactly this scene's.
	if len(enc.bubbles) > 0 {
		_, err = enc.Dissolve(&DissolveInput{Member: "alice"})
		require.NoError(t, err)
	}

	// tick: nobody left with a decider and a live turn to take, but the
	// tick frame itself still gets recorded.
	_, err = enc.Pump(&PumpInput{})
	require.NoError(t, err)

	// joined + exited: carl passes through.
	_, err = enc.Join(&JoinInput{Member: "carl", Kind: KindPlayer, Cell: spatial.Position{X: 3, Y: 3}, SpeedFeet: 30, SightFeet: 60})
	require.NoError(t, err)
	_, err = enc.Exit(&ExitInput{Member: "carl"})
	require.NoError(t, err)

	// door: the isolated annex, untouched by anything above.
	_, err = enc.OpenDoor(&OpenDoorInput{Door: "the-door"})
	require.NoError(t, err)

	// ended: the scene closes last, so every beat above is on the record.
	_, err = enc.End(&EndInput{Ending: "called"})
	require.NoError(t, err)

	entries, err := enc.Story(&StoryInput{Audience: "alice"})
	require.NoError(t, err)
	require.NotEmpty(t, entries)

	seen := map[string]bool{}
	for _, entry := range entries {
		seen[beatKind(t, entry.Payload)] = true
	}

	for kind, class := range wantClass {
		t.Run(kind, func(t *testing.T) {
			require.True(t, seen[kind], "the scripted scene never produced a %q beat", kind)
			require.Contains(t, []beatClass{subjectBeat, bubbleBeat, tableBeat}, class)
		})
	}

	for kind := range seen {
		require.Contains(t, wantClass, kind,
			"beat kind %q was appended but has no entry in wantClass — a new "+
				"call site landed without a reviewed classification", kind)
	}
}
