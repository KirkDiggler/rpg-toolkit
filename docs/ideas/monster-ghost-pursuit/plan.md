# Monster Ghost Pursuit Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let a fight-time monster pursue the closest remembered player position when nobody is visible, correct that position to explicitly unknown on arrival, and stop pursuing the resolved ghost without receiving concealed world truth.

**Architecture:** `rulebooks/dnd5e/encounter` owns a backward-compatible tagged location payload, projects held known positions separately from current `Seen`, and authors arrival correction through opaque `intel.Report`. `behavior.Basic` remains visible-first and uses remembered positions only as a fallback. `session` mirrors the additive view and delta contracts while preserving load-act-save persistence; `play/intel` does not change.

**Tech Stack:** Go, testify suites, nested Go modules, JSON persistence, `play/intel`, `tools/spatial`, repository ADR/idea conventions.

**Spec:** [design.md](design.md)

## Global Constraints

- SRD 5.1 is authoritative; Roll20 2024 material is clarification only.
- `play/intel` stays payload-opaque and gains no geometry, truth access, or position API.
- `MonsterView.Seen` stays current-only; remembered positions are additive and separate.
- A remembered target is never attackable, carries no hidden standing state, and has a path ending on the exact remembered cell.
- `behavior.Basic` always prefers a visible standing player. Remembered selection occurs only when no visible standing player exists; an unreachable visible target preserves today's `Pass` behavior rather than falling through to a ghost.
- Equal remembered distances temporarily break by subject ID.
- Correction occurs only after a driven fight-time monster reaches the exact remembered cell. Public `Step`, free-roam `Pump`, and visibility-before-arrival correction remain deferred.
- A newly visible player interrupts remembered pursuit on the next driver call.
- Intel corrections are returned in encounter-owned deltas and persisted; they are not silent mutations.
- No local `replace` directive or `go.work` file may be committed.
- Work module-by-module; other modules are read-only until their task begins.

## Locked Implementation Decisions

The approved design left these exact API choices open. This plan resolves them for review:

1. Canonical known JSON is flattened and tagged: `{"state":"known","x":0,"y":0}`. Unknown is `{"state":"unknown"}`. Legacy `{"x":0,"y":0}` remains readable as known.
2. `DecodeSightPayload` remains as a known-position compatibility wrapper; state-aware code uses `DecodeLocationPayload`.
3. Encounter outputs replace raw `*intel.SurveilOutput` values with an encounter-owned `*IntelDelta` containing `FirstContact`, `Refreshed`, `Faded`, and `Corrected`.
4. Session projects explicit location state on sight-channel `Sighting`; malformed sight testimony is rejected on encounter load rather than projected as unknown.
5. Remembered entries may carry an empty path when unreachable, but `Basic` skips them and selects the closest reachable memory.
6. Arrival correction reports `Channel: intel.Sight`, the mover as observer, and the current clock high-water as `At`, preserving `Held` status.

## File Map

- Create `rulebooks/dnd5e/encounter/location.go` for location state, strict codec, legacy compatibility, and arrival correction.
- Create `rulebooks/dnd5e/encounter/inteldelta.go` for composition-owned perception/correction deltas.
- Modify encounter sight, persistence, trigger, movement, clock, and turn-view files to propagate the new contracts.
- Modify `rulebooks/dnd5e/behavior/basic.go` for visible-first remembered fallback.
- Modify both session turn-driver adapters plus session projections and write outputs.
- Add focused tests to existing suites; do not create a parallel testing framework.
- Add ADR-0046 and an observed implementation record after verification.

---

### Task 1: Add Strict Encounter-Owned Location Testimony

**Files:**
- Create: `rulebooks/dnd5e/encounter/location.go`
- Modify: `rulebooks/dnd5e/encounter/encounter.go:20`
- Modify: `rulebooks/dnd5e/encounter/data.go`
- Test: `rulebooks/dnd5e/encounter/sightdecode_test.go`
- Test: `rulebooks/dnd5e/encounter/dialect_test.go`
- Test: `rulebooks/dnd5e/encounter/data_test.go`

**Interfaces:**
- Consumes: `spatial.Position`, legacy `SightPayload`, opaque `[]byte` payloads.
- Produces:

```go
type LocationState string

const (
	LocationKnown   LocationState = "known"
	LocationUnknown LocationState = "unknown"
)

type LocationKnowledge struct {
	State    LocationState
	Position spatial.Position
}

func EncodeLocationPayload(LocationKnowledge) ([]byte, error)
func DecodeLocationPayload([]byte) (LocationKnowledge, bool)
func DecodeSightPayload([]byte) (spatial.Position, bool)
```

- [ ] **Step 1: Write failing codec tests**

