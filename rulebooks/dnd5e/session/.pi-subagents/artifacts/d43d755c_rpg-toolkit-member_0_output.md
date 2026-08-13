## Verdict: **do not merge #924 as-is**

Reviewed exact GitHub PR range: base `a57f28b79451e2f119d550a35b4a1d2b62f1cff6` → head `41412f57bafb40068c50152d6d0fba9387cec60e` (13 commits; 11 files; +3202/-213), from a clean archive of that head.

### Blockers / high findings

1. **BLOCKER — nil decider reattachment panics on the first Pump.**  
   `LoadEncounter` inserts a map-present `nil` `Decider` into `e.deciders` without validation ([data.go:611–618](https://github.com/KirkDiggler/rpg-toolkit/blob/41412f57bafb40068c50152d6d0fba9387cec60e/rulebooks/dnd5e/encounter/data.go#L611-L618)). `Pump` treats map presence as a decider and calls `decider.Decide` ([encounter.go:996–1024](https://github.com/KirkDiggler/rpg-toolkit/blob/41412f57bafb40068c50152d6d0fba9387cec60e/rulebooks/dnd5e/encounter/encounter.go#L996-L1024)). Thus `LoadEncounter(data, map[MemberID]Decider{monsterID:nil})` succeeds, then panics.  
   This directly affects #793’s provider-at-load seam. Treat nil as absent/hold or reject it; add the regression test.

2. **HIGH — “identity cube mapping” accepts non-integral positions, then truncates them.**  
   Encounter selects `AxialHexGrid` ([encounter.go:82–98](https://github.com/KirkDiggler/rpg-toolkit/blob/41412f57bafb40068c50152d6d0fba9387cec60e/rulebooks/dnd5e/encounter/encounter.go#L82-L98)). Its published v0.9.0 `IsValidPosition` checks only bounds, so `(-0.5, 0)` is accepted ([hex_grid.go:332–335](https://github.com/KirkDiggler/rpg-toolkit/blob/tools/spatial/v0.9.0/tools/spatial/hex_grid.go#L332-L335)), while cube conversion truncates Q/R via `int` ([hex_grid.go:474–479](https://github.com/KirkDiggler/rpg-toolkit/blob/tools/spatial/v0.9.0/tools/spatial/hex_grid.go#L474-L479)).  
   This is not a total axial/cube-cell model and cannot be an identity projection to integer wire cube coordinates. Enforce integral Q/R at the spatial/hex ingress and cover Setup, Load, Move, and endpoints.

3. **HIGH — returned intel deltas are nondeterministically ordered.**  
   `refreshSight` constructs percepts by ranging `e.members` ([encounter.go:1299–1330](https://github.com/KirkDiggler/rpg-toolkit/blob/41412f57bafb40068c50152d6d0fba9387cec60e/rulebooks/dnd5e/encounter/encounter.go#L1299-L1330)). `intel.Surveil` preserves that input order in public `FirstContact`/`Refreshed` slices ([intel.go:182–211](https://github.com/KirkDiggler/rpg-toolkit/blob/41412f57bafb40068c50152d6d0fba9387cec60e/play/intel/intel.go#L182-L211)), which are returned through Move/Traverse/Pump `IntelDeltas`.  
   State/JSON may remain deterministic, but the observable mutation outputs are not. Sort visible subjects before Surveil or make the delta contract explicitly unordered; API stream projection needs the former.

### What is solid

- Connection endpoints are validated at Setup and Load; missing persisted endpoints reject rather than silently becoming `(0,0)` ([data.go:404–433](https://github.com/KirkDiggler/rpg-toolkit/blob/41412f57bafb40068c50152d6d0fba9387cec60e/rulebooks/dnd5e/encounter/data.go#L404-L433)).
- Grid persistence now uses stable strings, not `GridShape` iotas ([data.go:54–67](https://github.com/KirkDiggler/rpg-toolkit/blob/41412f57bafb40068c50152d6d0fba9387cec60e/rulebooks/dnd5e/encounter/data.go#L54-L67)).
- Connections and members serialize deterministically by ID ([data.go:120–145](https://github.com/KirkDiggler/rpg-toolkit/blob/41412f57bafb40068c50152d6d0fba9387cec60e/rulebooks/dnd5e/encounter/data.go#L120-L145)); connections are sorted on both construction/load.
- Traverse correctly uses Spatial’s managed `TransitionEntity` then `PlaceEntity`, and only updates `member.Room` after placement ([encounter.go:751–787](https://github.com/KirkDiggler/rpg-toolkit/blob/41412f57bafb40068c50152d6d0fba9387cec60e/rulebooks/dnd5e/encounter/encounter.go#L751-L787)).
- Pump’s former copied-member aliasing flaw is correctly repaired by live member lookup ([encounter.go:1061–1075](https://github.com/KirkDiggler/rpg-toolkit/blob/41412f57bafb40068c50152d6d0fba9387cec60e/rulebooks/dnd5e/encounter/encounter.go#L1061-L1075)).
- `View` is a pure holdings read; it does not Pump ([encounter.go:398–414](https://github.com/KirkDiggler/rpg-toolkit/blob/41412f57bafb40068c50152d6d0fba9387cec60e/rulebooks/dnd5e/encounter/encounter.go#L398-L414)).

### #793 blocker matrix

| #793 need | #924 status | Evidence / consequence |
|---|---|---|
| Multi-room topology and crossing | **Closed (narrowly)** | Endpoint topology, Traverse, and IntentTraverse are toolkit-owned. |
| Hex support | **Partial** | Axial shape is real, but fractional accepted/truncated positions invalidate a total cube mapping. |
| Per-cell VISIBLE/REMEMBERED View | **Open** | `View` returns only `[]intel.Holding`; it has no cells, room geometry, own position, or total visible/remembered projection. `Member` exposes room but no position ([field.go:181–186](https://github.com/KirkDiggler/rpg-toolkit/blob/41412f57bafb40068c50152d6d0fba9387cec60e/rulebooks/dnd5e/encounter/field.go#L181-L186)). |
| Stable local↔absolute room mapping | **Partial** | Q/R identity is documented/persisted for authored hex rooms, but no absolute embedding/mapping contract or public projection exists. |
| Traverse/path intent adaptation | **Partial** | Step-level Traverse exists; no target/route verb owns absolute-to-local multi-room decomposition. |
| Decider contribution/re-registration | **Partial** | Non-persistence and reattachment seam are correct, but keyed-nil providers panic. API still must own a provider registry. |
| Event/delta seam | **Partial** | Outputs expose deltas and Story sequences, but deltas lack total cell state and subject ordering is unstable. |
| Reads must not Pump | **Preserved** | View remains a pure read. |

### Release / CI / review

- **`v0.2.0` is the right release class**: the `Decider.Decide` signature is breaking, and pre-v1 minor is the honest Go module version. CI’s `gorelease-dnd5e-encounter` check passed.
- The compatibility narrative is incomplete: adding exported fields to `RoomInput` and `ConnectionInput` also breaks external **unkeyed** composite literals ([field.go:27–57](https://github.com/KirkDiggler/rpg-toolkit/blob/41412f57bafb40068c50152d6d0fba9387cec60e/rulebooks/dnd5e/encounter/field.go#L27-L57), [71–86](https://github.com/KirkDiggler/rpg-toolkit/blob/41412f57bafb40068c50152d6d0fba9387cec60e/rulebooks/dnd5e/encounter/field.go#L71-L86)). This does not change the v0.2.0 conclusion, but “one incompatible change” is not fully honest.
- CI is green selectively: changed-module test, ci-status, and Gorelease succeeded; many full/lint jobs were intentionally skipped. Clean-head `go test -race ./...` for the encounter module passed.
- Copilot’s two substantive #924 comments were fixed in `095e6f5`, but both GitHub review threads remain marked unresolved. No approval review decision is recorded.
- #923’s canonical triplet has a stale Task-3 statement saying phase-2 failure atomically aborts ([plan.md:46–53](https://github.com/KirkDiggler/rpg-toolkit/blob/9fa14c5fe47ed7752b94a49087201a3d16ab6f6b/docs/ideas/encounter-transitions/plan.md#L46-L53)); the design and implementation correctly say execution failures skip ([design.md:79–86](https://github.com/KirkDiggler/rpg-toolkit/blob/9fa14c5fe47ed7752b94a49087201a3d16ab6f6b/docs/ideas/encounter-transitions/design.md#L79-L86)). Fix before calling it canonical ratification.

### Thin honest API seam after repairs

API may: load/save the opaque aggregate once, reattach non-nil decider providers, authenticate, call toolkit verbs/Pump on activity, and project toolkit-owned view/change batches.

It cannot honestly yet project the required existing wire knowledge model without inventing semantics: toolkit still needs a total per-cell observer projection plus stable change batch. It also needs a Kirk decision on absolute-route ownership: #793 says API keeps stitching/pathing, but that conflicts with the Boundary Rule if API must choose room transitions/path behavior. #924 provides transition execution, not the route-intent owner.