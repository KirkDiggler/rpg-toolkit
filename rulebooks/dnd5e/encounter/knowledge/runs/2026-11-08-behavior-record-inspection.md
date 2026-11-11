# Run — 2026-11-08 — canonical behavior inspection records

## Shipped

- Implemented GM-only configuration, decision and work records through the real canonical runtime; API/session/web adoption remains outstanding — [Machine inspection S2](https://github.com/KirkDiggler/rpg-project/issues/900).

## Decided

- Keep configuration-description bookkeeping beside the immutable BehaviorData definition, covering placed and reserve members without a duplicate identity map — [R1](https://github.com/KirkDiggler/rpg-project/pull/902).
- Link a decision to its actual answered record and selected work back to that decision; the existing pick and execution acts are different and must not be presented as one — [R4](https://github.com/KirkDiggler/rpg-project/pull/902).
- Reopen the existing execution act only while appending diagnostic work evidence; do not move the actual body-state/clock operations into a different act — [R5](https://github.com/KirkDiggler/rpg-project/pull/902).
- Leave table arithmetic on its existing record; only a suppressed repeated no-candidate answer carries that verdict inside diagnostics. No row reevaluation is introduced — [R2–R3](https://github.com/KirkDiggler/rpg-project/pull/902).

## Found

- Full nine-package race and lint pass after the bounded fixes; the real movement/pause/reload controls retain one choice through two pauses, original configuration after source edit, and explicit partial versus completed work — [#900](https://github.com/KirkDiggler/rpg-project/issues/900).
- The actual YAML compiler journey emits the exact two independently bound actor configurations once across reload; omitting symbol emission fails expected2/actual0 while the valid control passes — [#900](https://github.com/KirkDiggler/rpg-project/issues/900).
- N10 local garrison comparison: default135.0→133.7ms/op, asleep118.3→133.3ms/op. This is a bounded pair, not a statistically established speed change or API latency. Diagnostic payload additions are103293/45395B per four-cell op and121.2/58 records per op, including first-use symbols amortized over10ops; opening bookkeeping adds2475B. Counts are taken before the benchmark's32-entry final save trim — [performance coordination](https://github.com/KirkDiggler/rpg-project/issues/879).

## Mistaken

- Initial JSONv2 map emission was nondeterministic. A new repeated-symbol assertion failed; deterministic emission closes it without weakening exact full-record replay comparison — [#900](https://github.com/KirkDiggler/rpg-project/issues/900).
- The replay harness decoded every record into a player-command shape, interpreting a diagnostic string on as a boolean sneak toggle. A header-first command dispatch closes the new RED→GREEN test; diagnostics remain in the complete replay comparison — [#900](https://github.com/KirkDiggler/rpg-project/issues/900).
- A retention test assumed one player's filtered sequence was globally gapless. It now preserves the five visible beats and asserts all seven global records survive before the exact trim-to-one boundary — [#900](https://github.com/KirkDiggler/rpg-project/issues/900).
- The first compiler-symbol omission control used the standalone32-entry retention default, so both normal and mutant lacked early symbols. The corrected control uses session launch's actual unbounded retention; only the valid control/mutant comparison is evidence — [#900](https://github.com/KirkDiggler/rpg-project/issues/900).
