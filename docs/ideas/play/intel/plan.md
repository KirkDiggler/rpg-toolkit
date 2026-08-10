# play/intel Implementation Plan (the HOW)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship `play/intel` exactly as pinned by `design.md`: per-observer, channel-sourced, possibly-false holdings; `Surveil`/`Report` testimony; truth-blind by construction.

**Architecture:** One concrete `Intel` container (observer → subject → holding); per-channel currency sets; deltas returned in verb Outputs; family laws throughout.

**Tech Stack:** Go 1.24 (sibling convention), testify, repo `make` CI, `compat.yml` gorelease gate.

**Normative source:** `docs/ideas/play/intel/design.md` — on any disagreement, STOP and reconcile the design (with Kirk sign-off), never code around it.

**Docs-PR flow (the ratification rule):** PR #906 stays OPEN as the
living review surface through implementation — plans may change, and
amendments land on its branch as the build discovers them (the clock
precedent: its docs branch accumulated every mid-build amendment and
merged as ratification alongside the module). During the build, the
normative design and this plan live in the docs worktree at
`/home/kirk/game-dev/.worktrees/toolkit-axis2/docs/ideas/play/` —
dispatches point implementers there, never at origin/main. `doc.go`'s
design-contract path is a forward reference that resolves at
ratification; PR #906 merges just before or alongside this module's
implementation PR.

**Working setup** (module isolation: only `play/intel/` files plus the one `compat.yml` edit in Task 6):

```bash
git -C /home/kirk/game-dev/rpg-toolkit fetch origin main
git -C /home/kirk/game-dev/rpg-toolkit worktree add /home/kirk/game-dev/.worktrees/toolkit-play-intel -b feat/play-intel origin/main
```

Family exemplar: shipped `play/clock` (and `play/record` if merged first). License headers on every Go file; commit per task, NO `--no-verify`; do NOT push until the final task.

---

### Task 1: Scaffold — `go.mod`, `doc.go`, `errors.go`

**Files (under `play/intel/`):** Create `go.mod`, `go.sum`, `doc.go`, `errors.go`, `errors_test.go`

- [ ] **Step 1:** `go mod init github.com/KirkDiggler/rpg-toolkit/play/intel && go get github.com/KirkDiggler/rpg-toolkit/core@v0.11.0`; edit `go.mod` to directive `go 1.24` and REMOVE any `toolchain` line.
- [ ] **Step 2: `doc.go`:**

```go
// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package intel stores per-observer intel: channel-sourced, possibly
// false, possibly stale holdings about opaque subjects. Two testimony
// verbs — Surveil (sustained collection, complete-percept contract) and
// Report (discrete testimony) — plus read queries and persistence. The
// module never sees the world and cannot verify anything: illusions,
// disguises, and planted lies are ordinary testimony. Deciders consult
// HeldBy(themselves) and nothing else — monsters act on their intel,
// not the world.
//
// Design contract: docs/ideas/play/intel/design.md (R1–R10). Leaf
// module: depends only on core, no context.Context, deltas returned as
// values, never published.
package intel
```

- [ ] **Step 3: `errors.go`** — six sentinels, verbatim:

```go
// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package intel

import "errors"

// Sentinel errors — the module's error vocabulary (design: Errors).
// All returned errors wrap exactly one of these; callers dispatch with
// errors.Is. Messages are user-facing.
var (
	// ErrNilInput reports a nil *XxxInput. Caller defect, dedicated sentinel.
	ErrNilInput = errors.New("nil input")
	// ErrNoObserver reports an empty observer ID.
	ErrNoObserver = errors.New("empty observer")
	// ErrNoChannel reports an empty channel identifier — the vocabulary
	// is open, but the identifier is required.
	ErrNoChannel = errors.New("empty channel")
	// ErrNoSubject reports a report or query with an empty subject.
	ErrNoSubject = errors.New("empty subject")
	// ErrNotHeld reports that the observer holds nothing on that subject.
	ErrNotHeld = errors.New("nothing held on subject")
	// ErrInvalidData reports persisted state rejected by LoadIntel (design R9).
	ErrInvalidData = errors.New("invalid intel data")
)
```

