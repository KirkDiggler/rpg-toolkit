# Run — 2026-11-09 — inspection catches up to the new test gates

## Shipped

- Joined published main through f61e4379 into the unmerged inspection candidate: reviewed small-keep oracles, CI shards/budget, per-encounter cache hooks and play/record v0.3.0. Machinev0.2.0/behaviorv0.22.0 pins stay intact; this is branch catch-up, not an inspection release — [inspection PR](https://github.com/KirkDiggler/rpg-toolkit/pull/2326).

## Decided

- Preserve merged PR#2339's reviewed fixture rather than reintroduce the separate10k/no-roster variant; PR#2337 is closed superseded, with its audit retained and no branches/worktrees deleted — [supersession](https://github.com/KirkDiggler/rpg-toolkit/pull/2337#issuecomment-6749652701).
- Reuse the published upstream review closures and refresh the joined module's tests; no new whole-wave review or copied CI implementation — [small-keep review](https://github.com/KirkDiggler/rpg-toolkit/pull/2339), [cache fix](https://github.com/KirkDiggler/rpg-toolkit/pull/2320), [record adoption](https://github.com/KirkDiggler/rpg-toolkit/pull/2331).
- Keep the storage-format consequence visible: recordv0.3.0 deliberately refuses older inline-audience blobs. No running preview is rebuilt/reset by this catch-up; a later owned-stack rebuild must account for that published break — [provider contract](https://github.com/KirkDiggler/rpg-toolkit/pull/2331).

## Found

- At the first joined main b6e7441e, the full local nine-package race+atomic-coverage suite passes; encounter root357.398s versus the earlier481.304s run. Door oracle3.47s and placed reach9.10s; unchanged castle observer oracle73.01s. These are local observations, not a statistical latency promise — [candidate and gates](https://github.com/KirkDiggler/rpg-toolkit/pull/2326).
- After incorporating the subsequently published f61e4379, the full local race suite passes all nine packages: root127.416s, keepgen233.820s; plain new-oracle cases3.787s and lint0. This no-coverage workload is not directly comparable to the earlier coverage-enabled run — [new gate definition](https://github.com/KirkDiggler/rpg-toolkit/pull/2338).
- CI now runs heavy/rest/subpackages shards; one regex selects heavy and excludes exactly those tests from rest. The25-minute cap buys time, while smaller fixtures and removal of unused coverage change the actual workload — [shards](https://github.com/KirkDiggler/rpg-toolkit/pull/2338), [budget](https://github.com/KirkDiggler/rpg-toolkit/pull/2336).

## Mistaken

- The first catch-up was not the final published main: cache/record/shard PRs landed during its local checks. A second normal merge includes those published commits; no old-head CI result is substituted for the final branch head — [inspection PR](https://github.com/KirkDiggler/rpg-toolkit/pull/2326).
