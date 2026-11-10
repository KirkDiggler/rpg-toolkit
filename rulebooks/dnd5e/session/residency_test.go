// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

// residency_test.go pins THE RESIDENT COPY (rpg-project#879 S6,
// rpg-project#927, residency.go): a write verb's encounter kept between
// verbs as a cache of the store, checked by one version read. A warm verb
// skips the load; a save by anybody else drops the copy; every exit but a
// landed final commit keeps nothing; and a warm run writes, after every verb,
// what a run that loads every verb writes.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter/dungeonspec/keepgen"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// versionedStore is the in-memory pieces store with the version reader: the
// host S6 keeps copies for. It counts the fenced loads and the version reads,
// and its hooks stand in for another writer, a canceled request and a host
// that panics.
type versionedStore struct {
	*piecesStore
	fencedGets  int
	versionGets int

	// onVersion, when set, runs once after a version read has taken its
	// answer: whatever happens between a verb's check and its save.
	onVersion func()
	// onPieces, when set, runs at the start of every SavePieces with how
	// many have been asked; an error refuses the save.
	onPieces func(n int) error
	saves    int
	// versionErr, when set, fails the next version read.
	versionErr error
}

func newVersionedStore() *versionedStore { return &versionedStore{piecesStore: newPiecesStore()} }

func (v *versionedStore) GetFencedEncounter(
	ctx context.Context, in *session.GetFencedEncounterInput,
) (*session.GetFencedEncounterOutput, error) {
	v.fencedGets++
	return v.piecesStore.GetFencedEncounter(ctx, in)
}

func (v *versionedStore) GetCheckpointVersion(_ context.Context, enc string) (uint64, error) {
	v.versionGets++
	if err := v.versionErr; err != nil {
		v.versionErr = nil
		return 0, err
	}
	version := v.versions[enc]
	if hook := v.onVersion; hook != nil {
		v.onVersion = nil
		hook()
	}
	return version, nil
}

// SavePieces refuses a canceled request, as a store's client does, runs the
// test's hook, and checks the fence before anything else.
func (v *versionedStore) SavePieces(ctx context.Context, in *session.SavePiecesInput) (*session.SaveFencedOutput, error) {
	v.saves++
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if v.onPieces != nil {
		if err := v.onPieces(v.saves); err != nil {
			return nil, err
		}
	}
	// The fence first, as a store's one script checks it before it writes:
	// the stale verb's segment would overlap the other writer's.
	if in.Expected != v.versions[in.Encounter] {
		return nil, fmt.Errorf("expected %d, store at %d: %w", in.Expected, v.versions[in.Encounter], session.ErrStaleCheckpoint)
	}
	return v.piecesStore.SavePieces(ctx, in)
}

// residentManager is a Manager over the store with its own residency, as
// one process of several over one store would be.
func residentManager(
	t *testing.T, store *piecesStore, encounters session.EncounterRepository, res session.Residency,
	tune ...func(*session.Config),
) *session.Manager {
	t.Helper()
	cfg := &session.Config{
		Record: session.DiscardRecord{}, Acts: &fakeStream{}, Seats: store.seats,
		CharacterConfig: newFakeCharacterConfig(), PresentationIDs: testPresentationIDs{},
		Dice: testDice{}, TurnDriver: session.Driver(),
		Sessions: storeSessions{store: store.fencedStore}, Encounters: encounters, Characters: store.chars,
		Residency: res,
	}
	for _, f := range tune {
		f(cfg)
	}
	mgr, err := session.NewManager(cfg)
	require.NoError(t, err)
	return mgr
}

var (
	plainKeepOnce     sync.Once
	plainKeepCompiled dungeonspec.Compiled
	errPlainKeep      error
)

