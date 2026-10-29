# Conditions Are Composed of Effects

**Status:** Draft, for the operator's agreement on the idea PR

**Story:** rpg-project story issue (linked on the PR once opened)

**Decision:** none yet. The seam between resolution and a condition
(`ConditionBehavior`, the registry, the blob) does not move; if a later slice
moves it, that slice adds an ADR.

**Execution:** `plan.md`, added after design agreement

**Knowledge:** the decisions this design rests on, and the ones it makes, are
lines in [`rulebooks/dnd5e/knowledge/decisions.md`](../../../rulebooks/dnd5e/knowledge/decisions.md).
This story is the first to be run through the knowledge process from its
first day; see "Measures".

## The operator's direction

Recorded close to verbatim, 2026-10-28:

> they are really a series of effects with a budget. what if we made conditions
> composable from the underlying effects. rage is a damage modifier for str
> backed attacks … for some duration?

> if we stack effects in a meaningful way, a conditional effect that could act
> on a shared data structure so it could remove itself. we have literal
> conditions created and spells that could be decomposed. I am looking to
> ultimately getting this into the encounter studio and just want a data driven
> composable component to build with.

> coded hand written conditions are all we need right now … breaking up
> conditions and then spells into their effect components, allowing a grouping
> under a ref like dnd5e:conditions:raging that gets one data structure and the
> effects can drive that condition fully. no need to move quickly here.

> the epic is converting all current conditions to be composed of effects.

> cost is outside the executing of a condition or spell.

> core.Effect was ahead of its time. not imported means it is free for the
> taking or at min deleted … the machine in resolution now speaks that
> language. a modern core.Effects would do it the new way. the goal of
> configuring to subscribe and act on things with the state param filled in
> is what we want.

## Context, measured

What the tree holds on 2026-10-28, at `rpg-toolkit` main:

- `rulebooks/dnd5e/conditions/` has 58 non-test files. `raging.go` is 615
  lines and nine subscriptions: four modifiers (damage bonus on Strength
  melee weapon attacks, resistance to bludgeoning, piercing and slashing,
  advantage on Strength saves, advantage on Strength checks), five upkeep
  handlers (was hit, did attack, turn end with the ten-round cap, dropped to
  zero, rest, combat end), and the subscribe-with-rollback, unsubscribe,
  ToJSON and loadJSON boilerplate that 41 of the 58 files repeat.
- Eleven conditions already carry a free-standing rule value
  (`ragingDamageRule`, `sneakAttackRule`, `duelingRule`, …) that reads only
  the typed frame from `contributions` and answers applies, does not apply or
  depends, with its reason. The fold that lands the answer on a chain is
  near-identical across them. `physicalResistances`, `subscribeRemoveOnRest`,
  `publishStateChanged` and `executeRule` are shared helpers that grew the
  same way. The composition this design names is the shape those pieces were
  already growing toward.
- Rage's assessment answers only its damage row; its comment says the
  resistance and the two advantages "are not part of this answer". The
  information surface is narrower than the behaviour.
- Two earlier attempts at this idea are dead with no importer among the 29
  modules: `mechanics/effects` (a `ComposedCondition` with Temporary, Dice,
  Stackable and Saves behaviours over string-named events and wall-clock
  durations) and `rulebooks/dnd5e/effects` (an `Effect` struct carrying
  bonuses as strings such as `"+1d4"`, and a Rage whose damage bonus is a
  constant with a comment that it varies by level). Both modelled filters as
  strings that a generic interpreter had to know; the first filter the strings
  could not express sent the rule back to hand-written code. `core/effect`
  (an `Effect[T]` with Apply and Remove on a chain) is a third, also unused:
  the chain-modifier kind lifted to core before anything pulled on it.
