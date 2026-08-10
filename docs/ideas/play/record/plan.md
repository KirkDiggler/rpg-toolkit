# play/record Implementation Plan (the HOW)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship `play/record` exactly as pinned by `design.md`: the append-only, sequence-ordered, audience-projected, tag-queryable story log.

**Architecture:** One concrete `Log`; entries immutable; envelope owns Seq/At/Correlation/Audience/Tags/Payload; explicit `TrimBefore` retention; family laws throughout (Input/Output + error, nil guards, no ctx, no dice, copy-out, `ToData`/`LoadLog`).

**Tech Stack:** Go 1.24 (sibling convention — CI's golangci-lint is built with go1.24), testify, repo `make` CI (auto-discovers modules), `compat.yml` gorelease gate.

**Normative source:** `docs/ideas/play/record/design.md` — on any disagreement, STOP and reconcile the design (with Kirk sign-off), never code around it.

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

**Working setup** (plain worktree; module isolation rules apply — only files under `play/record/` plus the one `compat.yml` edit in Task 6):

```bash
git -C /home/kirk/game-dev/rpg-toolkit fetch origin main
git -C /home/kirk/game-dev/rpg-toolkit worktree add /home/kirk/game-dev/.worktrees/toolkit-play-record -b feat/play-record origin/main
```

Family reference: the shipped `play/clock` module is the style/convention exemplar (license headers, doc comments, sentinel wrapping with `%w`, black-box suites). Commit per task, NO `--no-verify`, do NOT push until the final task.

---

### Task 1: Scaffold — `go.mod`, `doc.go`, `errors.go`

**Files (under `play/record/`):** Create `go.mod`, `go.sum`, `doc.go`, `errors.go`, `errors_test.go`

- [ ] **Step 1:**

```bash
mkdir -p /home/kirk/game-dev/.worktrees/toolkit-play-record/play/record
cd /home/kirk/game-dev/.worktrees/toolkit-play-record/play/record
go mod init github.com/KirkDiggler/rpg-toolkit/play/record
go get github.com/KirkDiggler/rpg-toolkit/core@v0.11.0
```
Then edit `go.mod`: directive `go 1.24`, and REMOVE any `toolchain` line
`go mod init` added (a 1.25.x local toolchain writes both; the clock
addenda hit this twice).

- [ ] **Step 2: `doc.go`** (license header as in play/clock):

```go
// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package record is the retained story: an append-only, sequence-ordered,
// audience-projected, tag-queryable log of opaque entries. Storage and
// query only — streaming the appends is the host's business; payloads and
// tag vocabularies belong to the composition; record never interprets.
//
// Design contract: docs/ideas/play/record/design.md (R1–R10). Leaf module:
// depends only on core, takes no context.Context, returns results as
// values, and never publishes.
package record
```

- [ ] **Step 3: `errors.go`** — seven sentinels, verbatim:

```go
// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package record

import "errors"

// Sentinel errors — the module's error vocabulary (design: Errors).
// All returned errors wrap exactly one of these; callers dispatch with
// errors.Is. Messages are user-facing.
var (
	// ErrNilInput reports a nil *XxxInput. Caller defect, dedicated sentinel.
	ErrNilInput = errors.New("nil input")
	// ErrBadAudience reports an audience with empty IDs or duplicates.
	ErrBadAudience = errors.New("invalid audience")
	// ErrBadTag reports a tag (or filter) key that is empty.
	ErrBadTag = errors.New("invalid tag key")
	// ErrNoPayload reports a nil payload (empty non-nil is legal).
	ErrNoPayload = errors.New("nil payload")
	// ErrBadSeq reports a trim point beyond NextSeq — you cannot forget the future.
	ErrBadSeq = errors.New("sequence out of range")
	// ErrNoViewer reports an empty viewer ID.
	ErrNoViewer = errors.New("empty viewer")
	// ErrInvalidData reports persisted state rejected by LoadLog (design R9).
	ErrInvalidData = errors.New("invalid log data")
)
```

- [ ] **Step 4:** smoke test (`errors_test.go`, black-box, license header): a single `TestSentinelsAreDistinct` asserting all seven sentinels non-nil and pairwise distinct via `errors.Is` (a compile-pin with real content, mirroring clock's vocabulary test upgrade).
- [ ] **Step 5:** `go mod tidy && go test ./... && golangci-lint run ./...` → clean. Commit `feat(play/record): module scaffold — sentinel vocabulary`.

---

### Task 2: `Log`, `Entry`, `Append`

**Files:** Create `log.go`, `log_test.go`

- [ ] **Step 1: failing tests** (`log_test.go`, `RecordSuite` testify suite, `SetupTest` creating `s.log, err = record.NewLog()`):

```go
func (s *RecordSuite) TestAppendAssignsGaplessSeqFromOne() {
	out, err := s.log.Append(&record.AppendInput{
		At: 7, Correlation: "act-1",
		Audience: []core.EntityID{"alice", "bob"},
		Tags:     map[string]string{"kind": "clock.turn_started", "actor": "alice"},
		Payload:  []byte(`{"x":1}`),
	})
	s.Require().NoError(err)
	s.Equal(uint64(1), out.Seq)
	out, err = s.log.Append(&record.AppendInput{Audience: []core.EntityID{"alice"}, Payload: []byte{}})
	s.Require().NoError(err)
	s.Equal(uint64(2), out.Seq)
	next, err := s.log.NextSeq()
	s.Require().NoError(err)
	s.Equal(uint64(3), next)
}

func (s *RecordSuite) TestAppendValidationOrderAndSentinels() {
	_, err := s.log.Append(nil)
	s.Require().ErrorIs(err, record.ErrNilInput)
	// multi-defect input: audience checked before tags before payload
	_, err = s.log.Append(&record.AppendInput{
		Audience: []core.EntityID{"a", "a"},
		Tags:     map[string]string{"": "x"},
		Payload:  nil,
	})
	s.Require().ErrorIs(err, record.ErrBadAudience)
	_, err = s.log.Append(&record.AppendInput{Tags: map[string]string{"": "x"}, Payload: nil})
	s.Require().ErrorIs(err, record.ErrBadTag)
	_, err = s.log.Append(&record.AppendInput{Payload: nil})
	s.Require().ErrorIs(err, record.ErrNoPayload)
	_, err = s.log.Append(&record.AppendInput{Audience: []core.EntityID{""}, Payload: []byte{}})
	s.Require().ErrorIs(err, record.ErrBadAudience)
	// R5: nothing changed
	next, err := s.log.NextSeq()
	s.Require().NoError(err)
	s.Equal(uint64(1), next)
}

func (s *RecordSuite) TestAppendNormalizesAndCopies() {
	aud := []core.EntityID{"alice"}
	tags := map[string]string{"k": "v"}
	_, err := s.log.Append(&record.AppendInput{Audience: aud, Tags: tags, Payload: []byte("p")})
	s.Require().NoError(err)
	aud[0] = "mutated"
	tags["k"] = "mutated"
	all, err := s.log.All(&record.AllInput{FromSeq: 1})
	s.Require().NoError(err)
	s.Equal([]core.EntityID{"alice"}, all[0].Audience)
	s.Equal(map[string]string{"k": "v"}, all[0].Tags)
	// empty-non-nil normalized to nil on store (family convention)
	_, err = s.log.Append(&record.AppendInput{Audience: []core.EntityID{}, Tags: map[string]string{}, Payload: []byte("q")})
	s.Require().NoError(err)
	all, err = s.log.All(&record.AllInput{FromSeq: 2})
	s.Require().NoError(err)
	s.Nil(all[0].Audience)
	s.Nil(all[0].Tags)
}
```

(Tests reference `All`/`NextSeq` — declare them in this task as minimal implementations so the suite compiles: `All` without filtering lands fully in Task 4; here it may return copy-out entries with no tag filter yet. Alternatively split assertions; the implementer may stage compilation however TDD requires, but every listed assertion must pass by end of Task 4 at the latest, and Steps below implement enough for them to pass NOW.)

- [ ] **Step 2:** verify FAIL (undefined types).
- [ ] **Step 3: implement** (`log.go`): internal `entry` mirror struct; `Log{entries []entry, nextSeq uint64}` (doc comment carries the family notes: "Not safe for concurrent use (design R10)"; "zero value not usable; construct via NewLog or LoadLog"); `NewLog() (*Log, error)` starting `nextSeq: 1`; `Entry` read-side struct per design; `AppendInput/AppendOutput`; `Append` with first-failure-wins validation (nil → Audience: dup/empty-ID check → Tags: empty-key check → Payload nil check), empty→nil normalization, defensive copies, seq assignment; `NextSeq() (uint64, error)`; a minimal `All(in *AllInput) ([]Entry, error)` (nil guard + copy-out, `FromSeq` honored, no tag filter yet) and the deep-copy helpers (`copyAudience`, `copyTags`, `copyEntryOut`).
- [ ] **Step 4:** suite green; full gate clean.
- [ ] **Step 5:** Commit `feat(play/record): Log + Append with validation order and normalization`.

---

### Task 3: `TrimBefore`

**Files:** Modify `log.go`; test in `log_test.go`

Helpers (declare in `log_test.go`): `appendN(n int)` appends n entries
with `s.Require().NoError`; `appendBeat(aud []core.EntityID, tags
map[string]string, payload string)` likewise; `seqs(es []record.Entry)
[]uint64` maps entries to their Seq values.

- [ ] **Step 1: failing tests:** append seqs 1..4 (via `appendN`); `TrimBefore{Seq: 3}` → `Removed: 2`, `All{FromSeq:1}` returns seqs `[3,4]` (never renumbered); `TrimBefore{Seq: 3}` again → `Removed: 0` (at/below oldest retained = no-op); `TrimBefore{Seq: 5}` (== NextSeq) → legal, empties, `Removed: 2`; `TrimBefore{Seq: 6}` (> NextSeq) → `ErrBadSeq`, state unchanged (R5: `NextSeq` still 5); `TrimBefore(nil)` → `ErrNilInput`; `TrimBefore{Seq: 1}` on a fresh log → `Removed: 0`, no error.
- [ ] **Step 2:** FAIL. **Step 3: implement** per design row. **Step 4:** gate. **Step 5:** Commit `feat(play/record): TrimBefore — retention as visible policy`.

---

### Task 4: Queries — tag filter, `SliceFor`, full `All`

**Files:** Modify `log.go`; test in `log_test.go`

- [ ] **Step 1: failing tests:**

```go
func (s *RecordSuite) TestSliceForProjectsAudienceAndTags() {
	s.appendBeat([]core.EntityID{"alice", "bob"}, map[string]string{"kind": "shared"}, "b1")
	s.appendBeat([]core.EntityID{"alice"}, map[string]string{"kind": "intel.first_contact", "subject": "door-3"}, "b2")
	s.appendBeat([]core.EntityID{"bob"}, map[string]string{"kind": "intel.first_contact", "subject": "door-3"}, "b3")
	s.appendBeat(nil, map[string]string{"kind": "gm.note"}, "b4") // empty audience: no viewer

	alice, err := s.log.SliceFor(&record.SliceForInput{Viewer: "alice", FromSeq: 1})
	s.Require().NoError(err)
	s.Equal([]uint64{1, 2}, seqs(alice))

	firsts, err := s.log.SliceFor(&record.SliceForInput{Viewer: "alice", FromSeq: 1,
		Tags: map[string]string{"kind": "intel.first_contact"}})
	s.Require().NoError(err)
	s.Equal([]uint64{2}, seqs(firsts))

	door, err := s.log.All(&record.AllInput{FromSeq: 1, Tags: map[string]string{"subject": "door-3"}})
	s.Require().NoError(err)
	s.Equal([]uint64{2, 3}, seqs(door))

	gm, err := s.log.All(&record.AllInput{FromSeq: 1, Tags: map[string]string{"kind": "gm.note"}})
	s.Require().NoError(err)
	s.Equal([]uint64{4}, seqs(gm))
	for _, v := range []core.EntityID{"alice", "bob"} {
		sl, serr := s.log.SliceFor(&record.SliceForInput{Viewer: v, FromSeq: 4})
		s.Require().NoError(serr)
		s.Empty(sl, "empty audience means no viewer")
	}
}
```

Plus: `All(nil)` and `SliceFor(nil)` → `ErrNilInput`; empty viewer → `ErrNoViewer`; empty filter key → `ErrBadTag` (and viewer-before-tags precedence: `{Viewer: "", Tags: {"": "x"}}` → `ErrNoViewer`); `All` with empty filter key → `ErrBadTag`; filter with empty VALUE matches only empty-valued tags (flags case); copy-out immunity (mutate returned entries' audience/tags/payload; internal state unchanged, re-query identical).

- [ ] **Steps 2-4:** FAIL → implement (`matchTags` helper: every filter k=v present exactly; `SliceFor` audience-contains + filter; `All` gains the filter; both `FromSeq`-bounded, Seq order, copy-out) → gate.
- [ ] **Step 5:** Commit `feat(play/record): SliceFor/All with tag question-surface`.

---

### Task 5: Persistence — `ToData` / `LoadLog`

**Files:** Modify `log.go` (or new `data.go`); test in `log_test.go`

- [ ] **Step 1: failing tests** covering the design's encoding convention exactly:
  - Fresh: `NewLog().ToData()` deep-equals zero `LogData`; marshals `{}`; `LoadLog(LogData{})` → fresh log, `NextSeq()` answers 1.
  - Trimmed-empty: append 2, trim at NextSeq → `ToData` = `{NextSeq: 3, Entries: nil}`; round-trips; next append gets Seq 3.
  - Populated round-trip: behavior-identical (append after reload continues the sequence; `SliceFor` identical).
  - **Post-trim POPULATED round-trip** (the natural-mistake catcher): append 4, `TrimBefore{3}` → snapshot has entries `[3,4]`, `NextSeq: 5`, first retained Seq > 1; `LoadLog` MUST accept (a contiguity check that demands Seq start at 1 fails here); behavior-identical after reload.
  - Golden JSON: populated log marshals to the exact expected string (`{"next_seq":3,"entries":[{"seq":1,...,"payload":"..."}...]}` — payloads base64 per encoding/json `[]byte`; tags keys sorted by the marshaler); trimmed-empty marshals `{"next_seq":3}`.
  - R9 rejection table (each `ErrInvalidData`): non-contiguous seqs `[1,3]`; zero seq; duplicate seqs; out-of-order seqs; non-empty with `NextSeq != last+1` (both directions); empty with `NextSeq: 1`; audience with empty ID; audience with duplicates; tag map with empty key; nil payload entry.
  - Snapshot immunity: `ToData`, then `Append`; snapshot unchanged. Load-side aliasing: mutate the caller's `LogData` after `LoadLog`; log unchanged.
- [ ] **Steps 2-4:** FAIL → implement (`LogData`/`EntryData` structs per design tags; `ToData` fresh-encodes NextSeq 1+empty as zero-value; `LoadLog` validates then deep-copies; empty→nil normalization preserved) → gate.
- [ ] **Step 5:** Commit `feat(play/record): persistence — fresh-log encoding, R9 validation, wire pin`.

---

### Task 6: AC1 spine + compat gate

**Files:** Create `story_test.go`; Modify `.github/workflows/compat.yml`

- [ ] **Step 1: the two-viewer story** (plain functions + require, the documented family exception): script the design's AC1 verbatim — shared beat; opener's audience-of-one big reveal (`kind: intel.first_contact`); follower's smaller reveal; GM-only beat; `SliceFor` each viewer asserts their exact story (seqs + payload order); unfiltered `All` returns everything; tag questions across mixed-shape beats (all first-contacts for the opener; everything tagged one subject; every `kind=clock.turn_started` via `All`); `TrimBefore` after "acknowledgment" then a `SliceFor` from an early `FromSeq` returns only retained entries with original seqs. Expected to PASS immediately; any failure is a regression — report, never bend expectations.
- [ ] **Step 2: compat.yml** — add `play/record/**` to the workflow `paths` and a `gorelease-play-record` job cloned from the clock job (heads-up: play/intel's plan touches the same file; whichever lands second rebases a one-hunk conflict) (same pinned gorelease version, `base=$(git tag -l 'play/record/v*' ...)`, cd `play/record`). Commit with explicit `git add .github/workflows/compat.yml play/record/` (the -a-misses-untracked lesson).
- [ ] **Step 3:** Commit `test(play/record): AC1 two-viewer story; ci: compat gate`.

---

### Task 7: Full gate + PR

- [ ] `go mod tidy && go test ./... -count=1 && golangci-lint run ./... && go vet ./... && gofmt -l .` all clean.
- [ ] Push `feat/play-record`; `gh pr create` ready-for-review: title `feat(play/record): the story log — Append/TrimBefore/SliceFor with tag question-surface`, body citing design.md, the AC evidence, family laws held (core-only, no ctx, 7 sentinels errors.Is-tested), signature footnote + attribution per repo convention.

---

## Execution notes

- Tasks strictly ordered. Per-task pipeline: fresh implementer + spec review + quality review, fix cycles, per the clock precedent.
- Test style: `RecordSuite` for per-type tests; `story_test.go` plain functions (documented exception).
- If lint fires on repeated string literals: function-local consts (file precedent), never nolint. Repo issue #904 background applies.
- Any deviation forced by implementation: STOP, design first.

## Execution addenda (logged during subagent-driven execution)

Fix cycles and deviations, recorded so the executed tree and this plan
stay in agreement (clock precedent):

- **With Task 2**: repeated literals resolved via function-local consts
  after an initial `nolint:goconst` reflex (the clock precedent holds:
  nolint never); error wrapping normalized to verb-context prefixes
  (`fmt.Errorf("append: %w", ...)`) with message pins.
- **With Task 4**: spec review caught `matchTags` treating a missing key
  as a match when the filter value was the empty string (`entryTags[k]
  != v` compares against the zero value); comma-ok fix with negative
  pins separating missing-key from empty-value.
- **With Task 5**: quality review probe confirmed `LoadLog` accepted
  `NextSeq: 0` alongside a `MaxUint64` entry — uint64 wraparound made
  the `NextSeq == lastSeq+1` contiguity check pass. Fixed by rejecting
  `NextSeq: 0` on the non-empty path (the fresh encoding is empty-only
  by construction; R9 wording tightened in spirit, not letter). The same
  review found immunity pins mutating via `append` — reallocation makes
  the pin theatrical; replaced with in-place index writes on both the
  `ToData` and `LoadLog` sides. Minors folded: nil-`*Log` method
  assertions, empty-non-nil `Entries` pins, empty-payload wire
  round-trip.