// plainKeep is the small keep without its garrison: keepgen's plan at 1,230
// hexes, seed 1, the default density (eight monsters, each on its table, on a
// side neutral to the party). It launches in about a second under -race, where the garrison takes six.
func plainKeep(t testing.TB) dungeonspec.Compiled {
	t.Helper()
	plainKeepOnce.Do(func() {
		plan := keepgen.DefaultPlan()
		for i := range plan.Rooms {
			r := &plan.Rooms[i]
			r.Width, r.Depth = 5, 3
			switch r.ID {
			case "hidden-stash", "tower":
				r.Width = 3
			case "larder":
				r.Depth = 6
			}
		}
		plan.Grounds.West.Width, plan.Grounds.South.Depth = 3, 3
		opts := keepgen.DefaultOptions()
		opts.Hexes, opts.Seed, opts.Key = 1230, 1, "small-keep"
		raw, _, err := keepgen.Generate(plan, opts)
		if err != nil {
			errPlainKeep = err
			return
		}
		plainKeepCompiled, errPlainKeep = dungeonspec.Load(raw)
		// Neutral to the party, as the garrison is, so the party walks on
		// the world clock and the keep's creatures pick from their tables
		// when a round raises.
		field := &plainKeepCompiled.Field
		field.Factions = append(field.Factions, encounter.FactionInput{ID: plainKeepCamp})
		field.Dispositions = append(field.Dispositions, encounter.DispositionInput{
			Between: [2]encounter.FactionID{plainKeepCamp, encounter.FactionParty}, Stance: encounter.StanceNeutral,
		})
		for i := range plainKeepCompiled.Monsters {
			plainKeepCompiled.Monsters[i].Faction = string(plainKeepCamp)
		}
	})
	if errPlainKeep != nil {
		t.Fatal(errPlainKeep)
	}
	return plainKeepCompiled
}

// plainKeepCamp is the plain keep's creatures' side, neutral to the party.
const plainKeepCamp encounter.FactionID = "keep"

// launchPlainKeep launches the plain keep with alice and bob.
func launchPlainKeep(t *testing.T, mgr *session.Manager) {
	t.Helper()
	compiled := plainKeep(t)
	_, err := mgr.Launch(context.Background(), &session.LaunchInput{
		Session: testSession, Dungeon: &compiled, Party: []string{"alice", "bob"},
	})
	require.NoError(t, err)
}

// routeOf is up to budget cells of who's route toward the nth member of the
// stored world (wrapping), read from the store.
func routeOf(t *testing.T, mgr *session.Manager, who string, nth, budget int) []spatial.Position {
	t.Helper()
	enc, err := mgr.LoadWorldForTest(context.Background(), testSession)
	require.NoError(t, err)
	members, err := enc.Members()
	require.NoError(t, err)
	toward := members[nth%len(members)]
	route, err := enc.Route(encounter.RouteInput{
		Mover: encounter.MemberID(who), Policy: encounter.MoveToward, Anchor: toward.Position, Budget: budget,
	})
	require.NoError(t, err)
	return route.Path
}

// move walks alice along path.
func move(t *testing.T, mgr *session.Manager, path []spatial.Position) {
	t.Helper()
	_, err := mgr.Move(context.Background(), &session.MoveInput{Session: testSession, Member: "alice", Path: path})
	require.NoError(t, err)
}

// pace is alice's one-cell walk out and back, found once from the store.
type pace struct{ out, back []spatial.Position }

func paceOf(t *testing.T, mgr *session.Manager) pace {
	t.Helper()
	enc, err := mgr.LoadWorldForTest(context.Background(), testSession)
	require.NoError(t, err)
	var start spatial.Position
	members, err := enc.Members()
	require.NoError(t, err)
	for _, m := range members {
		if m.ID == "alice" {
			start = m.Position
		}
	}
	out := routeOf(t, mgr, "alice", 3, 1)
	require.Len(t, out, 1, "alice has a cell to step to")
	return pace{out: out, back: []spatial.Position{start}}
}

// TestAWarmVerbSkipsTheLoad: Launch keeps its board; every verb after it
// reads the version once and never loads the world, and the copy is out of
// the residency while the verb runs.
func TestAWarmVerbSkipsTheLoad(t *testing.T) {
	store := newVersionedStore()
	res := session.NewResidency(nil)
	mgr := residentManager(t, store.piecesStore, store, res)
	launchPlainKeep(t, mgr)
	require.Equal(t, 1, res.Len(), "Launch keeps its board")
	require.Zero(t, store.fencedGets, "Launch loads nothing")
	p := paceOf(t, mgr)

	for i, path := range [][]spatial.Position{p.out, p.back, p.out} {
		store.onVersion = func() { require.Zero(t, res.Len(), "a copy in use is in no map") }
		move(t, mgr, path)
		require.Zero(t, store.fencedGets, "verb %d ran on the copy", i)
		require.Equal(t, i+1, store.versionGets, "verb %d read the version once", i)
		require.Equal(t, 1, res.Len(), "verb %d kept its world", i)
	}
}

