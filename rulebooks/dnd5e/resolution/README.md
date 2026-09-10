# resolution

The one place an interaction event bus exists.

A `Resolve` call validates data, loads the world, attaches every participant,
purely preflights the machine, pays any cost, drives sealed steps, tears down,
and returns data. Nothing runtime survives the call.

See the [D&D 5e layer overview](../overview.md) for how resolution composes with
`encounter`, `session`, rulebook content, and the reusable `play/*` leaves.

## Actions are shared data

Producers hand resolution a `combat/actions.Definition`:

```go
machine, err := resolution.NewAction(&resolution.ActionInput{
    Definition: definition,
    AttackerID: "wolf",
    TargetID:   "hero",
    Roller:     roller,
})
```

`NewAction` validates the definition and dispatches by populated profile arm.
Content identity is attribution, never routing. An unknown monster/weapon ref
with a valid Attack profile still resolves through Strike. Attack profiles use
`TargetID`; cast profiles use the canonical ordered `TargetIDs` list and reject
the singular field.

Character definitions come from `character.AssembleAttack`; monster factories
persist the same definitions directly. Resolution owns no producer compiler,
action config decoder, or `AttackProfile` type.

## One call

```go
out, err := resolution.Resolve(ctx, &resolution.Input{
    World: worldData,
    Participants: []resolution.Participant{
        {Character: heroData},
        {Monster: skeletonData},
    },
    Machine: machine,
    Cost: cost,
    Initiative: initiative,
    Standing: standing,
    Sight: sight,
    TurnDriver: turns,
    Roller: loaderRoller,
})
```

Order:

1. validate input;
2. load the active encounter world and take its room;
3. attach all sheets/effects on one instrumented bus;
4. install the game context through the one door, `installTruth` — the room,
   the cast, and the reaction readiness derived from that cast;
5. call `Machine.Start` as pure preflight;
6. pay the declared cost only after preflight succeeds;
7. drive `Gather | Request | Done` steps;
8. tear down newest-first;
9. return world, dirty sheets, outcome, and hook ledger.

`Start` may validate and read attached sheets. It may not roll, spend, publish,
or mutate. Invalid definitions, participants, delivery range, or condition
construction therefore consume nothing.

## Cast

A cast validates the complete ordered target list before the resolution door
pays. Cardinality, empty/duplicate IDs, participant eligibility, range, save
construction, and condition construction are all preflight-only. After one
payment, a concentration recast removes only the caster's qualified old owner
and children, resolves each target in caller order, and installs one qualified
new owner even when every target saves and it owns zero children.

Each save asks its recipient's current condition owner for contributions when
that save executes. Descriptions are never cached during whole-cast preflight,
so a recast cannot apply a contribution from the concentration it just ended.

## Strike

Strike interprets `Definition.Attack`:

- melee reach and ranged normal/long range are enforced against the installed
  room, converting authored feet at the comparison boundary;
- long-range attacks seed attributable disadvantage;
- attack/damage events carry the actual delivery kind;
- ordered typed damage pools fold once and apply once;
- a condition-only attack skips damage and still processes on-hit declarations;
- `ConditionApplication` behavior is prepared during Start, before payment;
- automatic and save-gated conditions execute in declaration order and produce
  ordered `StrikeOutcome.Conditions`;
- attack, ordinary save, concentration save, and death-save results carry a
  strict sourced calculation; selected contribution dice retain positive faces
  and a separate add/subtract operator;
- post-roll Inspiration freezes strike version 2 with that settled calculation.
  Keep reuses it unchanged; spend appends only the offered die component.

A save-gated condition requests Contest. The declaration names the condition,
parameters, and `SaveGate`; the condition implementation owns executable
behavior and lifecycle.

## Contracts

- [ADR-0038](../../../docs/adr/0038-resolution-owns-the-bus.md): one bus and
  sealed machine steps.
- [ADR-0039](../../../docs/adr/0039-the-save-gate.md): contestability is data.
- [ADR-0041](../../../docs/adr/0041-composable-attack-damage.md): ordered typed
  damage pools.
- ADR-0045 is tracked in PR #1214 until the implementation record lands.

See [doc.go](doc.go) for laws R1–R7 and [ARCHITECTURE.md](ARCHITECTURE.md) for
the cross-cutting map.