- [ ] **Step 4:** `errors_test.go` — black-box `package intel_test` (AC5) — `TestSentinelsAreDistinct` (all six, pairwise `errors.Is`-distinct). **Step 5:** tidy/test/lint clean; commit `feat(play/intel): module scaffold — sentinel vocabulary`.

---

### Task 2: Types + `Report` (the simpler verb first)

**Files:** Create `intel.go`, `intel_test.go`

- [ ] **Step 1: failing tests** (`IntelSuite`; `SetupTest`: `s.intel, err = intel.NewIntel()`; helper `holdingOn(observer, subject)` wrapping `On` with `Require`):

```go
func (s *IntelSuite) TestReportLandsHeldIntel() {
	out, err := s.intel.Report(&intel.ReportInput{
		Observer: "alice", Channel: intel.Channel("hearing"), At: 5,
		Reports: []intel.Report{{Subject: "behind-door-3", Payload: []byte("crashing")}},
	})
	s.Require().NoError(err)
	s.Equal([]intel.Report{{Subject: "behind-door-3", Payload: []byte("crashing")}}, out.FirstContact)
	s.Empty(out.Updated)

	h := s.holdingOn("alice", "behind-door-3")
	s.Equal([]byte("crashing"), h.Payload)
	s.Equal(intel.Channel("hearing"), h.Channel)
	s.Equal(uint64(5), h.At)
	s.Nil(h.CurrentVia)
	s.Equal(intel.Held, h.Status)
}

func (s *IntelSuite) TestReportOverwritesLastWins() {
	_, err := s.intel.Report(&intel.ReportInput{Observer: "alice", Channel: "hearing", At: 5,
		Reports: []intel.Report{{Subject: "s", Payload: []byte("v1")}}})
	s.Require().NoError(err)
	out, err := s.intel.Report(&intel.ReportInput{Observer: "alice", Channel: "rumor", At: 9,
		Reports: []intel.Report{{Subject: "s", Payload: []byte("v2")}}})
	s.Require().NoError(err)
	s.Empty(out.FirstContact)
	s.Equal([]intel.Subject{"s"}, out.Updated)
	h := s.holdingOn("alice", "s")
	s.Equal([]byte("v2"), h.Payload)
	s.Equal(intel.Channel("rumor"), h.Channel) // provenance follows latest testimony
	s.Equal(uint64(9), h.At)
}

func (s *IntelSuite) TestReportDedupeAndValidationOrder() {
	// dedupe-first, last wins, survivor at last occurrence's position
	out, err := s.intel.Report(&intel.ReportInput{Observer: "a", Channel: "c", Reports: []intel.Report{
		{Subject: "x", Payload: []byte("old")}, {Subject: "y", Payload: []byte("y")},
		{Subject: "x", Payload: []byte("new")},
	}})
	s.Require().NoError(err)
	s.Equal([]intel.Report{{Subject: "y", Payload: []byte("y")}, {Subject: "x", Payload: []byte("new")}}, out.FirstContact)
	// validation order: nil → observer → channel → subjects; first failure wins; R5 no mutation
	_, err = s.intel.Report(nil)
	s.Require().ErrorIs(err, intel.ErrNilInput)
	_, err = s.intel.Report(&intel.ReportInput{Observer: "", Channel: "", Reports: []intel.Report{{Subject: ""}}})
	s.Require().ErrorIs(err, intel.ErrNoObserver)
	_, err = s.intel.Report(&intel.ReportInput{Observer: "a", Channel: "", Reports: []intel.Report{{Subject: ""}}})
	s.Require().ErrorIs(err, intel.ErrNoChannel)
	_, err = s.intel.Report(&intel.ReportInput{Observer: "a", Channel: "c", Reports: []intel.Report{{Subject: ""}}})
	s.Require().ErrorIs(err, intel.ErrNoSubject)
	held, err := s.intel.HeldBy(&intel.HeldByInput{Observer: "a"})
	s.Require().NoError(err)
	s.Len(held, 2, "failed calls changed nothing (R5)")
}
```

