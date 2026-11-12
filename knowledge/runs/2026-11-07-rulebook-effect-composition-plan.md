# 2026-11-07 — approved Raging composition execution plan

processed: 2026-11-12T165555Z

Work: [#703](https://github.com/KirkDiggler/rpg-project/issues/703), [design #2295](https://github.com/KirkDiggler/rpg-toolkit/pull/2295).

## Shipped

- Authorized runtime merge-out completed: effects v0.4.0, D&D v0.214.0, resolution v0.74.3, session v0.170.0; item example intentionally untagged; API 4b4b081b and web adc9f641 merged to dev with released pins and fresh head gates — [release receipts](../../docs/ideas/rulebook-effect-composition/implementation.md#released-adoption).
- Accepted design/ADR and checked execution plan published before runtime edits; no completed implementation, native walk or release is claimed — [plan](../../docs/ideas/rulebook-effect-composition/plan.md).

## Decided

- Preserve parallel landed work while releasing: dependency-only conflict resolutions retained newer encounter v0.184.0 and published proto v0.1.260 instead of downgrading their consumers. Catch-up merges preserved their source; re-pin edits themselves changed manifests/checksums only. Journal project#863 stays open and no dev-to-main promotion or operator-stack restart is included — [release scope](../../docs/ideas/rulebook-effect-composition/implementation.md#released-adoption).
- KirkDiggler approved the complete design and proceeding. R1–R7 are settled; no additional planning approval, legacy Rage compatibility, automatic character reset or unrequested merge is authorized — [design](../../docs/ideas/rulebook-effect-composition/design.md).
- Slice by deliverable: S9 neutral provider support, S10 shared rulebook creator, S11 item consumer, S12 native persistence/information/reader adoption; nested toolkit modules remain separate PRs — [tracked slices](../../docs/ideas/rulebook-effect-composition/plan.md#slice-sequence-and-worktrees).

## Found

- Bounded QA and focused #2312 closure completed: normal re-admission, reload, separate budgets, end/rest and the castle's native 7 + 3 + 2 damage trace are recorded. Positive damage-only sustain remains explicitly test-backed under the walk skill's dice rule because recheck NPC attacks missed; no forced roll or false live-hit claim — [QA receipt](../../docs/ideas/rulebook-effect-composition/qa.md).
- After the operator's normal-behavior report, the independent round published seven reviews with no original code findings. Real native QA then exposed the inherited damage-sustain subscription gap; the root binding and a real resolution regression were fixed and consumer gates passed, with affected recheck/focused closure still pending at that finding checkpoint — [#2312](https://github.com/KirkDiggler/rpg-toolkit/issues/2312).
- Follow-on after operator publication: v0.1.259 at generated 7dcb1b2 is adopted by API 2bf39e23 and web 156a39bc; full local consumer gates pass. The isolated :3307/:8307 stack boots with verified heads/content and error-free landing render, awaiting the operator's early look before gameplay QA/review — [boot and SDK receipts](../../docs/ideas/rulebook-effect-composition/implementation.md#published-sdk-adoption-and-isolated-boot).
- Initial execution checkpoint: shared provider/root/item and native resolution/session/API/web branches are published; measured owning gates passed, but source-ID SDK publication, stack boot/walk, independent runtime review and release remain incomplete — [implementation record](../../docs/ideas/rulebook-effect-composition/implementation.md).
- Native consumption exposed missing source identity in SDK/wire rows; session preserved it and additive proto #422 was checked, awaiting operator publication authority at that initial checkpoint — [publication question](https://github.com/KirkDiggler/rpg-project/issues/884#issuecomment-6713320177).

- Inspected toolkit main 3972f948, API dev 590a201a and web dev 2150256f. Fresh mechanics/effects baseline race tests pass in the isolated S9 worktree — [provider](../../mechanics/effects/README.md).
- The plan's requirement and seam tables carry the approved scope through typed provider APIs, registry construction/loading, actual native persistence and the web reader; no source-only success substitutes for integrated evidence — [checks](../../docs/ideas/rulebook-effect-composition/plan.md#requirement-coverage).

## Mistaken

- Migrating the old damage subscriber and its manually published fixtures preserved an existing native-path bug. Actual post-apply damage did not set the saved sustain flag; the correction uses the documented notification and a real damage → persistence → new-interaction test rather than republishing the legacy instruction — [native evidence](../../docs/ideas/rulebook-effect-composition/implementation.md#independent-round-and-qa-after-the-early-look).
- The initial wire assumption missed SourceID; the plan now names the additive field and generated consumer pins rather than permitting an opaque-ID workaround. Initial session/API/web command budgets also ended incomplete gates; larger bounded reruns retained the full suites — [execution evidence](../../docs/ideas/rulebook-effect-composition/implementation.md).

- No new execution mistake observed at this planning checkpoint; the earlier unnecessary compatibility assumption remains visibly superseded in the design record — [ADR-0050](../../docs/adr/0050-composed-condition-instances.md).

— cross-team agent, on behalf of KirkDiggler