```go
func TestLocationPayloadRoundTrip(t *testing.T) {
	known := LocationKnowledge{State: LocationKnown, Position: spatial.Position{X: 0, Y: -2}}
	payload, err := EncodeLocationPayload(known)
	require.NoError(t, err)
	require.JSONEq(t, `{"state":"known","x":0,"y":-2}`, string(payload))

	got, ok := DecodeLocationPayload(payload)
	require.True(t, ok)
	require.Equal(t, known, got)

	unknown, err := EncodeLocationPayload(LocationKnowledge{State: LocationUnknown})
	require.NoError(t, err)
	require.JSONEq(t, `{"state":"unknown"}`, string(unknown))
}

func TestDecodeLocationPayloadReadsLegacyKnownPosition(t *testing.T) {
	got, ok := DecodeLocationPayload([]byte(`{"x":0,"y":4}`))
	require.True(t, ok)
	require.Equal(t, LocationKnowledge{
		State: LocationKnown,
		Position: spatial.Position{X: 0, Y: 4},
	}, got)
}
```

Add rejection rows for empty input, `null`, unsupported state, known without either axis, unknown with either axis, extra fields, non-numeric axes, and trailing JSON.

- [ ] **Step 2: Run the focused tests and confirm RED**

Working directory: `rulebooks/dnd5e/encounter`

```bash
go test ./... -run 'TestLocationPayload|TestDecodeLocationPayloadReadsLegacyKnownPosition' -count=1
```

Expected: compile failure because the new types and codec do not exist.

- [ ] **Step 3: Implement the strict tagged codec**

Use a private presence-aware wire type so `(0,0)` is not confused with omitted axes:

```go
type locationWire struct {
	State string   `json:"state,omitempty"`
	X     *float64 `json:"x,omitempty"`
	Y     *float64 `json:"y,omitempty"`
}
```

`EncodeLocationPayload` emits only canonical tagged forms. `DecodeLocationPayload` uses `json.Decoder.DisallowUnknownFields`, accepts one JSON value, maps absent state plus both axes to legacy known, and rejects contradictory forms. `DecodeSightPayload` becomes a compatibility wrapper that succeeds only for `LocationKnown`.

Change `rebuildPercepts` to call:

```go
payload, err := EncodeLocationPayload(LocationKnowledge{
	State: LocationKnown, Position: pos,
})
```

- [ ] **Step 4: Write failing load-validation tests**

Extend the existing one-defect fixture style with cases for malformed sight location and `Current + Unknown`:

```go
tests := []struct {
	name       string
	payload    []byte
	currentVia []intel.Channel
}{
	{name: "unknown carries coordinate", payload: []byte(`{"state":"unknown","x":1}`)},
	{name: "current unknown", payload: []byte(`{"state":"unknown"}`), currentVia: []intel.Channel{intel.Sight}},
}
for _, tt := range tests {
	t.Run(tt.name, func(t *testing.T) {
		data := validEncounterData(t)
		setSightHolding(t, &data, tt.payload, tt.currentVia)
		_, err := LoadEncounter(data, validLoadConfig())
		require.ErrorIs(t, err, ErrInvalidData)
	})
}
```

Add `setSightHolding(t, data, payload, currentVia)` beside the suite's existing data mutators; it finds one valid sight holding, deep-copies the inputs, and changes no other field. Preserve the test proving another channel's arbitrary payload is not interpreted.

- [ ] **Step 5: Validate sight testimony during load**

In the existing sight-dialect validation pass:

```go
if holding.Channel == intel.Sight {
	location, ok := DecodeLocationPayload(holding.Payload)
	if !ok || location.State == LocationUnknown && len(holding.CurrentVia) > 0 {
		return fmt.Errorf("load encounter sight location: %w", ErrInvalidData)
	}
}
```

Do not decode payloads whose provenance channel is not `intel.Sight`.

- [ ] **Step 6: Run encounter tests and update canonical goldens**

Working directory: `rulebooks/dnd5e/encounter`

```bash
go test ./... -count=1
```

Expected: PASS after expected canonical known payloads are updated. Legacy fixtures remain readable.

- [ ] **Step 7: Commit**

```bash
git add rulebooks/dnd5e/encounter/location.go rulebooks/dnd5e/encounter/encounter.go rulebooks/dnd5e/encounter/data.go rulebooks/dnd5e/encounter/sightdecode_test.go rulebooks/dnd5e/encounter/dialect_test.go rulebooks/dnd5e/encounter/data_test.go
git commit -m "feat(encounter): type remembered location knowledge"
```

---

### Task 2: Introduce Encounter-Owned Intel Deltas

