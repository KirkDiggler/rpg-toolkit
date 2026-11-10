// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter_test

// rebind_test.go is THE EQUIVALENCE GATE for the resident copy (rpg-project#879
// S6, rpg-project#927): a host that keeps one loaded encounter across verbs and
// rebinds it ([encounter.Encounter.Rebind]) must write, after every verb, the
// bytes a host that saved and reloaded the checkpoint before every verb writes,
// and answer each verb the same. Anything the instance keeps that the
// checkpoint does not, and that changes what a verb does, fails here by the
// verb it first differs at.

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// residentRun plays one sequence of verbs two ways: warm is one instance kept
// and rebound before every verb; the store is the checkpoint the reload side
// loads before every verb and saves after it.
type residentRun struct {
	t    *testing.T
	warm *encounter.Encounter
	// warmIDs is the id source the warm copy was loaded with. The id source
	// is not a capability (rebind.go): a kept copy keeps its own. A host's is
	// random, so a kept copy and a reload mint different ids and both are
	// right; this test's counts, so it is reset before every verb, where the
	// reload side's is fresh, and the two mint the same ids.
	warmIDs *fixedIDs
	// stored is the checkpoint as the store holds it: JSON.
	stored []byte
	since  uint64
	verbs  int
	failed int
	// quiet is how many verbs strike nobody, so the world clock runs before
	// a fight forms.
	quiet int
	// down is who the rulebook reports down, set between verbs so both sides
	// are asked the same question.
	down []encounter.MemberID
	// posts are the members whose machine listens (a garrison's posts), and
	// clanged says the scripted clang beside one has been played.
	posts   []encounter.MemberID
	clanged bool
}

// caps is a fresh set of capabilities for one verb: new values, so a kept
// instance that still asked the last verb's would be asking another one.
func (r *residentRun) caps() encounter.Capabilities {
	caps := checkpointCapabilities()
	caps.Participation = &downList{down: append([]encounter.MemberID(nil), r.down...)}
	caps.Sheets = hearsAndWalks{}
	return caps
}

// hearsAndWalks is [walksThirty] with ears: a passive Perception above every
// sound's DC, so a clang reaches whoever is in its range and a listening post
// hears it.
type hearsAndWalks struct{}

func (hearsAndWalks) Sheets(members []encounter.MemberID) (map[encounter.MemberID]encounter.SheetFacts, error) {
	out := make(map[encounter.MemberID]encounter.SheetFacts, len(members))
	for _, id := range members {
		out[id] = encounter.SheetFacts{SpeedFeet: 30, PassivePerception: 40}
	}
	return out, nil
}

func (r *residentRun) load() *encounter.Encounter {
	r.t.Helper()
	enc, _ := r.loadWith()
	return enc
}

func (r *residentRun) loadWith() (*encounter.Encounter, *fixedIDs) {
	r.t.Helper()
	ids := newFixedIDs()
	var data encounter.EncounterData
	require.NoError(r.t, json.Unmarshal(r.stored, &data))
	enc, err := encounter.LoadEncounter(&encounter.LoadEncounterInput{Data: data, Capabilities: r.caps(), IDs: ids})
	require.NoError(r.t, err)
	return enc, ids
}

// newResidentRun opens the compiled site with its party and every monster and
// stores it; the warm side starts from that store, as a host's first verb does.
func newResidentRun(t *testing.T, compiled dungeonspec.Compiled) *residentRun {
	t.Helper()
	members := seatedParty(compiled)
	require.GreaterOrEqual(t, len(members), checkpointParty)
	for _, m := range compiled.Monsters {
		members = append(members, encounter.MemberInput{
			ID: encounter.MemberID(m.MemberID), Kind: encounter.KindMonster, Position: m.At,
			Faction: encounter.FactionID(m.Faction), Table: m.Table, Options: m.Options,
			Arrives: m.Arrives, Temper: soldierTemper(m.Temper), Facing: m.FacingDegrees, Behaviors: m.Behaviors,
			Machine: m.Machine, BlockingStates: m.BlockingStates, Posts: m.Posts,
		})
	}
	r0 := &residentRun{t: t}
	enc, err := encounter.NewEncounter(&encounter.SetupInput{
		Field:        compiled.Field,
		Members:      members,
		Endings:      append([]encounter.EndingInput{{Key: "withdrawn", Trigger: encounter.TriggerExternal{}}}, compiled.Endings...),
		Capabilities: r0.caps(),
		IDs:          newFixedIDs(),
		Retention:    encounter.RetentionUnbounded,
	})
	require.NoError(t, err)
	data := enc.ToData()
	r := &residentRun{t: t, stored: mustJSON(t, data), since: max(data.Log.NextSeq, 1)}
	r.warm, r.warmIDs = r.loadWith()
	for _, m := range compiled.Monsters {
		if m.Behaviors == nil {
			continue
		}
		for _, b := range m.Behaviors.Instances {
			if ref := b.Instance.Recipe.Ref; ref.String() == listeningPost {
				r.posts = append(r.posts, encounter.MemberID(m.MemberID))
			}
		}
	}
	return r
}

