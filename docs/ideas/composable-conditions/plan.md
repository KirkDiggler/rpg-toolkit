# Plan: Conditions Are Composed of Effects

Live execution state for [design.md](design.md). Completed, superseded and
blocked steps are shown as they are; a step the design does not settle is
named as a gap and brought back, never guessed.

## Header

- **Goal and scope.** Every dnd5e condition becomes a recipe run by two tools,
  `machine` and `effects`, formed in `examples/effects` and promoted; Rage
  first, then the families in the design's slice order. Non-goals: the grant
  noun (loot), studio UI, any change to the resolution seam, derived display,
  an expression language.
- **Finish line.** S1 merged with its tests and benchmarks green; S2 merged
  with `RagingCondition` a recipe and the existing rage tests unchanged; each
  later slice merged on tags with its run record; `implementation.md` added
  before the idea PR merges; measures posted on rpg-project#703 as slices land.
- **Authority.** [design.md](design.md), approved 2026-10-28; rulings 1–7
  recorded in its "Settled, and open"; story
  [rpg-project#703](https://github.com/KirkDiggler/rpg-project/issues/703);
  idea PR rpg-toolkit#2144.
- **Constraints.** E1–E11 as written. One nearest-`go.mod` module per PR.
  Team signature only. `examples/effects` imports `core`, `events` and the
  standard library only; `effects/machine` imports nothing from `effects`.
  Blob keys for Rage are exactly today's: `ref`, `member_id`, `source`,
  `saw_turn_end`, `round_activated`, `turns_active`, `was_hit_this_turn`,
  `did_attack_this_turn`, plus `state`. First vocabulary is what Rage reads
  (ruling 4). Recipes are Go literals with a JSON round-trip test (ruling 5).
- **Inspected baseline.** rpg-toolkit main at the idea PR's base; tags
  `core/v0.12.0`, `events/v0.6.3`; `events.DefineTypedTopic[T]` /
  `DefineChainedTopic[T]` with `.On(bus).Subscribe` / `.SubscribeWithChain`
  returning a subscription id and `Unsubscribe(ctx, id)`; `core/chain.Chain[T].Add(stage, id, modifier)`;
  `rulebooks/dnd5e/conditions/raging.go` at 615 lines, nine subscriptions;
  `rulebooks/dnd5e/conditions/descriptor.go` (ref, display, load, create,
  ownAction, againstHolder, heldRule, attackOverride); CI discovers a module
  by the nearest `go.mod` of a changed file and runs Go 1.25 until
  rpg-toolkit#2145 step 0 lands.
- **Sequence.** rpg-toolkit#2145 step 0 (CI on 1.27) → S1 → #2145 dnd5e root
  bump → S2 (promotion PR, then dnd5e PR, then `core/effect` deletion PR) →
  S3 → S4 → S5 → S6 → S7. Development is outside-in on pushed commits; merge
  is inside-out on minted tags; the dnd5e PR of S2 pins the `effects` tag
  minted by the promotion PR.
- **Open items.** None blocking S1. S3–S7 task contracts are filled when each
  slice starts, from the design's slice text; a leaf, kind or beat the design's
  table does not name is a finding under E8, not a task.

## S1: `examples/effects`, with `effects/machine` inside

### Task 1: Module scaffold and the two seam tests

**Delivers:** E9 (imports), E11. A module CI can build and lint.
**Owner:** rpg-toolkit, new module `examples/effects`.
**Prerequisites:** rpg-toolkit#2145 step 0 merged (CI on 1.27).
**Files:** create `examples/effects/go.mod` (module
`github.com/KirkDiggler/rpg-toolkit/examples/effects`, `go 1.27.0`, require
`core v0.12.0`, `events v0.6.3`, `testify v1.11.1`), `examples/effects/doc.go`,
`examples/effects/machine/doc.go`, `examples/effects/seam_test.go`,
`examples/effects/machine/seam_test.go`.
**Interfaces:** none produced yet.
**Behavior:** `TestImportsStayInsideTheSeam` in each package parses every
non-test `.go` file with `go/parser` (imports only) and asserts each import
path is in the allow-list: machine → std, `core`, `core/chain`, `events`;
effects → the same plus `…/examples/effects/machine`. Failure names the file
and the path.
**Tests:** the two seam tests, red when a forbidden import is added by hand.

- [ ] Create the module, `go mod tidy`, both doc.go with the package contract
  sentence from design E9.
- [ ] Add both seam tests; prove red by importing `fmt`-only is green and a
  scratch import of `rulebooks/dnd5e` is red, then remove the scratch.
- [ ] `golangci-lint run ./...` clean.

**Verification:** `cd examples/effects && go vet ./... && go test ./... && golangci-lint run ./...`.
**Completion evidence:** green run pasted on the PR.

### Task 2: Machine data structs, vocabulary plug, Validate, blob

**Delivers:** E1 (one blob), E3 (closed vocabulary, Validate at load), E4
(bag is the blob), rulings 1, 3, 4, 5.
**Owner:** `examples/effects/machine`.
**Prerequisites:** Task 1.
**Files:** create `machine/recipe.go` (Recipe, State, Transition, Tracker,
Guard, FactLeaf, RoundsLeaf, ClassLevelLeaf, Act, SetAct, Stacks, Beat,
Selector, Op, FactName as in design "The component" and the struct listing on
the idea PR), `machine/vocabulary.go` (Vocabulary{Facts, Beats}, FactSpec{Kind,
Read func(any) (any, bool)}, BeatSpec{Selectors}), `machine/validate.go`,
`machine/blob.go` (Blob{Ref, MemberID, Source, State, Bag `json:",inline"`}),
`machine/recipe_test.go`, `machine/validate_test.go`, `machine/blob_test.go`.
**Interfaces:** `func Validate(r Recipe, v Vocabulary) error` returning the
first refusal by name; `func (r Recipe) Compile(v Vocabulary) (*Program, error)`
binding fact names to readers once; `Blob` round-trips with `encoding/json/v2`.
Outputs on a State are `[]json.RawMessage`-shaped opaque data the machine
never reads (`Outputs []Output` where `Output` is `jsontext.Value`).
**Behavior:** Validate refuses, each by name: a fact not in the vocabulary; a
flag not declared in Bag; `To` naming no state; an initial state that is
terminal; a state no transition reaches; a transition into a terminal state
with empty Reason; a guard node with zero or two fields set; an amount or act
with zero or two fields set; a bag key no guard, act or tracker reads
(outputs are opaque here, so the composer re-checks keys at its level); a
`Stacks: by_source` recipe without a `source_id` bag key; a beat not in the
vocabulary; a selector the beat does not support.
**Tests:** `TestValidateRefuses` table with one bad recipe per case above and
the expected substring; `TestValidateAcceptsRageShaped`; `TestRecipeRoundTrip`
marshals the Rage-shaped recipe with json/v2, unmarshals, asserts
`reflect.DeepEqual`; `TestBlobIsFlat` marshals a Blob with bag
`{turns_active: 3, was_hit_this_turn: true}` and asserts the JSON object has
`member_id` and `turns_active` as sibling keys and no nested `bag` key;
`TestBlobLoadsTodaysRageKeys` unmarshals a literal copy of today's raging blob
(the eight keys) and asserts every key landed in the bag with its type.

- [ ] Write the tests red against empty types.
- [ ] Implement structs, Validate, Compile, Blob.
- [ ] Green; lint clean.

**Verification:** `cd examples/effects && go test ./machine/ -run 'Validate|RoundTrip|Blob' -v`.
**Completion evidence:** the test names above passing.

### Task 3: Guard evaluation, three-valued

**Delivers:** E3 (Depends is not false).
**Owner:** `examples/effects/machine`.
**Prerequisites:** Task 2.
**Files:** create `machine/guard.go`, `machine/guard_test.go`.
**Interfaces:** `type Verdict int` with `Applies`, `DoesNotApply`, `Depends`;
`func (g *compiledGuard) Answer(frame any, bag Bag, ev any) (Verdict, string)`
where the string is the reason (the first leaf that decided).
**Behavior:** a `fact` leaf whose reader reports unknown answers Depends; `all`
is DoesNotApply if any leaf is, else Depends if any leaf is, else Applies;
`any` is Applies if any leaf is, else Depends if any leaf is, else
DoesNotApply; `not` swaps Applies and DoesNotApply and leaves Depends; `flag`
reads the bag; `rounds.since_anchor` is Depends when the event carries no
round or the bag anchor is zero; `rounds.went_backwards` is Applies when the
event's round is below the anchor; `class_level` is Depends when the frame
does not carry the class.
**Tests:** `TestGuardTruthTable` over every combination of two leaves under
`all`, `any`, `not` with values in {Applies, DoesNotApply, Depends};
`TestGuardOps` for `eq`, `gte` on int and string facts; `TestRoundsLeaf` for
anchor 0, event round 0, round below anchor, round 9 after anchor 1
(`since_anchor gte 9` Applies).

- [ ] Tests red; implement; green.

**Verification:** `go test ./machine/ -run Guard -v`.

### Task 4: The runner, trackers, beats, transition events; the Rage walk

**Delivers:** E2, E5, E6 (lifecycle as transitions, recipe order), the graced
turn and clock-anchored round as a state and an act.
**Owner:** `examples/effects/machine`.
**Prerequisites:** Tasks 2, 3.
**Files:** create `machine/machine.go`, `machine/acts.go`, `machine/machine_test.go`,
`machine/rage_walk_test.go`, `machine/testvocab_test.go`.
**Interfaces:** `func New(p *Program, holder string, source *core.Ref, bag Bag, state string) *Machine`;
`Apply(ctx, bus events.EventBus) error` subscribes trackers and one
subscription per beat the recipe names; `Remove(ctx, bus) error`;
`Transitions() <-chan`-free: instead `OnTransition(func(ctx, Transition) error)`
registering the composer's listener, `Transition{From, To, Reason string}`;
`State() string`; `Bag() Bag`; `ToBlob() Blob`.
**Behavior:** on a beat, the current state's transitions run in recipe order
with the event; the first whose guard Applies runs its acts, then if `To` is
set enters that state and fires the transition listener; a guard that answers
Depends does not fire and is not an error; a tracker whose selector does not
match the holder is ignored; a tracker sets the flag once per turn and calls
the changed hook once; `anchor_round` sets `round_activated` from the event's
round when it is > 0 and `saw_turn_end` true; `derive_turns_active` recomputes
from the anchor only when anchored; `clear_flags` zeroes the named bools;
entering a terminal state calls the end hook with the reason and the machine
detaches everything.
**Tests:** `testvocab_test.go` defines the Rage-shaped vocabulary over test
events: typed topics `turn_end{SubjectID, Round}`, `damage_received{TargetID}`,
`post_attack_roll{AttackerID}`, `dropped_to_zero{MemberID}`, `rest{CharacterID}`,
`combat_end{SubjectID}`, named as Rage names them. `TestRageWalk` drives the
Rage recipe from the design, state `fresh`, through: first turn end → `raging`
with `round_activated` = 1, `saw_turn_end` true, no sustain check; an attack
then turn end → stays `raging`, flags cleared, `turns_active` 2; a turn end
with no flag → `ended`, reason `no_combat_activity`; separately, nine more
active turn ends from round 1 → `ended` at round 10 with `duration_expired`;
a turn end with round 0 after an anchor → no cap evaluation; a turn end with
round below the anchor → `ended`, `clock_reset`; `dropped_to_zero` → `ended`,
`unconscious`; `rest` → `rest`; `combat_end` → `combat_ended`. Each asserts
the transition listener saw exactly one `Transition{From, To, Reason}` and the
bag after. `TestOrderingOnOneBeat` asserts the sustain check precedes the cap
on the same beat by placing a recipe where both would fire and asserting the
reason. `TestTrackerIdempotent` asserts two hits in one turn call changed once.

- [ ] Tests red; implement runner and acts; green.

**Verification:** `go test ./machine/ -run 'Rage|Ordering|Tracker' -v`.
**Completion evidence:** the walk's assertions passing, pasted.

### Task 5: The composer, output kinds plug, benchmarks

**Delivers:** E2 (outputs are the composer's), E5 (end is one verb), E8 rows.
**Owner:** `examples/effects` (package `effects`).
**Prerequisites:** Task 4.
**Files:** create `effects/composed.go`, `effects/vocabulary.go`
(`OutputVocabulary{Kinds, Amounts}`, `KindSpec{Attach func(ctx, bus, Host, params) (Detach, error); Assess func(frame any, bag machine.Bag, params) (Row, error)}`,
`AmountFn func(frame any, params) (int, error)`), `effects/output.go`
(the `Output` struct of the design with one-of Validate),
`effects/composed_test.go`, `effects/bench_test.go`.
**Interfaces:** `func Compose(r machine.Recipe, v machine.Vocabulary, ov OutputVocabulary, holder string, source *core.Ref, bag machine.Bag, state string) (*Composed, error)`;
`Composed` has `Ref() *core.Ref`, `IsApplied() bool`, `Apply(ctx, bus) error`,
`Remove(ctx, bus) error`, `ToJSON() (json.RawMessage, error)`, `Rows(frame any) ([]Row, error)`;
the method set matches the rulebook's `ConditionBehavior` structurally.
**Behavior:** Compose validates outputs against `ov` (one-of, kind known,
amount known, every bag key an output reads declared); Apply applies the
machine, attaches the current state's outputs, registers the transition
listener; on a transition it detaches the old state's outputs and attaches
the new; on the terminal state it publishes the removal through a hook the
plug supplies (the machine knows no removal topic), then Remove; Rows is the
sum of the current outputs' Assess.
**Tests:** a test chain `DefineChainedTopic[*testDamage]` and a test kind
`bonus` that adds `Amount` to `testDamage.Total` at a stage when its guard
Applies; `TestOutputsAttachedOnlyWhileInState` publishes on the chain in
`fresh` (no bonus), `raging` (bonus), `ended` (no bonus); `TestEndPublishesThenDetaches`
asserts the removal hook fires before the chain stops answering;
`TestRowsSumOutputs` asserts two outputs give two rows and a Depends guard
gives a Depends row, not an error. `BenchmarkGuardAnswer` on the Rage damage
guard with a known frame; `BenchmarkBeatTurnEnd` on the Rage machine in
`raging` with flags set.

- [ ] Tests red; implement; green; run benchmarks and paste `ns/op`.

**Verification:** `cd examples/effects && go test ./... && go test -run '^$' -bench . ./...`.

### Task 6: Record and PR

**Delivers:** the run record (R13), the ideas index line, the S1 PR.
**Owner:** rpg-toolkit, `examples/effects` PR.
**Prerequisites:** Tasks 1–5 green.
**Files:** create `rulebooks/dnd5e/knowledge/runs/<date>-composable-conditions-s1.md`
(shipped, decided, found, mistaken; the benchmark table under found),
modify `docs/ideas/README.md` (status line), this plan (tick S1).
**Behavior:** PR body carries `Part of rpg-project#703`, `Closes rpg-project#<S1 sub-issue>`,
the Try-it from the sub-issue, the team signature.

- [ ] Run record written with every line linked.
- [ ] PR opened ready for review; CI green on 1.27.

**Verification:** `scripts/check-knowledge-map.sh` green (new module must be
in exactly one map; it belongs to the root map's examples row until S2 moves
it); `scripts/check-doc-genres.sh` on the run record.

## S2: Promotion, the dnd5e plug, and Rage

Three PRs, inside-out. Contracts at the level the design settles; file lists
are filled at slice start from the inspected S1 result.

### Task 7: Promote to `effects/`

**Owner:** new module `effects` (top-level, beside `play/`), one PR that also
deletes `examples/effects`. **Prerequisites:** S1 merged; `rulebooks/dnd5e/effects`
and `mechanics/effects` deleted first (two small PRs, no importers) so the
name is free. **Files:** `git mv examples/effects effects`, module path
`github.com/KirkDiggler/rpg-toolkit/effects`, `knowledge/map.md` row for the
new module (drift check), `docs/architecture/overview.md` module map.
**Verification:** `scripts/check-knowledge-map.sh`; CI mints `effects/v0.1.0`.

### Task 8: The dnd5e plug and the Rage recipe

**Owner:** `rulebooks/dnd5e` root (`conditions`). **Prerequisites:** Task 7
tag; dnd5e root on Go 1.27 (#2145). **Files:** create
`conditions/vocabulary.go` (facts `action.melee`, `action.weapon_pool`,
`action.ability` over `contributions.Frame`; beats `turn_end`, `dropped_to_zero`,
`rest`, `combat_end`, `post_attack_roll`, `damage_received` over
`dnd5eEvents` topics with the selectors Rage uses; kinds `damage_bonus`
(`DamageChain`, `StageFeatures`, `DamageSourceCondition`, weapon damage type),
`resist` (`IncomingDamageChain`, `StageFinal`, physical types), `advantage`
(`SavingThrowChain` / `AbilityCheckChain`, `StageConditions`); amount
`by_class_level`; the removal hook publishing `ConditionRemovedTopic`),
`conditions/recipe_raging.go` (the recipe from the design, `Stacks: by_ref`),
modify `conditions/raging.go` (descriptor `load`/`create` build
`effects.Compose(...)`; the hand-written type is deleted in the same PR once
the tests below are green), `conditions/raging_test.go` unchanged.
**Tests:** every existing raging test unchanged and green;
`TestRagingBlobIdentical` builds the old type and the recipe with the same
inputs, drives both through the same event sequence, and asserts byte-equal
`ToJSON` at each step (run against the old type before it is deleted, then
kept as a golden); `TestRagingRowsAreFour` asserts the information rows now
include damage, resistance and both advantages.
**Verification:** `cd rulebooks/dnd5e && go test ./conditions/... ./resolution/... ./session/...`;
`TestNoInnerTypeCrossesTheBoundary` in session still green.

### Task 9: Delete `core/effect`

**Owner:** `core`. One PR. **Verification:** `go build ./...` in every module
that pins `core` still green at their current pins (nothing imports it).

## S3–S7

Each starts by filling its task contracts here from the design's slice
paragraph and the inspected code at that time, under E8: a leaf, kind or beat
the design's table names arrives with the first recipe that reads it and its
test; one the table does not name is a finding brought back before it is
built.

- **S3** the 1d4 family: adds `dice` amount, `roll_bonus` and `offer_die`
  kinds, `offer_taken` beat, `in` op; `Stacks: by_source` recipes; retires the
  hand-written address methods on those five.
- **S4** the against-holder family: adds pair and held facts, `save_result`
  beat; outputs that fold the attacker's chain through the holder's bearing.
- **S5** features become Definitions: `Pools` costs; the four `core.Action`
  features; rpg-api adoption is a consumer pin bump, no api logic.
- **S6** the long tail with the composed/hand-written count.
- **S7** the studio form; an ADR only if the resolution seam moves.

## Visible plan checks

### Requirement coverage

| Requirement / ruling | Task(s) | Concrete proof |
|---|---|---|
| E1 one ref, one descriptor, one blob | 2, 5, 8 | `TestBlobIsFlat`, descriptor swap in one line, `TestRagingBlobIdentical` |
| E2 machine composed with outputs; three kinds | 4, 5 | `TestRageWalk`, `TestOutputsAttachedOnlyWhileInState` |
| E3 closed vocabulary, Validate at load, Depends ≠ false | 2, 3 | `TestValidateRefuses`, `TestGuardTruthTable` |
| E4 bag is the blob, today's keys | 2, 8 | `TestBlobLoadsTodaysRageKeys`, `TestRagingBlobIdentical` |
| E5 end is one verb, publish then detach | 4, 5, 8 | `TestEndPublishesThenDetaches` |
| E6 lifecycle as ordered transitions; graced turn; clock anchor | 4 | `TestRageWalk`, `TestOrderingOnOneBeat`, `TestRoundsLeaf` |
| E7 cost never on a condition | 8 (by absence) | recipe has no cost field; `Validate` has no cost case |
| E8 hand-written stays legal; vocabulary grows by finding | S3–S7 | the composed/hand-written count in each slice's run record |
| E9 two tools, seams held by tests, no rulebook import | 1, 7 | `TestImportsStayInsideTheSeam` ×2 |
| E10 features become Definitions | S5 | filled at S5 |
| E11 build it new | 1 | go.mod requires only core, events, testify |
| Ruling 1 stacking on the recipe | 2, 8 | `Validate` by_source needs `source_id`; Rage `by_ref` |
| Ruling 2 authored display | 8 | descriptor `display` unchanged |
| Ruling 3 instance = recipe + parameters into the bag | 2, 5 | undeclared parameter key refused by name |
| Ruling 4 vocabulary trimmed to Rage | 4, 8 | the test vocabulary and `conditions/vocabulary.go` list exactly those rows |
| Ruling 5 Go literals + round-trip | 2 | `TestRecipeRoundTrip` |
| Ruling 6, 7 names and homes | 1, 7 | module paths |
| Rage answers four rows | 5, 8 | `TestRowsSumOutputs`, `TestRagingRowsAreFour` |

### Provider/consumer seams

| Provider | Consumer | Contract | Availability | Proof |
|---|---|---|---|---|
| `machine` (Task 4) | `effects` (Task 5) | `OnTransition(Transition{From,To,Reason})`, `Bag`, `ToBlob` | same module, same PR | `TestOutputsAttachedOnlyWhileInState` |
| `effects.Composed` (Task 5) | `conditions` descriptor (Task 8) | method set equals `dnd5eEvents.ConditionBehavior` structurally | `effects` tag from Task 7 pinned in dnd5e go.mod | compile of `var _ dnd5eEvents.ConditionBehavior = (*effects.Composed)(nil)` in conditions |
| `conditions` vocabulary (Task 8) | `machine`/`effects` | `Vocabulary`, `OutputVocabulary` values | same PR | `Validate` accepts the Rage recipe against them |
| `conditions` blob (Task 8) | resolution, session, rpg-api, web | unchanged keys | no consumer change | `TestRagingBlobIdentical`; session boundary test |
| `core v0.12.0`, `events v0.6.3` | `examples/effects` | existing APIs listed in the baseline | released | go.mod pins |
| rpg-toolkit#2145 step 0 | S1 | CI on Go 1.27 | merge before S1 PR | green CI on the S1 PR |
