# Shared Raging composition — implementation checkpoint

Written 2026-11-07. Scope and completion remain owned by [story #703](https://github.com/KirkDiggler/rpg-project/issues/703) and [S9–S12](plan.md#slice-sequence-and-worktrees). This is a source-stage checkpoint, not a release or native-walk closeout.

## Source heads

| Slice / owner | PR / head | Observed scope |
|---|---|---|
| S9 mechanics/effects | [#2297](https://github.com/KirkDiggler/rpg-toolkit/pull/2297), `605694f3` | Owned snapshot/output observations, complete-output validation, nonterminal Changed notification |
| S10 D&D root | [#2299](https://github.com/KirkDiggler/rpg-toolkit/pull/2299), `3e771a63` | Shared kind registry, one composed Raging definition/codec, native Rage creator and four-domain information |
| S11 items | [#2300](https://github.com/KirkDiggler/rpg-toolkit/pull/2300), `b2dfcf14` | Item grant uses shared creator; private Rage recipe/vocabulary removed; spent loot and active-condition restoration retained |
| S12 resolution | [#2304](https://github.com/KirkDiggler/rpg-toolkit/pull/2304), `9425c9da` | Real native activation → boundary → interaction reload and current-format fixtures |
| S12 session | [#2306](https://github.com/KirkDiggler/rpg-toolkit/pull/2306), `2909290a` | Native Afford/payment/progress tests, plural rows, source entity ID retained |
| S12 protos | [#422](https://github.com/KirkDiggler/rpg-api-protos/pull/422), `8657b1e` | Additive EffectRow.source_id field 9; publication pending |
| S12 API | [#1193](https://github.com/KirkDiggler/rpg-api/pull/1193), `eb1ee21` | Current raw data, origin ref and registry description; real handler/Redis integration; source-ID wire mapping pending |
| S12 web | [#1398](https://github.com/KirkDiggler/rpg-dnd5e-web/pull/1398), `71937ad6` | Canonical reader and badge progress/server description; no client bonus formula; generated SDK adoption pending |

Runtime PRs remain drafts on development pins or incomplete integration. No provider was merged merely to unblock a consumer; no legacy Rage codec, silent reset, replace/go.work or manual tag was added.

## Verification receipts

The individual module run records and PRs retain their RED/GREEN and gate evidence. S9/root/items/resolution reported their full module race/vet/lint gates before the session/API/web checkpoint; these are not claims of a new all-workspace gate.

- Session: full race suite passed after final source changes in 416.490 seconds; vet/lint passed. The prior 180-second command timed out. A new resolution pin exposed two deprecated TargetID initializers; both now use singleton TargetIDs, with the full gate rerun. Head CI [42416512228](https://github.com/KirkDiggler/rpg-toolkit/actions/runs/42416512228) passed, including changed-module tests.
- Protos: make format/make test passed; head CI [42415681182](https://github.com/KirkDiggler/rpg-api-protos/actions/runs/42415681182) passed lint/format and Go/TypeScript generation validation. [Head-specific mechanical-addition verdict](https://github.com/KirkDiggler/rpg-api-protos/pull/422#pullrequestreview-5815114083) is not merge authorization or an independent runtime review.
- API: the converter test first failed on missing origin/description; direct metadata mapping fixed it. Full make test passed, including cmd/server, and make ci-check passed in 483.95 seconds after fixing one new import group. The initial combined 600-second command had completed make test before timing out during ci-check. The new native test crosses real handlers and Redis-backed storage; it is not a browser walk.
- Web: ten focused assertions failed before implementation; 52 focused cases and type checking passed afterward. Complete ci-check passed in 569.24 seconds: 760 files passed, one existing file skipped; 9,921 tests passed and six existing tests skipped. Initial 300-second command timeout was not counted as passing. Dependency advisories and Browserslist/jsdom/nonfatal npm notices remain recorded in the web PR; no skips or dependency auto-fixes were added.

## Consumer-driven correction

The approved provenance requirement was not fully representable in the existing SDK/wire row. `contributions.Source.SourceID` existed, but session EffectRow and proto EffectRow omitted it. A failing projection test now guards the SDK copy. Proto #422 adds a dedicated field; parsing an opaque row ID or inventing a source label would not preserve the fact.

The character reader also discarded the new origin ref and never supplied the registry description. Its existing conversion point now maps those metadata values and passes raw JSON unchanged. It does not calculate turns remaining, class scaling, fixed-item benefits or eligibility. The web reader treats bindings as opaque and reads only persisted progress and server text.

This updates the original plan's wire assumption without revisiting Raging-only scope or the rejected legacy-save path. The [publication question](https://github.com/KirkDiggler/rpg-project/issues/884#issuecomment-6713320177) is explicit: authorize only the additive proto merge so CI can publish bindings. The rest of the wave stays draft.

## Still unproved / incomplete

- Published source-ID SDK adoption, API mapping and client value preservation.
- Normal fresh-character creation on the exact isolated stack; served image/source/content verification.
- Operator early view/go, standing/relevant native walk and one independent runtime review round.
- Authorized inside-out merges, actual provider tags, final consumer re-pins and release receipts.

No stack was booted or changed in this checkpoint. The operator's :3030/:8115 and character data were untouched. No slice is closed by this record.