// listeningPost is the garrison's listening recipe (keepgen PostsListen).
const listeningPost = "keepgen:behaviors:listening-post"

// clangBesideAPost is a struck outcome by the member nearest a listening post
// on the nearest walker: a clang the post hears, which wakes it through the
// listeners its machine's state set (wakebeats.go). The post is the first in
// file order; the one verb that has to reach the listeners.
func (r *residentRun) clangBesideAPost() (string, verb) {
	r.t.Helper()
	post := r.posts[0]
	at := whereIs(r.t, r.warm, post)
	members, err := r.warm.Members()
	require.NoError(r.t, err)
	var maker encounter.MemberID
	best := -1.0
	for _, m := range members {
		if m.ID == post {
			continue
		}
		if d := r.warm.Distance(at, m.Position); best < 0 || d < best {
			maker, best = m.ID, d
		}
	}
	require.NotEmpty(r.t, maker)
	return "clang by " + string(maker) + " beside " + string(post), strikeVerb(maker, keepPlayers[0])
}

// verb is one host verb: what it answered, or why it refused.
type verb func(enc *encounter.Encounter) (any, error)

// play runs v both ways and holds the two to each other: the same answer or
// the same refusal, then the same checkpoint (rest and appended segment). A
// refused verb is not saved and its warm copy is dropped, as a host does.
func (r *residentRun) play(name string, v verb) {
	r.t.Helper()
	r.verbs++
	at := fmt.Sprintf("verb %d (%s)", r.verbs, name)

	reload := r.load()
	require.NoError(r.t, r.warm.Rebind(&encounter.RebindInput{Capabilities: r.caps()}), at)
	*r.warmIDs = fixedIDs{}

	coldOut, coldErr := v(reload)
	warmOut, warmErr := v(r.warm)
	if coldErr != nil || warmErr != nil {
		require.Equal(r.t, errText(coldErr), errText(warmErr), "%s: the same refusal", at)
		r.t.Logf("%s refused: %v", at, coldErr)
		r.failed++
		r.warm, r.warmIDs = r.loadWith()
		return
	}
	sameBytes(r.t, mustJSON(r.t, coldOut), mustJSON(r.t, warmOut), at+": the same answer")

	cold, err := reload.ToCheckpoint(&encounter.ToCheckpointInput{Since: r.since})
	require.NoError(r.t, err)
	warm, err := r.warm.ToCheckpoint(&encounter.ToCheckpointInput{Since: r.since})
	require.NoError(r.t, err)
	sameBytes(r.t, mustJSON(r.t, cold.Appended), mustJSON(r.t, warm.Appended), at+": the same beats")
	sameBytes(r.t, mustJSON(r.t, cold.Rest), mustJSON(r.t, warm.Rest), at+": the same rest")

	r.stored = mustJSON(r.t, reload.ToData())
	r.since = cold.Rest.Log.NextSeq
}

// sameBytes holds two encodings equal (encoding/json sorts map keys, so equal
// values encode to equal bytes), and shows the difference as JSON when not.
func sameBytes(t *testing.T, want, got []byte, msg string) {
	t.Helper()
	if !bytes.Equal(want, got) {
		require.JSONEq(t, string(want), string(got), msg)
		require.Equal(t, string(want), string(got), msg)
	}
}

