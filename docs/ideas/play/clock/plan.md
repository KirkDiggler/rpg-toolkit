# play/clock Implementation Plan (the HOW)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship the `play/clock` leaf module — `Turn`, `Tick`, `Milestone`, `Transfer` — exactly as pinned by `design.md`, with the DOS2 acceptance spine green.

**Architecture:** Two concrete clock types with no shared clock interface; milestones returned as values; Input/Output signatures per design R3; persistence via the toolkit family `ToData`/`LoadTurn`/`LoadTick` pattern. Leaf law: depends on `core` and stdlib only, no `context.Context` anywhere.

**Tech Stack:** Go 1.24, testify suite (black-box `package clock_test`), golangci-lint, repo `make lint-all`/`test-all` CI (auto-discovers new modules).

**Normative source:** `docs/ideas/play/clock/design.md` — when this plan and the design disagree, the design wins; stop and reconcile rather than improvising.

**Working setup:** the main `rpg-toolkit` checkout may sit on someone's feature branch. Work in plain git worktrees:

```bash
git -C /home/kirk/game-dev/rpg-toolkit fetch origin main
git -C /home/kirk/game-dev/rpg-toolkit worktree add /home/kirk/game-dev/.worktrees/toolkit-core-entityid -b feat/core-entity-id origin/main
git -C /home/kirk/game-dev/rpg-toolkit worktree add /home/kirk/game-dev/.worktrees/toolkit-play-clock -b feat/play-clock origin/main
```

Module isolation rule: Task 1 (core) and Tasks 2+ (play/clock) are **separate PRs**; never touch the other module's files from either branch.

---

### Task 0: Spec amendment — `Transfer` takes `Membership`

While planning the R6 atomicity mechanics, the design's `TransferInput{From Leaver, To Joiner}` proved insufficient: the only rollback that needs no snapshots and no new Output fields is **join-first, compensating-leave**, and compensation requires `To` to also be a `Leaver`. Both concrete types implement both seams anyway; the amendment is a strict widening.

**Files:**
- Modify: `docs/ideas/play/clock/design.md` (Transfer section, on the `docs/journey-051-stage-reset` branch worktree at `/home/kirk/game-dev/.worktrees/toolkit-journey-051`)

- [ ] **Step 1: Amend the Transfer section.** Add above the `Transfer` table:

```markdown
`Transfer` requires both sides to speak both seams:

```go
type Membership interface {
    Leaver
    Joiner
}
```

`TransferInput` is `{From, To Membership, ID core.EntityID, Pos int}`.
Execution order is an implementation detail; the observable contract (R6:
both clocks unchanged on failure; milestones reported leave-then-join) is
what tests assert.
```

Replace the `Transfer` row's Input column with `{From, To Membership, ID EntityID, Pos int}`.

- [ ] **Step 2: Commit and push on the docs branch.**

```bash
cd /home/kirk/game-dev/.worktrees/toolkit-journey-051
git add docs/ideas/play/clock/design.md
git commit -m "docs(ideas): play/clock — Transfer takes Membership (join-first compensation needs To as Leaver)" --no-verify
git push
```

---

### Task 1: `core.EntityID` (separate PR, merges first)

**Files:**
- Modify: `/home/kirk/game-dev/.worktrees/toolkit-core-entityid/core/entity.go`
- Test: `/home/kirk/game-dev/.worktrees/toolkit-core-entityid/core/entity_test.go`

- [ ] **Step 1: Write the failing test** (append to `entity_test.go`, matching its existing style):

```go
func TestEntityIDIsStringNewtype(t *testing.T) {
	id := core.EntityID("goblin-1")
	if string(id) != "goblin-1" {
		t.Fatalf("EntityID round-trip: got %q", string(id))
	}
	var zero core.EntityID
	if zero != "" {
		t.Fatalf("zero EntityID should be empty string")
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd /home/kirk/game-dev/.worktrees/toolkit-core-entityid/core && go test ./... -run TestEntityIDIsStringNewtype`
Expected: FAIL — `undefined: core.EntityID`

- [ ] **Step 3: Add the type** to `core/entity.go`, directly above `EntityType`:

```go
// EntityID is the shared identity value for game entities across toolkit
// modules. Leaf modules (play/*) key members, subjects, and audiences by
// EntityID so composition layers never pay a conversion tax between
// module-local ID newtypes. Entity.GetID() remains string for
// compatibility; EntityID(entity.GetID()) bridges.
type EntityID string
```

- [ ] **Step 4: Run tests, lint**

Run: `cd /home/kirk/game-dev/.worktrees/toolkit-core-entityid/core && go test ./... && golangci-lint run ./...`
Expected: PASS, no lint findings (the doc comment above satisfies the exported-symbol rule).

- [ ] **Step 5: Commit, push, open PR (ready for review, not draft)**

```bash
cd /home/kirk/game-dev/.worktrees/toolkit-core-entityid
git add core/entity.go core/entity_test.go
git commit -m "feat(core): add EntityID shared identity newtype"
git push -u origin feat/core-entity-id
gh pr create --title "feat(core): add EntityID shared identity newtype" --body "Additive string newtype; shared identity vocabulary for the play/ leaf family per docs/ideas/play/clock/design.md (Prerequisite section). No behavior change."
```

**Gate:** this PR must be MERGED before Task 2 Step 1's `go get github.com/KirkDiggler/rpg-toolkit/core@main` can resolve the EntityID type. Do not run that command until it is.

---

### Task 2: Module scaffold — `go.mod`, `doc.go`, `errors.go`, `milestone.go`

**Files (all under `/home/kirk/game-dev/.worktrees/toolkit-play-clock/play/clock/`):**
- Create: `go.mod`, `doc.go`, `errors.go`, `milestone.go`, `milestone_test.go`

- [ ] **Step 1: Scaffold the module**

```bash
mkdir -p /home/kirk/game-dev/.worktrees/toolkit-play-clock/play/clock
cd /home/kirk/game-dev/.worktrees/toolkit-play-clock/play/clock
go mod init github.com/KirkDiggler/rpg-toolkit/play/clock
go get github.com/KirkDiggler/rpg-toolkit/core@main   # resolves to a pseudo-version containing EntityID
```

- [ ] **Step 2: Write `doc.go`**

```go
// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package clock provides scheduling policies for live play: who may act,
// and what advances time. Two concrete clocks — Turn (a localized
// initiative bubble) and Tick (the player-driven world clock) — plus the
// Milestone vocabulary their verbs return and the Transfer helper that
// moves an entity between clocks atomically.
//
// Design contract: docs/ideas/play/clock/design.md (R1–R10). This is a
// leaf module: it depends only on core, takes no context.Context, returns
// milestones as values, and never publishes.
package clock
```

- [ ] **Step 3: Write `errors.go`** — the nine sentinels, verbatim from the design's Errors table:

```go
package clock

import "errors"

// Sentinel errors — the module's state vocabulary (design: Errors).
// All returned errors wrap exactly one of these; callers dispatch with
// errors.Is. Messages are user-facing.
var (
	// ErrIdle reports a clock with no order set / nothing to act on.
	ErrIdle = errors.New("clock is idle")
	// ErrNotActive reports that the named actor is not the active entity.
	ErrNotActive = errors.New("not the active entity")
	// ErrNotMember reports an entity that is not in this clock.
	ErrNotMember = errors.New("entity is not a member of this clock")
	// ErrDuplicateMember reports an entity already present.
	ErrDuplicateMember = errors.New("entity is already a member of this clock")
	// ErrBadPosition reports an insert position outside [0, len].
	ErrBadPosition = errors.New("position out of range")
	// ErrBadOrder reports an empty order or a merge order that is not a
	// permutation of the union of both member sets.
	ErrBadOrder = errors.New("invalid order")
	// ErrBadAmount reports a non-positive spend or negative displacement.
	ErrBadAmount = errors.New("invalid amount")
	// ErrInsufficientBudget reports a spend exceeding the member's budget.
	ErrInsufficientBudget = errors.New("insufficient budget")
	// ErrInvalidData reports persisted state rejected by LoadTurn/LoadTick (design R9).
	ErrInvalidData = errors.New("invalid clock data")
)
```

- [ ] **Step 4: Write `milestone.go`**

```go
package clock

import "github.com/KirkDiggler/rpg-toolkit/core"

// MilestoneKind names a temporal boundary a clock verb crossed.
type MilestoneKind string

// The closed v1 milestone kind set (design: Milestone).
const (
	// TurnStarted marks Subject's turn beginning.
	TurnStarted MilestoneKind = "turn_started"
	// TurnEnded marks Subject's turn ending.
	TurnEnded MilestoneKind = "turn_ended"
	// RoundStarted marks a new round beginning on a Turn clock.
	RoundStarted MilestoneKind = "round_started"
	// Ticked marks a world-clock advance driven by Subject.
	Ticked MilestoneKind = "ticked"
	// MemberJoined marks Subject joining a clock.
	MemberJoined MilestoneKind = "member_joined"
	// MemberLeft marks Subject leaving a clock.
	MemberLeft MilestoneKind = "member_left"
	// Merged marks two Turn clocks combining.
	Merged MilestoneKind = "merged"
	// Dissolved marks a Turn clock emptying at fight end.
	Dissolved MilestoneKind = "dissolved"
)

// Milestone is a temporal boundary returned — never published — by the
// verb that caused it, in causal order (design R4). Turn-emitted
// milestones carry the clock's Round at the moment of emission.
type Milestone struct {
	Kind    MilestoneKind
	Subject core.EntityID // zero when not about a specific entity
	Round   int           // Turn clocks only; zero otherwise
}
```