**Files:**
- Create: `rulebooks/dnd5e/encounter/inteldelta.go`
- Modify: `rulebooks/dnd5e/encounter/encounter.go`
- Modify: `rulebooks/dnd5e/encounter/field.go`
- Modify: `rulebooks/dnd5e/encounter/step.go`
- Modify: `rulebooks/dnd5e/encounter/doorverbs.go`
- Modify: `rulebooks/dnd5e/encounter/trigger.go`
- Test: `rulebooks/dnd5e/encounter/encounter_test.go`

**Interfaces:**
- Consumes: `*intel.SurveilOutput`.
- Produces:

```go
type IntelDelta struct {
	FirstContact []intel.Report
	Refreshed    []intel.Subject
	Faded        []intel.Subject
	Corrected    []intel.Subject
}

func intelDeltaFromSurveil(*intel.SurveilOutput) *IntelDelta
func mergeIntelDeltas(dst, src map[MemberID]*IntelDelta) map[MemberID]*IntelDelta
```

- [ ] **Step 1: Write failing copy and merge tests**

```go
func TestIntelDeltaCopiesSurveilOutput(t *testing.T) {
	in := &intel.SurveilOutput{
		FirstContact: []intel.Report{{Subject: "billy", Payload: []byte("known")}},
		Refreshed: []intel.Subject{"david"}, Faded: []intel.Subject{"alice"},
	}
	got := intelDeltaFromSurveil(in)
	in.FirstContact[0].Payload[0] = 'X'
	require.Equal(t, []byte("known"), got.FirstContact[0].Payload)
	require.Empty(t, got.Corrected)
}
```

Add a merge test proving `Faded` and `Corrected` for the same subject remain separate facts and per-category duplicates are removed without reordering first occurrence.

- [ ] **Step 2: Run the focused tests and confirm RED**

Working directory: `rulebooks/dnd5e/encounter`

```bash
go test ./... -run 'TestIntelDelta|TestMergeIntelDeltas' -count=1
```

Expected: compile failure because `IntelDelta` does not exist.

- [ ] **Step 3: Implement deep-copy conversion and deterministic merge**

Deep-copy all slices and payload bytes. Merge per observer, retaining category order and deduplicating within each category.

- [ ] **Step 4: Propagate the new delta type through refresh and trigger paths**

Change these signatures:

```go
func (e *Encounter) refreshSight([]MemberID) (map[MemberID]*IntelDelta, *FormedBubble, error)
func (e *Encounter) rebuildPercepts([]MemberID) (map[MemberID]*IntelDelta, error)
func (e *Encounter) classify(map[MemberID]*IntelDelta, map[MemberID]bool) (*trigger, error)
func (e *Encounter) applyTrigger(map[MemberID]*IntelDelta) (*FormedBubble, error)
```

Change existing `IntelDeltas` fields in movement, step, join/exit, and door outputs to `map[MemberID]*IntelDelta`. Trigger classification keeps reading `FirstContact`, `Refreshed`, and `Faded`; `Corrected` never forms a fight.

- [ ] **Step 5: Run encounter regression tests**

Working directory: `rulebooks/dnd5e/encounter`

```bash
go test ./... -count=1
```

Expected: PASS with sight, door, movement, and trigger behavior unchanged.

- [ ] **Step 6: Commit**

```bash
git add rulebooks/dnd5e/encounter/inteldelta.go rulebooks/dnd5e/encounter/encounter.go rulebooks/dnd5e/encounter/field.go rulebooks/dnd5e/encounter/step.go rulebooks/dnd5e/encounter/doorverbs.go rulebooks/dnd5e/encounter/trigger.go rulebooks/dnd5e/encounter/encounter_test.go
git commit -m "refactor(encounter): own intel delta projection"
```

---

### Task 3: Project Remembered Positions into Fight-Time Views

**Files:**
- Modify: `rulebooks/dnd5e/encounter/turndriver.go:77`
- Modify: `rulebooks/dnd5e/encounter/clocks.go:642`
- Test: `rulebooks/dnd5e/encounter/monsterturn_test.go`

**Interfaces:**
- Consumes: the actor's own `intel.Holding` values and `DecodeLocationPayload`.
- Produces:

```go
type RememberedMember struct {
	ID            MemberID
	Kind          MemberKind
	Position      spatial.Position
	DistanceCells float64
	Path          []spatial.Position
}

type MonsterView struct {
	// existing fields
	Remembered []RememberedMember
}
```

- [ ] **Step 1: Write failing view-projection tests**

Pin these cases in `monsterturn_test.go`:

```go
require.Len(t, view.Seen, 1)
require.Empty(t, view.Remembered) // current known sight

require.Empty(t, view.Seen)
require.Equal(t, RememberedMember{
	ID: "billy", Kind: KindPlayer, Position: carpet,
	DistanceCells: enc.Distance(goblinStart, carpet),
	Path: expectedExactCellPath,
}, view.Remembered[0]) // held known sight
```

