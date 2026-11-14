# Native Rage — measured baseline for the maintenance contract

This is evidence at runtime source `c99f3d854acb49a647fd24c743ff52aa4c3e1b0f`, not a new implementation or proposed rules change. On 2026-11-14, `go test -count=1 -v ./conditions -run Raging` passed from the owning rulebook module. Full output is retained locally in the design worktree's `.design-notes/r9-next-raging-baseline.log`.

Source: [native recipe](https://github.com/KirkDiggler/rpg-toolkit/blob/c99f3d854acb49a647fd24c743ff52aa4c3e1b0f/rulebooks/dnd5e/conditions/raging_definition.go), [runtime tests](https://github.com/KirkDiggler/rpg-toolkit/blob/c99f3d854acb49a647fd24c743ff52aa4c3e1b0f/rulebooks/dnd5e/conditions/raging_test.go), [restore tests](https://github.com/KirkDiggler/rpg-toolkit/blob/c99f3d854acb49a647fd24c743ff52aa4c3e1b0f/rulebooks/dnd5e/conditions/composed_raging_test.go).

## Behavior we must preserve

| Case | Observed current contract | Existing proof |
|---|---|---|
| Activation grace | The first holder turn end consumes grace without checking activity, even without a valid round | `TestTheTurnARageStartedIsNotChecked`, `TestARoundlessTurnEndDoesNotGrantPerpetualGrace` |
| Activity | Holder attack attempts include misses; positive already-applied DamageTaken sustains; idle subsequent turn ends terminate | `TestRagingConditionSustainsOnMissedAttack`, `TestRagingConditionTracksDamage`, `TestRagingConditionEndsWithoutCombatActivity`; vocabulary routes the positive damage fact |
| Missed turn-end events | Rounds `[1,2,3,4,8,9,10]` still expire at10, despite only seven received events | `TestARageThatMISSESATurnEndStillExpiresOnTime` |
| Relative duration | Rage first anchored at round4 survives through12 and expires at13 | `TestTheDurationIsRelativeToWhenTheRageStarted` |
| Derived progress | Rounds4/5/6/9 from anchor4 report turns-active1/2/3/6, not an event counter | `TestTurnsActiveIsDerivedAndStillWhatTheClientRenders` |
| Late clock information | After a roundless first end, a qualifying active end at round6 becomes the first anchor | `TestAnUnanchoredRageAnchorsAsSoonAsARoundArrives` |
| Clock reset | With activity present, an anchored round7 followed by round1 ends with `clock_reset` | `TestARoundGoingBACKWARDSEndsTheRage` |
| Missing round is not reset | After anchor3, a roundless active end does not remove the effect or alter the anchor | `TestARoundlessTurnEndDoesNotREADAsAClockReset` |
| Negative round | Round-5 consumes grace but never anchors; later valid round2 anchors and reports turns-active1 | `TestANegativeRoundNeverBecomesTheAnchor` |
| Other endings | Holder rest, combat end and dropped-to-zero end Rage; unrelated holders do not | Existing rest/combat-end/zero-HP tests in `TestRagingConditionTestSuite` |
| Restore | Current progress/origin/state survive strict reload; incomplete/old state refuses | `TestSharedComposedRaging/TestStrictRestorePreservesProgressAndRefusesOldOrIncompleteState` |

The recipe's inspected terminal order on a live holder turn end is **no activity → elapsed duration → backwards clock → continued upkeep**. That ordering is source evidence; simultaneous-terminal parity needs an explicit new characterization assertion before decomposition, not a claim that the listed tests already cover every collision.

The vocabulary's DamageReceived-named beat observes positive DamageTaken, not an instruction-like damage-request topic. A dropped-to-zero fact has its own ending meaning. The replacement must not apply HP damage again.

## Why the existing Duration adapter is not enough

The current generic rulebook Duration part counts received holder turn ends. Native Rage uses a positive round anchor and derived elapsed rounds. The seven-events/ten-rounds case distinguishes them directly. Reusing the current counter under a new field name would change gameplay, even if ordinary consecutive-round tests passed.

A reusable maintenance decomposition must preserve both grace/activity and anchored-clock semantics and their end ordering. No new mechanical type or saved representation is selected by this evidence file; those are the next native-maintenance contract.

## Native reader seam that must be included

At inspected web source `aa481c03`, `src/types/conditionData.ts` has a composed-Raging-specific reader, and `src/components/features/ConditionBadge.tsx` reads `runtime.bag.turns_active`. This is different from the opaque linked-condition reader used to justify no web change for the earlier roll-payload refactor.

If native Raging changes from its single-machine snapshot to per-part state, the replacement plan must account for this reader/projection seam. It cannot assume an API-only re-pin is enough, retain duplicate mutable state for convenience, or silently drop the visible progress fact. No web change or compatibility decision is made here.
