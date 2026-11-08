// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package replaytest_test

// replay_test.go is REPLAY FROM THE RECORD (rpg-project#766 S5, design §5,
// R6): play a run on the host's crypto die, then open the same content
// again from round 0 with the record's own rolls as the die, re-issue the
// verbs the record names in the order it names them, and require the log
// the replay lands to be the run's.
//
// THREE CLAIMS, each its own failure:
//
//  1. Every world roll is on the record: the faces the run's die threw are
//     exactly [replaytest.RollsOf] the run's log, in order.
//  2. The verbs are on the record: the replay re-issues only what it reads
//     from the log ([verbsOf]), never the script that played the run.
//  3. The replay lands the run: the same beats, the same rolls, the same
//     picks, the same state beats, entry for entry — and it reads back every
//     roll the record holds, no more and no fewer.
//
// THE PROOF'S SCOPE, collaborator by collaborator. The run is played on a
// host that rolls: the world's die (crypto), and the host's own initiative
// and driven swings on a crypto die of their own. The replay's host:
//
//   - World's die ([replaytest.RecordRoller]): the record's `tempered` and
//     `answered` rolls. Claim 1 proves they are every roll it threw.
//   - Initiative (recordInitiative): each fight's order from its
//     `bubble-formed` beat. The record carries the order, not the dice.
//   - Driven swing (recordStriker): each act-tagged `struck`/`missed`, kind
//     and amount. The record carries the outcome, not the d20 or the d6.
//   - Who is down (standing): the `down` beats after a player's outcome.
//     The run's driven swings fell nobody, so a fall the HOST scores for a
//     driven swing is not exercised.
//   - Stealth (recordStealth, record_check_test.go): each sneak roll's
//     `snuck` beat, DC, total and verdict. The record carries the verdict,
//     not the d20. Any other check is unasked, and refused if asked.
//   - Witness (perceivesNothingAsked): outside the record, and unasked.
//     These routes open no concealed door, and it refuses if asked.
//   - Sight, sheets, equipment, conditions, mover and announcer: the host's
//     static facts and listeners, the same content both times.
//   - Not on the record at all: Speak's reach, the host's constant. A
//     sneak declaration is: an accepted one writes a `sneaking` beat
//     (sneaking R10 as amended by R11, rpg-project#768), and a refused one
//     changes nothing, so a replay that never re-issues it lands the same
//     log.
//
// IDS ARE MINTED, NOT ROLLED. A record id (and the correlation and act tag
// built from one) is a ULID off the wall clock; the replay mints its own.
// The comparison holds them to a bijection — the n-th id the run minted is
// the n-th the replay minted, everywhere it appears — which is "the same
// beats" without asking a clock to repeat itself.

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/play/record"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/replaytest"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// verb is one player verb the record names, ready to re-issue.
type verb struct {
	// seq is the beat the verb was read from.
	seq uint64
	// name says what it is, for a failure message and the roller.
	name  string
	issue func(*world) error
}

// verbBody is the fields of a player verb's beat the decoder reads.
type verbBody struct {
	Beat     string             `json:"beat"`
	Member   string             `json:"member"`
	Mover    string             `json:"mover"`
	Cell     *spatial.Position  `json:"cell"`
	Verb     string             `json:"verb"`
	Position *spatial.Position  `json:"position"`
	Actor    string             `json:"actor"`
	Target   string             `json:"target"`
	Prop     string             `json:"prop"`
	Option   string             `json:"option"`
	Skill    string             `json:"skill"`
	Beaten   bool               `json:"beaten"`
	DC       int                `json:"dc"`
	Total    int                `json:"total"`
	Door     string             `json:"door"`
	State    string             `json:"state"`
	Targets  []string           `json:"targets"`
	Amount   *int               `json:"amount"`
	Critical bool               `json:"critical"`
	Attack   *attackOnTheRecord `json:"attack"`
	Calc     *json.RawMessage   `json:"calculation"`
	On       *bool              `json:"on"`
}

// verbHeader reads only routing identity before a record is known to be a
// player's command. Other kinds own their fields: a diagnostic's string "on"
// is not the boolean mode of a sneaking command. All records still participate
// in requireSameLog; this only controls which inputs the replay re-issues.
func verbHeader(t *testing.T, entry record.EntryData) verbBody {
	t.Helper()
	var header struct {
		Beat   string `json:"beat"`
		Member string `json:"member"`
		Actor  string `json:"actor"`
		Mover  string `json:"mover"`
	}
	require.NoError(t, json.Unmarshal(entry.Payload, &header), "beat seq %d", entry.Seq)
	return verbBody{Beat: header.Beat, Member: header.Member, Actor: header.Actor, Mover: header.Mover}
}

