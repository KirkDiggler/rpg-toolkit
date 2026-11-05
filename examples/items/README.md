# Item composition: consumer seam probe

A **test-only exploratory consumer**, not a public item library or production
integration. [Keep equipment S1](https://github.com/KirkDiggler/rpg-project/issues/862)
uses this example to inform [Proposed ADR-0048](https://github.com/KirkDiggler/rpg-toolkit/pull/2272).

The first fixture is a one-charge Second Wind ring, with an authored level and
short-or-long-rest recovery. The user selected this fixture so we can investigate
the seams rather than finish specifying every item rule.

## Probe brief

```text
item configuration + instance state
    ├── effects.Compose
    │     ├── grant_healing output: attaches/detaches an inert local offer
    │     └── recover_pool output: attaches/detaches a targeted rest listener
    ├── mechanics/resources.Resource: remaining/maximum charges
    └── use: validate/pay → healing.Resolve → sourced calculation
```

The real effects composer receives a consumer-owned vocabulary. It does not
learn item rules, the healing formula, or a new resource implementation. The
consumer owns one item-instance budget. The real `healing.Resolve` supplies the
calculation; the probe returns that result and **does not mutate HP**.

The adapters and the local healing offer live in `_test.go`: they are proposed
consumer wiring, not a new canonical action definition. There is no public item
API, inventory, attunement system, class-feature migration or studio UI here.
The existing effects/machine and mind/machine packages remain untouched.

## Checks to exercise

| Boundary | Observable check |
|---|---|
| Authored level → shared healing | Fixed d10 of 6 gives 7/9 for level-1/3 grants; no Fighter levels supplied |
| Effects attachment → availability | Grant appears through `effects.Compose`/`Apply` and disappears on `Remove` |
| Item instance → pool | Spending one of two same-definition rings leaves the other untouched |
| Item source → result | Healing calculation identifies the particular source item |
| Detach / transfer / reload → remaining uses | None silently refills the item |
| Authored recovery → real bus | Matching short/long rest restores; unrelated actor and detached listener do not |
| Same effect, alternate recovery fixture | A long-only test variant differs through configuration, not a ref switch |
| Refused use → cost/roll | No grant or exhausted pool refuses before dice; failed pure calculation restores the fixture's reserved charge |

For this probe, `Attach` supplies the eligible holder explicitly, like an
interaction harness. It is **not a ruling** that a ring must be equipped, that a
resting character owns every carried ring, or that detached items cannot recharge
in production. Transfer is simulated by detaching one holder and attaching the
same item to another. Failed-calculation refund is local probe wiring before any
HP delivery, not a general transaction/rollback guarantee.

## Run

```sh
cd examples/items
go test -count=1 -race ./...
go test -run Example -v ./...
```

Dependencies use real pushed-commit pseudo-versions. No `replace` or `go.work`.
The effects pin includes the [named-input provider cleanup](https://github.com/KirkDiggler/rpg-toolkit/pull/2276):

```go
composed, err := effects.Compose(effects.ComposeInput{
    Recipe:           recipe,
    OutputVocabulary: i.vocabulary(),
    HolderID:         holder,
    SourceRef:        &ref,
})
```

This fixture needs no machine facts/beats or saved effect state, so those fields
use their existing defaults; its mutable pool remains owned by the item.
The module is an example and is not tagged or consumed by shipped code.

## Observed seams and limits

| Responsibility | What this probe actually uses |
|---|---|
| Attach/detach and output validation | Real `effects.Compose`, `Apply`, `Remove`, `KindSpec` |
| Grant and restoration meanings | Consumer-owned `grant_healing` / `recover_pool` vocabulary callbacks |
| Item identity and remaining uses | Consumer instance ID plus real `mechanics/resources.Resource` |
| Healing arithmetic and sourced breakdown | Real `healing.Resolve`; fixed dice only at the randomness seam |
| Actual HP delivery and action economy | **Not integrated**; the fixture returns a calculation |

**Identity is not supplied by the recipe.** Two instances report the identical
provider output ID `example:items:second_wind_ring#present.0`; the consumer supplies
`ring-a` and `ring-b` as separate item sources and owns separate pools. This is a
local-ID boundary, not evidence that the provider promises globally unique IDs
and breaks that promise. The probe does not abuse a content ref as instance state.

**The effects tool does not already contain D&D effect implementations.** Its
consumer supplies the output vocabulary. Here the grant callback creates a local
inert healing offer, and the recovery callback wires a rest listener. These are
candidate adapters, not proof that a reusable grant vocabulary has been designed.

**A real action consumer remains missing.** The local offer is not
`actions.Definition`; this probe neither invents a standalone Healing arm nor
disguises the ring as a spell. Integration with actual availability, payment,
resolution and HP delivery is a separate design question. Class-pool coexistence
is also not exercised against a real character; two-item independence is.

**The item owns state outside the effect bag.** The saved item record contains
only its ID and remaining charges; its authored definition is supplied again.
The current effects `Changed` hook does not report these external pool changes.
A production consumer must define that reporting/persistence seam. One-state
attachment suffices here: no state transition is needed, so this does not prove
that item usage needs a new machine or that existing machines should be unified.

**The assertions detect a real wiring error.** Removing the holder filter from
the recovery callback makes the transfer/reload test fail: the old holder's rest
refills the transferred ring. Restoring that filter returns the suite to green.
This demonstrates the fixture's targeted-rest behavior, not the final equipment
eligibility policy.

The runnable example verifies this output:

```text
source: ring-17; healing calculation: 9
charges after use: 0
charges after transfer: 0
charges after old holder rests: 0
charges after current holder rests: 1
```

A passing probe is not approval to promote this fixture or the effects module.
The journal and ADR carry the reasoning and the next architectural decision.
