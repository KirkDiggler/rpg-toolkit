# Composable maintenance — implementation receipts

This is the case file, not a release claim. [Design](design.md), [plan](plan.md), [QA](qa.md), [story #706](https://github.com/KirkDiggler/rpg-project/issues/706).

## Reviewed and walked source checkpoints

| Release unit | PR | Checked head | Evidence / remaining boundary |
|---|---|---|---|
| Generic effects | [#2327](https://github.com/KirkDiggler/rpg-toolkit/pull/2327) | `de822f5e` | Independent assemblies; strict pending/live lifetime; exactly-once per-entry finalization; full race/vet/lint and targeted mutants. The bounded joined review covers this R11 delta; earlier `3952feac` coverage remains separate. |
| D&D root | [#2329](https://github.com/KirkDiggler/rpg-toolkit/pull/2329) | `87602d8f` | Old named/concentration paths retired; pure candidate construction, exact finalization/resume; metadata/private status; full race/vet/lint. |
| Resolution | [#2344](https://github.com/KirkDiggler/rpg-toolkit/pull/2344) | `6618456d` | Native paused delivery/save/resume/damage, self-Bless ordering, complete affected-holder custody, native spell-only refusal; full race/vet/lint after site-provider catch-up. |
| Session | [#2377](https://github.com/KirkDiggler/rpg-toolkit/pull/2377) | `ccb082c9` | Current-act identity and real Manager persistence across a fresh-manager reload; structural gates and full race/vet/lint after checkpoint/offer/hail catch-up. |
| Item examples | [#2379](https://github.com/KirkDiggler/rpg-toolkit/pull/2379) | `1f3848c7` | Pins only; all existing behavior assertions and race/vet/lint pass. |
| API | [#1225](https://github.com/KirkDiggler/rpg-api/pull/1225) | `14a3edec` | Normally created cleric, actual Bless acquisition/cast/payment/reload, both private APIs and exact end cleanup; make test/ci-check pass. |
| Web | [#1410](https://github.com/KirkDiggler/rpg-dnd5e-web/pull/1410) | `87f50a35` | Current reader refusal, provider facts and mounted HUD/rail proof; full ci-check passes: 794 files, 10274 tests, existing skips unchanged. |

## Authorized release progress

The operator authorized merge-out on [#706](https://github.com/KirkDiggler/rpg-project/issues/706#issuecomment-6808119565). Base catch-up preserved parallel recognition/captain/vouch work; dependency conflicts were resolved without source conflict resolutions. Local gates and fresh current-head CI passed before each runtime merge below. Each had a final-head MERGE-READY comment review, and each tagged release was checked against its merge commit.

| Unit | Merge | Release |
|---|---|---|
| Effects #2327 | `3ed9583cd1116f42c104061008f49c9282f10dff` | v0.5.0 |
| Root #2329 | `558e499f647083affb2ad297a9c40518e058771f` | v0.215.0 |
| Resolution #2344 | `07094d313c88eb3db42d8dd9549839811a42f6e4` | v0.75.1 |
| Item examples #2379 | `43df1a5f45a5bb430db9ba3d54a5d65064c216c3` | intentionally untagged |
| Session #2377 | `270660ae135dd2adcbb01f5a9bd36d4effd77e9c` | v0.185.0 |
| API #1225 | `c6183e148b1b840037dc93b1453b794887f55cf3` | dev |
| Web #1410 | `e00205d863b0f98706003d092a19465f66ca622a` | dev |

**Resolved adoption stop:** the first final tag probe failed `TestCastleCopiesAgree`: encounter v0.198.0 had the captain/vouch castle that API dev did not yet carry. The probe was saved/restored and the content owner notified, without weakening a guard, downgrading or taking over parallel content ([stop receipt](https://github.com/KirkDiggler/rpg-project/issues/706#issuecomment-6809478590)). [API #1227](https://github.com/KirkDiggler/rpg-api/pull/1227) then merged at `eba2475a`; merging dev on top retained its canonical castle. Full `make test`/`make ci-check`, including parity, and final-head CI43130612323/43130612329 passed at `be5daab1` before API #1225 merged.

Web also merged dev's separately shipped studio line-grant/topic-wake work (#1412) on top. Full local ci-check passed **799 files / 10325 tests** (six existing skips); CI43131575295 and preview43131575317 passed at `1eb0491c` before its dev merge. No new effect-reader seam or whole-review restart was introduced.

The API's committed graph uses effects v0.5.0, root v0.215.0, resolution v0.75.1 and session v0.185.0, retaining encounter v0.198.0 and generated SDK `876f5202eb86`. No development toolkit pins remain. This is adoption into API/web **dev**, not a production dev→main release.

Journal #863 remains open. The operator's :3042/:8142 stack/run stays on its walked cut, so its served worktrees and supporting branch references are retained; cleanup must not destroy that live consumer. Machine-local receipts are preserved under `/tmp/rpg-maintenance-release-2026-11-12.Rpfwdq`. Finished unserved worktrees can be removed after the ship receipt.

The resolution wait initially guessed a minor version, but its unchanged PR title produced v0.75.1. The tagger succeeded in 14 seconds; matching the merged commit against all module tag refs corrected the wait without a retitle, hand tag or tagger rerun.

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