// attackOnTheRecord is an outcome beat's attack identity.
type attackOnTheRecord struct {
	Ref        string `json:"ref"`
	Name       string `json:"name"`
	DamageType string `json:"damage_type"`
}

// verbsOf reads the players' verbs off a log, in log order. A beat is a
// player's verb when a player wrote it and no driven act carries it (a
// driven act's beats are tagged `act`):
//
//   - `moved` of a player is a Step to the cell it names;
//   - `spoke` by a player is a Speak with the verdict, DC and total it names;
//   - `door` by a player is the door verb that left the door in that state;
//   - a player's own outcome (`struck`, `missed`) is a RecordTrain unit, and
//     each `down` beat that follows it is a fall the host scored first;
//   - `barred` of a player is the attempt a body stopped: a Step to the
//     cell it names, or for a door the verb it names (rpg-project#842);
//   - `turn-ended` of a player is an EndTurn;
//   - `sneaking` of a player is a SetSneaking to the mode it names.
//
// A player's beat of any other kind is a verb this decoder cannot re-issue,
// and fails the test by name: the record is then missing what the replay
// needs, which is this slice's finding to report, never to guess.
func verbsOf(t *testing.T, entries []record.EntryData, players map[encounter.MemberID]bool) []verb {
	t.Helper()
	var out []verb
	for i, entry := range entries {
		if _, driven := entry.Tags["act"]; driven {
			continue
		}
		b := verbHeader(t, entry)
		if !isVerbBeat(b, players) {
			continue
		}
		require.NoError(t, json.Unmarshal(entry.Payload, &b), "beat seq %d", entry.Seq)
		member, actor := encounter.MemberID(b.Member), encounter.MemberID(b.Actor)
		switch {
		case b.Beat == encounter.BeatMoved && players[member]:
			require.NotNil(t, b.Position, "moved beat seq %d names a cell", entry.Seq)
			to := *b.Position
			out = append(out, verb{entry.Seq, fmt.Sprintf("Step %s to %v", member, to), func(w *world) error {
				return w.step(member, to)
			}})
		case b.Beat == encounter.BeatSpoke && players[actor]:
			require.Nil(t, b.Calc, "spoke beat seq %d: a calculation the replay does not re-issue yet", entry.Seq)
			in := encounter.SpeakInput{
				Actor: actor, Target: encounter.MemberID(b.Target), Prop: encounter.PropID(b.Prop),
				OptionID: b.Option, Skill: b.Skill, Beaten: b.Beaten, DC: b.DC, Total: b.Total,
			}
			out = append(out, verb{entry.Seq, fmt.Sprintf("Speak %s %s/%q", actor, b.Target, b.Option), func(w *world) error {
				return w.speak(in)
			}})
		case b.Beat == encounter.BeatDoor && players[actor]:
			require.Equal(t, string(encounter.DoorOpen), b.State,
				"door beat seq %d: only an opened door is re-issued; %q needs its verb read here", entry.Seq, b.State)
			door := encounter.DoorID(b.Door)
			out = append(out, verb{entry.Seq, fmt.Sprintf("OpenDoor %s by %s", door, actor), func(w *world) error {
				return w.openDoor(door, actor)
			}})
		case (b.Beat == string(encounter.OutcomeStruck) || b.Beat == string(encounter.OutcomeMissed)) && players[actor]:
			outcome := outcomeOf(t, entry.Seq, b)
			falls := fallsAfter(t, entries[i+1:], players)
			out = append(out, verb{entry.Seq, fmt.Sprintf("RecordTrain %s %s %v (falls %v)", actor, b.Beat, b.Targets, falls),
				func(w *world) error { return w.strike(outcome, falls) }})
		case b.Beat == encounter.BeatBarred && players[encounter.MemberID(b.Mover)]:
			mover := encounter.MemberID(b.Mover)
			if b.Door == "" {
				require.NotNil(t, b.Cell, "barred beat seq %d names a cell", entry.Seq)
				to := *b.Cell
				out = append(out, verb{entry.Seq, fmt.Sprintf("Step %s to %v (barred)", mover, to), func(w *world) error {
					return w.step(mover, to)
				}})
				continue
			}
			require.Equal(t, "open", b.Verb,
				"barred beat seq %d: only a barred open is re-issued; %q needs its verb read here", entry.Seq, b.Verb)
			door := encounter.DoorID(b.Door)
			out = append(out, verb{entry.Seq, fmt.Sprintf("OpenDoor %s by %s (barred)", door, mover), func(w *world) error {
				return w.openDoor(door, mover)
			}})
		case b.Beat == encounter.BeatTurnEnded && players[member]:
			out = append(out, verb{entry.Seq, fmt.Sprintf("EndTurn %s", member), func(w *world) error {
				return w.endTurn(member)
			}})
		case b.Beat == encounter.BeatSneaking && players[member]:
			require.NotNil(t, b.On, "sneaking beat seq %d names the mode", entry.Seq)
			on := *b.On
			out = append(out, verb{entry.Seq, fmt.Sprintf("SetSneaking %s %v", member, on), func(w *world) error {
				return w.sneak(member, on)
			}})
		case b.Beat == encounter.BeatMoved, b.Beat == encounter.BeatSpoke, b.Beat == encounter.BeatDoor,
			b.Beat == encounter.BeatTurnEnded:
			// a monster's, outside any driven act: the world's, not a verb
		}
	}

	return out
}