// TestAnotherWritersSaveDropsTheCopy: two Managers, two residencies, one
// store, alternating verbs. Each verb after the other's save reloads, every
// save lands (none overwrites another), and the stored walk is all four.
func TestAnotherWritersSaveDropsTheCopy(t *testing.T) {
	store := newVersionedStore()
	resA, resB := session.NewResidency(nil), session.NewResidency(nil)
	a := residentManager(t, store.piecesStore, store, resA)
	b := residentManager(t, store.piecesStore, store, resB)
	launchPlainKeep(t, a)
	p := paceOf(t, a)

	move(t, a, p.out)
	require.Zero(t, store.fencedGets, "a's own copy")
	move(t, b, p.back)
	require.Equal(t, 1, store.fencedGets, "b holds no copy and loads")
	require.Equal(t, 1, resA.Len(), "a still holds its copy; the version will drop it")
	move(t, a, p.out)
	require.Equal(t, 2, store.fencedGets, "b saved since a's copy: a reloads")
	move(t, b, p.back)
	require.Equal(t, 3, store.fencedGets, "a saved since b's copy: b reloads")
	require.Equal(t, uint64(5), store.versions[testSession], "Launch and four landed saves")
	require.Equal(t, p.back[0], storedCellOf(t, a, "alice"), "the last save is b's, over a's")
}

// storedCellOf is a member's cell in the stored world.
func storedCellOf(t *testing.T, mgr *session.Manager, who string) spatial.Position {
	t.Helper()
	enc, err := mgr.LoadWorldForTest(context.Background(), testSession)
	require.NoError(t, err)
	members, err := enc.Members()
	require.NoError(t, err)
	for _, m := range members {
		if string(m.ID) == who {
			return m.Position
		}
	}
	t.Fatalf("%s is not placed", who)
	return spatial.Position{}
}

// TestAStaleSaveKeepsNoCopy: another writer saves between a warm verb's
// version read and its save. The save is refused stale, and nothing is kept.
func TestAStaleSaveKeepsNoCopy(t *testing.T) {
	store := newVersionedStore()
	res := session.NewResidency(nil)
	mgr := residentManager(t, store.piecesStore, store, res)
	other := residentManager(t, store.piecesStore, store, session.NewResidency(nil))
	launchPlainKeep(t, mgr)
	p := paceOf(t, mgr)

	store.onVersion = func() { move(t, other, p.out) }
	_, err := mgr.Move(context.Background(), &session.MoveInput{Session: testSession, Member: "alice", Path: p.out})
	require.ErrorIs(t, err, session.ErrStaleCheckpoint)
	require.Zero(t, res.Len(), "a stale verb keeps nothing")
}

// TestARefusedVerbKeepsNoCopy: a warm verb refused after its open keeps
// nothing, and the next verb loads.
func TestARefusedVerbKeepsNoCopy(t *testing.T) {
	store := newVersionedStore()
	res := session.NewResidency(nil)
	mgr := residentManager(t, store.piecesStore, store, res)
	launchPlainKeep(t, mgr)
	p := paceOf(t, mgr)

	far := spatial.Position{X: p.out[0].X + 40, Y: p.out[0].Y}
	_, err := mgr.Move(context.Background(), &session.MoveInput{Session: testSession, Member: "alice", Path: []spatial.Position{far}})
	require.Error(t, err)
	require.Zero(t, res.Len(), "a refused verb keeps nothing")
	move(t, mgr, p.out)
	require.Equal(t, 1, store.fencedGets, "the next verb loads")
	require.Equal(t, 1, res.Len())
}

