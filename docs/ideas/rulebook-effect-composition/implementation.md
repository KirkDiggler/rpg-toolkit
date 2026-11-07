# Shared Raging composition — implementation checkpoint

Written 2026-11-07. Scope and completion remain owned by [story #703](https://github.com/KirkDiggler/rpg-project/issues/703) and [S9–S12](plan.md#slice-sequence-and-worktrees). This records source-stage work and the subsequent isolated boot, not a release or native-walk closeout.

## Source heads

| Slice / owner | PR / head | Observed scope |
|---|---|---|
| S9 mechanics/effects | [#2297](https://github.com/KirkDiggler/rpg-toolkit/pull/2297), `605694f3` | Owned snapshot/output observations, complete-output validation, nonterminal Changed notification |
| S10 D&D root | [#2299](https://github.com/KirkDiggler/rpg-toolkit/pull/2299), `3e771a63` | Shared kind registry, one composed Raging definition/codec, native Rage creator and four-domain information |
| S11 items | [#2300](https://github.com/KirkDiggler/rpg-toolkit/pull/2300), `b2dfcf14` | Item grant uses shared creator; private Rage recipe/vocabulary removed; spent loot and active-condition restoration retained |
| S12 resolution | [#2304](https://github.com/KirkDiggler/rpg-toolkit/pull/2304), `9425c9da` | Real native activation → boundary → interaction reload and current-format fixtures |
| S12 session | [#2306](https://github.com/KirkDiggler/rpg-toolkit/pull/2306), `2909290a` | Native Afford/payment/progress tests, plural rows, source entity ID retained |
| S12 protos | [#422](https://github.com/KirkDiggler/rpg-api-protos/pull/422), merge `1fc63b4` | Operator merged; generated v0.1.259 at `7dcb1b2` published and adopted |
| S12 API | [#1193](https://github.com/KirkDiggler/rpg-api/pull/1193), `2bf39e23` | Current raw data, origin/description and source-ID wire mapping; real handler/Redis integration |
| S12 web | [#1398](https://github.com/KirkDiggler/rpg-dnd5e-web/pull/1398), `156a39bc` | Canonical reader; no client bonus formula; published SDK and source-ID preservation |

Runtime PRs remain drafts on development pins or incomplete integration. No toolkit provider was merged merely to unblock a consumer; no legacy Rage codec, silent reset, replace/go.work or manual tag was added.

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

## Still unproved / incomplete

- Normal fresh-character creation and the complete Rage journey on the exact isolated stack.
- Operator early view/go, standing/relevant native walk and one independent runtime review round.
- Authorized inside-out merges, actual provider tags, final consumer re-pins and release receipts.

No stack was booted in the initial source checkpoint; the follow-on isolated boot is recorded above. The operator's :3030/:8115 and character data were untouched. No gameplay walk or independent runtime review has started, and no slice is closed by this record.