- [ ] **Step 5: Compile-and-vocabulary test** (`milestone_test.go`, black-box):

```go
package clock_test

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/play/clock"
	"github.com/stretchr/testify/assert"
)

func TestMilestoneKindVocabularyIsClosed(t *testing.T) {
	kinds := []clock.MilestoneKind{
		clock.TurnStarted, clock.TurnEnded, clock.RoundStarted, clock.Ticked,
		clock.MemberJoined, clock.MemberLeft, clock.Merged, clock.Dissolved,
	}
	assert.Len(t, kinds, 8)
}
```

- [ ] **Step 6: Run, tidy, commit**

Run: `cd /home/kirk/game-dev/.worktrees/toolkit-play-clock/play/clock && go mod tidy && go test ./... && golangci-lint run ./...`
Expected: PASS, clean.

```bash
git add play/clock/
git commit -m "feat(play/clock): module scaffold — sentinels + milestone vocabulary"
```

---

### Task 3: `Turn` — construction, `SetOrder`, queries

**Files:**
- Create: `play/clock/turn.go`, `play/clock/turn_test.go`

- [ ] **Step 1: Write failing tests** (`turn_test.go`, testify suite — this suite grows through Tasks 3–7):

```go
package clock_test

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/play/clock"
	"github.com/stretchr/testify/suite"
)

type TurnSuite struct {
	suite.Suite
	turn *clock.Turn
}

func (s *TurnSuite) SetupTest() { s.turn = &clock.Turn{} }

func TestTurnSuite(t *testing.T) { suite.Run(t, new(TurnSuite)) }

func (s *TurnSuite) TestZeroValueIsIdle() {
	_, err := s.turn.Active()
	s.Require().ErrorIs(err, clock.ErrIdle)
	_, err = s.turn.Round()
	s.Require().ErrorIs(err, clock.ErrIdle)
	order, err := s.turn.Order()
	s.Require().NoError(err) // empty list is an answer
	s.Empty(order)
}

func (s *TurnSuite) TestSetOrderStartsRoundOne() {
	out, err := s.turn.SetOrder(&clock.SetOrderInput{Order: []core.EntityID{"a", "b", "c"}})
	s.Require().NoError(err)
	s.Equal([]clock.Milestone{
		{Kind: clock.RoundStarted, Round: 1},
		{Kind: clock.TurnStarted, Subject: "a", Round: 1},
	}, out.Milestones)
	active, err := s.turn.Active()
	s.Require().NoError(err)
	s.Equal(core.EntityID("a"), active)
	round, err := s.turn.Round()
	s.Require().NoError(err)
	s.Equal(1, round)
}

func (s *TurnSuite) TestSetOrderRejectsEmptyAndDuplicates() {
	_, err := s.turn.SetOrder(&clock.SetOrderInput{Order: nil})
	s.Require().ErrorIs(err, clock.ErrBadOrder)
	_, err = s.turn.SetOrder(&clock.SetOrderInput{Order: []core.EntityID{"a", "a"}})
	s.Require().ErrorIs(err, clock.ErrDuplicateMember)
	_, err = s.turn.Active() // still idle after both rejections (R5 atomicity)
	s.Require().ErrorIs(err, clock.ErrIdle)
}

func (s *TurnSuite) TestContains() {
	_, err := s.turn.SetOrder(&clock.SetOrderInput{Order: []core.EntityID{"a"}})
	s.Require().NoError(err)
	got, err := s.turn.Contains(&clock.ContainsInput{ID: "a"})
	s.Require().NoError(err)
	s.True(got)
	got, err = s.turn.Contains(&clock.ContainsInput{ID: "zz"})
	s.Require().NoError(err)
	s.False(got)
}
```

- [ ] **Step 2: Run to verify failure** — `go test ./... -run TestTurnSuite` → FAIL (undefined types).

- [ ] **Step 3: Implement** (`turn.go`):

```go
package clock

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
)

// Turn is a localized initiative bubble (design: Turn). The zero value is
// valid and idle. Not safe for concurrent use (design R10).
type Turn struct {
	order     []core.EntityID
	activeIdx int
	round     int
}

// SetOrderInput carries the rulebook-rolled initiative order.
type SetOrderInput struct {
	Order []core.EntityID
}

// SetOrderOutput reports the milestones SetOrder caused.
type SetOrderOutput struct {
	Milestones []Milestone
}

// SetOrder replaces the order, starting round 1 with the first member
// active. Errors: ErrBadOrder (empty), ErrDuplicateMember.
func (t *Turn) SetOrder(in *SetOrderInput) (*SetOrderOutput, error) {
	if len(in.Order) == 0 {
		return nil, fmt.Errorf("set order: empty: %w", ErrBadOrder)
	}
	seen := make(map[core.EntityID]struct{}, len(in.Order))
	for _, id := range in.Order {
		if _, dup := seen[id]; dup {
			return nil, fmt.Errorf("set order: %q appears twice: %w", id, ErrDuplicateMember)
		}
		seen[id] = struct{}{}
	}
	t.order = append([]core.EntityID(nil), in.Order...)
	t.activeIdx = 0
	t.round = 1
	return &SetOrderOutput{Milestones: []Milestone{
		{Kind: RoundStarted, Round: 1},
		{Kind: TurnStarted, Subject: t.order[0], Round: 1},
	}}, nil
}

// Active returns the entity whose turn it is; ErrIdle when no order is set.
func (t *Turn) Active() (core.EntityID, error) {
	if len(t.order) == 0 {
		return "", fmt.Errorf("active: %w", ErrIdle)
	}
	return t.order[t.activeIdx], nil
}

// Round returns the current round; ErrIdle when no order is set.
func (t *Turn) Round() (int, error) {
	if len(t.order) == 0 {
		return 0, fmt.Errorf("round: %w", ErrIdle)
	}
	return t.round, nil
}

// Order returns a copy of the current order. An idle clock answers with an
// empty slice and nil error — an empty list is an answer.
func (t *Turn) Order() ([]core.EntityID, error) {
	return append([]core.EntityID(nil), t.order...), nil
}

// ContainsInput names the entity being asked about.
type ContainsInput struct {
	ID core.EntityID
}

// Contains reports membership; false is an answer, never an error today.
func (t *Turn) Contains(in *ContainsInput) (bool, error) {
	return t.indexOf(in.ID) >= 0, nil
}

func (t *Turn) indexOf(id core.EntityID) int {
	for i, m := range t.order {
		if m == id {
			return i
		}
	}
	return -1
}
```

- [ ] **Step 4: Run to verify pass** — `go test ./... -run TestTurnSuite -v` → PASS.
- [ ] **Step 5: Commit** — `git add play/clock/ && git commit -m "feat(play/clock): Turn — SetOrder + queries"`

---

### Task 4: `Turn.End`

**Files:** Modify `play/clock/turn.go`; Test `play/clock/turn_test.go`

- [ ] **Step 1: Write failing tests:**

```go
func (s *TurnSuite) TestEndAdvancesAndWraps() {
	_, err := s.turn.SetOrder(&clock.SetOrderInput{Order: []core.EntityID{"a", "b"}})
	s.Require().NoError(err)

	out, err := s.turn.End(&clock.EndInput{Actor: "a"})
	s.Require().NoError(err)
	s.Equal(core.EntityID("b"), out.Next)
	s.False(out.RoundWrapped)
	s.Equal([]clock.Milestone{
		{Kind: clock.TurnEnded, Subject: "a", Round: 1},
		{Kind: clock.TurnStarted, Subject: "b", Round: 1},
	}, out.Milestones)

	out, err = s.turn.End(&clock.EndInput{Actor: "b"})
	s.Require().NoError(err)
	s.Equal(core.EntityID("a"), out.Next)
	s.True(out.RoundWrapped)
	// Wrap: TurnEnded carries the OLD round; RoundStarted/TurnStarted the NEW (design: Milestone).
	s.Equal([]clock.Milestone{
		{Kind: clock.TurnEnded, Subject: "b", Round: 1},
		{Kind: clock.RoundStarted, Round: 2},
		{Kind: clock.TurnStarted, Subject: "a", Round: 2},
	}, out.Milestones)
}

func (s *TurnSuite) TestEndErrors() {
	_, err := s.turn.End(&clock.EndInput{Actor: "a"})
	s.Require().ErrorIs(err, clock.ErrIdle)
	_, err = s.turn.SetOrder(&clock.SetOrderInput{Order: []core.EntityID{"a", "b"}})
	s.Require().NoError(err)
	_, err = s.turn.End(&clock.EndInput{Actor: "b"})
	s.Require().ErrorIs(err, clock.ErrNotActive)
	active, err := s.turn.Active() // unchanged (R5)
	s.Require().NoError(err)
	s.Equal(core.EntityID("a"), active)
}
```

- [ ] **Step 2: Run to verify failure.**
- [ ] **Step 3: Implement:**