// TestACanceledVerbKeepsNoCopy: the request is canceled mid-verb, the save
// is refused for it, and nothing is kept.
func TestACanceledVerbKeepsNoCopy(t *testing.T) {
	store := newVersionedStore()
	res := session.NewResidency(nil)
	mgr := residentManager(t, store.piecesStore, store, res)
	launchPlainKeep(t, mgr)
	p := paceOf(t, mgr)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	store.onVersion = cancel
	_, err := mgr.Move(ctx, &session.MoveInput{Session: testSession, Member: "alice", Path: p.out})
	require.ErrorIs(t, err, context.Canceled)
	require.Zero(t, res.Len(), "a canceled verb keeps nothing")
}

// TestAPanicKeepsNoCopy: a host collaborator that panics, before the save
// (the store) and after it (the act stream, whose batches go out after the
// save lands), leaves nothing kept once the test host recovers.
func TestAPanicKeepsNoCopy(t *testing.T) {
	store := newVersionedStore()
	res := session.NewResidency(nil)
	acts := &panickyStream{}
	mgr := residentManager(t, store.piecesStore, store, res, func(c *session.Config) { c.Acts = acts })
	launchPlainKeep(t, mgr)
	p := paceOf(t, mgr)

	store.onPieces = func(int) error { panic("the store fell over") }
	mustPanic(t, func() { move(t, mgr, p.out) })
	require.Zero(t, res.Len(), "a store that panicked keeps nothing")
	store.onPieces = nil

	move(t, mgr, p.out)
	require.Equal(t, 1, res.Len())
	acts.panics = true
	mustPanic(t, func() { move(t, mgr, p.back) })
	require.Zero(t, res.Len(), "a stream that panicked after the save keeps nothing")
}

// panickyStream is an act stream that panics while panics is set.
type panickyStream struct{ panics bool }

func (s *panickyStream) PublishActs(context.Context, []session.Act) error {
	if s.panics {
		panic("the stream fell over")
	}
	return nil
}

// mustPanic runs fn and requires it to panic, as a host's recover would.
func mustPanic(t *testing.T, fn func()) {
	t.Helper()
	defer func() { require.NotNil(t, recover(), "it panicked") }()
	fn()
}

// TestAClosedRunKeepsNoCopy: the verb that ends the run keeps nothing.
func TestAClosedRunKeepsNoCopy(t *testing.T) {
	store := newVersionedStore()
	res := session.NewResidency(nil)
	mgr := residentManager(t, store.piecesStore, store, res)
	launchScene(t, mgr, scene{
		Field:   encounter.FieldInput{Canvas: pointyCanvas(), Regions: []encounter.RegionInput{rectRegion("hall", 0, 0, 6, 4)}},
		Party:   []sceneSeat{seatAt("alice", 1, 1)},
		Endings: []encounter.EndingInput{{Key: "out", Trigger: encounter.TriggerExternal{}}},
	})
	require.Equal(t, 1, res.Len())
	_, err := mgr.End(context.Background(), &session.EndInput{Session: testSession, Ending: "out"})
	require.NoError(t, err)
	require.Zero(t, res.Len(), "a closed run keeps nothing")
}

// TestAContinuationThatFailsAfterAPaceCommitKeepsNoCopy: a continued walk's
// pace commit lands, its final save is refused, and nothing is kept, though
// a save of the verb landed.
func TestAContinuationThatFailsAfterAPaceCommitKeepsNoCopy(t *testing.T) {
	store := newVersionedStore()
	res := session.NewResidency(nil)
	runner := &capturingRunner{}
	mgr := residentManager(t, store.piecesStore, store, res, func(c *session.Config) { c.Continuations = runner })
	goblin := monsterAt("goblin", refs.Monsters.Goblin().String(), 11, 3)
	goblin.Faction = string(continuationCamp)
	launchScene(t, mgr, scene{
		Field: encounter.FieldInput{
			Canvas:   pointyCanvas(),
			Regions:  []encounter.RegionInput{rectRegion("hall", 0, 0, 14, 4)},
			Factions: []encounter.FactionInput{{ID: continuationCamp}},
			Dispositions: []encounter.DispositionInput{
				{Between: [2]encounter.FactionID{continuationCamp, encounter.FactionParty}, Stance: encounter.StanceNeutral},
			},
		},
		Party:    []sceneSeat{seatAt("alice", 1, 1)},
		Monsters: []dungeonspec.MonsterPlacement{goblin},
		Endings:  []encounter.EndingInput{{Key: "out", Trigger: encounter.TriggerExternal{}}},
	})
	require.Equal(t, 1, res.Len())

	out, err := mgr.Move(context.Background(), &session.MoveInput{Session: testSession, Member: "alice", Path: eastFrom(10)})
	require.NoError(t, err)
	require.Equal(t, session.MovementContinuing, out.Status)
	require.Zero(t, res.Len(), "the continuation holds the copy")

	errRefused := errors.New("the final save is refused")
	saves := store.saves
	store.onPieces = func(n int) error {
		if n == saves+2 {
			return errRefused
		}
		return nil
	}
	require.Len(t, runner.handed, 1)
	report, err := runner.handed[0].Continue(context.Background())
	require.ErrorIs(t, err, errRefused)
	require.NotNil(t, report)
	require.Equal(t, 1, report.Commits, "the pace commit landed")
	require.Zero(t, res.Len(), "a rest that failed after a pace commit keeps nothing")
}

