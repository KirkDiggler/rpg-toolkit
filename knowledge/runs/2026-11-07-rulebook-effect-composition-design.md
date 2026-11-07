# 2026-11-07 — shared rulebook effects design

Work: [story #703](https://github.com/KirkDiggler/rpg-project/issues/703).

## Shipped

- Design proposal and walkthrough for one shared Raging definition/creator, reusable rulebook kinds, current saved data and explicit native consumer adoption; no product code or implementation plan is claimed — [walkthrough](../../docs/ideas/rulebook-effect-composition/README.md).

## Decided

- KirkDiggler selected the Raging-only vertical cut because both the item and native Barbarian should actually use the shared creator; a class-shaped example alone would leave convergence unfinished — [scope ruling](https://github.com/KirkDiggler/rpg-project/issues/703#issuecomment-6700218813).
- KirkDiggler rejected legacy-save compatibility: existing characters can be recreated, so there is no need for an old-shaped reader, writer or normalization pass. The saved representation can be chosen for the new model — [R2](../../docs/ideas/rulebook-effect-composition/design.md#rulings).
- Propose immutable bindings/origin separately from mutable upkeep, a rulebook protocol adapter rather than a second engine, and small neutral provider queries/validation; these are a coherent technical proposal, not attributed operator rulings — [R3–R7](../../docs/ideas/rulebook-effect-composition/design.md#rulings).

## Found

- Toolkit baseline 3972f948 passes `go test -count=1 ./conditions ./features ./effects` from the D&D root; its old effects package has no tests/importers. Native Rage constructs the handwritten type directly, while the example owns its recipe/vocabulary and captures item provenance — [current feature](../../rulebooks/dnd5e/features/rage.go).
- Machine already exposes State/Bag/ToBlob; composer exposes only JSON persistence and narrow rows. KindSpec.Check sees Params, not the complete output; complete-output validation and observational queries are the proposed consumer-driven additions — [provider](../../mechanics/effects/composed.go).
- Composer onTransition currently swaps outputs without invoking Changed on a nonterminal transition; native first-turn Rage persistence needs an explicit change notification, not merely an updated in-memory bag — [transition hook](../../mechanics/effects/composed.go).
- The native action Frame validates only attack/save roll kinds. Four benefits require domain-honest inspection, not four modifiers presented as applying to every Strength action — [frame](../../rulebooks/dnd5e/contributions/frame.go).
- Web dev 2150256f still requires removed cached damage_bonus data to recognize Raging, while API dev 590a201a pins D&D v0.213.5 without that field. Source and minimal guard-probe evidence are filed separately; no browser reproduction or web fix is claimed — [web#1396](https://github.com/KirkDiggler/rpg-dnd5e-web/issues/1396).

## Mistaken

- The first persistence recommendation carried a legacy-reading obligation the operator did not need. The proposal now omits it, and technical choices are presented as one complete contract rather than stopping for each field — [design](../../docs/ideas/rulebook-effect-composition/design.md).

— cross-team agent, on behalf of KirkDiggler
