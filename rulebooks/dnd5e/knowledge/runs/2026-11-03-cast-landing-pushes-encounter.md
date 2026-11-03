# 2026-11-03 · A cast's shoves land inside its landing · Encounter

Bug [rpg-project#831](https://github.com/KirkDiggler/rpg-project/issues/831), law [rpg-project#785](https://github.com/KirkDiggler/rpg-project/issues/785) R3, seen on the S1 walk of [rpg-project#825](https://github.com/KirkDiggler/rpg-project/issues/825). Encounter PR [rpg-toolkit#2248](https://github.com/KirkDiggler/rpg-toolkit/pull/2248), with session [rpg-toolkit#2247](https://github.com/KirkDiggler/rpg-toolkit/pull/2247).

## Shipped

- `RecordCastInput.Pushes` at [`cast.go:283`](https://github.com/KirkDiggler/rpg-toolkit/blob/3a325117/rulebooks/dnd5e/encounter/cast.go#L283) carries the cast's routed shoves, one [`CastPush`](https://github.com/KirkDiggler/rpg-toolkit/blob/3a325117/rulebooks/dnd5e/encounter/cast.go#L289) each: a mover and its route.
- `RecordCast` walks them at [`cast.go:440`](https://github.com/KirkDiggler/rpg-toolkit/blob/3a325117/rulebooks/dnd5e/encounter/cast.go#L440), after the cast's own beats and before `landAttack`. On every clock the record reads cast, saves and damage, moved, stance, bubble-formed, first turns, world round.
- [`preparePushes`](https://github.com/KirkDiggler/rpg-toolkit/blob/3a325117/rulebooks/dnd5e/encounter/cast.go#L497) refuses a push before the first append when it names a creature the cast did not target, when the spell ref does not parse, or when a walk is held.
- [`walkCastPushes`](https://github.com/KirkDiggler/rpg-toolkit/blob/3a325117/rulebooks/dnd5e/encounter/cast.go#L538) walks through `Direct` with `Provokes: false` written out. It refuses a pause rather than carry it into the deed.
- The tests are [`TestACastsShovesLandInsideItsLandingOnTheWorldClock`](https://github.com/KirkDiggler/rpg-toolkit/blob/3a325117/rulebooks/dnd5e/encounter/cast_landing_test.go#L146) and [`TestAPushOnSomebodyTheCastDidNotReachIsRefusedBeforeAnyBeat`](https://github.com/KirkDiggler/rpg-toolkit/blob/3a325117/rulebooks/dnd5e/encounter/cast_landing_test.go#L207). Walking the pushes after the deed fails three assertions.

## Decided

- A push is part of the deed landing (#785 R3), so the encounter walks it inside `RecordCast`. The session no longer walks it after the record returns.
- Only shoves come in. A flee, which can stop to ask a player about an opportunity attack, stays with the caller after the record. A held walk cannot sit inside a landing this verb finishes before it returns.
- The walk uses `context.Background()`, as clocks.go does. No verb on this composition takes a caller context.

## Found

- Inside a fight, a cast's moved beats now come before its own heard beats, its down beats and the session's retaliation train. A downed target now slides, then falls on its landing cell. No test or golden changed, and the in-fight Thunderwave scene reads the same beat for beat.
- The flee half is still open: a Dissonant Whispers or Command flee from free roam still lands after the fight its cast formed. It is [rpg-project#832](https://github.com/KirkDiggler/rpg-project/issues/832).
- A merge with [rpg-toolkit#2245](https://github.com/KirkDiggler/rpg-toolkit/pull/2245) breaks the build at `cast.go:547` and only there. Resolve it as `Kind: MoveDisplaced`. Deleting the field trips #2245's `ErrBadMoveKind` at runtime, which fails closed.

## Mistaken

- The first shape of the fix, the session-only first commit of #2247, walked the shoves before `RecordCast`. That put the moved beats ahead of the cast beat. The record is the replay, so that told the wrong story, and the fix moved here.
- The first head of this PR shipped without this record, and with an unformatted test file, so CI failed its dirty-tree check.

— cross-team agent, on behalf of KirkDiggler