```go
// EndInput names the actor ending their turn.
type EndInput struct {
	Actor core.EntityID
}

// EndOutput reports what End caused and who acts next.
type EndOutput struct {
	Milestones   []Milestone
	Next         core.EntityID
	RoundWrapped bool
}

// End advances past Actor's turn. Errors: ErrIdle, ErrNotActive (with no
// state change — R5).
func (t *Turn) End(in *EndInput) (*EndOutput, error) {
	if len(t.order) == 0 {
		return nil, fmt.Errorf("end turn: %w", ErrIdle)
	}
	active := t.order[t.activeIdx]
	if in.Actor != active {
		return nil, fmt.Errorf("end turn: %q is not the active entity (%q is): %w", in.Actor, active, ErrNotActive)
	}
	ms := []Milestone{{Kind: TurnEnded, Subject: active, Round: t.round}}
	t.activeIdx++
	wrapped := false
	if t.activeIdx >= len(t.order) {
		t.activeIdx = 0
		t.round++
		wrapped = true
		ms = append(ms, Milestone{Kind: RoundStarted, Round: t.round})
	}
	next := t.order[t.activeIdx]
	ms = append(ms, Milestone{Kind: TurnStarted, Subject: next, Round: t.round})
	return &EndOutput{Milestones: ms, Next: next, RoundWrapped: wrapped}, nil
}
```

- [ ] **Step 4: Run to verify pass.**
- [ ] **Step 5: Commit** — `git commit -am "feat(play/clock): Turn.End with round wrap"`

---

### Task 5: `Turn.Insert` / `Turn.Remove` — index correctness

**Files:** Modify `play/clock/turn.go`; Test `play/clock/turn_test.go`

- [ ] **Step 1: Write failing tests.** The active-preservation table is the heart of this task; every row asserts the active *entity* (not index) survives:

```go
func (s *TurnSuite) TestInsertKeepsActiveEntityActive() {
	cases := []struct {
		name string
		pos  int
	}{
		{"before active", 0},
		{"at active", 1},
		{"after active", 2},
		{"at end", 3},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			// fresh clock per subtest; advance so "b" (idx 1) is active
			s.turn = &clock.Turn{}
			_, err := s.turn.SetOrder(&clock.SetOrderInput{Order: []core.EntityID{"a", "b", "c"}})
			s.Require().NoError(err)
			_, err = s.turn.End(&clock.EndInput{Actor: "a"})
			s.Require().NoError(err)

			out, err := s.turn.Insert(&clock.InsertInput{ID: "x", Pos: tc.pos})
			s.Require().NoError(err)
			s.Equal([]clock.Milestone{{Kind: clock.MemberJoined, Subject: "x", Round: 1}}, out.Milestones)
			active, err := s.turn.Active()
			s.Require().NoError(err)
			s.Equal(core.EntityID("b"), active, "active entity must survive insert at pos %d", tc.pos)
		})
	}
}

func (s *TurnSuite) TestInsertErrors() {
	_, err := s.turn.Insert(&clock.InsertInput{ID: "x", Pos: 0})
	s.Require().ErrorIs(err, clock.ErrIdle)
	_, err = s.turn.SetOrder(&clock.SetOrderInput{Order: []core.EntityID{"a"}})
	s.Require().NoError(err)
	_, err = s.turn.Insert(&clock.InsertInput{ID: "a", Pos: 0})
	s.Require().ErrorIs(err, clock.ErrDuplicateMember)
	_, err = s.turn.Insert(&clock.InsertInput{ID: "x", Pos: 2})
	s.Require().ErrorIs(err, clock.ErrBadPosition)
	_, err = s.turn.Insert(&clock.InsertInput{ID: "x", Pos: -1})
	s.Require().ErrorIs(err, clock.ErrBadPosition)
}

func (s *TurnSuite) TestRemoveSemantics() {
	// design Remove row: non-active adjusts index; active advances
	// (MemberLeft, TurnStarted{next}, round unchanged even from last slot);
	// last member empties the clock (MemberLeft only).
	s.Run("non-active before active", func() {
		s.turn = &clock.Turn{}
		_, _ = s.turn.SetOrder(&clock.SetOrderInput{Order: []core.EntityID{"a", "b", "c"}})
		_, _ = s.turn.End(&clock.EndInput{Actor: "a"}) // b active
		out, err := s.turn.Remove(&clock.RemoveInput{ID: "a"})
		s.Require().NoError(err)
		s.Equal([]clock.Milestone{{Kind: clock.MemberLeft, Subject: "a", Round: 1}}, out.Milestones)
		active, _ := s.turn.Active()
		s.Equal(core.EntityID("b"), active)
	})
	s.Run("active mid-order", func() {
		s.turn = &clock.Turn{}
		_, _ = s.turn.SetOrder(&clock.SetOrderInput{Order: []core.EntityID{"a", "b", "c"}})
		_, _ = s.turn.End(&clock.EndInput{Actor: "a"}) // b active
		out, err := s.turn.Remove(&clock.RemoveInput{ID: "b"})
		s.Require().NoError(err)
		s.Equal([]clock.Milestone{
			{Kind: clock.MemberLeft, Subject: "b", Round: 1},
			{Kind: clock.TurnStarted, Subject: "c", Round: 1},
		}, out.Milestones)
	})
	s.Run("active in last slot: next is first, round unchanged", func() {
		s.turn = &clock.Turn{}
		_, _ = s.turn.SetOrder(&clock.SetOrderInput{Order: []core.EntityID{"a", "b"}})
		_, _ = s.turn.End(&clock.EndInput{Actor: "a"}) // b active, last slot
		out, err := s.turn.Remove(&clock.RemoveInput{ID: "b"})
		s.Require().NoError(err)
		s.Equal([]clock.Milestone{
			{Kind: clock.MemberLeft, Subject: "b", Round: 1},
			{Kind: clock.TurnStarted, Subject: "a", Round: 1},
		}, out.Milestones)
		round, _ := s.turn.Round()
		s.Equal(1, round)
	})
	s.Run("last member empties", func() {
		s.turn = &clock.Turn{}
		_, _ = s.turn.SetOrder(&clock.SetOrderInput{Order: []core.EntityID{"a"}})
		out, err := s.turn.Remove(&clock.RemoveInput{ID: "a"})
		s.Require().NoError(err)
		s.Equal([]clock.Milestone{{Kind: clock.MemberLeft, Subject: "a", Round: 1}}, out.Milestones)
		_, err = s.turn.Active()
		s.Require().ErrorIs(err, clock.ErrIdle)
	})
	s.Run("absent errors", func() {
		s.turn = &clock.Turn{}
		_, _ = s.turn.SetOrder(&clock.SetOrderInput{Order: []core.EntityID{"a"}})
		_, err := s.turn.Remove(&clock.RemoveInput{ID: "zz"})
		s.Require().ErrorIs(err, clock.ErrNotMember)
	})
}
```

- [ ] **Step 2: Run to verify failure.**
- [ ] **Step 3: Implement:**

```go
// InsertInput places a fall-in or reinforcement at a caller-chosen position.
type InsertInput struct {
	ID  core.EntityID
	Pos int
}

// InsertOutput reports the milestones Insert caused.
type InsertOutput struct {
	Milestones []Milestone
}

// Insert adds a member at Pos. Errors: ErrIdle (bubbles start via
// SetOrder), ErrDuplicateMember, ErrBadPosition. Inserting at or before
// the active position keeps the currently active entity active.
func (t *Turn) Insert(in *InsertInput) (*InsertOutput, error) {
	if len(t.order) == 0 {
		return nil, fmt.Errorf("insert %q: %w", in.ID, ErrIdle)
	}
	if t.indexOf(in.ID) >= 0 {
		return nil, fmt.Errorf("insert %q: %w", in.ID, ErrDuplicateMember)
	}
	if in.Pos < 0 || in.Pos > len(t.order) {
		return nil, fmt.Errorf("insert %q at %d (order length %d): %w", in.ID, in.Pos, len(t.order), ErrBadPosition)
	}
	t.order = append(t.order, "")
	copy(t.order[in.Pos+1:], t.order[in.Pos:])
	t.order[in.Pos] = in.ID
	if in.Pos <= t.activeIdx {
		t.activeIdx++
	}
	return &InsertOutput{Milestones: []Milestone{
		{Kind: MemberJoined, Subject: in.ID, Round: t.round},
	}}, nil
}

// RemoveInput names the member leaving (death, flight, transfer).
type RemoveInput struct {
	ID core.EntityID
}

// RemoveOutput reports the milestones Remove caused.
type RemoveOutput struct {
	Milestones []Milestone
}

// Remove drops a member, keeping the active entity correct (design: Turn
// verbs). Errors: ErrNotMember.
func (t *Turn) Remove(in *RemoveInput) (*RemoveOutput, error) {
	idx := t.indexOf(in.ID)
	if idx < 0 {
		return nil, fmt.Errorf("remove %q: %w", in.ID, ErrNotMember)
	}
	wasActive := idx == t.activeIdx
	t.order = append(t.order[:idx], t.order[idx+1:]...)
	ms := []Milestone{{Kind: MemberLeft, Subject: in.ID, Round: t.round}}
	switch {
	case len(t.order) == 0:
		t.activeIdx = 0
	case wasActive:
		if t.activeIdx >= len(t.order) {
			t.activeIdx = 0 // active was last; next is first, round unchanged
		}
		ms = append(ms, Milestone{Kind: TurnStarted, Subject: t.order[t.activeIdx], Round: t.round})
	case idx < t.activeIdx:
		t.activeIdx--
	}
	return &RemoveOutput{Milestones: ms}, nil
}
```

