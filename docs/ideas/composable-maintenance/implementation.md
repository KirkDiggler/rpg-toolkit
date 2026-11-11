# Composable maintenance — implementation receipts

This is the case file, not a release claim. [Design](design.md), [plan](plan.md), [QA](qa.md), [story #706](https://github.com/KirkDiggler/rpg-project/issues/706).

## Current source checkpoints

| Release unit | PR | Checked head | Evidence / remaining boundary |
|---|---|---|---|
| Generic effects | [#2327](https://github.com/KirkDiggler/rpg-toolkit/pull/2327) | `de822f5e` | Independent assemblies; strict pending/live lifetime; exactly-once per-entry finalization; full race/vet/lint and targeted mutants. The bounded joined review covers this R11 delta; earlier `3952feac` coverage remains separate. |
| D&D root | [#2329](https://github.com/KirkDiggler/rpg-toolkit/pull/2329) | `87602d8f` | Old named/concentration paths retired; pure candidate construction, exact finalization/resume; metadata/private status; full race/vet/lint. |
| Resolution | [#2344](https://github.com/KirkDiggler/rpg-toolkit/pull/2344) | `6618456d` | Native paused delivery/save/resume/damage, self-Bless ordering, complete affected-holder custody, native spell-only refusal; full race/vet/lint after site-provider catch-up. |
| Session | [#2377](https://github.com/KirkDiggler/rpg-toolkit/pull/2377) | `ccb082c9` | Current-act identity and real Manager persistence across a fresh-manager reload; structural gates and full race/vet/lint after checkpoint/offer/hail catch-up. |
| Item examples | [#2379](https://github.com/KirkDiggler/rpg-toolkit/pull/2379) | `1f3848c7` | Pins only; all existing behavior assertions and race/vet/lint pass. |
| API | [#1225](https://github.com/KirkDiggler/rpg-api/pull/1225) | `14a3edec` | Normally created cleric, actual Bless acquisition/cast/payment/reload, both private APIs and exact end cleanup; make test/ci-check pass. |
| Web | [#1410](https://github.com/KirkDiggler/rpg-dnd5e-web/pull/1410) | `87f50a35` | Current reader refusal, provider facts and mounted HUD/rail proof; full ci-check passes: 794 files, 10274 tests, existing skips unchanged. |

Operator early look, the bounded joined review/focused finding closure and bounded native QA are recorded. Release is not authorized. Provider #2327 has no remaining implementation/review/QA blocker; consumers still need real release re-pins. A fresh GitHub check also reports session/API base conflicts (read-only merge-tree identifies only dependency-file conflicts), queued root/web CI and no current-head API run. Catch-up must preserve parallel work and rerun affected gates before release; these receipts do not cover future material deltas.

## Consumer findings that changed the implementation

1. R11: native Bane persists an earlier delivery while another target answers. The whole-application seal was the wrong granularity. Kirk settled fixed intended recipients, per-entry finalization and valid pending entries; the coordinator owns pending/live membership, not a second global delivery state machine.
2. Monster pure loading previously retained raw blobs until Apply. Binding before attachment therefore could not see a saved monster recipient. Root now prepares actual behaviors before binding, keeps one state copy and preserves failed-attach retry/rollback.
3. Freezing the bound declaration turned absent RawMessage config into literal null. Explicit declaration data tags preserve absence; there is no runtime normalization.
4. Bless/Bane content still requested an opaque source_id parameter after its old codec was retired. The declaration-to-creator probe failed; provenance/address bindings now carry that fact explicitly.
5. API's string-only ref envelope silently dropped object-ref linked data. Current toolkit loading/description now owns the reader, and bytes remain unchanged.
6. Fixing the older API conversion did not fix the native v2 private sheet. Its StatusView used the generic registry label. Root now supplies the same instance-aware maintenance description to both readers.
7. Joined review found that the private SourceMember projection still used replacement address identity. A coordinator's address contains an invocation, not a member. Root `87602d8f` now reads immutable origin only for the linked metadata protocol; API `14a3edec` and web `87f50a35` prove the same source reaches the native rail. Current Raging/unmigrated conventions remain unchanged.

## Review and evidence limits

The original provider findings and API-addition review remain closed at their historical heads. The joined round covered provider R11 plus root/resolution/session/API/web, reporting no Critical and one Important finding: [source identity](https://github.com/KirkDiggler/rpg-toolkit/pull/2329#discussion_r4542938014). [Focused closure](https://github.com/KirkDiggler/rpg-toolkit/pull/2329#pullrequestreview-5865232723) marks it Addressed at root `87602d8f`, API `14a3edec`, web `87f50a35`; it did not restart unaffected scopes. The reviewer ran focused root/API proof and a killed source mutant, reusing exact-head full gates; no web dependency installation or independent browser run was claimed.

[QA](qa.md) records two authenticated browser contexts, normally acquired characters, real casts/delivery/private reads/reload/replacement/rest/departure and the relevant castle fight. These are distinct from the operator's earlier report of Bless's attack die and Bane casting saves. Positive caster damage, a failed concentration check and a paused Bane reaction window were not manufactured in the walk; named real-path tests stand for those branches. The resistance-with/without-concentration example remains toolkit proof, not a native item or inventory feature. No old-save reader, reset, local replacement graph, hand-generated SDK or manual tag was added. Journal project #863 remains open.