func errText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// walkVerb walks who along path inside one move act, the last step ending
// the walk, as the session's Move does.
func walkVerb(who encounter.MemberID, path []spatial.Position) verb {
	return func(enc *encounter.Encounter) (any, error) {
		if _, err := enc.OpenAct(&encounter.OpenActInput{Verb: "move"}); err != nil {
			return nil, err
		}
		defer func() { _ = enc.CloseAct() }()
		outs := make([]*encounter.StepOutput, 0, len(path))
		for i, to := range path {
			out, err := enc.Step(&encounter.StepInput{Member: who, To: to, EndWalk: i == len(path)-1})
			if err != nil {
				return outs, err
			}
			outs = append(outs, out)
		}
		return outs, nil
	}
}

// strikeVerb lands a struck outcome by who on target inside one attack act.
func strikeVerb(who, target encounter.MemberID) verb {
	return func(enc *encounter.Encounter) (any, error) {
		if _, err := enc.OpenAct(&encounter.OpenActInput{Verb: "attack"}); err != nil {
			return nil, err
		}
		defer func() { _ = enc.CloseAct() }()
		return enc.RecordTrain(&encounter.RecordTrainInput{Units: []encounter.TrainUnit{{Outcome: &encounter.RecordInput{
			Kind: encounter.OutcomeStruck, Actor: who, Targets: []encounter.MemberID{target},
		}}}})
	}
}

// endTurnVerb ends who's turn inside one end_turn act.
func endTurnVerb(who encounter.MemberID) verb {
	return func(enc *encounter.Encounter) (any, error) {
		if _, err := enc.OpenAct(&encounter.OpenActInput{Verb: "end_turn"}); err != nil {
			return nil, err
		}
		defer func() { _ = enc.CloseAct() }()
		return enc.EndTurn(&encounter.EndTurnInput{Member: who})
	}
}

// nearestMonster is the standing monster closest to who on the warm copy.
func (r *residentRun) nearestMonster(who encounter.MemberID) (encounter.MemberID, bool) {
	t, enc := r.t, r.warm
	t.Helper()
	fallen := make(map[encounter.MemberID]bool, len(r.down))
	for _, id := range r.down {
		fallen[id] = true
	}
	members, err := enc.Members()
	require.NoError(t, err)
	at := whereIs(t, enc, who)
	var best encounter.MemberID
	bestD := -1.0
	for _, m := range members {
		if m.Kind != encounter.KindMonster || fallen[m.ID] {
			continue
		}
		if d := enc.Distance(at, m.Position); bestD < 0 || d < bestD {
			best, bestD = m.ID, d
		}
	}
	return best, bestD >= 0
}

// nextVerb picks the next verb from rng and the warm copy's state (which the
// last verb held equal to the reload side's). In a fight it is the active
// member's: a seat the walkers do not hold ends its turn, a walker ends it
// half the time. Otherwise, after the quiet verbs, a walker now and then
// strikes the nearest standing monster, fells it with the blow, or sees it
// fall to nothing; and otherwise walks one to four cells of its route toward
// a seeded member.
func (r *residentRun) nextVerb(rng *rand.Rand) (string, verb) {
	who := keepPlayers[rng.IntN(checkpointParty)]
	clk, err := r.warm.ClockOf(&encounter.ClockOfInput{Member: who})
	require.NoError(r.t, err)
	if clk.Kind == encounter.ClockTurn && clk.Active != "" {
		if clk.Active != keepPlayers[0] && clk.Active != keepPlayers[1] || rng.IntN(2) == 0 {
			return "end_turn " + string(clk.Active), endTurnVerb(clk.Active)
		}
		who = clk.Active
	}
	if r.verbs >= r.quiet && len(r.posts) > 0 && !r.clanged {
		r.clanged = true
		return r.clangBesideAPost()
	}
	falls := ""
	if r.verbs >= r.quiet {
		target, ok := r.nearestMonster(who)
		switch roll := rng.IntN(6); {
		case ok && roll == 0:
			return "strike " + string(who) + "→" + string(target), strikeVerb(who, target)
		case ok && roll == 1:
			// The blow fells it: the fall names who struck it.
			r.down = append(r.down, target)
			return "strike " + string(who) + " fells " + string(target), strikeVerb(who, target)
		case ok && roll == 2:
			// It falls to nobody's blow, noticed by the walk below; a blow an
			// earlier verb landed must not explain it.
			r.down = append(r.down, target)
			falls = " as " + string(target) + " falls"
		}
	}
	// One to four cells along the route toward a seeded member.
	members, err := r.warm.Members()
	require.NoError(r.t, err)
	toward := members[rng.IntN(len(members))]
	route, err := r.warm.Route(encounter.RouteInput{
		Mover: who, Policy: encounter.MoveToward, Anchor: toward.Position, Budget: 1 + rng.IntN(4),
	})
	require.NoError(r.t, err)
	if len(route.Path) == 0 {
		route.Path = []spatial.Position{whereIs(r.t, r.warm, who)}
	}
	return "walk " + string(who) + " toward " + string(toward.ID) + falls, walkVerb(who, route.Path)
}