Add cases for held unknown producing neither collection, stale position differing from concealed live position, deterministic ID order, `(0,0)`, and unreachable memory carrying an empty path.

- [ ] **Step 2: Run focused tests and confirm RED**

Working directory: `rulebooks/dnd5e/encounter`

```bash
go test ./... -run 'TestMonsterView.*Remembered|TestMonsterView.*Unknown' -count=1
```

Expected: compile failure because the remembered view types do not exist.

- [ ] **Step 3: Add remembered view types and godoc**

Document that remembered entries are plain knowledge data, are never attackable, contain no hidden standing fact, and use exact-cell paths.

- [ ] **Step 4: Split current and held projection in `buildMonsterView`**

For each sight-provenance holding whose subject maps to a member:

```go
location, ok := DecodeLocationPayload(h.Payload)
if !ok || location.State == LocationUnknown {
	continue
}

switch h.Status {
case intel.Current:
	// Preserve existing Seen construction and reach-aware BFS.
case intel.Held:
	path, reachable := e.bfsShortestPath(ownCell, func(cell spatial.Position) bool {
		return cell == location.Position
	})
	if !reachable {
		path = nil
	}
	remembered = append(remembered, RememberedMember{
		ID: MemberID(h.Subject), Kind: other.Kind,
		Position: location.Position,
		DistanceCells: e.Distance(ownCell, location.Position), Path: path,
	})
}
```

Sort `Seen` and `Remembered` independently by ID. Do not use `standingNow` to populate remembered data.

- [ ] **Step 5: Run encounter tests**

Working directory: `rulebooks/dnd5e/encounter`

```bash
go test ./... -count=1
```

Expected: PASS; existing `Seen` assertions remain unchanged.

- [ ] **Step 6: Commit**

```bash
git add rulebooks/dnd5e/encounter/turndriver.go rulebooks/dnd5e/encounter/clocks.go rulebooks/dnd5e/encounter/monsterturn_test.go
git commit -m "feat(encounter): expose remembered turn targets"
```

---

### Task 4: Correct Remembered Location on Driven Arrival

**Files:**
- Modify: `rulebooks/dnd5e/encounter/location.go`
- Modify: `rulebooks/dnd5e/encounter/clocks.go`
- Modify: `rulebooks/dnd5e/encounter/field.go`
- Test: `rulebooks/dnd5e/encounter/monsterturn_test.go`
- Test: `rulebooks/dnd5e/encounter/data_test.go`

**Interfaces:**
- Consumes: the mover's own held known location, destination cell, complete percept represented by its refresh delta, and `intel.Report`.
- Produces:

```go
func (e *Encounter) correctArrivedLocations(
	observer MemberID,
	at uint64,
	perceived *IntelDelta,
) ([]intel.Subject, error)
```

Add these test helpers beside the monster-turn fixture:

```go
func requireHolding(t *testing.T, enc *Encounter, observer MemberID, subject intel.Subject) intel.Holding
func requireKnownLocation(t *testing.T, payload []byte, want spatial.Position)
func requireUnknownLocation(t *testing.T, payload []byte)
```

Each helper calls the public Intel/view seam already used by that suite and fails immediately through `require` rather than returning guessed zero values.

- [ ] **Step 1: Write failing arrival-correction tests**

Build a direct driven-turn fixture and assert:

```go
before := requireHolding(t, enc, goblin, "billy")
require.Equal(t, intel.Held, before.Status)
requireKnownLocation(t, before.Payload, carpet)

out, err := enc.EndTurn(&EndTurnInput{Member: activePlayer})
require.NoError(t, err)
require.Contains(t, out.IntelDeltas[goblin].Corrected, intel.Subject("billy"))

after := requireHolding(t, enc, goblin, "billy")
require.Equal(t, intel.Held, after.Status)
requireUnknownLocation(t, after.Payload)
```

Add cases proving no correction before exact-cell arrival, no correction when the subject is in the complete percept, only the mover changes, and all absent held-known subjects at one arrival cell are corrected in subject order.

- [ ] **Step 2: Run focused tests and confirm RED**

Working directory: `rulebooks/dnd5e/encounter`

```bash
go test ./... -run 'Test.*Arrival.*Correct|Test.*Remembered.*Arrival' -count=1
```

Expected: compile failure because driven outputs do not expose deltas and correction does not exist.

- [ ] **Step 3: Implement lawful arrival correction**

Build a perceived-subject set only from `perceived.FirstContact` and `perceived.Refreshed`, which are the complete percept just landed. Inspect only `HeldBy(observer)`:

```go
if h.Channel != intel.Sight || h.Status != intel.Held {
	continue
}
location, ok := DecodeLocationPayload(h.Payload)
if !ok || location.State != LocationKnown || location.Position != observerCell {
	continue
}
if _, seen := perceivedSubjects[h.Subject]; seen {
	continue
}
```

Encode `LocationUnknown`, sort reports by subject, and call `intel.Report` once with the mover as observer, `intel.Sight`, and current clock high-water. Append the corrected subjects to the mover's `IntelDelta.Corrected`.

- [ ] **Step 4: Thread deltas through driven execution**

Change internal signatures:

```go
func (e *Encounter) executeTurnIntent(...) (done bool, deltas map[MemberID]*IntelDelta, err error)
func (e *Encounter) driveOneMonsterTurn(...) (seq uint64, wrapped bool, deltas map[MemberID]*IntelDelta, err error)
func (e *Encounter) driveMonsterTurns(...) (wrapped bool, lastSeq uint64, deltas map[MemberID]*IntelDelta, err error)
```

For driven `Move`, merge `refreshSight` deltas, run arrival correction for the active monster, and return the map. Add `IntelDeltas map[MemberID]*IntelDelta` to `EndTurnOutput`, `FormOutput`, and `TransferOutput`. Merge driven deltas into any enclosing move/join/exit/door output that caused form or transfer driving. Public `Step` and free-roam `Pump` do not call arrival correction.

- [ ] **Step 5: Add persistence round-trip tests**

After correction, call `ToData`, reload with `LoadEncounter`, assert `Held + Unknown`, then build the next monster view and assert Billy appears in neither `Seen` nor `Remembered`.

- [ ] **Step 6: Run encounter tests with race detection**

Working directory: `rulebooks/dnd5e/encounter`

```bash
go test -race ./... -count=1
```

Expected: PASS, including drive re-entrancy, movement, formation, transfer, and persistence suites.

- [ ] **Step 7: Commit**

```bash
git add rulebooks/dnd5e/encounter/location.go rulebooks/dnd5e/encounter/clocks.go rulebooks/dnd5e/encounter/field.go rulebooks/dnd5e/encounter/monsterturn_test.go rulebooks/dnd5e/encounter/data_test.go
git commit -m "feat(encounter): correct ghosts on driven arrival"
```

---

### Task 5: Add Visible-First Remembered Fallback to `behavior.Basic`

**Files:**
- Modify: `rulebooks/dnd5e/behavior/basic.go`
- Test: `rulebooks/dnd5e/behavior/basic_test.go`
- Modify: `rulebooks/dnd5e/behavior/go.mod`
- Modify: `rulebooks/dnd5e/behavior/go.sum`

**Interfaces:**
- Consumes: `MonsterView.Seen`, `MonsterView.Remembered`, and turn budget. The existing visible branch remains authoritative whenever `closest(view.Seen)` finds a standing player.
- Produces:

```go
func closestRemembered([]encounter.RememberedMember) (encounter.RememberedMember, bool)
```

The helper skips empty paths, chooses smallest `DistanceCells`, and breaks ties by `ID`.

- [ ] **Step 1: Point local behavior development at the encounter provider**

Use a temporary local `replace` or pushed encounter pseudo-version. Never commit a filesystem replacement.

- [ ] **Step 2: Write failing behavior tests**

```go
func TestBasicPrefersSeenOverRemembered(t *testing.T) {
	view := encounter.MonsterView{
		Seen: []encounter.SeenMember{{ID: "david", Kind: encounter.KindPlayer, Standing: true, Path: []spatial.Position{{X: 1}}}},
		Remembered: []encounter.RememberedMember{{ID: "billy", Kind: encounter.KindPlayer, DistanceCells: 1, Path: []spatial.Position{{X: -1}}}},
		Budget: encounter.TurnBudget{MovementFeet: 30},
	}
	intent, err := (Basic{}).Act(view)
	require.NoError(t, err)
	require.Equal(t, encounter.Move{Path: []spatial.Position{{X: 1}}}, intent)
}

func TestBasicChoosesClosestReachableRemembered(t *testing.T) {
	view := encounter.MonsterView{
		Remembered: []encounter.RememberedMember{
			{ID: "alice", Kind: encounter.KindPlayer, DistanceCells: 1, Path: nil},
			{ID: "billy", Kind: encounter.KindPlayer, DistanceCells: 2, Path: []spatial.Position{{X: 1}}},
		},
		Budget: encounter.TurnBudget{AttacksLeft: 1, MovementFeet: 30},
	}
	intent, err := (Basic{}).Act(view)
	require.NoError(t, err)
	require.Equal(t, encounter.Move{Path: []spatial.Position{{X: 1}}}, intent)
}
```

