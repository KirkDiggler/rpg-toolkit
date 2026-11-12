# Run — 2026-11-08 — canonical behavior with continued movement

processed: 2026-11-12T165555Z

Part of [#866](https://github.com/KirkDiggler/rpg-project/issues/866), integrating [optimization #879](https://github.com/KirkDiggler/rpg-project/issues/879).

## Shipped

- Prepared the canonical launch/delegation seam on continuation-aware main, including verb-scoped monster participation memoization, using the real pushed encounter integration commit — [session PR](https://github.com/KirkDiggler/rpg-toolkit/pull/2296).

## Decided

- Preserve the upstream early-answer contract, session guard ownership, cancel-detached continuation, save-before-delivery and once-only completion; canonical behavior progress remains encounter-owned inside that same write scope — [continuations](https://github.com/KirkDiggler/rpg-toolkit/pull/2314).
- Keep S7's whole-act delta folding and optional subscriber-scoped live recipients, because both live delivery and catch-up still derive from each member's own retained story; private role progress gets no new public projection — [act folding](https://github.com/KirkDiggler/rpg-toolkit/pull/2319).
- Keep monster participation facts verb-scoped and invalidated by the existing sheet-write door; behavior selection does not become another source of compulsion or standing — [participation memo](https://github.com/KirkDiggler/rpg-toolkit/pull/2316).
- Rebuild the continuation fixture's explicit hold configuration instead of reinstating its retired actor Table input; whole/continued record ordering and lock/cancellation assertions remain intact — [R9](https://github.com/KirkDiggler/rpg-project/pull/877#issuecomment-6698028412).

## Found

- Canonical configuration/compulsion proofs and the continuation suite pass together, including whole-versus-continued delivery equality, request cancellation, once-only continuation and guard release controls — [#866](https://github.com/KirkDiggler/rpg-project/issues/866).

## Mistaken

- The incoming continuation fixture still assigned a legacy Table and failed launch explicitly; it now supplies canonical native behavior configuration through the same fixture vocabulary as the other session tests — [#866](https://github.com/KirkDiggler/rpg-project/issues/866).
