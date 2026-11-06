# Item composition experiment

An **experimental package under `examples/`**, not an extracted production API.
[Keep equipment S1](https://github.com/KirkDiggler/rpg-project/issues/862) uses it to
inform [ADR-0048's accepted composition/ownership decision](https://github.com/KirkDiggler/rpg-toolkit/pull/2272).
The first cut was test-only wiring; this cut gives that wiring an item-owned
boundary that a separate consumer can use and challenge.

The [Rage-granting loot example](RAGE.md) adds an ongoing effect and built items
stored/retrieved outside encounters. It uses the same item API and real effects
composition; items are the first consumer driving the planned effects extraction,
with production condition migration following later.

The existing top-level [`items` module](../../items/item.go) declares property and
equipment interfaces plus validation, not this grant/pool composer. This example
does not replace that module or claim to implement its full equipment interfaces;
extraction ownership remains a question to resolve with actual consumers.

## The shape

```text
Definition: grants + named pools + recovery policy
Config: definition + instance identity + optional saved Data + consumer validation
                          │
                     items.New
                          │
          Item.Attach on the caller's existing bus
                          │
        effects.New: one attached state, two output kinds
                ├── grant availability
                └── targeted pool recovery
                          │
          Offers: copied declarations + source + item affordability
                          │
          items.Use[Result]: reserve item charge
                → consumer resolves a typed value
                → return Result and settled budget facts
                          │
                caller delivers the result once
```

The item package contains **no D&D imports, dice or HP mutation**. The test
consumer supplies Second Wind's meaning through `ValidateGrant` and the pure
resolver in [`fixture_test.go`](fixture_test.go), using real `healing.Resolve`.
The item owns identity, pools, attachment, selection and accounting—not the rule.
The fixture's grant ref is `example:abilities:second_wind`: its parameter contract
belongs to this prototype consumer, not to the production feature persistence loader.

## Candidate API

- **`New(Config)`** creates or strictly restores an item. `Definition` is plain
  authored data; Config adds runtime validation and the supported recovery-trigger
  vocabulary. Nil State explicitly creates a full-budget item; non-nil State must
  match identity, definition and every declared pool, with in-range counts.
- **`Item.Attach(ctx, AttachInput)` / `Detach(ctx)`** use the real effects composer
  to attach/remove grant and recovery outputs. The caller supplies an eligible
  holder and owns the bus. Detaching never refills the item.
- **`Item.Offers(OffersInput)`** returns stable-order, copied grant declarations
  with item-instance identity, definition ref, holder, remaining uses and
  affordability. **Affordability is item-budget only**, not actor action economy
  or all rulebook eligibility.
- **`Use[Result](ctx, UseInput[Result])`** accepts a grant key, not caller-supplied
  price or parameters. It reserves the item's authored cost, invokes the consumer
  resolver, then returns its typed result and budget facts. A single generic
  operation keeps rulebook result types out of Item without `any` results or a
  JSON round-trip. Item and Config themselves are not generic.
- **`Item.Data()`** returns a detached snapshot of identity, definition ref and
  remaining pool counts. No projected offer, bus, attachment or runtime closure
  is saved. The generated effects recipe has one state and no mutable bag.

The consumer call looks like this (complete working code is in the fixture):

```go
out, err := items.Use(ctx, items.UseInput[*dndevents.RollCalculation]{
    Item:     ring,
    HolderID: holderID,
    GrantID:  "second_wind",
    Resolve:  resolve,
})
```

`resolve` receives an `items.Offer` with the validated rule ref/parameters and
source identity. It returns a calculation; it does not deliver healing or charge
the item again. The result remains typed all the way back to the consumer.

## Instance identity and ownership

Recipe-local `effects.Bound.ID` is not used as an item-instance ID. `Offer.Source`
separates ItemID, ItemRef and HolderID. Two same-definition rings own independent
resource objects; explicit grant pool references can also express sharing within
one item without creating a second mutation owner. That representation does not
select which future authored items should share budgets.

Definitions and returned parameter buffers are copied. Editing an offered grant
or saved pool map cannot alter the running item. Source Definition, live Item,
saved Data and projected Offer have different responsibilities; only Item mutates
its pools. A consumer may snapshot Data after use or recovery; there is no new
automatic persistence or Changed-event protocol in this experiment.

## What the example proves

The first fixture is a one-charge ring with authored level and short-or-long-rest
recovery. The consumer gives no Fighter class level. With a controlled d10 of 6,
level-1/3 grants return healing calculations of 7/9. The runnable example verifies:

```text
source: ring-17; healing calculation: 9
charges after use: 0
charges after transfer: 0
charges after old holder rests: 0
charges after current holder rests: 1
```

The suite exercises authored-definition JSON round-trip, independent items,
explicit pool references, copied views, strict restoration, cancellation,
invalid declarations, exhausted/wrong-holder
refusal before dice, and refund of a failed pure calculation. A long-only fixture
proves recovery is configured rather than selected by ability ref; it does not
change the first ring's chosen normal recovery. An import-boundary test keeps
rulebooks and dice outside the non-test package.

Three targeted mutants are caught: remove the recovery holder filter (the wrong
holder refills the ring), spend zero instead of the declared cost (successful use
leaves a charge), or expose the internal parameter buffer (editing an offer
corrupts later resolution). Restoring each returns the suite to green.

## Boundaries still to learn

- This is **not `actions.Definition` integration**. No production action-economy
  payment, native acquisition, resolution-machine or HP-delivery path is claimed.
  A production adapter must not charge the item both here and elsewhere.
- The resolver contract is deliberately narrow: validate/compute a typed outcome
  without world delivery, reentrant item mutation or publication of the result.
  Only then is a failed local reservation safely refunded. Arbitrary effects,
  interruption/reaction workflows and distributed rollback are not proved.
- The first package generates grant/recovery outputs; arbitrary passive or
  stateful item-effect recipes are **not** accepted by this API yet. A contrasting
  consumer must establish that shape instead of the package claiming universality.
- Attachment explicitly supplies the holder. This does not settle worn/carried/
  attuned eligibility or whether detached items recharge in the game.
- Class-pool coexistence is not exercised against a real character. Two-item
  independence and a no-class-level consumer are narrower evidence.
- The generated recipe needs no state transitions. This is not an argument for
  another machine or extraction of a shared machine.

## Run

```sh
cd examples/items
go test -count=1 -race ./...
go test -count=1 -run Example -v ./...
go vet ./...
golangci-lint run ./...
```

The effects cleanup [#2276](https://github.com/KirkDiggler/rpg-toolkit/pull/2276)
is merged; the consumer pins its canonical main commit through a real
pseudo-version. Examples are not tagged. No `replace` or `go.work`, no provider
changes in this package cut, and no promotion implied by passing tests.

[Experiment plan](PLAN.md). The journal and ADR record what the consumer teaches;
these names and interfaces remain free to change before extraction.