Also prove ID tie-breaking is independent of slice order, remembered candidates are never attacked, zero movement passes, and no knowledge passes.

- [ ] **Step 3: Run focused tests and confirm RED**

Working directory: `rulebooks/dnd5e/behavior`

```bash
go test ./... -run 'TestBasic.*Remembered|TestBasicPrefersSeen' -count=1
```

Expected: failures because `Basic` passes when `Seen` is empty.

- [ ] **Step 4: Implement minimal fallback**

Preserve the visible branch. Only after it finds no visible target:

```go
remembered, ok := closestRemembered(view.Remembered)
if !ok || view.Budget.MovementFeet <= 0 {
	return encounter.Pass{}, nil
}
return encounter.Move{Path: remembered.Path[:1]}, nil
```

Do not inspect attack budget or actions for remembered targets.

- [ ] **Step 5: Run behavior tests**

Working directory: `rulebooks/dnd5e/behavior`

```bash
go test -race ./... -count=1
```

Expected: PASS.

- [ ] **Step 6: Pin published encounter dependency and commit**

Remove local `replace`, run `go mod tidy`, and inspect the module diff.

```bash
git add rulebooks/dnd5e/behavior/basic.go rulebooks/dnd5e/behavior/basic_test.go rulebooks/dnd5e/behavior/go.mod rulebooks/dnd5e/behavior/go.sum
git commit -m "feat(behavior): pursue remembered targets"
```

---

### Task 6: Propagate Remembered Knowledge and Corrections through Session

**Files:**
- Modify: `rulebooks/dnd5e/session/turndriver.go`
- Modify: `rulebooks/dnd5e/session/behavior.go`
- Modify: `rulebooks/dnd5e/session/types.go`
- Modify: `rulebooks/dnd5e/session/convert.go`
- Modify: `rulebooks/dnd5e/session/turn.go`
- Modify: `rulebooks/dnd5e/session/move.go`
- Modify: `rulebooks/dnd5e/session/write.go`
- Modify: `rulebooks/dnd5e/session/doors.go`
- Test: `rulebooks/dnd5e/session/seen_internal_test.go`
- Test: `rulebooks/dnd5e/session/convert_test.go`
- Test: `rulebooks/dnd5e/session/monster_turn_test.go`
- Test: `rulebooks/dnd5e/session/wirecontract_test.go`
- Modify: `rulebooks/dnd5e/session/go.mod`
- Modify: `rulebooks/dnd5e/session/go.sum`

**Interfaces:**
- Consumes: published encounter `RememberedMember` and `IntelDelta`, plus published behavior.
- Produces session-owned twins:

```go
type RememberedMember struct {
	ID            string
	Kind          MemberKind
	Position      spatial.Position
	DistanceCells float64
	Path          []spatial.Position
}

type LocationState string

const (
	LocationKnown   LocationState = "known"
	LocationUnknown LocationState = "unknown"
)

type IntelCorrection struct {
	Observer string `json:"observer"`
	Subject  string `json:"subject"`
}
```

Add `LocationState LocationState` to `Sighting` and `Remembered []RememberedMember` to session `MonsterView`.

Add test helpers in `monster_turn_test.go`:

```go
func heldUnknownSightHolding(t *testing.T, subject intel.Subject) intel.Holding
func storedEncounter(t *testing.T, repo *fakeEncounters, id string) *encounter.EncounterData
func requireKnownStoredLocation(t *testing.T, data *encounter.EncounterData, observer, subject string, want spatial.Position)
func requireUnknownStoredLocation(t *testing.T, data *encounter.EncounterData, observer, subject string)
```

`storedEncounter` calls the existing `fakeEncounters.GetEncounter`; the location helpers find the named persisted holding and use `encounter.DecodeLocationPayload`.

- [ ] **Step 1: Point local session development at published providers**

Use temporary local overrides only during development. Committed `go.mod` must reference published encounter and behavior versions.

- [ ] **Step 2: Write failing tests for both view adapters**

Test `projectMonsterView` and `unprojectMonsterView` independently:

```go
require.Equal(t, []RememberedMember{{
	ID: "billy", Kind: KindPlayer, Position: carpet,
	DistanceCells: 2, Path: []spatial.Position{step, carpet},
}}, projected.Remembered)
```

Mutate projected path slices and assert the source is unchanged. Round-trip through `session.Behavior()` and assert its Basic delegate returns the remembered path's first cell.

- [ ] **Step 3: Run adapter tests and confirm RED**

Working directory: `rulebooks/dnd5e/session`

```bash
go test ./... -run 'Test.*Remembered|TestBehavior.*Remembered' -count=1
```

