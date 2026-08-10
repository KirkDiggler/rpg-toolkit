# play/interrupt Implementation Plan (the HOW)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship `play/interrupt` exactly as pinned by `design.md`: the ledger of open windows — `Pose`/`Answer` custody, projection queries, persistence. Suspension as a value; the module never interprets anything.

**Architecture:** One concrete `Ledger` (ordered window set, monotonic IDs); deltas returned in verb Outputs; family laws throughout; custody-not-execution as the module-specific law (R6).

**Tech Stack:** Go 1.24 (sibling convention), testify, repo `make` CI, `compat.yml` gorelease gate.

**Normative source:** `docs/ideas/play/interrupt/design.md` — on any disagreement, STOP and reconcile the design (with Kirk sign-off), never code around it.

**Docs-PR flow (the ratification rule):** this triplet's docs PR stays OPEN as the living review surface through implementation — amendments land on its branch as the build discovers them (the clock/intel precedent), and it merges as ratification alongside the module. Dispatches point implementers at the docs branch's copy of this triplet, never at origin/main.

**Working setup** (module isolation: only `play/interrupt/` files plus the one `compat.yml` edit in Task 6):

```bash
git -C /home/kirk/game-dev/rpg-toolkit fetch origin main
git -C /home/kirk/game-dev/rpg-toolkit worktree add /home/kirk/game-dev/.worktrees/toolkit-play-interrupt -b feat/play-interrupt origin/main
```

Family exemplars: shipped `play/clock`, `play/intel`, `play/record` (all v0.1.0 on main). License headers on every Go file; commit per task, NO `--no-verify`; do NOT push until the final task.

**Standing dispatch-brief standards (axis-two lessons, non-negotiable):** worktree-discipline header on every brief (name the ONE worktree; verify `pwd` before any git command); no unplanned scope (implement this task only — no "while I'm here"); the gate is ZERO issues (warnings-left-standing is the nolint reflex without the directive; `nolint` itself is forbidden — function-local consts for repeated literals); red CI on the PR is OURS to fix regardless of provenance; **mutation-proof pins from the first task** — any test that claims to pin a behavior must be shown to FAIL under the exact breakage it guards (temporarily introduce the breakage, run, watch the pin fail, revert, `git diff` clean) and the brief's report must include that evidence.

---

### Task 1: Scaffold — `go.mod`, `doc.go`, `errors.go`

**Files (under `play/interrupt/`):** Create `go.mod`, `go.sum`, `doc.go`, `errors.go`, `errors_test.go`

- [ ] **Step 1:** `go mod init github.com/KirkDiggler/rpg-toolkit/play/interrupt && go get github.com/KirkDiggler/rpg-toolkit/core@v0.11.0` (or the current latest core tag — check `git tag -l 'core/v*' --sort=-v:refname | head -1`); edit `go.mod` to directive `go 1.24` and REMOVE any `toolchain` line.
- [ ] **Step 2: `doc.go`:**

```go
// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package interrupt is the ledger of open windows: suspension-as-value
// custody for resolutions that stop and wait for an outside decision.
// Pose opens a window (one audience entity, offered option tokens, an
// opaque frozen payload); Answer validates, closes it, and returns the
// envelope for the rulebook to resume. The module never interprets an
// option, a choice, or a payload byte — custody, not execution. Human
// and machine deciders are indistinguishable here: an auto-taken
// reaction is an ordinary Pose answered immediately by composition.
//
// Design contract: docs/ideas/play/interrupt/design.md (R1–R10). Leaf
// module: depends only on core, no context.Context, deltas returned as
// values, never published.
package interrupt
```

- [ ] **Step 3: `errors.go`** — nine sentinels, verbatim:

```go
// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package interrupt

import "errors"

// Sentinel errors — the module's error vocabulary (design: Errors).
// All returned errors wrap exactly one of these; callers dispatch with
// errors.Is. Messages are user-facing.
var (
	// ErrNilInput reports a nil *XxxInput. Caller defect, dedicated sentinel.
	ErrNilInput = errors.New("nil input")
	// ErrNoAudience reports an empty audience entity ID (Pose.Audience,
	// Answer.By, PendingFor.Audience).
	ErrNoAudience = errors.New("empty audience")
	// ErrNoOptions reports a Pose with no options — an unanswerable
	// window would deadlock the encounter (the liveness guard).
	ErrNoOptions = errors.New("no options offered")
	// ErrNoOption reports an empty option token (in Pose options or as
	// an Answer choice).
	ErrNoOption = errors.New("empty option")
	// ErrDuplicateOption reports a repeated option token within one window.
	ErrDuplicateOption = errors.New("duplicate option")
	// ErrNotOpen reports no open window with that ID — unknown, never
	// posed, or already answered (one answer per window).
	ErrNotOpen = errors.New("window not open")
	// ErrNotAudience reports an answerer who is not the window's audience.
	ErrNotAudience = errors.New("not the window's audience")
	// ErrNotOffered reports a choice that is not among the window's options.
	ErrNotOffered = errors.New("choice not offered")
	// ErrInvalidData reports persisted state rejected by LoadLedger (design R9).
	ErrInvalidData = errors.New("invalid ledger data")
)
```

- [ ] **Step 4:** `errors_test.go` — black-box `package interrupt_test` (AC5) — `TestSentinelsAreDistinct` (all nine, pairwise `errors.Is`-distinct). **Step 5:** tidy/test/lint clean; commit `feat(play/interrupt): module scaffold — sentinel vocabulary`.

---

### Task 2: Types + `Pose`

**Files:** Create `ledger.go`, `ledger_test.go`

- [ ] **Step 1: failing tests** (`LedgerSuite`; `SetupTest`: `s.ledger, err = interrupt.NewLedger()`):

```go
func (s *LedgerSuite) TestPoseOpensWindow() {
	out, err := s.ledger.Pose(&interrupt.PoseInput{
		Audience: "aldric",
		Options:  []interrupt.Option{"shield", "decline"},
		Payload:  []byte("frozen-attack"),
		At:       7,
	})
	s.Require().NoError(err)
	s.Equal(interrupt.WindowID(1), out.Window.ID, "IDs are monotonic from 1")
	s.Equal(core.EntityID("aldric"), out.Window.Audience)
	s.Equal([]interrupt.Option{"shield", "decline"}, out.Window.Options)
	s.Equal([]byte("frozen-attack"), out.Window.Payload)
	s.Equal(uint64(7), out.Window.At)

	pending, err := s.ledger.PendingFor(&interrupt.PendingForInput{Audience: "aldric"})
	s.Require().NoError(err)
	s.Len(pending, 1)
	s.Equal(out.Window, pending[0])
}

func (s *LedgerSuite) TestPoseValidationOrderAndAtomicity() {
	// validation order: nil → audience → no options → empty option → duplicate
	_, err := s.ledger.Pose(nil)
	s.Require().ErrorIs(err, interrupt.ErrNilInput)
	_, err = s.ledger.Pose(&interrupt.PoseInput{Audience: "", Options: nil})
	s.Require().ErrorIs(err, interrupt.ErrNoAudience)
	_, err = s.ledger.Pose(&interrupt.PoseInput{Audience: "a", Options: nil})
	s.Require().ErrorIs(err, interrupt.ErrNoOptions)
	_, err = s.ledger.Pose(&interrupt.PoseInput{Audience: "a", Options: []interrupt.Option{"x", "", "x"}})
	s.Require().ErrorIs(err, interrupt.ErrNoOption, "empty token found before the duplicate")
	_, err = s.ledger.Pose(&interrupt.PoseInput{Audience: "a", Options: []interrupt.Option{"x", "y", "x"}})
	s.Require().ErrorIs(err, interrupt.ErrDuplicateOption)

	// R5: none of those consumed an ID or opened a window
	open, err := s.ledger.Open()
	s.Require().NoError(err)
	s.Empty(open)
	out, err := s.ledger.Pose(&interrupt.PoseInput{Audience: "a", Options: []interrupt.Option{"x"}})
	s.Require().NoError(err)
	s.Equal(interrupt.WindowID(1), out.Window.ID, "failed poses consume no IDs")
}

func (s *LedgerSuite) TestPoseNilPayloadIsLegal() { /* design: Types — payload may live elsewhere */ }
func (s *LedgerSuite) TestPoseCopyIn() {
	// mutate the caller's Options slice and Payload bytes after Pose;
	// re-query and assert the stored window is unchanged.
	// MUTATION-PROOF (standing standard): temporarily alias instead of
	// copying in the implementation, watch this fail, revert, git diff clean.
}
```