- [ ] **Step 4: Run to verify pass.**
- [ ] **Step 5: Commit** — `git commit -am "feat(play/clock): Turn Insert/Remove with active-entity preservation"`

---

### Task 6: `Turn.Merge` / `Turn.Dissolve`

**Files:** Modify `play/clock/turn.go`; Test `play/clock/turn_test.go`

- [ ] **Step 1: Write failing tests:**

```go
func (s *TurnSuite) TestMergeCombinesBubbles() {
	_, _ = s.turn.SetOrder(&clock.SetOrderInput{Order: []core.EntityID{"a", "b"}})
	_, _ = s.turn.End(&clock.EndInput{Actor: "a"}) // b active
	other := &clock.Turn{}
	_, _ = other.SetOrder(&clock.SetOrderInput{Order: []core.EntityID{"x", "y"}})

	out, err := s.turn.Merge(&clock.MergeInput{Other: other, Order: []core.EntityID{"x", "b", "y", "a"}})
	s.Require().NoError(err)
	s.Equal([]clock.Milestone{{Kind: clock.Merged, Round: 1}}, out.Milestones)
	active, _ := s.turn.Active()
	s.Equal(core.EntityID("b"), active, "receiving clock's active entity remains active")
	round, _ := s.turn.Round()
	s.Equal(1, round, "receiver's round retained")
	// Other reset to zero/idle state
	_, err = other.Active()
	s.Require().ErrorIs(err, clock.ErrIdle)
	otherOrder, _ := other.Order()
	s.Empty(otherOrder)
}

func (s *TurnSuite) TestMergeErrors() {
	other := &clock.Turn{}
	_, _ = other.SetOrder(&clock.SetOrderInput{Order: []core.EntityID{"x"}})
	_, err := s.turn.Merge(&clock.MergeInput{Other: other, Order: []core.EntityID{"x"}})
	s.Require().ErrorIs(err, clock.ErrIdle, "idle receiver refuses merge")

	_, _ = s.turn.SetOrder(&clock.SetOrderInput{Order: []core.EntityID{"a"}})
	_, err = s.turn.Merge(&clock.MergeInput{Other: other, Order: []core.EntityID{"a", "x", "ghost"}})
	s.Require().ErrorIs(err, clock.ErrBadOrder, "order must be an exact permutation of the union")
	activeAfter, _ := s.turn.Active()
	s.Equal(core.EntityID("a"), activeAfter, "failed merge changes nothing (R5)")
	otherOrder, _ := other.Order()
	s.Equal([]core.EntityID{"x"}, otherOrder, "failed merge leaves Other intact (R5)")
}

func (s *TurnSuite) TestDissolve() {
	_, err := s.turn.Dissolve(&clock.DissolveInput{})
	s.Require().ErrorIs(err, clock.ErrIdle, "dissolving an empty clock errors")

	_, _ = s.turn.SetOrder(&clock.SetOrderInput{Order: []core.EntityID{"a", "b"}})
	out, err := s.turn.Dissolve(&clock.DissolveInput{})
	s.Require().NoError(err)
	s.Equal([]core.EntityID{"a", "b"}, out.Members)
	s.Equal([]clock.Milestone{{Kind: clock.Dissolved, Round: 1}}, out.Milestones)
	_, err = s.turn.Active()
	s.Require().ErrorIs(err, clock.ErrIdle)
}
```

- [ ] **Step 2: Run to verify failure.**
- [ ] **Step 3: Implement:**

```go
// MergeInput combines Other into the receiver under a caller-supplied
// interleaved order (the rulebook decides how initiatives mesh).
type MergeInput struct {
	Other *Turn
	Order []core.EntityID
}

// MergeOutput reports the milestones Merge caused.
type MergeOutput struct {
	Milestones []Milestone
}

// Merge absorbs Other's members under Order, which must be a permutation
// of the union of both member sets. The receiver's active entity remains
// active and its round is retained; Other is reset to the zero/idle state.
// Errors: ErrIdle (idle receiver), ErrBadOrder.
func (t *Turn) Merge(in *MergeInput) (*MergeOutput, error) {
	if len(t.order) == 0 {
		return nil, fmt.Errorf("merge: receiver: %w", ErrIdle)
	}
	union := make(map[core.EntityID]struct{}, len(t.order)+len(in.Other.order))
	for _, id := range t.order {
		union[id] = struct{}{}
	}
	for _, id := range in.Other.order {
		union[id] = struct{}{}
	}
	if len(in.Order) != len(union) {
		return nil, fmt.Errorf("merge: order has %d entries, union has %d: %w", len(in.Order), len(union), ErrBadOrder)
	}
	seen := make(map[core.EntityID]struct{}, len(in.Order))
	for _, id := range in.Order {
		if _, ok := union[id]; !ok {
			return nil, fmt.Errorf("merge: %q is in neither clock: %w", id, ErrBadOrder)
		}
		if _, dup := seen[id]; dup {
			return nil, fmt.Errorf("merge: %q appears twice: %w", id, ErrBadOrder)
		}
		seen[id] = struct{}{}
	}
	active := t.order[t.activeIdx]
	t.order = append([]core.EntityID(nil), in.Order...)
	t.activeIdx = t.indexOf(active)
	in.Other.order = nil
	in.Other.activeIdx = 0
	in.Other.round = 0
	return &MergeOutput{Milestones: []Milestone{{Kind: Merged, Round: t.round}}}, nil
}

// DissolveInput is empty today; verbs keep their Input struct for
// additive evolution (design R3(b)).
type DissolveInput struct{}

// DissolveOutput returns the members for the composition to re-home.
type DissolveOutput struct {
	Members    []core.EntityID
	Milestones []Milestone
}

// Dissolve empties the clock at fight end. Errors: ErrIdle (already empty).
func (t *Turn) Dissolve(_ *DissolveInput) (*DissolveOutput, error) {
	if len(t.order) == 0 {
		return nil, fmt.Errorf("dissolve: %w", ErrIdle)
	}
	members := t.order
	round := t.round
	t.order = nil
	t.activeIdx = 0
	t.round = 0
	return &DissolveOutput{
		Members:    members,
		Milestones: []Milestone{{Kind: Dissolved, Round: round}},
	}, nil
}
```

- [ ] **Step 4: Run to verify pass.**
- [ ] **Step 5: Commit** — `git commit -am "feat(play/clock): Turn Merge/Dissolve"`

---

### Task 7: `Turn` persistence — `ToData` / `LoadTurn`

**Files:** Modify `play/clock/turn.go`; Test `play/clock/turn_test.go`

- [ ] **Step 1: Write failing tests:**

```go
func (s *TurnSuite) TestTurnRoundTrip() {
	_, _ = s.turn.SetOrder(&clock.SetOrderInput{Order: []core.EntityID{"a", "b", "c"}})
	_, _ = s.turn.End(&clock.EndInput{Actor: "a"})
	data := s.turn.ToData()
	loaded, err := clock.LoadTurn(data)
	s.Require().NoError(err)
	s.Equal(s.turn.ToData(), loaded.ToData())
	// behavior-identical: same next actor on End
	out, err := loaded.End(&clock.EndInput{Actor: "b"})
	s.Require().NoError(err)
	s.Equal(core.EntityID("c"), out.Next)
}

func (s *TurnSuite) TestLoadTurnRejectsInvalid() {
	cases := []struct {
		name string
		data clock.TurnData
	}{
		{"duplicate members", clock.TurnData{Order: []core.EntityID{"a", "a"}, Round: 1}},
		{"active idx out of range", clock.TurnData{Order: []core.EntityID{"a"}, ActiveIdx: 1, Round: 1}},
		{"negative active idx", clock.TurnData{Order: []core.EntityID{"a"}, ActiveIdx: -1, Round: 1}},
		{"idle with nonzero active idx", clock.TurnData{ActiveIdx: 2}},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			_, err := clock.LoadTurn(tc.data)
			s.Require().ErrorIs(err, clock.ErrInvalidData)
		})
	}
}

func (s *TurnSuite) TestRoundTripAfterMergeAndDissolve() {
	// design AC3: post-merge and post-dissolve are named round-trip states
	_, _ = s.turn.SetOrder(&clock.SetOrderInput{Order: []core.EntityID{"a", "b"}})
	other := &clock.Turn{}
	_, _ = other.SetOrder(&clock.SetOrderInput{Order: []core.EntityID{"x"}})
	_, err := s.turn.Merge(&clock.MergeInput{Other: other, Order: []core.EntityID{"a", "x", "b"}})
	s.Require().NoError(err)
	merged, err := clock.LoadTurn(s.turn.ToData())
	s.Require().NoError(err)
	s.Equal(s.turn.ToData(), merged.ToData())
	drained, err := clock.LoadTurn(other.ToData()) // drained Other is canonical idle
	s.Require().NoError(err)
	s.Equal(other.ToData(), drained.ToData())

	_, err = s.turn.Dissolve(&clock.DissolveInput{})
	s.Require().NoError(err)
	idle, err := clock.LoadTurn(s.turn.ToData()) // post-dissolve is canonical idle
	s.Require().NoError(err)
	s.Equal(s.turn.ToData(), idle.ToData())
}

func (s *TurnSuite) TestLoadTurnAcceptsCanonicalIdle() {
	loaded, err := clock.LoadTurn(clock.TurnData{})
	s.Require().NoError(err)
	_, err = loaded.Active()
	s.Require().ErrorIs(err, clock.ErrIdle)
}
```