Expected: compile failure because session-owned remembered types do not exist.

- [ ] **Step 4: Implement the remembered twins**

Copy remembered entries in both adapter directions and deep-copy every path. Do not expose encounter types, runtime objects, or core refs through the session contract.

- [ ] **Step 5: Write explicit-location projection tests**

Extend `seen_internal_test.go`:

```go
func TestHeldUnknownSightProjectsExplicitLocationState(t *testing.T) {
	h := heldUnknownSightHolding(t, "billy")
	got := projectSightings([]intel.Holding{h}, names, kinds, down)
	require.Equal(t, LocationUnknown, got[0].LocationState)
	require.Nil(t, got[0].Seen)
}
```

Known sight projects `LocationKnown` plus existing `Seen`. Add a load-path test proving malformed sight testimony returns `ErrInvalidWorld` before projection.

- [ ] **Step 6: Project encounter correction deltas**

Add:

```go
func projectIntelCorrections(in map[encounter.MemberID]*encounter.IntelDelta) []IntelCorrection
```

Sort by observer then subject. Add correction fields to every session output that already projects the matching encounter `IntelDeltas`, and to `EndTurnOutput`. Preserve discovery projection from `FirstContact`.

- [ ] **Step 7: Add load-act-save persistence tests**

Configure repository save failure after driven arrival and assert:

```go
require.ErrorIs(t, err, ErrSaveFailed)
stored := repository.MustLoadEncounter(encounterID)
requireKnownStoredLocation(t, stored, goblin, "billy", carpet)
require.Empty(t, publisher.Events())
```

Then run the success case and assert the stored holding is held unknown and `EndTurnOutput` contains the correction.

- [ ] **Step 8: Run session tests**

Working directory: `rulebooks/dnd5e/session`

```bash
go test -race ./... -count=1
```

Expected: PASS, including wire-contract and save-failure suites.

- [ ] **Step 9: Pin published dependencies and commit**

Remove local overrides, run `go mod tidy`, and inspect module diffs.

```bash
git add rulebooks/dnd5e/session/turndriver.go rulebooks/dnd5e/session/behavior.go rulebooks/dnd5e/session/types.go rulebooks/dnd5e/session/convert.go rulebooks/dnd5e/session/turn.go rulebooks/dnd5e/session/move.go rulebooks/dnd5e/session/write.go rulebooks/dnd5e/session/doors.go rulebooks/dnd5e/session/seen_internal_test.go rulebooks/dnd5e/session/convert_test.go rulebooks/dnd5e/session/monster_turn_test.go rulebooks/dnd5e/session/wirecontract_test.go rulebooks/dnd5e/session/go.mod rulebooks/dnd5e/session/go.sum
git commit -m "feat(session): project remembered monster knowledge"
```

---

### Task 7: Prove the Double-Door Behavior End to End

**Files:**
- Modify: `rulebooks/dnd5e/session/monster_turn_test.go`

**Interfaces:**
- Consumes: real `session.Behavior()`, encounter persistence, door geometry, and session EndTurn.
- Produces: discriminating issue-201 and visible-interruption proofs.

- [ ] **Step 1: Write the failing double-door scene**

Build `LEFT`, `CARPET`, and `RIGHT` regions with Door A and Door B. Wrap `session.Behavior()` with a recorder:

```go
type recordingBehavior struct {
	views []session.MonsterView
	next  session.TurnDriver
}

func (r *recordingBehavior) Act(view session.MonsterView) (session.TurnIntent, error) {
	r.views = append(r.views, cloneMonsterView(view))
	return r.next.Act(view)
}
```

Add the fixture helpers used by the assertions in the same test file:

```go
func cloneMonsterView(session.MonsterView) session.MonsterView
func requireRecordedView(t *testing.T, views []session.MonsterView, match func(session.MonsterView) bool)
func seen(session.MonsterView, string) bool
func remembered(session.MonsterView, string) bool
func rememberedAt(session.MonsterView, string, spatial.Position) bool
func requireNeverContainsPosition(t *testing.T, views []session.MonsterView, subject string, forbidden spatial.Position)
func persistedMonsterPosition(t *testing.T, repo *fakeEncounters, id string) spatial.Position
```

`cloneMonsterView` deep-copies `Seen`, `Remembered`, every path, and every `InReach` map. The assertion helpers iterate recorded plain data only; none reads a live encounter.

Assert milestones, not a fixed turn count:

```go
requireRecordedView(t, recorder.views, func(v session.MonsterView) bool {
	return rememberedAt(v, "billy", carpet) && !seen(v, "billy")
})
require.Equal(t, carpet, persistedMonsterPosition(t, repo))
requireUnknownStoredLocation(t, repo, goblin, "billy")
requireRecordedView(t, recorder.views, func(v session.MonsterView) bool {
	return !seen(v, "billy") && !remembered(v, "billy")
})
requireNeverContainsPosition(t, recorder.views, "billy", hiddenRightCell)
```

