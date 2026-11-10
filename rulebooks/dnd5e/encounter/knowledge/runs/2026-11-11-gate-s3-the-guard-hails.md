# Run — 2026-11-11 — gate conversation S3 the guard hails

Slice [rpg-project#923](https://github.com/KirkDiggler/rpg-project/issues/923) of story [rpg-project#769](https://github.com/KirkDiggler/rpg-project/issues/769). Base: `rpg-toolkit` `main` at [`ab61a7e5`](https://github.com/KirkDiggler/rpg-toolkit/commit/ab61a7e5).

## Shipped

- `interactions.<id>.approach: { within: N }` compiles onto `InteractionInput.Approach`. N < 1 and unknown keys are refused at compile — [https://github.com/KirkDiggler/rpg-toolkit/pull/2370](https://github.com/KirkDiggler/rpg-toolkit/pull/2370).
- The address `{member, guard}` is on the interaction instance, persisted on `InteractionData.Address` and validated on Load — [https://github.com/KirkDiggler/rpg-toolkit/pull/2370](https://github.com/KirkDiggler/rpg-toolkit/pull/2370).
- The step trigger, the `addressed` / `address-ended` beats, the `approached` wake, `StepOutput.Addressed`, `AddressFor` and `ValidateApproachReach` — [e7ca9e60](https://github.com/KirkDiggler/rpg-toolkit/commit/e7ca9e60).
- Castle `front-gate` approach 2, hail lines for gate-1, gate-2 and the captain, the castle route test, and the hail one-off site — [67fbd38e](https://github.com/KirkDiggler/rpg-toolkit/commit/67fbd38e).
- The `answered` beat carries `to`, the member a reaction's line is spoken to: the mover for a `barred` or `approached` row that says something, and empty otherwise. This follows the operator's early look ("nothing says WHO he is speaking to") — [https://github.com/KirkDiggler/rpg-toolkit/pull/2370](https://github.com/KirkDiggler/rpg-toolkit/pull/2370).

## Decided

- The encounter reports the address on the step and the session drops the walk's remaining legs, because `Encounter.Step` is one cell and the walk loop is the session's — [https://github.com/KirkDiggler/rpg-toolkit/pull/2370](https://github.com/KirkDiggler/rpg-toolkit/pull/2370).
- Reach is an input (`StepInput.Reach`, `AddressForInput.Reach`), floored at the approach distance, because the module owns no Speak reach (`SpeakInput.Range`) and an address must never end on a cell its own trigger admits — [https://github.com/KirkDiggler/rpg-toolkit/pull/2370](https://github.com/KirkDiggler/rpg-toolkit/pull/2370).
- Ending is a sibling beat `address-ended` with a reason, not `addressed` with an `ended` field, so each beat names its own kind — [https://github.com/KirkDiggler/rpg-toolkit/pull/2370](https://github.com/KirkDiggler/rpg-toolkit/pull/2370).
- Only the reported player step triggers and sweeps. A driven NPC walk does neither, and the read applies the predicate at once — [https://github.com/KirkDiggler/rpg-toolkit/pull/2370](https://github.com/KirkDiggler/rpg-toolkit/pull/2370).
- Carriers are members whose lines name the interaction in `requires` or `advance`, per the ruling "one place sounds way better" — [ruling](https://github.com/KirkDiggler/rpg-project/issues/923).
- The hail needs the guard to see the mover, using the same witness answer Speak uses, so a won sneak is not undone (the lead's assumption on the issue, not vetoed) — [https://github.com/KirkDiggler/rpg-project/issues/923](https://github.com/KirkDiggler/rpg-project/issues/923).
- `TestMadKingThePostSaysWhyItStays` paces at the party start rather than the gatehouse, because the gatehouse hail is now a voice that draws gate-1 off the crossing — [67fbd38e](https://github.com/KirkDiggler/rpg-toolkit/commit/67fbd38e).
- `to` is a new field rather than `target`, because `target` is the intent's target. It is written on every `answered` beat (empty when nobody), following that beat's every-field-written rule — [https://github.com/KirkDiggler/rpg-toolkit/pull/2370](https://github.com/KirkDiggler/rpg-toolkit/pull/2370).

## Found

- In the castle, walking in from the start, gate-2 hails first. Her hail is a voice that gate-1's `posted → alert` on `heard` within 6 answers, and he steps toward it off the crossing — [https://github.com/KirkDiggler/rpg-toolkit/pull/2370](https://github.com/KirkDiggler/rpg-toolkit/pull/2370).
- The address follows the patrol: gate-2 walks away, her address ends `out-of-reach`, and another gate speaker can hail on the hero's next step — [https://github.com/KirkDiggler/rpg-toolkit/pull/2370](https://github.com/KirkDiggler/rpg-toolkit/pull/2370).
- The content golden (`*.compiled.json`) carries no interactions, so an interaction's approach shows in no committed picture — [https://github.com/KirkDiggler/rpg-toolkit/pull/2370](https://github.com/KirkDiggler/rpg-toolkit/pull/2370).
- The local golangci-lint was v2.3.1 (built with go1.24) and cannot load this go1.27 module. `scripts/install-golangci-lint.sh` installs the pinned v2.15.0 — [https://github.com/KirkDiggler/rpg-toolkit/pull/2370](https://github.com/KirkDiggler/rpg-toolkit/pull/2370).
- The hailed or bumping player also gets the anonymous `heard {kind: voice}` beat for the same line. A reaction's `say` goes through `answerTime` to `makeSound(noiseSay)`, and `hearersOf` excludes only the maker: every standing member whose passive Perception beats the voice DC plus hops hears it, and players get a `heard` beat, with no sight test. The `answered` beat's own audience is `audienceFor(subjectBeat, guard)`, which is everyone in v1 — [https://github.com/KirkDiggler/rpg-toolkit/pull/2370](https://github.com/KirkDiggler/rpg-toolkit/pull/2370).

## Mistaken

- Three of the first mutation probes left a variable unused, so the mutants did not compile and looked like survivors. Rerun so that each still compiled, every one fails a named test — [https://github.com/KirkDiggler/rpg-toolkit/pull/2370](https://github.com/KirkDiggler/rpg-toolkit/pull/2370).