- The condition descriptor law (rpg-toolkit#1965, shipped in #2014) holds: a
  condition is described once beside its type, one ref, one display, one
  load, one create, its two bearings and any attack override, read through
  the one registry. Monster traits are conditions in that registry.
- Resolution already speaks data for the activation side. `actions.Definition`
  carries a `Cost` (`combat.SpendProfile`: slots, capacity, grants, keyed
  resource `Pools`, and `Requires`), and exactly one of an `Attack`, `Cast` or
  `Sequence` profile. A cast delivers conditions as `CastEffect{Recipient,
  Ref, Parameters, CounterpartKey, SaveDCKey, OptionKey}`; an attack delivers
  them as `ConditionApplication{Ref, Parameters, Save}`. Resolution builds the
  condition from the ref and parameters through the registry's `create` and
  attaches it to the one interaction bus (ADR-0038). Concentration is
  resolution's: `CastConcentration{TurnEnds, SkipFirstTurnEnd}` and
  `resolution/concentration.go`. Spells and monster actions are on this path.
  Features (`Rage`, `Reckless Attack`, `Action Surge`, `Flurry of Blows`)
  still implement `core.Action[FeatureInput]` and spend their resource by
  hand through `core/resources.ResourceAccessor`. Two activation shapes for
  one idea.
- `SpendProfile.Pools` is documented as "a feature's charges. Expressible and
  unexercised in v1". The price of Rage is already sayable as data; nothing
  says it yet.

## The shape

Four nouns, kept apart. Three of them already have a home.

```mermaid
flowchart LR
  subgraph grant [Grant: out of this story]
    G["class level · spell list · stat block · equipped item"]
  end
  subgraph activation [Activation: actions.Definition, exists]
    D["Cost (SpendProfile)<br/>Target · Save · Concentration<br/>delivers: CastEffect / ConditionApplication"]
  end
  R["resolution<br/>builds ref + parameters through the registry<br/>attaches to the one bus"]
  subgraph condition [Condition: one ref · one descriptor · one blob]
    S[("state bag<br/>(the blob)")]
    M["modifier effects<br/>fold a chain through a frame filter"]
    T["tracker effects<br/>set a bag field when a topic fires"]
    B["budget effects<br/>read the bag at a beat, call end"]
    E["end<br/>publish removed · detach every effect"]
    T --> S
    S --> B
    S -.-> M
    B --> E
  end
  G --> D --> R --> condition
```

- **Effect**: one atom. Contribute a value to a chain. Set a flag when a topic
  fires. Read the flags at a beat and end the condition. Three kinds,
  one shape: configure what to subscribe to, what must be true, and what to
  do, with the condition's state handed in.
- **Condition**: a bundle of effects grouped under one ref, sharing one
  state bag, ending through one verb. It is what the registry builds and what
  the blob persists. Its outward contract, `ConditionBehavior`, does not
  change.
- **Activation**: the moment of use, already data. Cost, targeting, the
  contest, concentration, and what is delivered. This story moves the four
  features onto it; it does not redesign it.
- **Grant**: how an activation reached a sheet. Named here so the loot door
  (an equipped item that grants an activation or a passive condition) has a
  word when it arrives. No code under it today; it is its own story.

## What a condition looks like

A Go literal, declarative enough that every field is a value. The exact
names are the plan's; the shape is the design's.

```go
var raging = composed{
    ref:   refs.Conditions.Raging(),
    state: &ragingBag{}, // saw_turn_end, round_activated, turns_active, was_hit_this_turn, did_attack_this_turn
    effects: []effect{
        damageBonus{stage: combat.StageFeatures, when: meleeWeaponUsing(abilities.STR), amount: byClassLevel(classes.Barbarian, 2, 3, 4)},
        resist{types: physicalTypes},
        advantage{on: saves, ability: abilities.STR},
        advantage{on: checks, ability: abilities.STR},
        track{topic: PostAttackRollChain, set: didAttack},
        track{topic: DamageReceivedTopic, set: wasHit},
    },
    budget: all(
        rounds(10),                        // clock-anchored, activation turn graced
        sustainedBy(didAttack, wasHit),    // checked at the holder's turn end
        endsOn(droppedToZero, rest, combatEnd),
    ),
}

var blessed = composed{
    ref:   refs.Conditions.Blessed(),
    state: &sourcedBag{}, // member_id, source_id
    effects: []effect{
        rollBonus{on: attackRolls, dice: "1d4"},
        rollBonus{on: savingThrows, dice: "1d4"},
    },
    budget: endsOn(rest), // concentration and its turn ends are resolution's, not the condition's
}

var resistanceDie = composed{
    ref:   refs.Conditions.Resistance(),
    state: &sourcedBag{},
    effects: []effect{
        offerDie{on: PostSaveRollOfferChain, die: "1d4", description: ResistanceOfferDescription},
    },
    budget: all(endsOn(rest), spentWhenTaken()),
}
```

The descriptor stays exactly as it is: ref, display, load, create, bearings,
attack override. `load` reads the bag; `create` fills it from the cast's
parameters. Display may be authored or derived from the effects; each effect
answers its own information row, so the condition's assessment is the sum of
its effects' answers.

## Working agreements of this design

Numbered so slices and knowledge lines can cite them by text. They are
agreements under measurement, not laws; the measures below say whether they
hold.

- **E1. One ref, one descriptor, one blob.** A composed condition registers
  through the same descriptor table as a hand-written one and loads from the
  same blob shape. Effects are internal structure, never registered, never
  addressable from outside the condition. *Because* six parallel tables keyed
  by condition ref drifted once already (rpg-toolkit#1965).
- **E2. An effect is configuration, not code.** It names what it subscribes
  to (a chain or a topic), what must be true (a filter), and what it does (a
  fold, a bag write, or end), and it receives the condition's state. Three
  kinds: modifier, tracker, budget. *Because* "configuring to subscribe and
  act on things with the state param filled in is what we want" (operator,
  2026-10-28).
- **E3. Filters name frame facts.** A modifier's filter reads the typed frame
  and the bag, never the live sheet, and an unknown fact answers Depends, not
  false. *Because* the two dead attempts invented their own filter strings and
  needed an interpreter that knew every one; the frame is a closed, typed
  vocabulary that already exists and already answers three ways.
- **E4. The bag is the blob.** The state the effects share is the state the
  condition persists, with its keys kept stable where a consumer duck-types
  them (`turns_active` for the web's rage badge). *Because* a composition
  that persisted differently from what it ran would be the dual representation
  the toolkit's laws forbid.
- **E5. End is one verb.** Publish the removal, then detach every effect. Only
  a budget calls it, and only through it does a condition leave. *Because*
  every hand-written condition writes this twice today (`endRage`, `end`) and
  the publish-before-detach order is a rule an activation's effect collector
  depends on.
- **E6. Budgets are effects, and they compose.** Ten rounds anchored to the
  clock's round, sustained by a bag predicate at the holder's turn end, ending
  on dropped-to-zero, rest or combat end, spent when an offer is taken.
  Combined with `all`. *Because* three past defects (rpg-toolkit#752, #755,
  rpg-project#253) all lived in hand-rolled upkeep; a budget written once is
  a budget debugged once. The graced activation turn (operator, 2026-08-27)
  and the clock-anchored round are carried into `rounds`, not re-derived per
  condition.
- **E7. Cost is never on a condition.** A condition's price is its
  activation's `SpendProfile`. Concentration is resolution's. *Because* "cost
  is outside the executing of a condition or spell" (operator, 2026-10-28),
  and the dead `Effect` struct is the warning: it carried `Concentration` and
  `Duration` beside a bonus string, cost smeared into effect.
- **E8. Hand-written conditions stay legal.** A condition whose behaviour has
  no atom yet keeps a hand-written effect inside the composed shell, and an
  atom is extracted on its second use, not its first. *Because* a vocabulary
  widened on a gap is the thing the dead attempts became.
- **E9. Rulebook-neutral by lift, not by birth.** The shell (ref, bag,
  effects, budget, end, blob) grows in `rulebooks/dnd5e/conditions` and is
  lifted to a neutral module when a second puller arrives: a second rulebook,
  or the studio. `core/effect` is deleted in the first slice so the ground
  and the import path are free. *Because* `core` is bus-free by design and
  the lift that happened early is the one sitting unused; and "no sense
  pretending like previous us knew how this whole thing would turn out"
  (operator, 2026-10-28).
- **E10. Features become Definitions.** Rage, Reckless Attack, Action Surge
  and Flurry of Blows move from `core.Action` to `actions.Definition`, their
  charges as `SpendProfile.Pools`, their condition as the delivered effect.
  *Because* resolution already speaks that language and two activation
  shapes for one idea is the drift this story exists to close.

## Approaches considered

1. **Strings and an interpreter.** Conditions as data from day one, with a
   generic engine reading bonus strings and filter names. Tried twice in this
   tree; both dead. Rejected: the interpreter must know every filter, and the
   first one it does not sends the rule back to code.
2. **Typed composition in Go, lifted later.** Atoms are Go values whose
   filters name frame facts; the shell is a small type in the conditions
   package; the literal is declarative enough to become a studio form by
   transliteration when the atom list stops moving. **Recommended.** It is
   the path the code is already on, it keeps the descriptor law and the
   resolution seam untouched, and it defers the neutral home to the moment
   something pulls on it.
3. **Keep hand-writing, extract helpers.** The status quo drift: more shared
   functions, no shell. Rejected: it never yields one data structure per
   condition, so the studio never gets a component to build with, and the
   information surface stays narrower than the behaviour.

## Slices

Named here; the plan creates the sub-issues and may re-cut them. Each code
slice is one module PR (one nearest `go.mod`), docs may ride along.

- **S1. The shell, the budgets, and Rage.** `composed`, the bag, `end`, the
  budget combinators (`rounds`, `sustainedBy`, `endsOn`, `spentWhenTaken`),
  the modifier and tracker atoms Rage needs, and `RagingCondition` rebuilt as
  the literal above with the identical blob. The existing rage tests run
  unchanged. Separate module PR: delete `core/effect`.
- **S2. The 1d4 family.** Blessed, Baned, Divine Favor, Guided, Inspired:
  roll bonuses and offered dice, source-qualified bags.
- **S3. The against-holder family.** Prone, Held, Faerie Fire, Sanctuary:
  effects that fold the attacker's chain through the holder's bearing, and
  the save-gated ones.
- **S4. Features become Definitions.** The four `core.Action` features move
  to `actions.Definition` with `Pools` costs; `features` keeps the sheet-side
  surface (status, description) it still owns.
- **S5. The long tail.** The remaining conditions, each either composed from
  atoms or kept hand-written inside the shell under E8, with a count of each.
- **S6. The lift decision.** With five slices behind it, decide the neutral
  home (a new module, or `events`), what moves, and what the studio form is.
  An ADR if the seam moves.

Spells are already on the activation path; the decomposition they need is
of the conditions they deliver, which S2 and S3 cover. Instantaneous spells
(damage, healing, stabilize) are already instantaneous data and are not
conditions.

## Settled, and open

**Settled in conversation, 2026-10-28, ruled by the operator:** the target
is one data structure per condition, driven fully by its effects, under one
ref (E1, E2); hand-written Go is the form for now, content authoring comes
with the studio (E9); cost lives outside the condition (E7); `core/effect` is
free for the taking or deleted (E9); features and spells converge on one
activation shape (E10); no need to move quickly.

**Open, for the design PR's checklist:**

- [ ] Stacking identity: by ref, or by ref plus source, declared per condition
  on the literal. Recommendation: a `stacks` field defaulting to by-ref, with
  the source-qualified conditions (Resistance, Blessed) declaring by-source,
  which is what `ConditionAddressProvider` already distinguishes.
- [ ] Display: authored on the descriptor as today, or derived from the
  effects' rows. Recommendation: authored stays; derivation is a later win
  once every effect answers a row and the words are checked against the
  authored ones.
- [ ] Bag versus parameters: a cast's `Parameters` fill the bag through
  `create`; the keys a cast binds (`CounterpartKey`, `SaveDCKey`,
  `OptionKey`) are bag keys. Recommendation: yes, and nothing else; a bag key
  no effect reads is a finding.
- [ ] Which neutral home at S6. Not decided before S6 by design.

## Measures

Posted on the story issue as slices land, the way the team-memory story
posts its rows on rpg-project#676:

- lines per converted condition, before and after, and the count of atoms;
- conditions converted, kept hand-written under E8, and the ratio;
- information rows a condition answers, before and after (Rage: 1 → 4);
- knowledge lines at the dnd5e level this story adds, and how many of the
  existing lines gain an `upheld` link from its PRs;
- defects found in budgets after S1, against the three that preceded it.

## Delivery finish line

The idea PR carries `design.md`, then `plan.md` after agreement, and stays
open while slices merge in their own module PRs; `implementation.md` is
added with tags and observed deviations before the idea PR merges. Every
slice's PR carries a run record at `rulebooks/dnd5e/knowledge/runs/`, and
every ruling walked in conversation lands as a knowledge line the same day.