// countingResidency counts every call the session makes on it.
type countingResidency struct{ takes, keeps int }

func (c *countingResidency) Take(string) *session.Resident  { c.takes++; return nil }
func (c *countingResidency) Keep(string, *session.Resident) { c.keeps++ }

// TestAHostWithoutTheReaderNeverTouchesTheResidency: a pieces host with no
// version reader loads every verb and never reaches the residency.
func TestAHostWithoutTheReaderNeverTouchesTheResidency(t *testing.T) {
	store := newPiecesStore()
	counting := &countingResidency{}
	mgr := residentManager(t, store, store, counting)
	launchScene(t, mgr, scene{
		Field: encounter.FieldInput{Canvas: pointyCanvas(), Regions: []encounter.RegionInput{rectRegion("hall", 0, 0, 6, 4)}},
		Party: []sceneSeat{seatAt("alice", 1, 1)},
	})
	move(t, mgr, []spatial.Position{hexCell(2, 1)})
	move(t, mgr, []spatial.Position{hexCell(1, 1)})
	require.Zero(t, counting.takes)
	require.Zero(t, counting.keeps)
}

// TestAWarmRunWritesWhatAColdRunWrites is the session-level equivalence:
// the same launch and the same paces on a host that keeps copies and on one
// that loads every verb leave, after every verb, the same stored world, ids
// made ordinal (each run mints its own); the save carries the same pieces;
// and the streams have been handed the same acts.
func TestAWarmRunWritesWhatAColdRunWrites(t *testing.T) {
	warmStore, coldStore := newVersionedStore(), newPiecesStore()
	res := session.NewResidency(nil)
	warmActs, coldActs := &batchStream{}, &batchStream{}
	warm := residentManager(t, warmStore.piecesStore, warmStore, res, func(c *session.Config) { c.Acts = warmActs })
	cold := residentManager(t, coldStore, coldStore, nil, func(c *session.Config) { c.Acts = coldActs })
	launchPlainKeep(t, warm)
	launchPlainKeep(t, cold)
	same := func(at string) {
		t.Helper()
		require.Equal(t, canonicalWorld(t, coldStore), canonicalWorld(t, warmStore.piecesStore), "%s: the same stored world", at)
	}
	same("launch")

	// Paces out and back: a round raises every few cells, and the keep's
	// creatures pick from their tables at each.
	p := paceOf(t, cold)
	for i := range residentWalks {
		path := p.out
		if i%2 == 1 {
			path = p.back
		}
		move(t, cold, path)
		move(t, warm, path)
		same(fmt.Sprintf("walk %d", i+1))
		require.Equal(t, coldStore.lastCall().keys, warmStore.lastCall().keys,
			"walk %d: the warm save carries the pieces the cold one does", i+1)
		require.Equal(t, perRecipient(coldActs.flat()), perRecipient(warmActs.flat()),
			"walk %d: the same acts delivered", i+1)
	}
	require.GreaterOrEqual(t, worldRound(t, []byte(canonicalWorld(t, coldStore))), 1, "rounds raised")
	require.Zero(t, warmStore.fencedGets, "every warm verb ran on the copy")
}

// residentWalks is how many paces the equivalence plays.
const residentWalks = 7

