# Decisions — mechanics/effects

One line per entry, dated and linked. Ingest is the writer; entries arrive through its reviewed PR.
A decision reads `- <date> · <the words> · options: <the options it closed> · because: <why> · <link> · upheld: <links, or none yet>`;
a finding, friction or open line reads `- <date> · <the words> · <link>`. A finding that needs more than a line links its case file in `findings/`.
Friction names both entries and never settles them.

## Decisions

- 2026-11-12 · complete output observation stays in mechanics/effects: `Output.Clone` and `Bound.Definition` expose detached authored data, and the rulebook consumes that provider instead of keeping a second definition authority · options: the rulebook duplicating descriptor state, or weakening configuration equality · because: consumer finding on rpg-toolkit#2399: Params omitted the guard and amount definitions an exact application check needs · https://github.com/KirkDiggler/rpg-toolkit/blob/3e03721e88bfc1c97f695d61ed93e7de4c65c063/mechanics/effects/knowledge/runs/2026-11-12-owned-output-definitions.md, https://github.com/KirkDiggler/rpg-toolkit/blob/3e03721e88bfc1c97f695d61ed93e7de4c65c063/rulebooks/dnd5e/knowledge/runs/2026-11-12-authored-effect-definitions.md, https://github.com/KirkDiggler/rpg-toolkit/pull/2399#issuecomment-6836695182 · upheld: none yet

## Findings

- 2026-11-12 · The named-input prototype is completed by promotion to mechanics/effects: Compose(ComposeInput) constructs composition, while its machine remains in the same release unit and uses New(Config). The retired legacy tags are retracted, not deleted; example consumers adopt the released replacement rather than retaining examples/effects. · https://github.com/KirkDiggler/rpg-toolkit/blob/6d8a753a4e255e38040b1b63a2bfb731832095c2/knowledge/runs/2026-10-28-composable-conditions-s1.md, https://github.com/KirkDiggler/rpg-toolkit/blob/6d8a753a4e255e38040b1b63a2bfb731832095c2/examples/effects/knowledge/runs/2026-11-04-explicit-construction-inputs.md, https://github.com/KirkDiggler/rpg-toolkit/blob/1f66f3c7dc1a1d293d19cda89a7a4f1b9397613d/knowledge/runs/2026-11-06-effects-extraction-plan.md, https://github.com/KirkDiggler/rpg-toolkit/blob/1f66f3c7dc1a1d293d19cda89a7a4f1b9397613d/knowledge/runs/2026-11-07-effects-extraction-closeout.md, https://github.com/KirkDiggler/rpg-toolkit/blob/1f66f3c7dc1a1d293d19cda89a7a4f1b9397613d/knowledge/runs/2026-11-07-effects-release-adoption.md
- 2026-10-28 · Blob flattens its declared bag with json/v2’s embed tag and refuses header-key collisions; its explicit state header means old/new condition blobs cannot be asserted byte-equal without accounting for that field. Rage-shaped anchoring sets the first available round only and derives turns_active on the graced turn too. · https://github.com/KirkDiggler/rpg-toolkit/blob/6d8a753a4e255e38040b1b63a2bfb731832095c2/knowledge/runs/2026-10-28-composable-conditions-s1.md
- 2026-11-12 · Kind validation sees the complete Output; Snapshot owns its copied state, and CurrentOutputs observes without attaching or running rules. Successful nonterminal transitions notify Changed after attachment, including propagation of callback errors; provider tests alone do not prove a native dirty save. · https://github.com/KirkDiggler/rpg-toolkit/blob/1f66f3c7dc1a1d293d19cda89a7a4f1b9397613d/mechanics/effects/knowledge/runs/2026-11-07-rulebook-consumer-support.md
- 2026-11-12 · Ending an assembly closes reaction permission before ordered removals while contribution queries remain available during End. Part, attachment and application endings are different scopes; checking only post-teardown subscriptions misses nested reactions and disappearing query outputs. · https://github.com/KirkDiggler/rpg-toolkit/blob/1f66f3c7dc1a1d293d19cda89a7a4f1b9397613d/mechanics/effects/knowledge/runs/2026-11-08-composable-maintenance-provider.md
- 2026-11-12 · A configured-effect projection follows CurrentOutputs, which retains pre-terminal observations, rather than inferring absence from Snapshot, which already reports terminal state. · https://github.com/KirkDiggler/rpg-toolkit/blob/3e03721e88bfc1c97f695d61ed93e7de4c65c063/knowledge/runs/2026-11-12-data-authored-effects-brief.md

## Friction

- 2026-11-12 · Linked lifetime distinguishes the fixed intended keys, pending keys and live members. Finalizing one recipient does not wait for every recipient, and a finalized key cannot reopen after removal; restoration preserves unresolved delivery without manufacturing live membership. This replaces the earlier whole-application seal described in the maintenance record. · https://github.com/KirkDiggler/rpg-toolkit/blob/1f66f3c7dc1a1d293d19cda89a7a4f1b9397613d/knowledge/runs/2026-11-08-composable-maintenance-design.md, https://github.com/KirkDiggler/rpg-toolkit/blob/1f66f3c7dc1a1d293d19cda89a7a4f1b9397613d/mechanics/effects/knowledge/runs/2026-11-08-composable-maintenance-provider.md

## Open

- none yet