// outcomeOf is a player's outcome beat as the RecordInput that wrote it.
func outcomeOf(t *testing.T, seq uint64, b verbBody) encounter.RecordInput {
	t.Helper()
	in := encounter.RecordInput{Kind: encounter.OutcomeKind(b.Beat), Actor: encounter.MemberID(b.Actor), Critical: b.Critical}
	for _, id := range b.Targets {
		in.Targets = append(in.Targets, encounter.MemberID(id))
	}
	if b.Attack != nil {
		in.Attack = &encounter.AttackIdentity{Ref: b.Attack.Ref, Name: b.Attack.Name, DamageType: b.Attack.DamageType}
	}
	if b.Amount != nil {
		in.Values = map[encounter.OutcomeValue]int{encounter.ValueAmount: *b.Amount}
	}
	require.NotEmpty(t, in.Targets, "outcome beat seq %d names its targets", seq)

	return in
}

// fallsAfter is every member a `down` beat puts down between an outcome and
// the next player verb's beat: the falls the host scored for that outcome.
func fallsAfter(t *testing.T, rest []record.EntryData, players map[encounter.MemberID]bool) []encounter.MemberID {
	t.Helper()
	var out []encounter.MemberID
	for _, entry := range rest {
		if _, driven := entry.Tags["act"]; driven {
			continue
		}
		b := verbHeader(t, entry)
		if b.Beat == string(encounter.OutcomeDown) {
			out = append(out, encounter.MemberID(b.Member))
			continue
		}
		if isVerbBeat(b, players) {
			break
		}
	}

	return out
}

// isVerbBeat is whether a beat is one [verbsOf] reads as a player's verb.
func isVerbBeat(b verbBody, players map[encounter.MemberID]bool) bool {
	switch b.Beat {
	case encounter.BeatMoved, encounter.BeatTurnEnded, encounter.BeatSneaking:
		return players[encounter.MemberID(b.Member)]
	case encounter.BeatSpoke, encounter.BeatDoor, string(encounter.OutcomeStruck), string(encounter.OutcomeMissed):
		return players[encounter.MemberID(b.Actor)]
	case encounter.BeatBarred:
		return players[encounter.MemberID(b.Mover)]
	}

	return false
}

// playersOf is the players a setup seats.
func playersOf(t testing.TB, open opener) map[encounter.MemberID]bool {
	t.Helper()
	out := map[encounter.MemberID]bool{}
	for _, m := range open(t, rollingHost(newCountingDie()), &standing{}).Members {
		if m.Kind == encounter.KindPlayer {
			out[m.ID] = true
		}
	}

	return out
}

// replayed is what a replay landed: the log, and the roller it read.
type replayed struct {
	log    []record.EntryData
	roller *replaytest.RecordRoller
	host   host
}

// replay opens the content again from round 0 on the record's own rolls and
// re-issues the record's verbs in order.
func replay(t *testing.T, open opener, original []record.EntryData) replayed {
	t.Helper()
	roller, err := replaytest.NewRecordRoller(original)
	require.NoError(t, err)
	roller.During("Setup")
	h := recordHost(t, roller, original)
	w := openWorld(t, open, h)
	for _, v := range verbsOf(t, original, playersOf(t, open)) {
		roller.During(fmt.Sprintf("%s (beat seq %d)", v.name, v.seq))
		require.NoError(t, v.issue(w), "the replay re-issues %s (beat seq %d)", v.name, v.seq)
	}

	return replayed{log: w.enc.ToData().Log.Entries, roller: roller, host: h}
}

