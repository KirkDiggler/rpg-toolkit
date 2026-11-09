# Shared Raging composition — implementation and release record

Written 2026-11-07. Scope and completion remain owned by [story #703](https://github.com/KirkDiggler/rpg-project/issues/703) and [S9–S12](plan.md#slice-sequence-and-worktrees). This preserves the source-stage checkpoints and native QA, followed by the authorized release receipts below. The broader conditions/items stories remain open.

## Pre-release source checkpoints

| Slice / owner | PR / head | Observed scope |
|---|---|---|
| S9 mechanics/effects | [#2297](https://github.com/KirkDiggler/rpg-toolkit/pull/2297), `605694f3` | Owned snapshot/output observations, complete-output validation, nonterminal Changed notification |
| S10 D&D root | [#2299](https://github.com/KirkDiggler/rpg-toolkit/pull/2299), `c00e0d76` | Shared kind registry, one composed Raging definition/codec, native Rage creator and four-domain information |
| S11 items | [#2300](https://github.com/KirkDiggler/rpg-toolkit/pull/2300), `085a7608` | Item grant uses shared creator; private Rage recipe/vocabulary removed; spent loot and active-condition restoration retained |
| S12 resolution | [#2304](https://github.com/KirkDiggler/rpg-toolkit/pull/2304), `e6e26a2d` | Real native activation and damage → dirty save → interaction reload; current-format fixtures |
| S12 session | [#2306](https://github.com/KirkDiggler/rpg-toolkit/pull/2306), `277ed80e` | Native Afford/payment/progress tests, plural rows, source entity ID retained |
| S12 protos | [#422](https://github.com/KirkDiggler/rpg-api-protos/pull/422), merge `1fc63b4` | Operator merged; generated v0.1.259 at `7dcb1b2` published and adopted |
| S12 API | [#1193](https://github.com/KirkDiggler/rpg-api/pull/1193), `f843fde` | Current raw data, origin/description and source-ID wire mapping; real handler/Redis integration |
| S12 web | [#1398](https://github.com/KirkDiggler/rpg-dnd5e-web/pull/1398), `156a39bc` | Canonical reader; no client bonus formula; published SDK and source-ID preservation |

At those checkpoints, runtime PRs remained drafts on development pins or incomplete integration. No toolkit provider was merged merely to unblock development; no legacy Rage codec, silent reset, replace/go.work or manual tag was added.

## Released adoption

The operator explicitly authorized inside-out merge/tag/re-pin after the bounded QA and focused review closure. Each module merged after CI-green head-pinned COMMENT gate verdicts; consumers adopted actual published tags, with tests/lint and fresh CI/verdicts after re-pins.

| Deliverable | Merged PR / commit | Published/adopted result |
|---|---|---|
| S9 generic observations/validation/notification | [toolkit #2297](https://github.com/KirkDiggler/rpg-toolkit/pull/2297), `f1169db7a33375a41989d6be217ab78e9020913a` | `mechanics/effects/v0.4.0` |
| S10 shared rulebook/native creator and sustain fix | [toolkit #2299](https://github.com/KirkDiggler/rpg-toolkit/pull/2299), `66cca6e8b05c45e0bd3a04e675906673fd8e1e3d` | `rulebooks/dnd5e/v0.214.0` |
| S11 shared item consumer | [toolkit #2300](https://github.com/KirkDiggler/rpg-toolkit/pull/2300), `b77d520a9fe6758a5302b837a07e5f8916106214` | Intentionally untagged example; effects v0.4.0 / D&D v0.214.0 |
| S12 resolution | [toolkit #2304](https://github.com/KirkDiggler/rpg-toolkit/pull/2304), `37c2460addaa19822a5684631e49e789f4855f82` | `rulebooks/dnd5e/resolution/v0.74.3` |
| S12 session | [toolkit #2306](https://github.com/KirkDiggler/rpg-toolkit/pull/2306), `6237e39cf73ce97e7c4fab1eed72f7b0f1209404` | `rulebooks/dnd5e/session/v0.170.0` |
| S12 API | [API #1193](https://github.com/KirkDiggler/rpg-api/pull/1193), `4b4b081b68c95580a1ffbb90735a25cddc9b2675` | Merged to `dev` with all toolkit pins released |
| S12 web | [web #1398](https://github.com/KirkDiggler/rpg-dnd5e-web/pull/1398), `adc9f64103f9f171d510f72d862f0ab46a1cfd4c` | Merged to `dev`; published proto v0.1.260 retained |

Each toolkit tag was verified at its exact merge commit, not inferred from CI intent. The proto source-ID addition first published v0.1.259 at 7dcb1b2; final API/web retain the later published v0.1.260 at generated d83fdfa5 from parallel work. The Go SDK's submodule pseudo-version identifies that published generated commit, not an unmerged provider. Final API module inventory contains no toolkit development pseudo-versions.

Release catch-ups preserved landed optional continuations, per-verb attachment, delta folding, commit-counted buffers and listener integration from the parallel performance wave; no source conflict was hand-resolved. Final API retains encounter v0.184.0. Re-pin edits themselves were only go.mod/go.sum; catch-up merges are separately identified in git. Full API make test/ci-check and final-head build/test CI passed; web full ci-check and head CI passed. No dev → main promotion or operator-stack rebuild was performed.

## Verification receipts

The individual module run records and PRs retain their RED/GREEN and gate evidence. S9/root/items/resolution reported their full module race/vet/lint gates before the session/API/web checkpoint; these are not claims of a new all-workspace gate.

- Session: full race suite passed after final source changes in 416.490 seconds; vet/lint passed. The prior 180-second command timed out. A new resolution pin exposed two deprecated TargetID initializers; both now use singleton TargetIDs, with the full gate rerun. Head CI [42416512228](https://github.com/KirkDiggler/rpg-toolkit/actions/runs/42416512228) passed, including changed-module tests.
- Protos: make format/make test passed; head CI [42415681182](https://github.com/KirkDiggler/rpg-api-protos/actions/runs/42415681182) passed lint/format and Go/TypeScript generation validation. [Head-specific mechanical-addition verdict](https://github.com/KirkDiggler/rpg-api-protos/pull/422#pullrequestreview-5815114083) is not merge authorization or an independent runtime review.
- API: the converter test first failed on missing origin/description; direct metadata mapping fixed it. Full make test passed, including cmd/server, and make ci-check passed in 483.95 seconds after fixing one new import group. The initial combined 600-second command had completed make test before timing out during ci-check. The new native test crosses real handlers and Redis-backed storage; it is not a browser walk.
- Web: ten focused assertions failed before implementation; 52 focused cases and type checking passed afterward. Complete ci-check passed in 569.24 seconds: 760 files passed, one existing file skipped; 9,921 tests passed and six existing tests skipped. Initial 300-second command timeout was not counted as passing. Dependency advisories and Browserslist/jsdom/nonfatal npm notices remain recorded in the web PR; no skips or dependency auto-fixes were added.

## Consumer-driven correction

The approved provenance requirement was not fully representable in the existing SDK/wire row. `contributions.Source.SourceID` existed, but session EffectRow and proto EffectRow omitted it. A failing projection test now guards the SDK copy. Proto #422 adds a dedicated field; parsing an opaque row ID or inventing a source label would not preserve the fact.

The character reader also discarded the new origin ref and never supplied the registry description. Its existing conversion point now maps those metadata values and passes raw JSON unchanged. It does not calculate turns remaining, class scaling, fixed-item benefits or eligibility. The web reader treats bindings as opaque and reads only persisted progress and server text.

This updates the original plan's wire assumption without revisiting Raging-only scope or the rejected legacy-save path. The [publication question](https://github.com/KirkDiggler/rpg-project/issues/884#issuecomment-6713320177) asked only for the additive proto merge so CI could publish bindings. The operator then merged it and reported generation complete. The rest of the wave stays draft.

## Published SDK adoption and isolated boot

- Verified proto merge `1fc63b4e3aca74cab8ef9181a471ccdd3fcc9b98`, successful publication [CI 42422302400](https://github.com/KirkDiggler/rpg-api-protos/actions/runs/42422302400), and v0.1.259 at generated `7dcb1b2ebf4c962f9c0943171668bd0677969de6`. Go uses the published submodule pseudo-version `v0.0.0-20261107224950-7dcb1b2ebf4c`; the web lockfile resolves the identical commit. No local generation/substitution.
- Declaration/held-row converter assertions and all four native Rage row source-ID assertions failed before direct mapping, then passed. Client target-overlay/held-row and DOM metadata tests also failed before preservation, then passed; row IDs remain unchanged and no source label is invented.
- At API `2bf39e23`, full make test passed in 426.37 seconds and make ci-check in 491.72 seconds. At web `156a39bc`, complete ci-check passed in 593.16 seconds: 760 test files passed, one existing skipped; 9,923 tests passed, six existing skipped. These are local gates, not an inferred fresh-head GitHub verdict.
- Booted `local/raging-composition`: API detached worktree at `2bf39e234fbeeefac1c648e1a7ad477541e70a43`, web managed snapshot at `156a39bc9340fe0bb28f0cdc4a842f441e417047`; web http://localhost:3307/?playerId=raging-preview, API :8307. Normal API image `sha256:bcdc4a70a7158dd0455faf39a18a8fcb67ca1ae4cbc67736ffccfa34bca649b3`; API/Redis healthy and Vite running.
- Local untracked `content/raging-composition-walk.yaml` cuts open courtyard cells and the unchanged captain template from the castle, placing the captain beside the party without a compulsory fight or travel. The live registry lists **Raging Composition Walk**. Local and container `/content/raging-composition-walk.yaml` SHA-256 both equal `660b20dd089b9cc4f7760fd84c267da7a54ec24d8e6c2d3f5a1e264e90c43115`.
- Landing page screenshot was rendered and inspected with zero browser console/page errors; live registry queried through the real web client. The shared screenshot helper lacked its Playwright Chromium binary, so the probe used installed Chrome in a fresh headless context. An initial container hash command used the wrong path; inspecting the mount identified `/content`, whose hash matches above. A direct JSON curl was refused (415); the real client's transport succeeded. None of these boot probes is gameplay QA.

## Independent round and QA after the early look

The operator reported “raging is acting like normal.” The one independent round then published seven head-specific reviews with no concrete code findings: toolkit [#2297](https://github.com/KirkDiggler/rpg-toolkit/pull/2297#pullrequestreview-5815655441), [#2299](https://github.com/KirkDiggler/rpg-toolkit/pull/2299#pullrequestreview-5815655472), [#2300](https://github.com/KirkDiggler/rpg-toolkit/pull/2300#pullrequestreview-5815655517), [#2304](https://github.com/KirkDiggler/rpg-toolkit/pull/2304#pullrequestreview-5815655559), [#2306](https://github.com/KirkDiggler/rpg-toolkit/pull/2306#pullrequestreview-5815655593), [API #1193](https://github.com/KirkDiggler/rpg-api/pull/1193#pullrequestreview-5815655627), [web #1398](https://github.com/KirkDiggler/rpg-dnd5e-web/pull/1398#pullrequestreview-5815655668). Remaining QA/release/CI prerequisites were explicit; no merge authorization was inferred.

API CI was absent because landed performance pins conflicted with the branch. Merged toolkit main into session (9f462d97) and dev into API (40fe078), preserving encounter v0.181.0 and the Raging graph. Full gates passed; API 40fe078 got green build/test CI. Later dev pin work advanced again; current API-head base reconciliation/CI remains pending, not a green result inferred from 40fe078.

Two fresh identities were created through normal draft/finalize RPCs, with no injected active condition or deleted character. Two isolated browser contexts rendered the real game. Native session `4b770757-0ae5-41de-9699-5028ca878501` showed actor Rage charge 2 → 1 while observer stayed 2; both received activation act `activate:01M6W9QMX5058S0S5WZK2YCDFN`. The first boundary persisted state raging, anchor 1, turns_active 1; full browser reload retained it and the spent charge. Four rows reached the rendered DOM with source identity, only +2 outgoing damage applying. A missed attack set the sustain flag and the next boundary reached turns_active 2. Combat end by decision removed Raging; short rest healed HP 2 → 10, spent a hit die, did not refill long-rest Rage charges and did not recreate the condition. These are bounded observations, not all seven checklist boxes declared complete. The native HUD uses server ConditionView; the otherwise unmounted raw ConditionBadge has component proof, not a claim it appeared in that HUD.

That walk found [#2312](https://github.com/KirkDiggler/rpg-toolkit/issues/2312): a real NPC hit correctly resisted 10 → 5 left was_hit_this_turn false. The shared vocabulary had inherited the old handwritten condition's legacy DamageReceived subscription. Native resolution intentionally publishes post-apply DamageTaken instead to avoid double application. Fixed the rulebook binding to positive applied facts, not the publisher. Strengthened root and real-resolution tests failed before the fix and passed afterward: consumed grace, one HP application, no legacy notification, dirty snapshot, reload, one sustain and later inactivity removal. Root c00e0d76 / resolution e6e26a2d / items 085a7608 / session 277ed80e / API f843fde carry the fix with full owning gates passing; session race took 374.526 seconds. The isolated stack now serves API f843fde with web 156a39bc, without resetting Redis. Only focused review closure of this concrete new QA finding was warranted; it is now published at the root/resolution heads, with a reviewer mutant failing on the old binding and passing after restoration.

The local untracked walk YAML initially contaminated shipped-content glob tests during catch-up gates. Moving only that local file out for the full gates and restoring it afterward resolved those failures without skipped tests or changes to served content; [API #1196](https://github.com/KirkDiggler/rpg-api/issues/1196) records the fixture-discovery gap. Browser mount transcripts retain React StrictMode cleanup AbortError logs; no warning-free game-view claim is made.

## Bounded QA completed

[QA receipt](qa.md) records the corrected walk-site run, normal long-rest re-admission, and castle repeat on API f843fde / web 156a39bc. The castle's native record carries greataxe 7 + Strength 3 + Raging 2 = 12, with the Raging source entity ID; world time advances once as the free-roam blow forms the fight. First-boundary/reload, attack sustain, inactivity/end removal, separate budgets and short-/long-rest behavior are evidenced. Both browser clients and local screenshots were inspected.

Positive damage-only sustain is explicitly test-backed under the walk skill's dice-dependent-row rule: native NPC attacks during the bounded post-fix recheck missed, and no outcomes were patched or rerolled to force a hit. The real-resolution regression and the reviewer's temporary legacy-binding mutant prove that branch. This is not relabeled as a post-fix live hit. Raw ConditionBadge remains component proof; the native HUD reads ConditionView.

Focused closure: [root review 5815981056](https://github.com/KirkDiggler/rpg-toolkit/pull/2299#pullrequestreview-5815981056), [resolution review 5815981097](https://github.com/KirkDiggler/rpg-toolkit/pull/2304#pullrequestreview-5815981097); no narrow finding. No new whole-diff round was run.

## Remaining scope and limits

- Other condition families, native item inventory and designer tooling were not part of this Raging-only cut.
- The positive incoming-damage sustain branch remains explicitly test-backed, not a manufactured live-hit claim; see the QA receipt.
- [API #1196](https://github.com/KirkDiggler/rpg-api/issues/1196) tracks local walk-fixture discovery in shipped-content tests.
- [Journal #863](https://github.com/KirkDiggler/rpg-project/pull/863) stays open; future field-note/recording work and the broader stories are separate.
- Promotion from API/web dev to main remains the operator's release gate.

No stack was booted in the initial source checkpoint; the follow-on isolated boot is recorded above. The operator's :3030/:8115 and character data were untouched. The independent round, focused QA-finding closure, bounded native QA and released adoption are recorded above. Slice closure and exact-path cleanup are reported on their owning issues after these receipts; this record does not authorize unrelated cleanup.