- [ ] **Step 2: Run the scene and confirm RED before integration**

Working directory: `rulebooks/dnd5e/session`

```bash
go test ./... -run 'Test.*DoubleDoor.*Ghost' -count=1 -v
```

Expected before Tasks 1–6: Basic passes after sight fades or the view lacks remembered data. Expected after Tasks 1–6: PASS.

- [ ] **Step 3: Add visible-interruption proof**

Arrange for David to become current sight after the first remembered-directed step. Assert the next recorded view contains David in `Seen`, the delegated move follows David's live path, and no view or intent contains Billy's concealed position.

- [ ] **Step 4: Run full session verification**

Working directory: `rulebooks/dnd5e/session`

```bash
go test -race ./... -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add rulebooks/dnd5e/session/monster_turn_test.go
git commit -m "test(session): prove monster corrects a ghost"
```

---

### Task 8: Update Sources of Truth and Verify

**Files:**
- Create: `docs/adr/0046-encounter-owns-location-knowledge.md`
- Modify: `docs/adr/DECISIONS.md`
- Modify: `rulebooks/dnd5e/encounter/turndriver.go`
- Modify: `rulebooks/dnd5e/encounter/doc.go`
- Modify: `rulebooks/dnd5e/session/types.go`
- Create: `docs/ideas/monster-ghost-pursuit/implementation.md`
- Modify: `docs/ideas/monster-ghost-pursuit/design.md`

**Interfaces:**
- Consumes: verified implementation, final APIs, module versions, and test evidence.
- Produces: ADR-0046 and observed implementation record.

- [ ] **Step 1: Write ADR-0046 from the shipped contract**

Record this decision and its consequences:

```text
Encounter owns typed Known(position)|Unknown location testimony.
play/intel remains opaque. Current sight and held memory are separate view
collections. Arrival correction is composition-authored testimony.

Legacy coordinates remain readable; canonical payloads are tagged; unknown
held testimony persists but is not actionable; session mirrors knowledge
without deciding it.
```

Add ADR-0046 to `docs/adr/DECISIONS.md`. Do not revise unrelated ADR history. Correct ADR-0043's stale status only in a separate documentation commit backed by `DECISIONS.md` and shipped code.

- [ ] **Step 2: Update godoc and implementation record**

Document remembered semantics, exact-cell paths, visible interruption, location compatibility, and correction deltas. Fill `implementation.md` with actual commit IDs, module versions, commands, and observed deviations. Change design status from `Approved` to `Implemented` only after verification succeeds.

- [ ] **Step 3: Verify changed modules independently**

In each of these working directories—`rulebooks/dnd5e/encounter`, `rulebooks/dnd5e/behavior`, then `rulebooks/dnd5e/session`—run:

```bash
go test -race ./... -count=1
golangci-lint run ./...
go mod tidy
git diff --exit-code -- go.mod go.sum
```

Expected: tests and lint exit 0; tidy produces no module-file diff after published dependency pins are installed.

- [ ] **Step 4: Run repository verification gates**

From repository root:

```bash
git diff --check
./scripts/verify.sh rulebooks/dnd5e/encounter
./scripts/verify.sh rulebooks/dnd5e/session
```

Then run the full gate documented in `docs/how-to/run-tests.md`. Expected: every command exits 0 with no generated or module-file changes.

- [ ] **Step 5: Audit final scope**

Confirm no `play/intel` change; no committed local override; no free-roam or public-Step correction; no sticky commitment, route utility, hiding, or sound/deception behavior; all public APIs have godoc; every acceptance criterion maps to a passing test.

- [ ] **Step 6: Commit source-of-truth updates**

```bash
git add docs/adr/0046-encounter-owns-location-knowledge.md docs/adr/DECISIONS.md rulebooks/dnd5e/encounter/turndriver.go rulebooks/dnd5e/encounter/doc.go rulebooks/dnd5e/session/types.go docs/ideas/monster-ghost-pursuit/design.md docs/ideas/monster-ghost-pursuit/implementation.md
git commit -m "docs: record monster ghost pursuit"
```

## Release and Dependency Order

The repository uses nested modules and no committed workspace override. Merge and publish inside-out:

1. Encounter provider changes and tag.
2. Behavior consumes the published encounter version, verifies, and tags.
3. Session consumes the published encounter and behavior versions, verifies, and tags.
4. Root documentation records actual versions and verification evidence.

If release authority is not included in the execution request, stop after verified local commits and present the exact tags and dependency updates Billy must authorize.