// requireEveryRollOnTheRecord holds the faces the run's die threw against
// the rolls its record carries: claim 1.
func requireEveryRollOnTheRecord(t *testing.T, die *countingDie, log []record.EntryData) {
	t.Helper()
	rolls, err := replaytest.RollsOf(log)
	require.NoError(t, err)
	onRecord := make([]thrown, 0, len(rolls))
	for _, r := range rolls {
		onRecord = append(onRecord, thrown{r.Face, r.Of})
	}
	threw := die.thrownFaces()
	for i := range min(len(threw), len(onRecord)) {
		require.Equal(t, threw[i], onRecord[i], "world roll %d: the die threw %v; the record's is %s", i, threw[i], rolls[i])
	}
	require.Len(t, onRecord, len(threw), "every world roll the die threw is on the record, and nothing else is")
	require.NotEmpty(t, threw, "the run rolled the world's die")
}

// requireSameLog holds the replay's log to the run's, entry for entry, ids
// held to a bijection: claim 3.
func requireSameLog(t *testing.T, want, got []record.EntryData) {
	t.Helper()
	wantLines, gotLines := canonical(want), canonical(got)
	for i := range min(len(wantLines), len(gotLines)) {
		if wantLines[i] != gotLines[i] {
			require.Failf(t, "the replay left the record",
				"entry %d\n run:    %s\n replay: %s", i, wantLines[i], gotLines[i])
		}
	}
	require.Len(t, gotLines, len(wantLines), "the replay lands as many beats as the run")
}

// ulid is a record id as play/record mints one.
var ulid = regexp.MustCompile(`[0-9A-HJKMNP-TV-Z]{26}`)

// canonical is each entry as one line — seq, round, correlation, audience,
// tags, payload — with every minted id replaced by the order it first
// appeared in, across the whole log.
func canonical(entries []record.EntryData) []string {
	ids := map[string]string{}
	name := func(s string) string {
		return ulid.ReplaceAllStringFunc(s, func(id string) string {
			n, ok := ids[id]
			if !ok {
				n = fmt.Sprintf("id#%d", len(ids))
				ids[id] = n
			}
			return n
		})
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		tags, _ := json.Marshal(e.Tags)
		audience, _ := json.Marshal(e.Audience)
		out = append(out, strings.Join([]string{
			fmt.Sprint(e.Seq), name(e.ID), fmt.Sprint(e.At), name(e.Correlation), string(audience), name(string(tags)), name(string(e.Payload)),
		}, " | "))
	}

	return out
}

// replayRun is the whole test of one run: play it on the crypto die, check
// its rolls are on its record, replay it from the record, and require the
// same log with every recorded roll read back.
func replayRun(t *testing.T, open opener, play func(w *world)) []record.EntryData {
	t.Helper()

	return replayRunUntil(t, open, play, 1, nil)
}

// replayRunUntil is [replayRun] of the first of up to tries played runs
// whose log carries what the test needs (nil: any run), for a claim the
// host's crypto die can deny one run: a driven hit, say.
func replayRunUntil(
	t *testing.T, open opener, play func(w *world), tries int, carries func([]record.EntryData) bool,
) []record.EntryData {
	t.Helper()
	var die *countingDie
	var original []record.EntryData
	for try := 0; try < tries; try++ {
		die = newCountingDie()
		run := openWorld(t, open, rollingHost(die))
		play(run)
		original = run.enc.ToData().Log.Entries
		if carries == nil || carries(original) {
			break
		}
		require.Less(t, try, tries-1, "none of %d played runs carried what the test needs", tries)
	}

	requireEveryRollOnTheRecord(t, die, original)
	got := replay(t, open, original)
	requireSameLog(t, original, got.log)
	require.Zero(t, got.roller.Remaining(), "the replay reads back every roll the record holds")
	swings, fights, sneaks := got.host.remaining()
	require.Zero(t, swings, "the replay lands every driven swing the record holds")
	require.Zero(t, fights, "the replay forms every fight the record holds")
	require.Zero(t, sneaks, "the replay rolls every sneak the record holds")

	return original
}

// countBeats is how many beats of a kind a log holds.
func countBeats(t *testing.T, log []record.EntryData, kind string) int {
	t.Helper()
	n := 0
	for _, e := range log {
		if beatOf(t, e.Payload) == kind {
			n++
		}
	}

	return n
}

