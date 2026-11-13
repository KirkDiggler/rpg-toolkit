# Run — 2026-11-08 — behavior inspection through session delivery

processed: 2026-11-12T165555Z

## Shipped

- Adopted the pushed encounter inspection provider and classified its three diagnostic kinds as record-only; API/web adoption and the live inspection feature remain incomplete — [Machine inspection S2](https://github.com/KirkDiggler/rpg-project/issues/900).

## Decided

- Merge the already-reviewed optional checkpoint fence from current main without replacing record delivery or the other session's Redis implementation; diagnostics remain after persist, including stale-save refusal — [upstream closure](https://github.com/KirkDiggler/rpg-toolkit/pull/2322#pullrequestreview-5819815506).
- Leave RecordEvent, save-before-delivery, continuation and locking implementations unchanged, because session carries opaque evidence and owns no machine interpretation — [R5](https://github.com/KirkDiggler/rpg-project/pull/902).
- Prove classification even for an audienced diagnostic entry, so a new kind cannot become an undecoded clock act or public state change — [R5](https://github.com/KirkDiggler/rpg-project/pull/902).

## Found

- Joined checkpoint/inspection tree passes the uncached actual root hook (race418.967s, lint0) and workbench race1.218s; original inspection production/test files are unchanged by the clean merge — [#900](https://github.com/KirkDiggler/rpg-project/issues/900).
- Actual fold/classification RED→GREEN prevents all three diagnostic kinds from becoming narrative; a real compiled two-guard launch/move/reload observes persisted records at publication and exact live-suffix/catch-up equality with four scripted choices — [#900](https://github.com/KirkDiggler/rpg-project/issues/900).
- Full module race passes (root402.971s, workbench1.162s) and lint reports0 issues; the whole continuation suite retains its actual guarded execution and complete-record comparison — [#900](https://github.com/KirkDiggler/rpg-project/issues/900).

## Mistaken

- The first catch-up hook reported a cache hit from a degenerate key: it searched a repository-relative path after changing into the module (0files versus335). The uncached rerun, not that hit, is joined-tree evidence; the existing hook issue tracks the mechanism — [toolkit#1736](https://github.com/KirkDiggler/rpg-toolkit/issues/1736).
- The first baseline regex named a nonexistent launch suite; the existing classification test ran, then the corrected command ran the actual LaunchSuite method too. The wrong name is not behavior evidence — [#900](https://github.com/KirkDiggler/rpg-project/issues/900).
- A new test initially equated initialization history with published verb deltas. Constructor scene-opened predates launch's write scope; comparing the actual committed suffix after a captured cursor fixes the test without inventing publication — [#900](https://github.com/KirkDiggler/rpg-project/issues/900).
- The existing continuation normalizer rewrote event/act IDs but not newly introduced record backlinks in payloads. A literal-target regression now proves those links normalize through the same identity map and a wrong target remains distinguishable; no field or diagnostic record is removed from comparison — [#900](https://github.com/KirkDiggler/rpg-project/issues/900).