- [ ] **Step 2:** verify FAIL. **Step 3: implement** (`intel.go`): `Channel`/`Subject` string types (+ `Sight Channel = "sight"` predeclared, doc-commented); `Report` struct; internal `holding{payload []byte, channel Channel, at uint64, currentVia map[Channel]struct{}}`; `Intel{holdings map[core.EntityID]map[Subject]*holding}`; `NewIntel() (*Intel, error)`; `Status` type with `Current`/`Held` constants; exported `Holding` per design (CurrentVia as sorted `[]Channel`, Status derived); `dedupeReports` helper (last-wins, survivor at last position); `validate(observer, channel, reports)` in design order; `Report` verb (validate → dedupe → land: create with nil currentVia in FirstContact order, or overwrite payload/channel/at leaving currentVia untouched, in Updated); minimal `HeldBy` (sorted by Subject, copy-out) and `On` (`ErrNotHeld`) so tests compile — hardened in Task 4. All payloads copied in and out.
- [ ] **Step 4:** green + full gate. **Step 5:** commit `feat(play/intel): types + Report — testimony lands, last wins`.

---

### Task 3: `Surveil` — the currency machinery

**Files:** Modify `intel.go`; test in `intel_test.go`

- [ ] **Step 1: failing tests:**

```go
func (s *IntelSuite) TestSurveilFirstContactRefreshFade() {
	out, err := s.intel.Surveil(&intel.SurveilInput{Observer: "alice", Channel: intel.Sight, At: 1,
		Percept: []intel.Report{{Subject: "goblin-A", Payload: []byte("wounded")}, {Subject: "hex-1", Payload: []byte("floor")}}})
	s.Require().NoError(err)
	s.Len(out.FirstContact, 2)
	s.Empty(out.Refreshed)
	s.Empty(out.Faded)
	s.Equal(intel.Current, s.holdingOn("alice", "goblin-A").Status)
	s.Equal([]intel.Channel{intel.Sight}, s.holdingOn("alice", "goblin-A").CurrentVia)

	// next percept: goblin gone, hex remains → ghost goblin fades, hex refreshes
	out, err = s.intel.Surveil(&intel.SurveilInput{Observer: "alice", Channel: intel.Sight, At: 2,
		Percept: []intel.Report{{Subject: "hex-1", Payload: []byte("floor")}}})
	s.Require().NoError(err)
	s.Empty(out.FirstContact)
	s.Equal([]intel.Subject{"hex-1"}, out.Refreshed)
	s.Equal([]intel.Subject{"goblin-A"}, out.Faded)
	g := s.holdingOn("alice", "goblin-A")
	s.Equal(intel.Held, g.Status)
	s.Equal([]byte("wounded"), g.Payload, "the ghost goblin: held at last observation")
	s.Nil(g.CurrentVia)
}

func (s *IntelSuite) TestSurveilPerChannelCurrency() {
	for _, ch := range []intel.Channel{intel.Sight, "tremorsense"} {
		_, err := s.intel.Surveil(&intel.SurveilInput{Observer: "m", Channel: ch, At: 1,
			Percept: []intel.Report{{Subject: "prey", Payload: []byte("near")}}})
		s.Require().NoError(err)
	}
	s.Equal([]intel.Channel{intel.Sight, "tremorsense"}, s.holdingOn("m", "prey").CurrentVia)
	// sight loses it; tremorsense still sustains → NOT faded, still Current
	out, err := s.intel.Surveil(&intel.SurveilInput{Observer: "m", Channel: intel.Sight, At: 2, Percept: nil})
	s.Require().NoError(err)
	s.Empty(out.Faded)
	s.Equal(intel.Current, s.holdingOn("m", "prey").Status)
	s.Equal([]intel.Channel{"tremorsense"}, s.holdingOn("m", "prey").CurrentVia)
	// tremorsense loses it too → fades now
	out, err = s.intel.Surveil(&intel.SurveilInput{Observer: "m", Channel: "tremorsense", At: 3, Percept: nil})
	s.Require().NoError(err)
	s.Equal([]intel.Subject{"prey"}, out.Faded)
	s.Equal(intel.Held, s.holdingOn("m", "prey").Status)
}

func (s *IntelSuite) TestSurveilEmptyPerceptIsLegalAndFadesAll() { /* per design: seeing nothing */ }
func (s *IntelSuite) TestSurveilDoesNotDisturbReportedHoldings() {
	// a Report-held subject (nil CurrentVia) absent from a sight percept is NOT faded again
}
func (s *IntelSuite) TestReportOverwriteKeepsCurrency() {
	// Surveil sight → Current; then Report rumor on same subject:
	// payload/provenance change, CurrentVia and Status unchanged (design: Report row)
}
```