// ulid is a record id as the store holds one.
var ulid = regexp.MustCompile(`[0-9A-HJKMNP-TV-Z]{26}`)

// canonicalWorld is the stored world as JSON with every record id replaced
// by its ordinal of first appearance: two runs mint different ids in the same
// order.
func canonicalWorld(t *testing.T, store *piecesStore) string {
	t.Helper()
	out, err := store.GetFencedEncounter(context.Background(), &session.GetFencedEncounterInput{Encounter: testSession})
	require.NoError(t, err)
	raw, err := json.Marshal(out.Data)
	require.NoError(t, err)
	seen := map[string]string{}
	return ulid.ReplaceAllStringFunc(string(raw), func(id string) string {
		if _, ok := seen[id]; !ok {
			seen[id] = fmt.Sprintf("#%d", len(seen)+1)
		}
		return seen[id]
	})
}

// countingObserver records every Hit and Miss the session reports.
type countingObserver struct {
	hits   int
	misses []session.MissReason
}

func (o *countingObserver) Hit(string) { o.hits++ }
func (o *countingObserver) Miss(_ string, reason session.MissReason) {
	o.misses = append(o.misses, reason)
}

// inFlightResidency hands back its copy with an act already open when spoil
// is set, a copy Rebind refuses.
type inFlightResidency struct {
	*session.MemoryResidency
	spoil bool
}

func (r *inFlightResidency) Take(id string) *session.Resident {
	kept := r.MemoryResidency.Take(id)
	if kept != nil && r.spoil {
		r.spoil = false
		if err := kept.OpenActForTest("spoiled"); err != nil {
			panic(err)
		}
	}
	return kept
}

// TestTheObserverNamesEveryGate: a warm verb is one Hit; each gate a verb
// can fail on the way to the copy is one Miss naming it, and the verb loads.
func TestTheObserverNamesEveryGate(t *testing.T) {
	store := newVersionedStore()
	obs := &countingObserver{}
	res := &inFlightResidency{MemoryResidency: session.NewResidency(nil)}
	mgr := residentManager(t, store.piecesStore, store, res, func(c *session.Config) { c.ResidencyObserver = obs })
	other := residentManager(t, store.piecesStore, store, session.NewResidency(nil))
	launchPlainKeep(t, mgr)
	require.Zero(t, obs.hits, "Launch observes nothing")
	require.Empty(t, obs.misses)
	p := paceOf(t, mgr)

	move(t, mgr, p.out)
	require.Equal(t, 1, obs.hits, "a warm verb is a hit")

	move(t, other, p.back)
	move(t, mgr, p.out)
	require.Equal(t, []session.MissReason{session.MissVersionMoved}, obs.misses, "another writer saved")

	store.versionErr = errors.New("the store is unreachable")
	move(t, mgr, p.back)
	require.Equal(t, session.MissReaderError, obs.misses[len(obs.misses)-1])

	res.spoil = true
	move(t, mgr, p.out)
	require.Equal(t, session.MissRebindRefused, obs.misses[len(obs.misses)-1])

	res.Take(testSession)
	move(t, mgr, p.back)
	require.Equal(t, session.MissNoCopy, obs.misses[len(obs.misses)-1])

	move(t, mgr, p.out)
	require.Equal(t, 2, obs.hits, "the copy the last load kept is used again")
	require.Len(t, obs.misses, 4, "one miss per gate, and nothing else")
}

// TestAHostWithoutTheReaderObservesNothing: no reader, no residency, nothing
// to observe.
func TestAHostWithoutTheReaderObservesNothing(t *testing.T) {
	store := newPiecesStore()
	obs := &countingObserver{}
	mgr := residentManager(t, store, store, nil, func(c *session.Config) { c.ResidencyObserver = obs })
	launchScene(t, mgr, scene{
		Field: encounter.FieldInput{Canvas: pointyCanvas(), Regions: []encounter.RegionInput{rectRegion("hall", 0, 0, 6, 4)}},
		Party: []sceneSeat{seatAt("alice", 1, 1)},
	})
	move(t, mgr, []spatial.Position{hexCell(2, 1)})
	require.Zero(t, obs.hits)
	require.Empty(t, obs.misses)
}
