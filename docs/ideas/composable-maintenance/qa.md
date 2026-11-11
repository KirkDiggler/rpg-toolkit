# Composable maintenance — bounded native QA

Owning checklist: [Composable conditions S3 #706](https://github.com/KirkDiggler/rpg-project/issues/706). Recorded 2026-11-11. This is evidence at the heads below, not release authorization or coverage of future catch-up changes.

## Served cut

- `local/composable-maintenance`: web `http://localhost:3042`, API `http://localhost:8142`.
- Detached served API `14a3edecac0a2db310bfac42543cee065a056748`; web `87f50a3546fedc2ae25351632ce3531f93840327`.
- Binary providers: root `87602d8f`, resolution `6618456d`, session `ccb082c9`, effects `de822f5e`; encounter `v0.196.0`, generated SDK `876f5202eb86`.
- Mounted content SHA-256 rechecked after QA, unchanged:
  - `composable-maintenance-walk.yaml`: `459c4b747e524958195df7762eec41cc8cd9410ab3d65488736dfe4a075e63d4`;
  - `the-mad-king.yaml`: `fc657abeef915f1ee942f61fc4fbbd9973554af73d9626d128efce2b26596aaa`.
- The local-only walk site retains two neutral stationary guards with `hold: {}`. No active effect injection, fixture behavior change, forced roll or roll fishing.
- The source-fix refresh used restart, not reset. Operator identities/run data and unrelated environments remain untouched. Only the two QA-owned runs were ended through normal APIs; the stack remains available.

## Operator view and review

The operator reported seeing Bless's attack die and Bane casting saves before joined review ([early-look receipt](https://github.com/KirkDiggler/rpg-project/issues/706#issuecomment-6806261456)). That report did not claim Bane subtraction or damage-triggered concentration; stationary guards were configured to hold.

The one bounded joined review found no Critical and one Important source-identity defect. [Focused closure](https://github.com/KirkDiggler/rpg-toolkit/pull/2329#pullrequestreview-5865232723) marks [4542938014](https://github.com/KirkDiggler/rpg-toolkit/pull/2329#discussion_r4542938014) Addressed at root `87602d8f` → API `14a3edec` → web `87f50a35`. No unaffected review scope was restarted. Reviewer-run root/API proof and the source mutant are separate from reused exact-head full gates and the builder's browser walk below.

## Two-client method

Two isolated Chrome contexts used the served application's generated clients and normal authenticated public RPCs. This is native transport/persistence and mounted GameView evidence, not a claim that every creation or cast gesture was clicked manually. The native status inspector and camera controls were clicked. Each phase captured both clients' public story/live stream, private projection, page errors and screenshot; reload phases reopened both contexts.

Fresh identities `maintenance-qa-cleric` and `maintenance-qa-bard` normally created/finalized **QA Mercy** (Life cleric) and **QA Cadence** (Bard):

- Mercy: `char_cfd563d0-4df5-49cd-9337-b1c902ad17e6`;
- Cadence: `char_f39f15f3-c882-4fce-b9dd-a4780425b214`.

Walk session: `350e55c1-f311-4e28-a6cf-2c9783dbd4d9`. Castle session: `a458284b-7482-4b52-87a2-be685e789daf`. Standard sandbox seeds and the operator's characters were not used.

## Walked checklist

- [x] **Acquisition/payment:** public draft validation accepted both characters. Life domain supplied Bless; Bard selected Bane. Bless changed only Mercy's first-level slots from 2 to 1; Bane paid Cadence's slot separately.
- [x] **Two-recipient delivery/private reads:** both clients received Blessed with Mercy's immutable member ID. Mercy separately held Concentrating on Bless; its inspector displayed `Source: QA Mercy`, not an invocation or `Name unavailable`. [Native inspector](qa/source-inspector.png).
- [x] **Reload:** fresh browser contexts/private reads retained payloads, their sources and independent maintainers. The first caster turn end consumed grace without prematurely reducing the ten remaining ends.
- [x] **Bane delivery:** one practice guard naturally failed `4 vs DC 13`; the other saved `13 vs DC 13`. Both live streams carried those distinct outcomes. No retry sought a different result.
- [x] **Signed dice:** Sacred Flame against the affected guard produced `1d20-1d4+0`, faces `[5,1]`, total **4**, with the subtracting d4 attributed to Cadence. Both live streams contain act `01M767HCMZ088PG75JDTKWW35X`.
- [x] **Bless attack:** Cadence's one unarmed attack was `4 + Bless 4 + modifier 1 = 9`, a miss against AC 13; act `01M767S8087B3Y8108Y5BVWF6J`. A local driver serialization error occurred after the RPC returned. The attack was **not retried**: both clients' saved story/replay supplied the receipt. This is not claimed as a captured first-delivery animation.
- [x] **Replacement/source isolation:** Mercy cast existing Guidance on Cadence. Both old Blessed attachments disappeared; Cadence received Guidance while retaining their own Concentrating on Bane. This does not claim migration of the Guided payload runtime.
- [x] **Departure:** Cadence's normal Exit removed their maintenance and target-held Guidance; Mercy's now-empty Guidance maintenance ended too. Both private sheets retained only the unrelated Opportunity Attack condition.
- [x] **Castle rest:** the same normally acquired characters entered the castle, equipped their owned gear, received a fresh Bless, then took one group short rest. Both linked payloads and the maintainer disappeared; Mercy's remaining slot stayed **1**, not refunded. Client-side Rest RPC time was **1438 ms**. No performance deferral or SDK-cost attribution is made from that transport timing.
- [x] **Relevant standing castle fight:** outside in free roam, normally approach the visible gate guards, recast Bless, then shoot `gate-1` with the acquired light crossbow. The attack was `13 + Bless 4 + modifier 2 = 19`, hit AC 13, and dealt **1** damage. Fight formation `01M768K8501PHG811WTBWWRMRH` included both players and the three visible garrison members (`gate-1`, `gate-2`, `captain`). Bane was cast on eligible seen guards on the Bard's turn. One bounded round was followed; the captain's longsword attack was **10 vs AC 16, miss**, so no positive caster damage/concentration claim follows. [Cleric view](qa/castle-cleric.png), [Bard view](qa/castle-bard.png).
- [x] **End cleanup:** normal lobby abandonment ended the QA castle run. Both private sheets again retained only Opportunity Attack; spent slots stayed spent. No Redis reset, fixture edit or operator-run teardown.
- [x] **Receipt checks:** 52 assertions over the saved two-client receipts pass, including exact source, payment, independent replacement, cleanup, signed dice and a single original attack. Completed browser phases report zero page errors. Screenshots were opened and checked for rendering.

Approach requests were allowed to stop at real obstructions; final placement, not requested destination, is reported. Unchanged castle stealth, posted-door entry/pass and dialogue branches were not re-walked. This wave's relevant castle coverage is the native effect/attack/fight path, not a blanket assertion about every castle route.

## Deliberately test-backed, not manufactured browser outcomes

| Branch | Named owning proof |
|---|---|
| Earlier Bane saved while another target answers; stable resume/payment | Session `TestCastSuite/TestLinkedApplicationPersistsEarlierDeliveryAcrossWindowReload`; root `TestApplicationBindingSuite/TestPendingDeliveryIsSaveableAndResumesByExactKey` |
| One concentration check for multiple recipients, exact failure cleanup/one HP application | Resolution `TestLinkedApplicationSuite/TestCastSealReloadAndExactRemoval`, `TestSelfBlessConcentrationUsesDieBeforeCleanup` (self-Bless `3 + CON 1 + 4 = 8` before cleanup) |
| Two Bless sources, independent Bane, selected source removal | Root `TestBlessedSuite/TestTwoBlessesDoNotStackAndBaneRemainsIndependent`, `TestDescribeSelectedRollContributionsKeepsOldestPerConditionGroup` |
| Clock expiry/grace across reload | Root `TestApplicationBindingSuite/TestClockCountsCasterEndsAndPreservesGraceAcrossReload` |
| Long rest across all affected holders | Resolution `TestLinkedApplicationSuite/TestLongRestPersistsEveryAffectedHolder` |
| Arbitrary maintained physical resistance without granting Rage | Root `TestApplicationBindingSuite/TestConcentratedResistanceWithAndWithoutMaintenance`, `TestIndependentRagingSurvivesWardEnd`; toolkit composition only, not native acquisition |

## Release boundary

- [x] Operator early view; joined review and focused disposition; bounded two-client/native/castle QA at the recorded served cut.
- [ ] Latest-base merge-on-top catch-up: GitHub reports session #2377/API #1225 conflicting; read-only merge-tree identifies their `go.mod`/`go.sum` conflicts. Preserve parallel changes and gate any resulting material delta.
- [ ] Current-head remote CI: provider/resolution/session/items receipts pass; root run `43097342080` and web CI `43098308976` are queued at this check. API has no current-head run. Local same-head full gates are not substituted for those missing remote results.
- [ ] Explicit merge-out authority, CI-produced provider tags, consumer re-pins and affected gates. Nothing in this wave has merged. Journal #863 remains open.

Machine-local raw receipts/drivers remain in `rpg-toolkit/.worktrees/maintenance-effects/.planning/qa-*`; `qa-verified-summary.json` carries exact act facts. The screenshots above are rendered game output, not licensed source assets or converted models.