Plus: an At-blindness pin (design MUST NOT: later testimony carrying a SMALLER At still wins — overwrite happens, holding shows the smaller At as provenance; intel never orders by At); Surveil validation order + dedupe tests mirroring Report's; Faded sorted by Subject when multiple fade (three subjects, assert order); observer isolation (bob's surveil never touches alice's holdings).

- [ ] **Step 2:** FAIL. **Step 3: implement** `Surveil` per design: validate → dedupe → fade pass FIRST from pre-mutation state (subjects sustained via this channel, absent from the deduped percept: remove channel; if set empties, collect for Faded; sort Faded) → land pass in percept order (unknown → create with `currentVia={channel}`, FirstContact with copied payloads; known → overwrite payload/channel/at + add channel to set, Refreshed). Lazy map creation only after validation (R5).
- [ ] **Step 4:** green + gate. **Step 5:** commit `feat(play/intel): Surveil — complete-percept currency, the ghost goblin`.

---

### Task 4: Queries hardened

**Files:** Modify `intel.go`; test in `intel_test.go`

- [ ] **Step 1: failing tests:** `HeldBy` returns all holdings sorted by Subject with derived statuses; unknown observer → empty slice, nil error; `HeldBy(nil)`/`On(nil)` → `ErrNilInput`; empty observer → `ErrNoObserver` (both queries); `On` empty subject → `ErrNoSubject`; `On` unknown subject → `ErrNotHeld` (and unknown observer → `ErrNotHeld` too — nothing held); copy-out immunity both directions (mutate returned Holding payload/CurrentVia and the caller's input Report slices/payloads post-call; internal state unchanged, re-query identical); the decider contract as executable documentation — a comment-annotated test reading `HeldBy("monster")` only.
- [ ] **Steps 2-4:** FAIL → implement → gate. **Step 5:** commit `feat(play/intel): queries — HeldBy/On with copy-out discipline`.

---

### Task 5: Persistence — `ToData` / `LoadIntel`

**Files:** Create `data.go`; test in `intel_test.go`

- [ ] **Step 1: failing tests:**
  - Idle convention: `NewIntel().ToData()` deep-equals zero `IntelData`; marshals `{}`; `LoadIntel(IntelData{})` usable.
  - Round-trips at: mixed Current/Held; multi-channel CurrentVia; post-fade; Report-only holdings. Behavior-identical (a Surveil after reload fades/refreshes exactly as pre-snapshot).
  - Golden JSON: one populated holding marshals exactly (`{"holdings":{"alice":{"behind-door-3":{"payload":"...", "channel":"hearing","at":5}}}}` — payload base64; `current_via` omitted when nil; map keys sorted by the marshaler).
  - Snapshot immunity + load-side aliasing (mutate caller's IntelData post-Load).
  - R9 rejections (each `ErrInvalidData`): empty observer key; empty subject key; duplicate CurrentVia entries; empty channel in holding.Channel; empty channel inside CurrentVia; nil inner map; empty inner map; nil payload in HoldingData is LEGAL — design-forced, not a judgment call: (1) empty payloads are legal per Types; (2) verb validation checks only observer/channel/subject, so payload-less holdings are reachable, and R9 rejects unreachable states only; (3) `payload` is `omitempty`, so ToData round-trips every empty payload back as nil — rejecting it would make LoadIntel refuse the module's own snapshots (R8 violation). Cite this chain in the implementation comment. (Contrast: record rejects nil payloads because ITS design has ErrNoPayload, making them unreachable there.)
- [ ] **Steps 2-4:** implement (`IntelData`/`HoldingData` per design tags; `ToData` normalizes empty containers → nil/zero-value idle; `LoadIntel` validates in a deterministic order then deep-copies; CurrentVia slice → internal set) → gate. **Step 5:** commit `feat(play/intel): persistence — R9 validation, wire pin`.

---

### Task 6: AC1 — the door scene + compat gate

**Files:** Create `doorscene_test.go`; Modify `.github/workflows/compat.yml`

- [ ] **Step 1:** the design's AC1 verbatim as a plain-function test (family exception): hearing `Report` on `behind-door-3` ("crashing"); sight `Surveil` elsewhere establishes a goblin; goblin leaves the percept → Faded, ghost holds at last observation; the door opens — sight `Surveil` includes `behind-door-3` ("pots, floor") → the sound-holding is overwritten *because the composition aimed the same subject*; a `Report` on channel "charm" plants a false holding and `On` returns it faithfully. Assert the full delta transcript at each step and the final `HeldBy` in sorted order. Expected to PASS immediately; failures are regressions — report, never bend.
- [ ] **Step 2: compat.yml** — add `play/intel/**` to `paths` + `gorelease-play-intel` job cloned from clock's (pinned version). Heads-up: `play/record`'s plan touches the same file — whichever lands second rebases a one-hunk conflict. Explicit `git add .github/workflows/compat.yml play/intel/`.
- [ ] **Step 3:** commit `test(play/intel): AC1 door scene; ci: compat gate`.

---

### Task 7: Full gate + PR

- [ ] `go mod tidy && go test ./... -count=1 && golangci-lint run ./... && go vet ./... && gofmt -l .` clean.
- [ ] Push `feat/play-intel`; PR ready-for-review: `feat(play/intel): per-observer intel — Surveil/Report, the truth-blind store`, body citing design.md, AC evidence, family laws held, signature footnote + attribution.

---

## Execution notes

- Task order is deliberate: Report before Surveil (holdings/overwrite machinery lands without currency complexity; Surveil then adds exactly one concept).
- Per-task pipeline: fresh implementer + spec review + quality review with fix cycles.
- Lint on repeated literals: function-local consts, never nolint (toolkit#904 background).
- Any deviation forced by implementation: STOP, design first.

## Execution addenda (logged during subagent-driven execution)

Fix cycles and deviations, recorded so the executed tree and this plan
stay in agreement (clock precedent):

- **With Task 2**: spec review caught `FirstContact` sharing a backing
  array with the stored holding — a caller mutating the returned delta
  would corrupt belief; independent second copy with an aliasing pin.
  Phantom-observer guard added: failed validation must not materialize
  an observer map entry (R5 from the empty side).
- **With Task 3**: two incidents worth the record. (1) The implementing
  agent began building in the docs worktree — caught via editor
  diagnostics before any commit, corrected mid-flight, docs tree
  verified clean; worktree discipline is now a standing header in every
  dispatch brief. (2) The same agent ported an unplanned persistence
  implementation ("Task 5 while I'm here") with a pointer-taking
  `LoadIntel(data *IntelData)` (violates the family by-value law), an
  R8 idle failure (non-nil empty map), and zero tests; quality review
  ruled REMOVE — stripped via amend, the code saved outside the tree as
  a reference for Task 5's TDD. Eight `nolint:goconst` directives from
  the same commit replaced with function-local consts. Unplanned scope
  is now an explicit prohibition in dispatch briefs.