- [ ] **Step 2: Run to verify failure.**
- [ ] **Step 3: Implement:**

```go
// TurnData is Turn's persisted shape (design R8). Plain data, no behavior.
type TurnData struct {
	Order     []core.EntityID `json:"order,omitempty"`
	ActiveIdx int             `json:"active_idx,omitempty"`
	Round     int             `json:"round,omitempty"`
}

// ToData snapshots the clock. Family-convention exemption from R3 (design).
func (t *Turn) ToData() TurnData {
	return TurnData{
		Order:     append([]core.EntityID(nil), t.order...),
		ActiveIdx: t.activeIdx,
		Round:     t.round,
	}
}

// LoadTurn reconstructs a Turn from persisted state. A constructor, not a
// verb — no milestones. Errors: ErrInvalidData for every R9 rejection.
func LoadTurn(data TurnData) (*Turn, error) {
	if len(data.Order) == 0 {
		if data.ActiveIdx != 0 {
			return nil, fmt.Errorf("load turn: idle clock with active idx %d: %w", data.ActiveIdx, ErrInvalidData)
		}
		return &Turn{round: data.Round}, nil
	}
	seen := make(map[core.EntityID]struct{}, len(data.Order))
	for _, id := range data.Order {
		if _, dup := seen[id]; dup {
			return nil, fmt.Errorf("load turn: duplicate member %q: %w", id, ErrInvalidData)
		}
		seen[id] = struct{}{}
	}
	if data.ActiveIdx < 0 || data.ActiveIdx >= len(data.Order) {
		return nil, fmt.Errorf("load turn: active idx %d out of range [0,%d): %w", data.ActiveIdx, len(data.Order), ErrInvalidData)
	}
	return &Turn{
		order:     append([]core.EntityID(nil), data.Order...),
		activeIdx: data.ActiveIdx,
		round:     data.Round,
	}, nil
}
```

- [ ] **Step 4: Run to verify pass.**
- [ ] **Step 5: Commit** — `git commit -am "feat(play/clock): Turn persistence with R9 validation"`

---

### Task 8: `Tick` — construction, `Join`, `Leave`

**Files:**
- Create: `play/clock/tick.go`, `play/clock/tick_test.go`

- [ ] **Step 1: Write failing tests** (new `TickSuite`, same conventions). Deliberately no `core` import yet — these tests never reference it (untyped string constants convert), Go hard-errors on unused imports, and Task 9 adds it with its first real use:

```go
package clock_test

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/play/clock"
	"github.com/stretchr/testify/suite"
)

type TickSuite struct {
	suite.Suite
	tick *clock.Tick
}

func (s *TickSuite) SetupTest() {
	var err error
	s.tick, err = clock.NewTick()
	s.Require().NoError(err)
}

func TestTickSuite(t *testing.T) { suite.Run(t, new(TickSuite)) }

func (s *TickSuite) TestJoinLeave() {
	out, err := s.tick.Join(&clock.JoinInput{ID: "goblin"})
	s.Require().NoError(err)
	s.Equal([]clock.Milestone{{Kind: clock.MemberJoined, Subject: "goblin"}}, out.Milestones)
	b, err := s.tick.Budget(&clock.BudgetInput{ID: "goblin"})
	s.Require().NoError(err)
	s.Zero(b)

	_, err = s.tick.Join(&clock.JoinInput{ID: "goblin"})
	s.Require().ErrorIs(err, clock.ErrDuplicateMember)

	lout, err := s.tick.Leave(&clock.LeaveInput{ID: "goblin"})
	s.Require().NoError(err)
	s.Equal([]clock.Milestone{{Kind: clock.MemberLeft, Subject: "goblin"}}, lout.Milestones)
	_, err = s.tick.Budget(&clock.BudgetInput{ID: "goblin"})
	s.Require().ErrorIs(err, clock.ErrNotMember)
	_, err = s.tick.Leave(&clock.LeaveInput{ID: "goblin"})
	s.Require().ErrorIs(err, clock.ErrNotMember)
}

func (s *TickSuite) TestMembersEmptySliceConvention() {
	members, err := s.tick.Members()
	s.Require().NoError(err)
	s.Empty(members)
}
```

- [ ] **Step 2: Run to verify failure.**
- [ ] **Step 3: Implement** (`tick.go`):

```go
package clock

import (
	"fmt"
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/core"
)

// Tick is the player-driven world clock (design: Tick): it advances only
// because players act. Accrual is high-water max-displacement. Construct
// via NewTick or LoadTick; the zero value is not usable. Not safe for
// concurrent use (design R10).
type Tick struct {
	budgets        map[core.EntityID]int
	driverProgress map[core.EntityID]int
	highWater      int
}

// NewTick constructs a valid, idle world clock. The error return conforms
// to design R3(a); construction cannot fail today, but a future
// TickConfig can.
func NewTick() (*Tick, error) {
	return &Tick{
		budgets:        make(map[core.EntityID]int),
		driverProgress: make(map[core.EntityID]int),
	}, nil
}

// JoinInput names the entity joining the world clock.
type JoinInput struct {
	ID core.EntityID
}

// JoinOutput reports the milestones Join caused.
type JoinOutput struct {
	Milestones []Milestone
}

// Join adds a member at budget 0. Errors: ErrDuplicateMember.
func (k *Tick) Join(in *JoinInput) (*JoinOutput, error) {
	if _, ok := k.budgets[in.ID]; ok {
		return nil, fmt.Errorf("join %q: %w", in.ID, ErrDuplicateMember)
	}
	k.budgets[in.ID] = 0
	return &JoinOutput{Milestones: []Milestone{{Kind: MemberJoined, Subject: in.ID}}}, nil
}

// LeaveInput names the entity leaving the world clock.
type LeaveInput struct {
	ID core.EntityID
}

// LeaveOutput reports the milestones Leave caused.
type LeaveOutput struct {
	Milestones []Milestone
}

// Leave removes a member and its budget. Errors: ErrNotMember.
func (k *Tick) Leave(in *LeaveInput) (*LeaveOutput, error) {
	if _, ok := k.budgets[in.ID]; !ok {
		return nil, fmt.Errorf("leave %q: %w", in.ID, ErrNotMember)
	}
	delete(k.budgets, in.ID)
	return &LeaveOutput{Milestones: []Milestone{{Kind: MemberLeft, Subject: in.ID}}}, nil
}

// BudgetInput names the member whose budget is being read.
type BudgetInput struct {
	ID core.EntityID
}

// Budget returns the member's current budget. Errors: ErrNotMember —
// never an ambiguous zero.
func (k *Tick) Budget(in *BudgetInput) (int, error) {
	b, ok := k.budgets[in.ID]
	if !ok {
		return 0, fmt.Errorf("budget %q: %w", in.ID, ErrNotMember)
	}
	return b, nil
}

// Members returns the member set in stable (sorted) order. An empty clock
// answers with an empty slice and nil error.
func (k *Tick) Members() ([]core.EntityID, error) {
	out := make([]core.EntityID, 0, len(k.budgets))
	for id := range k.budgets {
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out, nil
}

// Contains reports membership; false is an answer.
func (k *Tick) Contains(in *ContainsInput) (bool, error) {
	_, ok := k.budgets[in.ID]
	return ok, nil
}
```

- [ ] **Step 4: Run to verify pass.**
- [ ] **Step 5: Commit** — `git commit -am "feat(play/clock): Tick membership + queries"`

---

### Task 9: `Tick.Advance` (high-water) / `Tick.Spend`

**Files:** Modify `play/clock/tick.go`; Test `play/clock/tick_test.go`

- [ ] **Step 1: Write failing tests.** First add `"github.com/KirkDiggler/rpg-toolkit/core"` to `tick_test.go`'s imports (these tests are its first use). The 4×3→3 example from the design is the named headline test:

```go
func (s *TickSuite) TestAdvanceHighWaterMaxNotSum() {
	_, _ = s.tick.Join(&clock.JoinInput{ID: "goblin"})
	// four players each report displacement 3: members accrue 3, not 12
	for _, driver := range []core.EntityID{"p1", "p2", "p3", "p4"} {
		out, err := s.tick.Advance(&clock.AdvanceInput{Driver: driver, Displacement: 3})
		s.Require().NoError(err)
		s.Equal([]clock.Milestone{{Kind: clock.Ticked, Subject: driver}}, out.Milestones)
	}
	b, _ := s.tick.Budget(&clock.BudgetInput{ID: "goblin"})
	s.Equal(3, b)

	// p1 pulls ahead by 2 more (cumulative 5): delta 2 granted
	out, err := s.tick.Advance(&clock.AdvanceInput{Driver: "p1", Displacement: 2})
	s.Require().NoError(err)
	s.Equal([]core.EntityID{"goblin"}, out.Ready)
	b, _ = s.tick.Budget(&clock.BudgetInput{ID: "goblin"})
	s.Equal(5, b)
}

func (s *TickSuite) TestAdvanceEdges() {
	_, _ = s.tick.Join(&clock.JoinInput{ID: "goblin"})
	// zero displacement: legal, Ticked emitted, no grant
	out, err := s.tick.Advance(&clock.AdvanceInput{Driver: "p1", Displacement: 0})
	s.Require().NoError(err)
	s.Empty(out.Ready)
	// negative rejected, nothing changed (R5)
	_, err = s.tick.Advance(&clock.AdvanceInput{Driver: "p1", Displacement: -1})
	s.Require().ErrorIs(err, clock.ErrBadAmount)
	b, _ := s.tick.Budget(&clock.BudgetInput{ID: "goblin"})
	s.Zero(b)
	// a member joining late starts at 0 and accrues only future deltas
	_, _ = s.tick.Advance(&clock.AdvanceInput{Driver: "p1", Displacement: 4})
	_, _ = s.tick.Join(&clock.JoinInput{ID: "late"})
	_, _ = s.tick.Advance(&clock.AdvanceInput{Driver: "p1", Displacement: 1})
	late, _ := s.tick.Budget(&clock.BudgetInput{ID: "late"})
	s.Equal(1, late)
}

func (s *TickSuite) TestSpend() {
	_, _ = s.tick.Join(&clock.JoinInput{ID: "goblin"})
	_, _ = s.tick.Advance(&clock.AdvanceInput{Driver: "p1", Displacement: 3})
	out, err := s.tick.Spend(&clock.SpendInput{ID: "goblin", Amount: 2})
	s.Require().NoError(err)
	s.Empty(out.Milestones) // Spend emits no milestones (closed kind set)
	b, _ := s.tick.Budget(&clock.BudgetInput{ID: "goblin"})
	s.Equal(1, b)

	_, err = s.tick.Spend(&clock.SpendInput{ID: "goblin", Amount: 2})
	s.Require().ErrorIs(err, clock.ErrInsufficientBudget)
	_, err = s.tick.Spend(&clock.SpendInput{ID: "goblin", Amount: 0})
	s.Require().ErrorIs(err, clock.ErrBadAmount)
	_, err = s.tick.Spend(&clock.SpendInput{ID: "zz", Amount: 1})
	s.Require().ErrorIs(err, clock.ErrNotMember)
}
```

- [ ] **Step 2: Run to verify failure.**
- [ ] **Step 3: Implement:**

```go
// AdvanceInput reports a driver's displacement since their last report.
// Units are the caller's; the clock never interprets them.
type AdvanceInput struct {
	Driver       core.EntityID
	Displacement int
}

// AdvanceOutput reports the tick and which members now have budget.
type AdvanceOutput struct {
	Milestones []Milestone
	Ready      []core.EntityID
}

// Advance records the driver's cumulative displacement; when it raises the
// high-water mark, the delta is granted to every member's budget
// (max-not-sum fairness — design: Tick). Drivers need not be members.
// Errors: ErrBadAmount (negative displacement).
func (k *Tick) Advance(in *AdvanceInput) (*AdvanceOutput, error) {
	if in.Displacement < 0 {
		return nil, fmt.Errorf("advance %q by %d: %w", in.Driver, in.Displacement, ErrBadAmount)
	}
	k.driverProgress[in.Driver] += in.Displacement
	if p := k.driverProgress[in.Driver]; p > k.highWater {
		delta := p - k.highWater
		k.highWater = p
		for id := range k.budgets {
			k.budgets[id] += delta
		}
	}
	ready := make([]core.EntityID, 0, len(k.budgets))
	for id, b := range k.budgets {
		if b > 0 {
			ready = append(ready, id)
		}
	}
	sort.Slice(ready, func(i, j int) bool { return ready[i] < ready[j] })
	return &AdvanceOutput{
		Milestones: []Milestone{{Kind: Ticked, Subject: in.Driver}},
		Ready:      ready,
	}, nil
}

// SpendInput deducts from a member's budget.
type SpendInput struct {
	ID     core.EntityID
	Amount int
}

// SpendOutput reports the milestones Spend caused (none in the v1 kind set).
type SpendOutput struct {
	Milestones []Milestone
}

// Spend deducts Amount from the member's budget. Errors: ErrNotMember,
// ErrBadAmount (non-positive), ErrInsufficientBudget.
func (k *Tick) Spend(in *SpendInput) (*SpendOutput, error) {
	b, ok := k.budgets[in.ID]
	if !ok {
		return nil, fmt.Errorf("spend %q: %w", in.ID, ErrNotMember)
	}
	if in.Amount <= 0 {
		return nil, fmt.Errorf("spend %q amount %d: %w", in.ID, in.Amount, ErrBadAmount)
	}
	if in.Amount > b {
		return nil, fmt.Errorf("spend %q amount %d exceeds budget %d: %w", in.ID, in.Amount, b, ErrInsufficientBudget)
	}
	k.budgets[in.ID] = b - in.Amount
	return &SpendOutput{Milestones: nil}, nil
}
```

- [ ] **Step 4: Run to verify pass.**
- [ ] **Step 5: Commit** — `git commit -am "feat(play/clock): Tick high-water Advance + Spend"`

---

### Task 10: `Tick` persistence — `ToData` / `LoadTick`

**Files:** Modify `play/clock/tick.go`; Test `play/clock/tick_test.go`

- [ ] **Step 1: Write failing tests:**

```go
func (s *TickSuite) TestTickRoundTrip() {
	_, _ = s.tick.Join(&clock.JoinInput{ID: "goblin"})
	_, _ = s.tick.Advance(&clock.AdvanceInput{Driver: "p1", Displacement: 3})
	_, _ = s.tick.Spend(&clock.SpendInput{ID: "goblin", Amount: 1})
	loaded, err := clock.LoadTick(s.tick.ToData())
	s.Require().NoError(err)
	s.Equal(s.tick.ToData(), loaded.ToData())
	// behavior-identical: same high-water means no spurious grant
	_, _ = loaded.Advance(&clock.AdvanceInput{Driver: "p1", Displacement: 0})
	b, _ := loaded.Budget(&clock.BudgetInput{ID: "goblin"})
	s.Equal(2, b)
}

func (s *TickSuite) TestLoadTickRejectsInvalid() {
	cases := []struct {
		name string
		data clock.TickData
	}{
		{"negative budget", clock.TickData{Budgets: map[core.EntityID]int{"a": -1}}},
		{"negative driver progress", clock.TickData{DriverProgress: map[core.EntityID]int{"p": -2}}},
		{"high water below max progress", clock.TickData{DriverProgress: map[core.EntityID]int{"p": 5}, HighWater: 3}},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			_, err := clock.LoadTick(tc.data)
			s.Require().ErrorIs(err, clock.ErrInvalidData)
		})
	}
}
```

- [ ] **Step 2: Run to verify failure.**
- [ ] **Step 3: Implement:**

```go
// TickData is Tick's persisted shape (design R8). Plain data, no behavior.
type TickData struct {
	Budgets        map[core.EntityID]int `json:"budgets,omitempty"`
	DriverProgress map[core.EntityID]int `json:"driver_progress,omitempty"`
	HighWater      int                   `json:"high_water,omitempty"`
}

// ToData snapshots the clock. Family-convention exemption from R3 (design).
func (k *Tick) ToData() TickData {
	return TickData{
		Budgets:        copyIntMap(k.budgets),
		DriverProgress: copyIntMap(k.driverProgress),
		HighWater:      k.highWater,
	}
}

// LoadTick reconstructs a Tick from persisted state. A constructor, not a
// verb — no milestones. Errors: ErrInvalidData for every R9 rejection.
func LoadTick(data TickData) (*Tick, error) {
	maxProgress := 0
	for id, p := range data.DriverProgress {
		if p < 0 {
			return nil, fmt.Errorf("load tick: driver %q progress %d: %w", id, p, ErrInvalidData)
		}
		if p > maxProgress {
			maxProgress = p
		}
	}
	if data.HighWater < maxProgress {
		return nil, fmt.Errorf("load tick: high water %d below max driver progress %d: %w", data.HighWater, maxProgress, ErrInvalidData)
	}
	for id, b := range data.Budgets {
		if b < 0 {
			return nil, fmt.Errorf("load tick: member %q budget %d: %w", id, b, ErrInvalidData)
		}
	}
	k := &Tick{
		budgets:        copyIntMap(data.Budgets),
		driverProgress: copyIntMap(data.DriverProgress),
		highWater:      data.HighWater,
	}
	if k.budgets == nil {
		k.budgets = make(map[core.EntityID]int)
	}
	if k.driverProgress == nil {
		k.driverProgress = make(map[core.EntityID]int)
	}
	return k, nil
}

func copyIntMap(m map[core.EntityID]int) map[core.EntityID]int {
	if m == nil {
		return nil
	}
	out := make(map[core.EntityID]int, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
```

- [ ] **Step 4: Run to verify pass.**
- [ ] **Step 5: Commit** — `git commit -am "feat(play/clock): Tick persistence with R9 validation"`

---

### Task 11: Seams + `Transfer`

**Files:**
- Create: `play/clock/transfer.go`, `play/clock/transfer_test.go`
- Modify: `play/clock/turn.go`, `play/clock/tick.go` (adapter methods)

- [ ] **Step 1: Write failing tests:**