Plus: several windows for one audience coexist (R6 — the ledger is
policy-free); windows for different audiences isolated; `At` is recorded
verbatim and never compared (pose At=9 after At=2 — both stored,
untouched).

- [ ] **Step 2:** verify FAIL. **Step 3: implement** (`ledger.go`): `WindowID`/`Option` newtypes (doc-commented); `Window` struct per design; internal `window` storage struct + `Ledger{nextID uint64, windows []window}` (ordered slice IS pose order; lookup by linear scan — window counts are game-sized, no index map to keep consistent); `NewLedger() (*Ledger, error)`; `Pose` (validate in design order → assign ID → deep-copy in → append → return deep copy out); minimal `PendingFor`/`Open` so tests compile (hardened Task 4). **Step 4:** green + full gate. **Step 5:** commit `feat(play/interrupt): types + Pose — the ledger opens windows`.

---

### Task 3: `Answer` — custody's other half

**Files:** Modify `ledger.go`; test in `ledger_test.go`

- [ ] **Step 1: failing tests:**

```go
func (s *LedgerSuite) TestAnswerClosesAndReturnsEnvelope() {
	posed, err := s.ledger.Pose(&interrupt.PoseInput{Audience: "aldric",
		Options: []interrupt.Option{"shield", "decline"}, Payload: []byte("frozen"), At: 7})
	s.Require().NoError(err)
	out, err := s.ledger.Answer(&interrupt.AnswerInput{Window: posed.Window.ID, By: "aldric", Choice: "shield"})
	s.Require().NoError(err)
	s.Equal(posed.Window, out.Window, "the envelope returns intact — custody proven")
	s.Equal(interrupt.Option("shield"), out.Choice)
	pending, err := s.ledger.PendingFor(&interrupt.PendingForInput{Audience: "aldric"})
	s.Require().NoError(err)
	s.Empty(pending, "answered windows leave the ledger")
	// one answer per window
	_, err = s.ledger.Answer(&interrupt.AnswerInput{Window: posed.Window.ID, By: "aldric", Choice: "shield"})
	s.Require().ErrorIs(err, interrupt.ErrNotOpen)
}

func (s *LedgerSuite) TestAnswerValidationOrderAndAtomicity() {
	posed, _ := s.ledger.Pose(&interrupt.PoseInput{Audience: "aldric",
		Options: []interrupt.Option{"shield", "decline"}, Payload: []byte("frozen")})
	// shape → existence → authorization → membership; first failure wins
	_, err := s.ledger.Answer(nil)
	s.Require().ErrorIs(err, interrupt.ErrNilInput)
	_, err = s.ledger.Answer(&interrupt.AnswerInput{Window: posed.Window.ID, By: "", Choice: ""})
	s.Require().ErrorIs(err, interrupt.ErrNoAudience)
	_, err = s.ledger.Answer(&interrupt.AnswerInput{Window: posed.Window.ID, By: "aldric", Choice: ""})
	s.Require().ErrorIs(err, interrupt.ErrNoOption)
	_, err = s.ledger.Answer(&interrupt.AnswerInput{Window: 99, By: "aldric", Choice: "shield"})
	s.Require().ErrorIs(err, interrupt.ErrNotOpen)
	_, err = s.ledger.Answer(&interrupt.AnswerInput{Window: posed.Window.ID, By: "fighter", Choice: "shield"})
	s.Require().ErrorIs(err, interrupt.ErrNotAudience)
	_, err = s.ledger.Answer(&interrupt.AnswerInput{Window: posed.Window.ID, By: "aldric", Choice: "fireball"})
	s.Require().ErrorIs(err, interrupt.ErrNotOffered)
	// R5: every failure left the window open and unchanged
	pending, err := s.ledger.PendingFor(&interrupt.PendingForInput{Audience: "aldric"})
	s.Require().NoError(err)
	s.Require().Len(pending, 1)
	s.Equal(posed.Window, pending[0])
}
```

Plus: `WindowID(0)` answers `ErrNotOpen` (IDs start at 1 — 0 is
never-posed, not a shape error); answering one of several open windows
removes exactly that one, pose order of the rest preserved; envelope
copy-out (mutate the answer Output's payload/options; a still-open
sibling window and re-queries unaffected).