// TestResidentCopyWritesWhatAReloadWrites is the gate: on the small keep
// (its party and its monsters on their tables), a seeded sequence of walks,
// strikes and turn ends played on one kept instance rebound before every verb
// answers and writes, after every verb, exactly what the same sequence played
// on a fresh load of the stored checkpoint does. The quiet verbs run the world
// clock (creatures picking from their tables at each raise); the strikes after
// them form a fight and run its turns.
func TestResidentCopyWritesWhatAReloadWrites(t *testing.T) {
	playResident(t, smallKeep(t, false), 40, 12)
}

// TestResidentCopyWritesWhatAReloadWritesOnTheGarrison is the gate on the
// small keep's garrison: posts that listen and a walking patrol, the members
// whose wake state (due, dueAt, listeners, the landed beats) a load rebuilds
// and a kept copy carries. Fewer verbs, since each costs several times the
// plain keep's.
func TestResidentCopyWritesWhatAReloadWritesOnTheGarrison(t *testing.T) {
	playResident(t, smallKeep(t, true), 10, 8)
}

// playResident plays verbs verbs, the first quiet of them striking nobody.
func playResident(t *testing.T, compiled dungeonspec.Compiled, verbs, quiet int) {
	t.Helper()
	r := newResidentRun(t, compiled)
	r.quiet = quiet
	rng := rand.New(rand.NewPCG(927, 6)) //nolint:gosec // a seeded run, not a secret
	for range verbs {
		name, v := r.nextVerb(rng)
		r.play(name, v)
	}
	require.Less(t, r.failed, r.verbs/2, "most verbs land, so most are compared at the checkpoint")
}

// TestRebindRefusesBetweenNothing: a nil input, and a rebind while an act is
// open, are refused by name; a refused rebind leaves the encounter bound to
// what it was, and closing the act lets the next one through. A missing
// capability and the concealment pair are refused by the same sentinels Setup
// and Load use (capabilities_test.go).
func TestRebindRefusesBetweenNothing(t *testing.T) {
	enc := openCheckpointKeep(t, encounter.RetentionUnbounded)

	require.ErrorIs(t, enc.Rebind(nil), encounter.ErrNilInput)

	_, err := enc.OpenAct(&encounter.OpenActInput{Verb: "move"})
	require.NoError(t, err)
	refused := checkpointCapabilities()
	refused.Sight = blind{}
	require.ErrorIs(t, enc.Rebind(&encounter.RebindInput{Capabilities: refused}), encounter.ErrVerbInFlight)
	require.NoError(t, enc.CloseAct())
	require.NoError(t, enc.RefreshPerception(), "the refused rebind left the sight it had")

	stepSome(t, enc, rand.New(rand.NewPCG(927, 1)), 2) //nolint:gosec // a seeded walk, not a secret
	require.NoError(t, enc.Rebind(&encounter.RebindInput{Capabilities: refused}))
	require.ErrorIs(t, enc.RefreshPerception(), errBlind, "the rebound sight is the one asked")
}

// blind refuses every ask, so a test can tell which Sight a look asked.
type blind struct{}

var errBlind = errors.New("blind")

func (blind) Sight([]encounter.MemberID) (map[encounter.MemberID]int, error) { return nil, errBlind }

// TestResidentCopyWritesWhatAReloadWritesOnTheCastle is the gate on the mad
// king's castle, the QA keep: its own tables, posts and doors.
func TestResidentCopyWritesWhatAReloadWritesOnTheCastle(t *testing.T) {
	playResident(t, compiledSite(t, 0), 12, 6)
}