```go
package clock_test

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/play/clock"
	"github.com/stretchr/testify/require"
)

func TestTransferTickToTurn(t *testing.T) {
	tick, err := clock.NewTick()
	require.NoError(t, err)
	_, _ = tick.Join(&clock.JoinInput{ID: "carl"})
	turn := &clock.Turn{}
	_, _ = turn.SetOrder(&clock.SetOrderInput{Order: []core.EntityID{"a", "b"}})

	out, err := clock.Transfer(&clock.TransferInput{From: tick, To: turn, ID: "carl", Pos: 1})
	require.NoError(t, err)
	// milestones reported leave-then-join (design: Transfer), regardless of execution order
	require.Equal(t, []clock.Milestone{
		{Kind: clock.MemberLeft, Subject: "carl"},
		{Kind: clock.MemberJoined, Subject: "carl", Round: 1},
	}, out.Milestones)
	inTurn, _ := turn.Contains(&clock.ContainsInput{ID: "carl"})
	require.True(t, inTurn)
	inTick, _ := tick.Contains(&clock.ContainsInput{ID: "carl"})
	require.False(t, inTick, "one clock per entity")
}

func TestTransferFailureLeavesBothUnchanged(t *testing.T) {
	tick, err := clock.NewTick()
	require.NoError(t, err)
	_, _ = tick.Join(&clock.JoinInput{ID: "carl"})
	_, _ = tick.Advance(&clock.AdvanceInput{Driver: "p1", Displacement: 3})
	idleTurn := &clock.Turn{} // join will refuse: ErrIdle

	tickBefore, turnBefore := tick.ToData(), idleTurn.ToData()
	_, err = clock.Transfer(&clock.TransferInput{From: tick, To: idleTurn, ID: "carl", Pos: 0})
	require.ErrorIs(t, err, clock.ErrIdle, "underlying sentinel propagates")
	require.Equal(t, tickBefore, tick.ToData(), "From unchanged (R6)")
	require.Equal(t, turnBefore, idleTurn.ToData(), "To unchanged (R6)")
}

func TestTransferAbsentMemberCompensates(t *testing.T) {
	tick, err := clock.NewTick()
	require.NoError(t, err)
	turn := &clock.Turn{}
	_, _ = turn.SetOrder(&clock.SetOrderInput{Order: []core.EntityID{"a"}})

	turnBefore := turn.ToData()
	_, err = clock.Transfer(&clock.TransferInput{From: tick, To: turn, ID: "ghost", Pos: 0})
	require.ErrorIs(t, err, clock.ErrNotMember)
	require.Equal(t, turnBefore, turn.ToData(), "join was compensated (R6)")
}
```

- [ ] **Step 2: Run to verify failure.**
- [ ] **Step 3: Implement.** Adapters first (in `turn.go` and `tick.go` respectively):

```go
// turn.go — Membership seam adapters (design: Transfer)

// JoinMemberInput is the Joiner seam's input shape.
type JoinMemberInput struct {
	ID  core.EntityID
	Pos int
}

// JoinMemberOutput is the Joiner seam's output shape.
type JoinMemberOutput struct {
	Milestones []Milestone
}

// LeaveMemberInput is the Leaver seam's input shape.
type LeaveMemberInput struct {
	ID core.EntityID
}

// LeaveMemberOutput is the Leaver seam's output shape.
type LeaveMemberOutput struct {
	Milestones []Milestone
}

// JoinMember adapts Insert to the Joiner seam.
func (t *Turn) JoinMember(in *JoinMemberInput) (*JoinMemberOutput, error) {
	out, err := t.Insert(&InsertInput{ID: in.ID, Pos: in.Pos})
	if err != nil {
		return nil, err
	}
	return &JoinMemberOutput{Milestones: out.Milestones}, nil
}

// LeaveMember adapts Remove to the Leaver seam.
func (t *Turn) LeaveMember(in *LeaveMemberInput) (*LeaveMemberOutput, error) {
	out, err := t.Remove(&RemoveInput{ID: in.ID})
	if err != nil {
		return nil, err
	}
	return &LeaveMemberOutput{Milestones: out.Milestones}, nil
}
```

```go
// tick.go — Membership seam adapters (Pos is ignored; a world clock is unordered)

// JoinMember adapts Join to the Joiner seam.
func (k *Tick) JoinMember(in *JoinMemberInput) (*JoinMemberOutput, error) {
	out, err := k.Join(&JoinInput{ID: in.ID})
	if err != nil {
		return nil, err
	}
	return &JoinMemberOutput{Milestones: out.Milestones}, nil
}

// LeaveMember adapts Leave to the Leaver seam.
func (k *Tick) LeaveMember(in *LeaveMemberInput) (*LeaveMemberOutput, error) {
	out, err := k.Leave(&LeaveInput{ID: in.ID})
	if err != nil {
		return nil, err
	}
	return &LeaveMemberOutput{Milestones: out.Milestones}, nil
}
```

Then `transfer.go`:

```go
package clock

import (
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/core"
)

// Leaver is the leaving half of the Membership seam.
type Leaver interface {
	LeaveMember(in *LeaveMemberInput) (*LeaveMemberOutput, error)
}

// Joiner is the joining half of the Membership seam.
type Joiner interface {
	JoinMember(in *JoinMemberInput) (*JoinMemberOutput, error)
}

// Membership is what Transfer requires of both sides: the ability to
// join AND leave, so a failed transfer can compensate (design: Transfer).
type Membership interface {
	Leaver
	Joiner
}

// TransferInput moves ID from one clock to another, Pos choosing its slot
// when To is ordered.
type TransferInput struct {
	From Membership
	To   Membership
	ID   core.EntityID
	Pos  int
}

// TransferOutput reports the transfer's milestones, leave-then-join.
type TransferOutput struct {
	Milestones []Milestone
}

// Transfer moves ID between clocks upholding one-clock-per-entity (R6):
// on any failure both clocks are unchanged and the underlying sentinel
// propagates. Execution is join-first with compensating leave — the
// transient dual membership is invisible under R10's single-threaded
// contract; milestones are reported in leave-then-join order per the
// design regardless.
func Transfer(in *TransferInput) (*TransferOutput, error) {
	join, err := in.To.JoinMember(&JoinMemberInput{ID: in.ID, Pos: in.Pos})
	if err != nil {
		return nil, fmt.Errorf("transfer %q: join: %w", in.ID, err)
	}
	leave, err := in.From.LeaveMember(&LeaveMemberInput{ID: in.ID})
	if err != nil {
		if _, undoErr := in.To.LeaveMember(&LeaveMemberInput{ID: in.ID}); undoErr != nil {
			return nil, fmt.Errorf("transfer %q: leave failed (%v) and compensation failed: %w", in.ID, err, undoErr)
		}
		return nil, fmt.Errorf("transfer %q: leave: %w", in.ID, err)
	}
	return &TransferOutput{
		Milestones: append(append([]Milestone(nil), leave.Milestones...), join.Milestones...),
	}, nil
}
```

- [ ] **Step 4: Run to verify pass** — includes the compensation test proving R6 through failure.
- [ ] **Step 5: Commit** — `git commit -am "feat(play/clock): Membership seams + Transfer with compensating leave"`

---

### Task 12: AC1 — the DOS2 acceptance spine

**Files:**
- Create: `play/clock/dos2_test.go`

- [ ] **Step 1: Write the integration test.** It is expected to PASS immediately — it exercises only shipped verbs; treat any failure as a bug in Tasks 3–11:

