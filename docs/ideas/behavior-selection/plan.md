# Behavior selection implementation plan

> Execute directly, task-by-task with TDD; one independent whole-PR review at the end.

**Goal:** deliver [Behavior selection S1#918](https://github.com/KirkDiggler/rpg-project/issues/918) of [story#917](https://github.com/KirkDiggler/rpg-project/issues/917).
**Spec:** [design.md](design.md), [walkthrough](README.md).
**Authority:** Kirk approved the package placement and the proposed value-only selection/consumer shape, and delegated choosing one implementation rather than another design menu. No merge or deployment authority.
**Baseline:** toolkit main `ba6d40f2`, worktree `.worktrees/behavior-selection`, branch `feat/behavior-selection`; whole behavior-module race baseline passes. Owning module `mind/behavior`, Go1.27. No dependency or wire/persistence changes.
**Finish line:** committed package and real consumer, module race/lint/uncached-hook and remote gates, published independent review; ready provider PR. Merge/tag remains separate. No visible game change to boot or walk.

## Constraints and implementation choices

Follow the owning repository and module discipline; only behavior code and accompanying repository docs change. Selection imports standard library only. Use nonnegative `int` relative shares and ordered integer multipliers, checked against the platform's maximum `int`. No percentage conversion: the behavior adapter supplies the existing raw percentage as a factor, preserving die sizes. Choose may explicitly select a zero-share eligible row; Draw may not. Evaluate/Inspect never take or obtain a die. A snapshot is not authorization to execute later.

New API:
- `Verdict`: `Unknown` (zero), `Unmet`, `Met`.
- `Requirement{ID string, Verdict Verdict, Reason string}`; IDs unique/nonblank per row.
- `Factor{ID string, Multiplier int}`; IDs unique/nonblank per row; nonnegative.
- `Candidate{ID string, Weight int, Requirements []Requirement, Factors []Factor}`; ordered unique/nonblank candidate IDs, nonnegative base weights.
- `Evaluate(*EvaluateInput{Candidates []Candidate}) (*Evaluation,error)`; captures inputs and returns a privately held immutable evaluation, including a valid empty pool.
- `(*Evaluation).Inspect() (*Inspection,error)`; detached `Rows []Row`, `Eligible int`, `Total int`. `Row{ID, BaseWeight, Requirements, Factors, Eligible, Weight}` preserves supplied evidence and computed load. Nil/zero evaluations are refused.
- `Choose(*ChooseInput{Evaluation *Evaluation, ID string}) (*Choice,error)`; `Choice{Index int, Row Row}`; unknown/ineligible IDs refused, no dice.
- `Draw(context.Context,*DrawInput{Evaluation *Evaluation, Die Die}) (*DrawOutput,error)`; `DrawOutput{Choice Choice, Roll int, Total int}`; `Die.Roll(context.Context,int)(int,error)` supplied, one call, valid face1..Total.
- Named errors cover nil inputs/evaluation/context, malformed candidates, overflow, no candidates, zero total, unknown/ineligible ID, missing die and invalid roll. Roller errors wrap transparently. No `(nil,nil)`.

Validation checks all row identities/verdicts/nonnegative inputs, including excluded rows; arithmetic applies only to eligible rows. Eligible products and totals refuse overflow. A nil/zero evaluation is distinct from an evaluated empty pool. Parent behavior preserves its public error meanings for nil input, missing die, hold/silence/zero-share cases, and wraps malformed selection arithmetic/dice as `ErrBadTable` with underlying cause.

## Review focus

- Invalid or duplicate IDs/verdicts must not yield a plausible but wrong choice.
- Input/output aliasing must not change a later choice from the same evaluation.
- Zero random weight is not an eligibility gate; explicit selection must retain that distinction.
- Overflow or a bad die must refuse before inventing a selected row.
- Nested groups must retain their separate probabilities, path, row evidence and actual roll count; no unselected group or routine may roll.

### Task 1: Captured evaluation and evidence

**Delivers:** R1–R4. **Owner:** behavior/selection. **Prerequisites:** existing standard library and testify in the owning module.
**Files:** create `mind/behavior/selection/{doc.go,selection.go,selection_test.go}`.
**Interfaces:** Evaluate/Evaluation/Inspect and value types above; no parent import.
**Behavior:** validate identities, requirements and factors; capture their arrays; distinguish Unknown/Unmet; fold eligibility and checked multiplication/sum; return detached evidence. Constructed empty is valid; nil/zero is not.
**Tests:** `TestSelectionSuite/TestRequirementsAndWeights` asserts literal shares6/4, total10, and separate unmet/unknown rows; `TestMalformedCandidates` covers blank/duplicate candidate, requirement and factor IDs, invalid verdict and negative values; `TestOverflowIsRefused` covers product and sum before any selection; `TestInspectionIsDetached` mutates input and an inspection and verifies original subsequent evidence; `TestEmptyAndUninitializedEvaluationsDiffer` distinguishes the constructor's empty pool from zero/nil.
- [ ] Write named tests and observe expected compile RED for the new API.
- [ ] Implement the value-only evaluator and inspect operation.
- [ ] Run focused tests and full owning-module race gate; commit the meaningful checkpoint.
**Verification:** from `mind/behavior`, `go test -race -count=1 ./selection` and `go test -race -count=1 ./...`; source boundary via `go list -f '{{join .Imports "\n"}}' ./selection`.
**Evidence:** RED/GREEN logs, unchanged manifests, commit; no game/wire/persistence surface involved.

### Task 2: Weighted and explicit choices, concrete examples

**Delivers:** R5, R7. **Prerequisites:** Task1 Evaluation.
**Files:** create `selection/choose.go`, `selection/choose_test.go`, `selection/example_test.go`.
**Interfaces:** Choose/Draw/Choice/DrawOutput/Die above, all consuming the same captured Evaluation; no re-evaluation or source callbacks.
**Tests:** `TestChoiceSuite/TestEveryFaceUsesAuthoredOrder` enumerates faces1..10 for a6/4 pool with a zero-share row and asserts selected IDs plus exact die size/count; `TestExplicitChoiceDoesNotNeedRandomness` selects an eligible zero-share row; `TestSelectionRefusals` covers unknown/ineligible ID, nil/zero evaluation, empty vs zero-weight, missing die and nil context; `TestInvalidFacesAndDieErrors` covers0/negative/too-large/error, with no fabricated result; `TestChoiceEvidenceIsDetached` mutates output and draws again. Executable `Example_guardAndDialogue` uses a fact-gated guard search and explicit player reply, literal expected output; it is a usage proof, not new game functionality.
- [ ] Add tests; observe expected missing-choice-API RED.
- [ ] Implement Choose and one-die Draw with no implicit entropy.
- [ ] Add/run the concrete example; run full module race gate and commit.
**Verification:** `go test -race -count=1 ./...`; actual draw arguments/calls asserted by a strict supplied test die.

### Task 3: The real behavior picker uses the primitive

**Delivers:** R6 plus joined consumer proof and documentation. **Prerequisites:** Tasks1/2.
**Files:** modify `mind/behavior/table.go`, `doc.go`; create `selection_integration_test.go`, module `knowledge/runs/2026-11-10-behavior-selection.md`; finish these design docs/plan and `implementation.md`.
**Interfaces:** existing PickInput/Output, Candidate/RowVerdict/Path stay unchanged. `judge` builds per-level selection candidates from independently evaluated When and affordability, passing the existing percent as one named factor; core Inspect provides weights/evidence. Pick uses core Draw per visited level; remove the old `land` algorithm. Behavior keeps nested recursion and its own hold/silence policy. Selection errors become ErrBadTable with cause.
**Tests:** `TestSelectionIntegrationSuite/TestBadFacesAreRefused` fails on the old fallback-to-row behavior; `TestOverflowDoesNotRoll` fails on old unchecked integer arithmetic. `TestNestedChoicePreservesEvidence` pins literal paths, candidate shares, parent/child rolls and an unselected sibling's no-roll evidence; `TestFactsAreJudgedOnce` counts the real Threat callback and requires one read per relevant row; `TestSpokenAnswerUsesTheSameChoice` proves a real event/Say table uses the actual picker. Retain the full existing TableSuite, GroupSuite, composer/trace/wake tests as parity controls; do not weaken them.
- [ ] Write red boundary tests and green unchanged behavior characterizations.
- [ ] Wire the real picker to selection; remove duplicate landing arithmetic.
- [ ] Run focused/full race tests, lint, same benchmark command before/after, and one assertion-bearing selection mutant.
- [ ] Update run record/docs and plan checkboxes with actual evidence; stage explicit owned files; run `PRE_COMMIT_CACHE=false .githooks/pre-commit`; commit/push and keep draft PR current.
- [ ] Verify exact-head remote jobs, then independent review and focused fixes; publish current readiness. Do not merge.
**Verification:** from `mind/behavior`, `go test -race -count=1 ./...`, `golangci-lint run ./...`, `go test -run '^$' -bench '^BenchmarkComposerDecide$' -benchmem -count=3`. Root uncached hook and `git diff --check`; docs genre check with explicit file count. Read actual remote job execution, not a skipped green wrapper.
**Evidence:** actual old-test refusals becoming green, unchanged valid chooser outputs, bounded microbenchmark costs (not game-latency evidence), mutation failure, checks/review on the PR head.

## Requirement coverage

| Requirement | Task | Concrete proof |
|---|---|---|
| Value-only, stable candidates and truthful missing requirements |1| RequirementsAndWeights; MalformedCandidates; import inspection |
| Checked, explained shares |1| literal6/4/10; product/total overflow |
| Detached evaluation and evidence |1,2| input/Inspect/Choice mutation controls |
| Weighted and explicit modes |2| every face; zero-share explicit choice; exact die calls |
| Real existing consumer without two algorithms |3| existing suites; nested/Say/read-count integration; old land removed |
| Visible, bounded proof rather than broad feature claims |2,3| executable example, docs and scoped run record |
| Gates and published review |3| race/lint/hook, actual CI jobs, independent PR verdict |

## Provider/consumer seam check

| Provider | Consumer | Contract comparison | Dependency | Proof |
|---|---|---|---|---|
| Evaluate (Task1) | Inspect/Choose/Draw (Task2) | Same private Evaluation; detached rows; eligible≠positive weight |Task1 before2| aliasing, refusal and face-boundary assertions |
| Evaluate/Draw | behavior judge/Pick (Task3) | IDs/index/order and raw integer shares preserved; one draw per selected level |Tasks1/2 before3| full nested evidence plus original suites |
| Existing domain Facts/Temper | behavior adapter | Facts read once, profile interpreted by behavior only |Existing inspected API| Threat read-count and profile/group controls |

Checked before implementation: names/types align across tasks; no new manifest, persistence, proto or UI seam is consumed; no provider tag is needed inside the same module. The only selected numeric policy is integer multiplication; no rounding/percentage default is hidden. The invalid-die/overflow behavior is a deliberate fail-closed improvement for malformed inputs, not a change to valid outcomes. Full package gates and meaningful consumer checks remain required; no global CI/test cleanup is in scope.