// countDrivenSwings is how many driven swings a log holds: the act-tagged
// outcomes the host's striker landed.
func countDrivenSwings(t *testing.T, log []record.EntryData) int {
	t.Helper()

	return countDriven(t, log, encounter.OutcomeStruck) + countDriven(t, log, encounter.OutcomeMissed)
}

// countDriven is how many driven swings of one outcome a log holds.
func countDriven(t testing.TB, log []record.EntryData, kind encounter.OutcomeKind) int {
	t.Helper()
	n := 0
	for _, e := range log {
		if strings.HasPrefix(e.Tags["act"], "attack:") && beatOf(t, e.Payload) == string(kind) {
			n++
		}
	}

	return n
}

// drivenHitsTries is how many fights the fight route may play before one
// lands a driven hit: each swing hits on an 11 or better, so a run of the
// route's swings missing every time is rare, and eight such runs rarer.
const drivenHitsTries = 8

// TestReplay_CastleGateScript replays the QA castle's script
// (castletest's castle_king_script_test.go) from its record: the routes
// through the gate that the record can name every verb of.
//
// The script's two sneaking pieces (4, 5) are [TestReplay_CastleSneakPieces].
func TestReplay_CastleGateScript(t *testing.T) {
	t.Parallel()

	// The troupe route, pieces 1, 2, 3 and 7's civil half: from the party
	// start the hero paces while the gate patrol walks, hears the stranger
	// out on a lost check, talks the troupe past the captain on a won one,
	// opens the gate on the far room (two arrivals, a temper dealt at each
	// door), and paces through it past the posted guard while the far room
	// walks up.
	t.Run("the troupe through the gate", func(t *testing.T) {
		t.Parallel()
		start := loadFile(t, kingFixture).PartyStart[0].At
		log := replayRun(t, castleOpener(start), func(w *world) {
			from := w.at(hero)
			w.pace(hero, from, spatial.Position{X: from.X - 1, Y: from.Y}, 4)
			w.walkTo(hero, w.at(kingPedlar))
			require.NoError(t, w.speak(encounter.SpeakInput{
				Actor: hero, Target: kingPedlar, OptionID: "tonight", Skill: "persuasion", Beaten: false, DC: 15, Total: 3,
			}))
			w.walkTo(hero, w.at(kingCaptain))
			require.NoError(t, w.speak(encounter.SpeakInput{
				Actor: hero, Target: kingCaptain, OptionID: "troupe", Skill: "performance", Beaten: true, DC: 15, Total: 20,
			}))
			// gate-1 stays posted beneath the gate (rpg-project#857): the
			// troupe opens it from the opening's other cell and paces
			// through it by its pass, in past him and back out.
			w.walkTo(hero, besideTheGate)
			if w.at(hero) != besideTheGate {
				require.NoError(t, w.step(hero, besideTheGate))
			}
			require.NoError(t, w.openDoor(kingGateDoor, hero))
			w.pace(hero, besideTheGate, castleCell(0, 6), 3)
			require.False(t, w.fighting(hero), "the troupe was let through: nobody fights")
		})
		require.Equal(t, 2, countBeats(t, log, encounter.BeatArrived), "precondition: the far room arrived")
		require.Equal(t, 2, countBeats(t, log, encounter.BeatSpoke), "precondition: two lines said")
		require.Positive(t, countBeats(t, log, encounter.BeatTempered), "precondition: tempers dealt")
	})

	// The fight, pieces 6 and 7's hostile half: the hero spears a gate
	// guard, the gate shouts the alarm, three rounds of driven turns pass,
	// the hero drops the gate's three, opens the gate on a far room that
	// comes hostile, and paces while it comes on.
	t.Run("the fight at the gate", func(t *testing.T) {
		t.Parallel()
		spear := &encounter.AttackIdentity{Ref: kingSpear.String(), Name: "Spear", DamageType: "piercing"}
		struck := func(target encounter.MemberID) encounter.RecordInput {
			return encounter.RecordInput{
				Kind: encounter.OutcomeStruck, Actor: hero, Targets: []encounter.MemberID{target}, Attack: spear,
				Values: map[encounter.OutcomeValue]int{encounter.ValueAmount: 6},
			}
		}
		// A RUN WITH A DRIVEN HIT (rpg-toolkit#2229 review): a run whose
		// swings all missed would pass a replay that read back no swing's
		// kind or amount, so the route plays until one hit lands.
		hit := func(log []record.EntryData) bool { return countDriven(t, log, encounter.OutcomeStruck) > 0 }
		log := replayRunUntil(t, castleOpener(gatehouseSeat), func(w *world) {
			require.NoError(t, w.strike(struck(kingGate1), nil))
			require.True(t, w.fighting(hero), "the fight forms at the gate")
			for i := 0; i < 3; i++ {
				require.NoError(t, w.endTurn(hero))
			}
			for _, id := range []encounter.MemberID{kingGate1, kingGate2, kingCaptain} {
				require.NoError(t, w.strike(struck(id), []encounter.MemberID{id}))
			}
			require.False(t, w.fighting(hero), "the gate's three are down and the fight is over")
			w.walkTo(hero, beneathTheGate)
			if w.at(hero) != beneathTheGate {
				require.NoError(t, w.step(hero, beneathTheGate))
			}
			require.NoError(t, w.openDoor(kingGateDoor, hero))
			w.pace(hero, beneathTheGate, besideTheGate, 3)
		}, drivenHitsTries, hit)
		require.Equal(t, 3, countBeats(t, log, string(encounter.OutcomeDown)), "precondition: three fell")
		require.Positive(t, countDrivenSwings(t, log), "precondition: driven attacks swung")
		require.Positive(t, countDriven(t, log, encounter.OutcomeStruck), "precondition: a driven swing hit")
		require.Positive(t, countBeats(t, log, encounter.BeatFightStarted), "precondition: a fight formed on the host's initiative")
		require.Equal(t, 2, countBeats(t, log, encounter.BeatArrived), "precondition: the far room arrived")
	})
}

