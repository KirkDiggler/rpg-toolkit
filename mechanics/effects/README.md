# mechanics/effects

Generic composition of configured effects. This module contains the composer
and its `machine` package; neither imports a rulebook. Consumers supply the
vocabulary and rules. The item examples are the first extraction consumers;
production condition migration is separate work.

The current API replaces the retired implementation formerly published under
this module path. Existing v0.2.1-and-earlier pins retain their own code; this is
not a compatibility wrapper or a fresh v0.1.0. CI determines the next release
after merge. No release tag is implied merely by this source tree.

- Design: [`docs/ideas/composable-conditions/design.md`](https://github.com/KirkDiggler/rpg-toolkit/blob/docs/idea-composable-conditions/docs/ideas/composable-conditions/design.md) on the idea branch
- Story: [rpg-project#703](https://github.com/KirkDiggler/rpg-project/issues/703)
- Extraction and current ordering: [#705](https://github.com/KirkDiggler/rpg-project/issues/705)
- Layer walkthrough: [item/effect composition](../../docs/ideas/item-effect-composition/README.md)
- Godoc: `go doc ./machine` and `go doc .` walk every piece in the order you
  meet it.

## Runtime and composition

**`machine` is a state machine over a bag that fires configured events on
transition.** A recipe names states, the transitions between them, and the
trackers that feed a bag. Each transition listens to a beat, such as a turn
end. It asks a guard, writes the bag, and may move to another state. Every
move fires `Transitioned{From, To, Reason}`. The bag is also the condition's
saved blob, flat and with today's keys. The machine knows no chain, no output
and no rulebook.

**`effects` is the composer.** It runs a machine from a recipe. While the
machine sits in a state, it attaches that state's outputs to the chains, such
as a damage bonus, a resistance or an advantage. It detaches them when the
machine leaves the state. On the terminal state it calls the rulebook's `End`
hook first, while the outputs still answer, and only then detaches. The
composer also answers the information rows: one per current output, from the
same guard that execution asks.

## Constructing an instance

Construction names the consumer vocabulary separately from the instance data:

```go
composed, err := effects.Compose(effects.ComposeInput{
    Recipe:            recipe,
    MachineVocabulary: factsAndBeats,
    OutputVocabulary:  outputKinds,
    HolderID:          holderID,
    SourceRef:         sourceRef,
    Parameters:        savedBag,
    State:             savedState,
})
```

For a fresh instance, omit `Parameters` and `State`: the recipe's bag defaults
and initial state are used. A nil `SourceRef` means no applying source.
Construction still validates before attachment; call `Apply` with the bus.

Direct machine consumers use `machine.New(machine.Config{Program: program,
HolderID: holderID})`. A supplied `Bag` remains owned by that machine without
copying; nil requests a fresh default bag. `State` defaults to initial, and
`Apply` retains responsibility for refusing unknown/terminal states.

## Validation and observation boundaries

`KindSpec.Check` receives the complete `Output`, not only its raw params. A kind
can reject a missing amount or an invalid combination before anything attaches.
Consumers migrating from a params-only Check decode `out.Params` in that one hook.

`Composed.Snapshot()` returns `{State, Bag}` as owned values for a caller-owned
instance codec; it does not include rulebook origin/bindings or replace the
generic `ToJSON` format. `CurrentOutputs()` returns a defensive slice of current
observational handles. `Bound.Params()` and `Host()` detach their returned data.
Guards on a retained handle still read live state; use Snapshot for saved state
and fetch current handles after transitions. `Bound.Definition()` returns the
complete authored output as owned data, including its guard and amount strategy;
`Output.Clone()` provides the same detached copy. Neither evaluates or captures a
live bag. Availability still comes from CurrentOutputs, not a retained definition.

`Changed` reports bag mutations and successful nonterminal state transitions,
after new outputs attach. Its error reaches the beat publisher; this is not
rollback of the changed runtime. Failed output attachment does not report a
successful change. Terminal transitions retain End-before-detach without an
extra Changed notification.

## Independent parts and linked lifetimes

`Assemble(AssemblyInput)` groups ordered `PartInput{Key, Compose, OnTerminal}`
values for one holder. Each uses the existing machine; transitions from different
parts are not flattened into one first-match list. `EndPart` completes only that
part and reports Changed. `EndAttachment` requires an end hook and ends all local
parts without requesting that other holders end; `EndApplication` requests the
shared application end. Both attachment/application scopes report
once while contributions are still attached, and then detaches all local parts.
During that report, query-only and subscribed contributions remain available,
including the terminating part's pre-terminal outputs. Machine beat and tracker
dispatch is gated before reactions run, so reentrant transport cannot mutate or
detach sibling parts mid-report.

The assembly owns part instance namespaces and End/Changed hooks. Supplying a
second namespace or nested hooks is refused, as are mixed holders, duplicate
keys and already-terminal required maintenance. `Remove` is interaction teardown,
not a gameplay end. `RestoreAssembly` requires exact saved part coverage and
never revives a completed local part. Saved snapshot inputs are copied; definition
recipes, vocabularies and referenced construction values remain borrowed and
must be immutable, as required by `Compose` and `Recipe.Compile`.

`BeginEnd` lets an external lifetime owner close local reaction permission before
ordered removal without calling a second end hook. Contributions remain available
until `Remove`; the operation neither selects a reason nor owns membership.

`Lifetime` is a separate neutral value for the shared coordinator: a fixed ordered
intended list, authorized ender keys, and one accepted end plan. All intended
members start pending. `FinalizeMember` resolves one pending entry exactly once,
with or without an actual delivered attachment. Pending entries are saveable but
not active members; finalized entries cannot reopen. `MemberRemoved` distinguishes
actual last-member loss from empty creation. `RestoreLifetime` requires explicit,
disjoint pending/live subsets rather than refilling membership from defaults.
Ending freezes both sets and its plan contains only live members. The consumer
delivers that plan, then calls `CompleteEnd`. No whole-application delivery seal,
rulebook address, bus publication or new lifecycle interpreter is introduced.

Amounts use a single `AmountResult` contract: a scalar or unresolved dice, not
both. `ScalarValue` refuses dice rather than rolling. Query-only kinds declare
`KindQuery` and have no Attach function; both query and subscribed kinds validate
outputs and provide assessment. Rows use a nil amount for absence, not zero.

Neutral `Guard.Bag` comparisons and `Act.Add` integer deltas support consumer
countdowns. A transition stages all its acts; overflow refuses it without partial
writes, a state transition or Changed notification. These operations know no D&D
clock or concentration rule.

## Selecting contributions without changing lifetimes

`SelectContributions` consumes already-applicable `SelectionCandidate` values in
caller-owned oldest-first order. Each has an opaque contribution ID and an explicit
`SelectionRule`: `AllApplicable` has no group; `OldestApplicable` selects the first
candidate in each nonempty group. The zero/unknown policy, malformed groups and
blank/duplicate IDs are errors. The entire input is validated before any decisions
are returned.

The result owns one `SelectionDecision` per candidate, in input order. A winner has
`Selected: true`; an excluded value names its winner in `SelectedInstead`.
`ValidateSelectionRule` checks an unbound authored rule without inventing candidate
IDs. Definition compilation and selection use the same policy validation.

**Nonselection is not removal or suspension.** The caller still owns applicability,
identity, durable order and independent lifetimes. The selector knows no quantities,
rolls, clocks, subscriptions or conditions; it neither sorts nor retains a winner.
If an older candidate leaves, the next call selects from the remaining input.
Information and execution can consume these same decisions while the operation
that owns the quantity still decides how selected contributions combine.

## Rage, line by line

This is the recipe the tests carry, which is the design's `raging`. It is
abbreviated: every turn end below is about `holder_is_subject`, and the Go
literal is `rageRecipe` in `machine/testvocab_test.go`. Each comment names
the piece that reads the field and what that does at the table.

```go
machine.Recipe{
    Ref:     dnd5e:conditions:raging, // Blob header; the loader routes on it
    Stacks:  by_ref,                  // a second Rage on the same barbarian is "already applied"
    Initial: "fresh",                 // New starts here: raging, not yet past a turn end
    Bag: {                            // Bag = Blob: these keys sit flat beside ref/member_id/source/state
        was_hit_this_turn: false,     //   set by a tracker, read by the sustain guard, cleared each turn
        did_attack_this_turn: false,  //   likewise
        saw_turn_end: false,          //   anchor_round sets it on the first turn end
        round_activated: 0,           //   the clock anchor; 0 means not anchored
        turns_active: 0,              //   derived for display; the web's rage badge reads it
    },
    Trackers: {                       // Tracker: set a flag when a topic fires about the holder
        {post_attack_roll, holder_is_actor,  did_attack_this_turn}, // every attack roll, hit or miss
        {damage_received,  holder_is_target, was_hit_this_turn},    // twice in a turn changes the bag once
    },
    States: {
        "fresh": {
            Outputs: rageOutputs,     // the composer attaches these: the bonuses hold from activation
            Transitions: {
                // The graced turn: the first turn end never checks for activity.
                {turn_end, holder_is_subject, To: "raging",
                    Do: anchor_round, derive_turns_active, clear_flags(was_hit, did_attack)},
                rageEndings...,
            },
        },
        "raging": {
            Outputs: rageOutputs,
            Transitions: {            // run in this order on each turn end; the first that fires wins
                {turn_end, When: not(any(flag was_hit, flag did_attack)), To: "ended", Reason: "no_combat_activity"},
                {turn_end, When: rounds.since_anchor >= 9,  To: "ended", Reason: "duration_expired"}, // ten rounds
                {turn_end, When: rounds.went_backwards,     To: "ended", Reason: "clock_reset"},      // a new fight's clock
                {turn_end, Do: anchor_round, derive_turns_active, clear_flags(...)},                  // To empty: stay
                rageEndings...,
            },
        },
        "ended": {Terminal: true},    // entering it: End publishes the removal, then everything detaches
    },
}

rageEndings = {
    {dropped_to_zero, holder_is_subject, To: "ended", Reason: "unconscious"},
    {rest,            holder_is_subject, To: "ended", Reason: "rest"},
    {combat_end,      holder_is_subject, To: "ended", Reason: "combat_ended"},
}

rageOutputs = {                       // Output: kind + when + amount + params, read only by the composer
    {damage_bonus, when: all(action.melee, action.weapon_pool, action.ability = str),
        amount: by_class_level(barbarian, {1: 2, 9: 3, 16: 4}), params: {stage: features}},
    {resist,    params: {types: [bludgeoning, piercing, slashing]}},
    {advantage, when: action.ability = str, params: {on: saving_throws}},
    {advantage, when: action.ability = str, params: {on: ability_checks}},
}
```

Some notes on how the anchor and the rows behave:

- `anchor_round` sets `round_activated` only while it is 0 and the turn end
  carries a round. So a rage whose first turn end carried no round anchors on
  the first turn end that does, and an anchored rage is never re-anchored.
  The cap cannot fire until the rage is anchored.
- Rage answers four information rows: damage, resistance and the two
  advantages. The hand-written condition answers one.

## The guard answers three ways

| leaf or node | Applies | DoesNotApply | Depends |
|---|---|---|---|
| `fact` | the op holds on the known value | the op fails | the frame does not carry the fact |
| `flag`, `rounds` | the bag or round says yes | it says no | no round on the beat, or not anchored |
| `all` | every child applies | any child does not | otherwise |
| `any` | any child applies | every child does not | otherwise |
| `not` | the child does not | the child applies | the child depends |

Depends is never false. A transition whose guard depends does not fire. An
output whose guard depends returns a Depends row and cannot fold.

## How a rulebook plugs in

The tools own the grammar and the rulebook owns the words. A rulebook such as
dnd5e's `conditions` supplies four kinds of word:

- **Facts** (`machine.Vocabulary.Facts`). Each has a name, a kind (bool, int
  or string) and a reader over the frame. Rage reads `action.melee`,
  `action.weapon_pool` and `action.ability`.
- **Beats** (`machine.Vocabulary.Beats`). Each is a typed topic with a
  `Subscribe`. It also has an `About` that says which id on the event is the
  attacker, the target or the subject, the `Selectors` it supports, and a
  `Round` when the event carries one. Rage uses turn end, dropped to zero,
  rest, combat end, post attack roll and damage received.
- **Kinds** (`effects.OutputVocabulary.Kinds`). Each is Go code that knows
  its chain and stage, with `Check`, `Attach` and `Assess`. Rage uses
  `damage_bonus`, `resist` and `advantage`.
- **Amounts** (`effects.OutputVocabulary.Amounts`). Rage uses
  `by_class_level`.

The rulebook also supplies `End`, which publishes its removal topic, and
`Changed`, which publishes its state-changed topic. `Validate` refuses at load,
by name, any recipe that uses a word the vocabulary lacks. A missing word is a
finding, added to the vocabulary with its test. It is never worked around.

## Run it

```sh
cd mechanics/effects
go test ./...                          # seams, Validate, guards, the Rage walk, the composer
go test ./machine -run Example -v      # the runnable candle example
go test -run '^$' -bench . ./...       # BenchmarkGuardAnswer, BenchmarkBeatTurnEnd
golangci-lint run ./...
```

The module declares `go 1.27.0`. It needs Go 1.27 for `encoding/json/v2` and
a golangci-lint built with Go 1.27.
