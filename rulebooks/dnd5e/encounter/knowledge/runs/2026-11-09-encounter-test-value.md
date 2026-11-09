# Run — 2026-11-09 — encounter test value and static-door fixture

## Shipped

- Prepared a test-only fixture simplification: every generated garrison door still agrees with the whole-floor crossing oracle, without launching the unrelated100-member roster; production behavior and benchmark roster remain unchanged — [scope and evidence](https://github.com/KirkDiggler/rpg-toolkit/issues/2236#issuecomment-6747614792).

## Decided

- Follow the operator's bounded test-value pass rather than reduce test counts blindly or adopt the existing CI-sharding proposal; keep the inspection release parked and this change on its own branch/PR — [approved scope](https://github.com/KirkDiggler/rpg-toolkit/issues/2236#issuecomment-6747006348).
- Extract the existing garrison compilation helper without changing its options/cache key; use the same compiled field with no roster for a static geometry question, retaining the complete oracle and every assertion — [checked plan](https://github.com/KirkDiggler/rpg-toolkit/issues/2236#issuecomment-6747430521).
- Leave observer-oracle, route and cast/fight integration claims intact; document smaller refusal fixtures and facing-test consolidation as follow-ups that must retain their distinct observer-facing proof — [cost/value table](https://github.com/KirkDiggler/rpg-toolkit/issues/2236#issuecomment-6747614792).

## Found

- Later merged PR#2339 supersedes this fixture cut with a reviewed small keep retaining all36doors and explicit10k benchmarks. Catch-up preserves that upstream test and removes this branch's now-unneeded helper; only this audit record remains different from main — [upstream delivery](https://github.com/KirkDiggler/rpg-toolkit/pull/2339).
- On exact inspection candidate93fdaedf, root race+atomic-coverage profiling passed443.027s;10of527top-level tests/groups accounted for290.88s. Expensive fixtures dominate this sample; it does not justify a suite-wide deletion policy — [measurement](https://github.com/KirkDiggler/rpg-toolkit/issues/2236#issuecomment-6747614792).
- An isolated comparison retains identical FieldData/DoorData,36doors and108crossings: setup49.644→1.721s, while the complete oracle remains5.750/5.249s. After extraction on main, setup50.670/1.747s and oracle5.425/5.492s repeat that result; cold focused test22.26s includes compilation — [controlled proof](https://github.com/KirkDiggler/rpg-toolkit/issues/2236#issuecomment-6747614792).
- A virtual production mutation dropping a crossing fails both old and new tests at the same nonempty-differences assertion, naming all36doors; source hash stays unchanged. The gameplay helper still opens its one player and99creatures — [refactor controls](https://github.com/KirkDiggler/rpg-toolkit/issues/2236#issuecomment-6747614792).

## Mistaken

- None observed in the fixture refactor. The test active at CI timeout is not necessarily its largest cost, and a short leaf-test time does not include its asynchronously shared setup; neither is used as a deletion argument — [audit qualifications](https://github.com/KirkDiggler/rpg-toolkit/issues/2236#issuecomment-6747614792).
