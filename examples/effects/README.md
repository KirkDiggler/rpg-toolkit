# examples/effects

Conditions composed from effects. Two tools, formed here together, with tests
as their only customer and a Rage-shaped vocabulary. When the dnd5e
conditions first consume them, they are promoted to a module of their own.

- Design: [`docs/ideas/composable-conditions/design.md`](https://github.com/KirkDiggler/rpg-toolkit/blob/docs/idea-composable-conditions/docs/ideas/composable-conditions/design.md) on the idea branch
- Story: [rpg-project#703](https://github.com/KirkDiggler/rpg-project/issues/703)
- Godoc: `go doc ./machine` and `go doc .` walk every piece in the order you
  meet it.

## The two tools

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
cd examples/effects
go test ./...                          # seams, Validate, guards, the Rage walk, the composer
go test ./machine -run Example -v      # the runnable candle example
go test -run '^$' -bench . ./...       # BenchmarkGuardAnswer, BenchmarkBeatTurnEnd
golangci-lint run ./...
```

The module declares `go 1.27.0`. It needs Go 1.27 for `encoding/json/v2` and
a golangci-lint built with Go 1.27.