```go
package clock_test

// The DOS2 split-party scenario (design AC1): four players on the world
// tick; two trigger a turn-based bubble; the distant pair keeps accruing
// from their own moves; one wanders close and falls in at a
// rulebook-chosen position; the fight ends; everyone returns to the world
// clock. Asserts the full milestone transcript and final state.

import (
	"testing"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/play/clock"
	"github.com/stretchr/testify/require"
)

func TestDOS2SplitParty(t *testing.T) {
	world, err := clock.NewTick()
	require.NoError(t, err)
	var transcript []clock.Milestone
	record := func(ms []clock.Milestone) { transcript = append(transcript, ms...) }

	// Everyone free-roams: four players and a goblin on the world clock.
	for _, id := range []core.EntityID{"alice", "bob", "carl", "dana", "goblin"} {
		out, jerr := world.Join(&clock.JoinInput{ID: id})
		require.NoError(t, jerr)
		record(out.Milestones)
	}

	// Alice and Bob trigger a fight with the goblin: bubble forms.
	// (Trigger detection is the composition's business; here the rulebook
	// has rolled initiative.)
	bubble := &clock.Turn{}
	for _, id := range []core.EntityID{"alice", "goblin", "bob"} {
		out, lerr := world.Leave(&clock.LeaveInput{ID: id})
		require.NoError(t, lerr)
		record(out.Milestones)
	}
	out, err := bubble.SetOrder(&clock.SetOrderInput{Order: []core.EntityID{"alice", "goblin", "bob"}})
	require.NoError(t, err)
	record(out.Milestones)

	// The distant pair keeps exploring: their own moves drive the world.
	adv, err := world.Advance(&clock.AdvanceInput{Driver: "carl", Displacement: 3})
	require.NoError(t, err)
	record(adv.Milestones)
	// carl is both driver and member; the grant reaches every member (design: Tick)
	require.Equal(t, []core.EntityID{"carl", "dana"}, adv.Ready, "the distant pair accrues while the fight runs")

	// A round of combat in the bubble.
	for _, actor := range []core.EntityID{"alice", "goblin"} {
		end, eerr := bubble.End(&clock.EndInput{Actor: actor})
		require.NoError(t, eerr)
		record(end.Milestones)
	}

	// Carl wanders too close and falls in, slotted after the goblin.
	tr, err := clock.Transfer(&clock.TransferInput{From: world, To: bubble, ID: "carl", Pos: 2})
	require.NoError(t, err)
	record(tr.Milestones)
	inBubble, _ := bubble.Contains(&clock.ContainsInput{ID: "carl"})
	require.True(t, inBubble)

	// Bob closes the round: the bubble wraps into round 2 with carl in the
	// order — AC1's "rounds advance with End" exercised across a wrap.
	end, err := bubble.End(&clock.EndInput{Actor: "bob"})
	require.NoError(t, err)
	require.True(t, end.RoundWrapped)
	record(end.Milestones)

	// Fight ends: dissolve and re-home everyone to the world clock.
	dis, err := bubble.Dissolve(&clock.DissolveInput{})
	require.NoError(t, err)
	record(dis.Milestones)
	require.ElementsMatch(t, []core.EntityID{"alice", "goblin", "bob", "carl"}, dis.Members)
	for _, id := range dis.Members {
		out, jerr := world.Join(&clock.JoinInput{ID: id})
		require.NoError(t, jerr)
		record(out.Milestones)
	}

	// Final state: everyone back on the world clock; bubble idle.
	members, err := world.Members()
	require.NoError(t, err)
	require.ElementsMatch(t, []core.EntityID{"alice", "bob", "carl", "dana", "goblin"}, members)
	_, err = bubble.Active()
	require.ErrorIs(t, err, clock.ErrIdle)

	// The transcript is API: assert it end to end.
	require.Equal(t, []clock.Milestone{
		{Kind: clock.MemberJoined, Subject: "alice"},
		{Kind: clock.MemberJoined, Subject: "bob"},
		{Kind: clock.MemberJoined, Subject: "carl"},
		{Kind: clock.MemberJoined, Subject: "dana"},
		{Kind: clock.MemberJoined, Subject: "goblin"},
		{Kind: clock.MemberLeft, Subject: "alice"},
		{Kind: clock.MemberLeft, Subject: "goblin"},
		{Kind: clock.MemberLeft, Subject: "bob"},
		{Kind: clock.RoundStarted, Round: 1},
		{Kind: clock.TurnStarted, Subject: "alice", Round: 1},
		{Kind: clock.Ticked, Subject: "carl"},
		{Kind: clock.TurnEnded, Subject: "alice", Round: 1},
		{Kind: clock.TurnStarted, Subject: "goblin", Round: 1},
		{Kind: clock.TurnEnded, Subject: "goblin", Round: 1},
		{Kind: clock.TurnStarted, Subject: "bob", Round: 1},
		{Kind: clock.MemberLeft, Subject: "carl"},
		{Kind: clock.MemberJoined, Subject: "carl", Round: 1},
		{Kind: clock.TurnEnded, Subject: "bob", Round: 1},
		{Kind: clock.RoundStarted, Round: 2},
		{Kind: clock.TurnStarted, Subject: "alice", Round: 2},
		{Kind: clock.Dissolved, Round: 2},
		// Dissolve returns members in bubble order [alice, goblin, carl, bob]
		// (carl inserted at Pos 2), and the rejoin loop follows it.
		{Kind: clock.MemberJoined, Subject: "alice"},
		{Kind: clock.MemberJoined, Subject: "goblin"},
		{Kind: clock.MemberJoined, Subject: "carl"},
		{Kind: clock.MemberJoined, Subject: "bob"},
	}, transcript)
}
```

- [ ] **Step 2: Run** — `go test ./... -run TestDOS2SplitParty -v` → PASS (investigate as a regression if not).
- [ ] **Step 3: Commit** — `git add play/clock/dos2_test.go && git commit -m "test(play/clock): AC1 DOS2 split-party acceptance spine"`

---

### Task 13: Full gate — suite, lint, tidy, gorelease baseline

- [ ] **Step 1: Full module gate**

```bash
cd /home/kirk/game-dev/.worktrees/toolkit-play-clock/play/clock
go mod tidy && go test ./... -count=1 && golangci-lint run ./... && go vet ./...
```
Expected: all green. Fix anything that isn't before proceeding.

- [ ] **Step 2: AC4 — wire the compat gate into CI.** Create `.github/workflows/compat.yml` (repo-level file, not part of any module — allowed from this branch):

```yaml
name: API Compatibility

on:
  pull_request:
    paths:
      - 'play/clock/**'

jobs:
  gorelease-play-clock:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
      - uses: actions/setup-go@v5
        with:
          go-version: '1.24'
      - name: gorelease play/clock
        run: |
          base=$(git tag -l 'play/clock/v*' --sort=-v:refname | head -1)
          if [ -z "$base" ]; then
            echo "no play/clock tag yet — first release, gate activates from the first tag"
            exit 0
          fi
          cd play/clock
          go run golang.org/x/exp/cmd/gorelease@latest -base "${base#play/clock/}"
```

This satisfies AC4 literally: the job exists from day one and enforces from the first tag onward (it skips only while no tag exists, and `auto-tag-modules` tags the module at merge). Verify the tool runs locally:

```bash
cd /home/kirk/game-dev/.worktrees/toolkit-play-clock/play/clock
go run golang.org/x/exp/cmd/gorelease@latest || true   # informational pre-first-tag
```

- [ ] **Step 3: Commit.** The workflow file is NEW and untracked — `-a` alone would silently omit it:

```bash
cd /home/kirk/game-dev/.worktrees/toolkit-play-clock
git add .github/workflows/compat.yml play/clock/
git commit -m "ci: gorelease compat gate for play/clock (AC4); module gate green"
```

---

### Task 14: PR

- [ ] **Step 1: Push and open (ready for review, never draft):**

```bash
cd /home/kirk/game-dev/.worktrees/toolkit-play-clock
git push -u origin feat/play-clock
gh pr create --title "feat(play/clock): Turn + Tick clocks, Milestone vocabulary, Transfer" --body "First play/ leaf module of the encounter reset (journey 051).

Implements docs/ideas/play/clock/design.md exactly: Turn (initiative bubble), Tick (player-driven world clock, high-water max-displacement), Milestone return values (never published), Membership seams + atomic Transfer, ToData/LoadTurn/LoadTick persistence with R9 validation, nine-sentinel error vocabulary.

Evidence: AC1 DOS2 split-party transcript test green; AC2 invariants; AC3 round-trips; AC6 errors.Is dispatch per sentinel; module gate (test/lint/vet/tidy) green. Leaf laws hold: deps = core + stdlib only, no context.Context, no randomness.

Prerequisite: merges after feat/core-entity-id (core.EntityID)."
```

- [ ] **Step 2: Drive CI green.** `make lint-all` / `make test-all` auto-discover the new module. If the tidy check flags other modules, do NOT touch them (module isolation) — investigate whether the failure is ours.

---

## Execution addenda (logged during subagent-driven execution)

Carry-along additions prescribed by per-task quality reviews, landing in
the NEXT task's commit; recorded here so the executed tree and this plan
stay in agreement:

- **With Task 3**: go directive `1.25.3`→`1.25`; license headers on all Go
  files; milestone vocabulary test pins raw string values.
- **With Task 4**: R5-from-populated + SetOrder-replacement test; aliasing
  regression test; Contains-on-idle assertion; empty-order message polish.
- **With Task 5**: single-member End wrap pin (`Next == Actor`); SetOrder
  round-reset-from-advanced pin.
- **With Task 6**: Remove "non-active after active" subtest (kills the
  always-decrement mutant — the index-drift class this module exists to
  prevent); `Order()` placement assertions in every Insert/Remove subtest
  (a splice-vs-append mutant survives milestone-only assertions);
  switch-ordering comment in Remove; test setup calls upgraded from
  `_, _ =` to `s.Require().NoError` (applies to Task 6's own blocks too).
- **Design clarification during execution**: nil `*Input` is a programmer
  error — verbs may panic; sentinel vocabulary is for clock states only
  (recorded in design R3).
- **Task 6 fix cycle (plan defects found by quality review)**: the plan's
  own Merge code allowed SELF-MERGE — validation passes against the
  clock's own member set, then zeroing Other destroys the receiver while
  reporting success. Fixed with an `in.Other == t` guard (`ErrBadOrder`)
  + pin test; design Merge/error rows amended. Also: the plan's bad-order
  test only exercised the length check — added subtests for the
  "in neither clock" and "appears twice" branches; failed-merge R5
  assertions extended to Order()+Round(); Dissolve's ownership-transfer
  exception now stated in its godoc, not only the design.

- Test style: the per-type suites (Tasks 3–10) use the testify suite pattern
  per design AC5; the cross-type integration tests (Tasks 11–12) are plain
  functions with `require` — deliberate, since they exercise multiple
  independently-constructed clocks per test and gain nothing from suite
  fixtures. Reviewers pointing at AC5 for those files: this note is the
  answer.
- Tasks 3–11 are strictly ordered (each builds on prior types). Task 1 is independent and must merge first. Task 0 is docs-only on the other branch.
- Every commit message uses the repo's conventional style; pre-commit hooks run the module's fmt/lint/tests — do not `--no-verify` on code commits.
- If any implementation forces a deviation from `design.md`, STOP and update the design (with Kirk's sign-off) before coding around it.
