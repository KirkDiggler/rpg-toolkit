## Files Reviewed
- PR #924 production: `rulebooks/dnd5e/encounter/{data.go,decider.go,encounter.go,field.go,errors.go}`.
- PR #924 changed-test/workbench manifest and module gate.
- Exact dependency: `tools/spatial` v0.9.0 (`0fbcbcae…`), especially `hex_grid.go`.
- PR #923: `docs/ideas/encounter-transitions/{design.md,plan.md}`.

## Critical
1. **Agree — high, merge-blocking:** map-present literal-nil `Decider` passes `LoadEncounter` then panics in `Pump`.
   - Load stores any present map value, including nil: [data.go#L611-L618](https://github.com/KirkDiggler/rpg-toolkit/blob/41412f57bafb40068c50152d6d0fba9387cec60e/rulebooks/dnd5e/encounter/data.go#L611-L618).
   - Pump sees the key and invokes `decider.Decide`: [encounter.go#L996-L1024](https://github.com/KirkDiggler/rpg-toolkit/blob/41412f57bafb40068c50152d6d0fba9387cec60e/rulebooks/dnd5e/encounter/encounter.go#L996-L1024).
   - Targeted probe passed: `LoadEncounter(..., map[MemberID]Decider{"monster": nil})` returned nil error; `Pump` panicked.
   - Smallest contract fix: make a map-present literal nil equivalent to no reattached decider (`ok && d != nil`), matching Setup/Join’s optional-decider behavior. Document/reject typed-nil implementations separately if that must be supported.

2. **Agree — high, merge-blocking:** #924 cannot honestly claim identity mapping to integer cube wire coordinates.
   - #924 constructs `AxialHexGrid` and claims identity mapping: [encounter.go#L76-L99](https://github.com/KirkDiggler/rpg-toolkit/blob/41412f57bafb40068c50152d6d0fba9387cec60e/rulebooks/dnd5e/encounter/encounter.go#L76-L99), [field.go#L44-L56](https://github.com/KirkDiggler/rpg-toolkit/blob/41412f57bafb40068c50152d6d0fba9387cec60e/rulebooks/dnd5e/encounter/field.go#L44-L56).
   - Exact consumed spatial v0.9.0 accepts fractional Q/R through range-only validity: [hex_grid.go#L328-L345](https://github.com/KirkDiggler/rpg-toolkit/blob/0fbcbcae9ae72a0c9ce22d6583023bc80cc098e9/tools/spatial/hex_grid.go#L328-L345), then truncates them with `int`: [hex_grid.go#L474-L484](https://github.com/KirkDiggler/rpg-toolkit/blob/0fbcbcae9ae72a0c9ce22d6583023bc80cc098e9/tools/spatial/hex_grid.go#L474-L484).
   - Probe: `(0.5,0.5)` is valid, has distance zero from `(0,0)`, and #924 accepts/persists it as a hex member position.
   - Smallest contract fix: enforce integral Q/R for every externally supplied hex position (setup/load members, movement/join, endpoints, occluders/boundaries), or remove the integer-wire/identity claim. The former is required for the stated wire contract.

## Warnings
3. **Agree, severity correction: observable but non-blocking ordering issue.**
   - `refreshSight` ranges `e.members` to build each percept: [encounter.go#L1299-L1331](https://github.com/KirkDiggler/rpg-toolkit/blob/41412f57bafb40068c50152d6d0fba9387cec60e/rulebooks/dnd5e/encounter/encounter.go#L1299-L1331).
   - `Surveil` preserves deduped percept order for `FirstContact`/`Refreshed`; only `Faded` is sorted: [intel.go#L129-L214](https://github.com/KirkDiggler/rpg-toolkit/blob/394181ec03d6d0750b08824b042c8dbaf94f7f9b/play/intel/intel.go#L129-L214).
   - `IntelDeltas` itself is publicly a map: [field.go#L239-L241](https://github.com/KirkDiggler/rpg-toolkit/blob/41412f57bafb40068c50152d6d0fba9387cec60e/rulebooks/dnd5e/encounter/field.go#L239-L241).
   - Probe observed public `Refreshed` orders `c,b` and `b,c`.
   - This does **not** block API stream projection: deltas are keyed/unordered, while sequence-bearing record beats are the streamable ordering surface. Document `IntelDeltas`, `FirstContact`, and `Refreshed` as unordered sets (or sort before `Surveil` if deterministic slice order becomes contractual).

4. **Correction:** #924 does add a limited public own-position projection: `Snapshot{Room, Position, Holdings}` for deciders, and adds `IntentTraverse{Connection}`. [decider.go#L43-L83](https://github.com/KirkDiggler/rpg-toolkit/blob/41412f57bafb40068c50152d6d0fba9387cec60e/rulebooks/dnd5e/encounter/decider.go#L43-L83)
   - It does **not** add a public total per-cell visible/remembered projection, client-facing own-position `View`, absolute room embedding, or route model/owner. `View` remains holdings-only: [encounter.go#L398-L415](https://github.com/KirkDiggler/rpg-toolkit/blob/41412f57bafb40068c50152d6d0fba9387cec60e/rulebooks/dnd5e/encounter/encounter.go#L398-L415).
   - `IntentTraverse` is a one-connection instruction, not route ownership; topology is explicitly construction-time decider state.

5. **Confirmed current:** PR #923 remains open at `9fa14c5…`. Its Task 3 plan still says failed preconditions atomically abort: [plan.md#L46-L53](https://github.com/KirkDiggler/rpg-toolkit/blob/9fa14c5fe47ed7752b94a49087201a3d16ab6f6b/docs/ideas/encounter-transitions/plan.md#L46-L53), while its design says phase-2 execution failures silently skip: [design.md#L79-L86](https://github.com/KirkDiggler/rpg-toolkit/blob/9fa14c5fe47ed7752b94a49087201a3d16ab6f6b/docs/ideas/encounter-transitions/design.md#L79-L86). #924 implements the design’s skip behavior: [encounter.go#L1087-L1099](https://github.com/KirkDiggler/rpg-toolkit/blob/41412f57bafb40068c50152d6d0fba9387cec60e/rulebooks/dnd5e/encounter/encounter.go#L1087-L1099).
   - #924 also currently has two unresolved Copilot threads, one outdated and one current despite fix replies: [thread 1](https://github.com/KirkDiggler/rpg-toolkit/pull/924#discussion_r3760175673), [thread 2](https://github.com/KirkDiggler/rpg-toolkit/pull/924#discussion_r3760175718). This is review hygiene, not a remaining instance of those two reported defects.

## Evidence
- GitHub current PR #924 head is exactly `41412f57bafb40068c50152d6d0fba9387cec60e`; it does not differ.
- Reviewed in a detached temporary worktree at that SHA; temporary probes were deleted and the temporary root removed.
- `go test -run '^TestReviewerProbe' -count=1 -v .` — passed nil-decider and fractional-axial reproductions.
- `go test -run '^TestReviewerProbeIntelDeltaSubjectOrderIsObservable$' -count=1 -v .` — passed; logged `c,b / b,c`.
- `go test ./...` in `rulebooks/dnd5e/encounter` — passed.
- `git diff --check main...HEAD` and gofmt diff check — clean.
- Persistent checkout ended unchanged: pre-existing `?? .pi-subagents/`; no staged or tracked diff.

## Verdict
**findings-before-merge.** Do not merge #924 until the nil-decider panic and fractional axial/cube contract are fixed. Resolve/document the ordering and #923 plan discrepancy before ratifying associated docs.