- [ ] **Step 2:** FAIL. **Step 3: implement** `Answer` per design (shape checks → linear-scan lookup → authorization → membership → splice out + deep-copy return). **Step 4:** green + gate. **Step 5:** commit `feat(play/interrupt): Answer — one answer per window, envelope returned`.

---

### Task 4: Queries hardened

**Files:** Modify `ledger.go`; test in `ledger_test.go`

- [ ] **Step 1: failing tests:** `PendingFor` returns only the audience's windows in ascending-ID order; unknown audience → empty slice, nil error; `PendingFor(nil)` → `ErrNilInput`; empty audience → `ErrNoAudience`; `Open()` returns everything in pose order (interleaved audiences — pose a/b/a, assert order 1,2,3); copy-out immunity for both queries (mutate returned windows' Options/Payload; internal state unchanged, re-query identical — MUTATION-PROOF the copy: alias temporarily, watch the pin fail, revert); the decider contract as executable documentation — a comment-annotated test in which a "monster" decider reads exactly `PendingFor("monster")` and answers through the ordinary verb.
- [ ] **Steps 2-4:** FAIL → implement → gate. **Step 5:** commit `feat(play/interrupt): queries — PendingFor/Open with copy-out discipline`.

---

### Task 5: Persistence — `ToData` / `LoadLedger`

**Files:** Create `data.go`; test in `ledger_test.go`

