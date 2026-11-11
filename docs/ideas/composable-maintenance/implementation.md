# Composable maintenance — implementation receipts

This is the case file, not a release claim. [Design](design.md), [plan](plan.md), [QA](qa.md), [story #706](https://github.com/KirkDiggler/rpg-project/issues/706).

## Current source checkpoints

| Release unit | PR | Checked head | Evidence / remaining boundary |
|---|---|---|---|
| Generic effects | [#2327](https://github.com/KirkDiggler/rpg-toolkit/pull/2327) | `de822f5e` | Independent assemblies; strict pending/live lifetime; exactly-once per-entry finalization; full race/vet/lint and targeted mutants. Earlier review at `3952feac` does not cover this R11 delta. |
| D&D root | [#2329](https://github.com/KirkDiggler/rpg-toolkit/pull/2329) | `678e9d74` | Old named/concentration paths retired; pure candidate construction, exact finalization/resume; metadata/private status; full race/vet/lint. |
| Resolution | [#2344](https://github.com/KirkDiggler/rpg-toolkit/pull/2344) | `6618456d` | Native paused delivery/save/resume/damage, self-Bless ordering, complete affected-holder custody, native spell-only refusal; full race/vet/lint after site-provider catch-up. |
| Session | [#2377](https://github.com/KirkDiggler/rpg-toolkit/pull/2377) | `ccb082c9` | Current-act identity and real Manager persistence across a fresh-manager reload; structural gates and full race/vet/lint after checkpoint/offer/hail catch-up. |
| Item examples | [#2379](https://github.com/KirkDiggler/rpg-toolkit/pull/2379) | `1f3848c7` | Pins only; all existing behavior assertions and race/vet/lint pass. |
| API | [#1225](https://github.com/KirkDiggler/rpg-api/pull/1225) | `a8463915` | Normally created cleric, actual Bless acquisition/cast/payment/reload, both private APIs and exact end cleanup; make test/ci-check pass. |
| Web | [#1410](https://github.com/KirkDiggler/rpg-dnd5e-web/pull/1410) | `970625f9` | Current reader refusal, provider facts and mounted HUD/rail proof; full ci-check passes: 794 files, 10274 tests, existing skips unchanged. |

Runtime PRs are **NOT MERGE-READY**: operator early look, joined independent review, bounded native QA and actual provider release re-pins remain. No merge authorization is implied by these source checkpoints.

## Consumer findings that changed the implementation

1. R11: native Bane persists an earlier delivery while another target answers. The whole-application seal was the wrong granularity. Kirk settled fixed intended recipients, per-entry finalization and valid pending entries; the coordinator owns pending/live membership, not a second global delivery state machine.
2. Monster pure loading previously retained raw blobs until Apply. Binding before attachment therefore could not see a saved monster recipient. Root now prepares actual behaviors before binding, keeps one state copy and preserves failed-attach retry/rollback.
3. Freezing the bound declaration turned absent RawMessage config into literal null. Explicit declaration data tags preserve absence; there is no runtime normalization.
4. Bless/Bane content still requested an opaque source_id parameter after its old codec was retired. The declaration-to-creator probe failed; provenance/address bindings now carry that fact explicitly.
5. API's string-only ref envelope silently dropped object-ref linked data. Current toolkit loading/description now owns the reader, and bytes remain unchanged.
6. Fixing the older API conversion did not fix the native v2 private sheet. Its StatusView used the generic registry label. Root now supplies the same instance-aware maintenance description to both readers.

## Review and evidence limits

The original provider findings and focused API-addition review are closed at their recorded historical heads. They do not certify the later R11/root/native changes. One joined runtime review remains after the operator's early look; no new review campaign has started.

No browser gameplay walkthrough is claimed yet. API integration uses the actual public creation/cast/storage paths; the resistance-with/without-concentration example remains toolkit composition proof, not a shipped native item or inventory feature. No old-save reader, automatic reset, local replacement graph, manually generated SDK or manual tag was added. Journal project #863 remains open.
