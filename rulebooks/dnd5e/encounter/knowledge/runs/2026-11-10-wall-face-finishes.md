# Run — 2026-11-10 — Courtyard S5 captured wall-face finishes

Work [Project #919](https://github.com/KirkDiggler/rpg-project/issues/919), implementation [Web #1402](https://github.com/KirkDiggler/rpg-dnd5e-web/issues/1402), design [Project #915](https://github.com/KirkDiggler/rpg-project/pull/915).

## Shipped

- Native `scene.wallFinishes` lowers to detached typed face choices on structural walls, persists in the immutable SiteData field, and follows the existing wall snapshot/reveal projection; consumer integration is still pending — [slice](https://github.com/KirkDiggler/rpg-project/issues/919).

## Decided

- Capture positive-z/negative-z plus opaque material ref only. Working wall-local mapping uses existing wall geometry; no asset lookup, regional chart inference, gameplay material, new blocking or door-state owner enters the module — [R9 approval](https://github.com/KirkDiggler/rpg-project/pull/915#issuecomment-6758627344).
- Preserve unknown editor-only YAML keys while validating consumed face values and rejecting orphan/duplicate wall targets or duplicate known face keys. Runtime/data lists require unique known sides and nonblank refs — [checked contract](https://github.com/KirkDiggler/rpg-project/blob/design/1402-studio-appearance/ideas/studio-appearance/plan.md).

## Found

- Construction/site persistence/permitted projection and native lowering regressions were RED before implementation, then GREEN. Cold reload uses a fresh empty site provider and serialized site data; later source edits and caller mutations cannot repaint the captured result — [slice](https://github.com/KirkDiggler/rpg-project/issues/919).
- Removing duplicate-side validation fails the targeted refusal test. Omitting face restoration from SiteData fails the cold-load test at the content-hash check; this is a real load failure, not a warm-cache success — [slice](https://github.com/KirkDiggler/rpg-project/issues/919).
- Full module race/atomic-coverage test run passes all nine packages using the upstream25-minute binary budget; root222.974s, dungeonspec111.080s; golangci-lint reports0issues. Other cached package passes remain identified as cached, not new timing measurements — [slice](https://github.com/KirkDiggler/rpg-project/issues/919).
- Six compiled-content golden files add only `FaceFinishes:null` on32wall rows. Parsed comparisons after removing those new nil fields equal every prior value; no source castle, geometry, actor, region or rule fact changes — [slice](https://github.com/KirkDiggler/rpg-project/issues/919).

- Independent review F1 found that full `Atlas()` omitted face choices although `AtlasFor` preserved them. Added the existing clone helper to that constructor and a RED→GREEN equality/alias-isolation regression for both readers. The full module gate/lint pass after refreshing one golden: eight face facts on four walls now match their authored source, with all other facts unchanged. The accepted player-rendering path is untouched — [F1](https://github.com/KirkDiggler/rpg-toolkit/pull/2356#discussion_r4543844099).

## Mistaken

- Initial projection tests covered only the player reader, so the full authoring reader's omission escaped. The new regression checks both real readers and mutates each result to prove their copies stay isolated — [F1](https://github.com/KirkDiggler/rpg-toolkit/pull/2356#discussion_r4543844099).
- The first full gate caught the six expected-but-not-yet-refreshed golden pictures. Inspected the failure, regenerated through the existing CONTENT_GOLDEN tool, and independently compared all old values before rerunning the full gate; no snapshot assertion was weakened — [slice](https://github.com/KirkDiggler/rpg-project/issues/919).