// TestReplay_GarrisonTwentyRounds replays twenty world rounds of a walker
// pacing beside the party start of the garrison keep: every patrol's pick
// each round, every post's, every state beat, from the record alone. Both
// keeps of rpg-project#766 S4: the posts on the default recipe, and the
// posts asleep until a sound lands near them.
//
// UNDER -race, THE POSTS ASLEEP FOR FOUR ROUNDS ([garrisonRounds]), about
// 30 s there. Twenty rounds of both keeps cost about 340 s under -race, and
// the encounter CI job ran 9m45s against its 10m budget with them in
// (rpg-toolkit#2229 review). Plain `go test` replays both keeps for twenty
// rounds, about 17 s and 21 s. The castle's mutants (an unrecorded roll, a
// wrong face read back, a fall or an EndTurn not read back, a driven swing
// or a fight's order not read back) fail under either.
func TestReplay_GarrisonTwentyRounds(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		listen bool
	}{{"default", false}, {"posts-asleep", true}} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if raceDetector && !tc.listen {
				t.Skip("the default keep's replay is ~190 s under -race; plain go test runs it")
			}
			open := garrisonOpener(tc.listen)
			log := replayRun(t, open, func(w *world) {
				start := w.at(garrisonWalker)
				w.pace(garrisonWalker, start, besideFloor(t, w, garrisonWalker, start), garrisonRounds())
				require.GreaterOrEqual(t, w.round(), garrisonRounds(), "the world rounds ran")
			})
			require.Positive(t, countBeats(t, log, encounter.BeatAnswered), "precondition: the garrison picked")
			require.Positive(t, countBeats(t, log, encounter.BeatTick), "precondition: the world clock ran")
		})
	}
}

// garrisonRounds is the twenty world rounds the garrison is replayed for,
// or under -race the four a race-detecting CI can carry beside the encounter
// package's own budget.
func garrisonRounds() int {
	if raceDetector {
		return 4
	}

	return 20
}

// besideFloor is the first hex neighbour of a cell the member steps onto
// and back from. The probe steps are verbs of the run like any other: the
// record names them and the replay re-issues them.
func besideFloor(t *testing.T, w *world, member encounter.MemberID, from spatial.Position) spatial.Position {
	t.Helper()
	for _, d := range []spatial.Position{{X: 1, Y: 0}, {X: -1, Y: 0}, {X: 0, Y: 1}, {X: 0, Y: -1}, {X: 1, Y: -1}, {X: -1, Y: 1}} {
		cell := spatial.Position{X: from.X + d.X, Y: from.Y + d.Y}
		if w.enc.Distance(from, cell) != 1 {
			continue
		}
		if err := w.step(member, cell); err != nil {
			continue
		}
		require.NoError(t, w.step(member, from))
		return cell
	}
	require.FailNowf(t, "no floor to pace", "beside %v", from)

	return spatial.Position{}
}