- [ ] **Step 1: failing tests:**
  - Idle convention: `NewLedger().ToData()` deep-equals zero `LedgerData`; marshals `{}`; `LoadLedger(LedgerData{})` usable.
  - Round-trips at every distinct state (design AC3): fresh; open windows (including one nil-payload window); all-answered (`NextID > 0`, no windows — MUST load; a `Pose` after reload continues the ID sequence, an `Answer` after reload returns the same envelope as pre-snapshot).
  - Golden JSON exact-string pins, all three shapes: fresh `{}`; open `{"next_id":3,"windows":[{"id":1,"audience":"aldric","options":["shield","decline"],"payload":"ZnJvemVu","at":7},{"id":2,"audience":"grunk","options":["attack","decline"]}]}` (note: window 2 pins the nil-payload + zero-At omitempty shape); all-answered `{"next_id":3}`.
  - Snapshot immunity + load-side aliasing (mutate the caller's `LedgerData` slices post-Load; ledger unaffected).
  - R9 rejections (each `ErrInvalidData`, per design): `NextID` 0 with windows present (the wraparound forgery — include the `math.MaxUint64` case per the record precedent); window ID 0; IDs not strictly ascending (both duplicate and descending cases); ID ≥ NextID; empty audience; nil options; empty options; empty option token; duplicate option. Nil payload LEGAL — design-forced: `Pose` accepts nil payloads, so they are reachable, and `payload` is `omitempty`, so `ToData` round-trips them back as nil — rejecting would make `LoadLedger` refuse the module's own snapshots (R8 violation). Cite this chain in the implementation comment.
  - **MUTATION-PROOF step (required, evidence in the task report):** each of the following breakages introduced temporarily MUST fail at least one test, then be reverted with `git diff` verified clean — (1) `ToData` deep-copy replaced by slice alias; (2) any wire tag renamed (`next_id` → `nextId`); (3) a bogus always-marshaled field added to `LedgerData`; (4) `LoadLedger` accepting `ID == NextID`.
- [ ] **Steps 2-4:** implement (`LedgerData`/`WindowData` per design tags; `ToData` normalizes empty slice → nil for the idle shape; `LoadLedger` validates in a deterministic order then deep-copies) → gate. **Step 5:** commit `feat(play/interrupt): persistence — R9 validation, mutation-proven wire pins`.

---

### Task 6: AC1 — the Shield scene + compat gate

**Files:** Create `shieldscene_test.go`; Modify `.github/workflows/compat.yml`

- [ ] **Step 1:** the design's AC1 verbatim as a plain-function test (family exception), fixture bytes standing in for the frozen resolution: freeze → `Pose` to Aldric (["shield","decline"], payload, At 7) → `PendingFor` renders the button → the fighter's answer rejected `ErrNotAudience` (window untouched) → "fireball" rejected `ErrNotOffered` → "shield" accepted, envelope byte-identical, ledger empty → the auto-OA beat (`Pose` + immediate policy `Answer`, no queries between — the ledger cannot tell) → the sequential beat (second window posed only after the first resolves; IDs strictly ascending). Full transcript asserted at each step with narrative-beat failure messages. Expected to PASS immediately; failures are regressions — report, never bend.
- [ ] **Step 2: compat.yml** — add `play/interrupt/**` to `paths` + `gorelease-play-interrupt` job cloned from the existing three (same pinned gorelease version). Explicit `git add .github/workflows/compat.yml play/interrupt/`.
- [ ] **Step 3:** commit `test(play/interrupt): AC1 shield scene; ci: compat gate`.

---

### Task 7: Full gate + PR

- [ ] `go mod tidy && go test ./... -count=1 && golangci-lint run ./... && go vet ./... && gofmt -l .` clean — and note toolkit#904: CI runs golangci-lint v2.3.1, local installs may be newer; local-clean is not CI-clean until versions agree (the intel prealloc lesson — prefer `make([]T, 0, n)` for append-only result slices).
- [ ] Push `feat/play-interrupt`; PR ready-for-review: `feat(play/interrupt): the ledger of open windows — Pose/Answer custody`, body citing design.md, AC evidence, family laws held, signature footnote + attribution. Drive CI green — red CI on this PR is ours regardless of provenance.

---

## Execution notes

- Task order is deliberate: Pose before Answer (window storage and
  validation land without removal semantics; Answer then adds exactly
  one concept), queries hardened before persistence pins them.
- Per-task pipeline: fresh implementer + spec review + quality review
  with fix cycles; Fable-tier review reserved for Task 5 (persistence
  encodings — the axis-two calibration).
- Mutation-proof is not a Task 5 special: EVERY aliasing/copy-out pin in
  Tasks 2–4 carries the same show-it-fails obligation.
- Any deviation forced by implementation: STOP, design first.

## Execution addenda (logged during subagent-driven execution)

Fix cycles and deviations, recorded so the executed tree and this plan
stay in agreement (family precedent):

- **With Task 2**: review's mutation sweep proved the suite survived a
  neutered `copyWindow` — implementation right, copy-out pin absent on
  all three read surfaces (the axis-two disease class, caught at Task 2
  this time by the standing mutation obligation). `TestCopyOutImmunity`
  added and mutation-proven; R5-from-populated-state pin added;
  `pending-for` wrap context conformed to family style; repeated test
  literals extracted to consts.
- **With Task 3 (design amendment: ownership transfer)**: review proved
  `TestAnswerEnvelopeCopyOut` unfalsifiable — after the splice nothing
  internal retains the window, so skipping Answer's defensive copy
  cannot corrupt ledger state and no test can make it fail. Resolved by
  amendment, not by keeping a lying pin: the design's Answer row now
  reads *ownership transfer* (queries copy out because internal state
  stays; Answer transfers because it removes), the dead copy and the
  can't-fail test were removed, and the review's second finding landed
  as a combo validation-order pin (wrong audience + unoffered choice →
  `ErrNotAudience` wins, authorization before membership).
- **With Task 5 (high-rigor persistence review, Opus tier)**: the
  implementation survived the full adversarial audit — an independent
  reachability derivation (15 reachable states load, 15 forgeries
  reject, plus a 6,000-state randomized walk with zero
  refuses-own-snapshot failures) and 40 of 43 mutations caught. The
  three survivors were all pin defects, the family's recurring disease:
  both aliasing pins were falsifiable only at window[0] (a
  copy-only-the-first-window mutant survived) — strengthened to
  mutate-every-window form and proven; and the dedicated
  NextID-0-with-windows branch was dead code (`ID >= NextID` subsumes
  it, wraparound included) with two pins no mutation of that line could
  break — branch deleted, state pins retained. Review minors landed in
  the same fix commit: `window[i]` context on every rejection (slice
  iteration is deterministic; intel's opacity justification does not
  transfer), all-answered snapshots emit nil `Windows` (deep-equal
  symmetric with caller literals, pinned), stored `NextID` 1 rejected as
  unreachable per the record precedent (design R9 amended), golden-fresh
  assertion arg order fixed.
