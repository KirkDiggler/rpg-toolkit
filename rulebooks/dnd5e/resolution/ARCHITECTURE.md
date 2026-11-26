# How an Action Resolves: Shared Data, Machines, and the One Bus

**Status:** current architecture after rpg-toolkit#1198.

```mermaid
flowchart TB
    subgraph PRODUCERS["Producers"]
        MON["monster factories\nauthor Definition directly"]
        CHAR["character.AssembleAttack\nderives Definition"]
    end

    DEF["combat/actions.Definition\nidentity · optional cost · typed profile"]

    subgraph SESSION["Active composition + host seam"]
        VIEW["encounter ActionView\nopaque ref · name · max range · kind"]
        SEAM["session selects/assembles definition\nprojects declared cost into resolution.Cost"]
    end

    subgraph RESOLUTION["resolution owns the one interaction bus"]
        NEW["NewAction\nvalidate + dispatch by profile arm"]
        START["Machine.Start\npure preflight"]
        PAY["pay cost"]
        DRIVE["drive Gather | Request | Done"]
        TEAR["teardown"]
    end

    subgraph RULES["Executable rules"]
        SEQ["Sequence\nordered steps · per-step disadvantage · stop rule"]
        STRIKE["Strike\nrange · attack · damage"]
        CONTEST["Contest\nsave-gated condition"]
        CONDITIONS["condition registry + behavior\nown subscriptions/lifecycle"]
    end

    MON --> DEF
    CHAR --> DEF
    DEF --> VIEW --> SEAM
    SEAM --> NEW --> START --> PAY --> DRIVE --> TEAR
    NEW -->|Attack arm| STRIKE
    NEW -->|Sequence arm| SEQ
    SEQ -->|Request per step| STRIKE
    STRIKE -->|ordered application| CONTEST
    STRIKE --> CONDITIONS
    CONTEST --> CONDITIONS
```

## Ownership

| Fact or behavior | Owner |
|---|---|
| action identity, name, declared price, typed profile | `combat/actions.Definition` |
| monster-authored numbers | `monster/monsters` factory literals |
| character equipment/ability derivation | `character.AssembleAttack` |
| opaque action projection for behavior | active `encounter.ActionView` |
| profile dispatch | `resolution.NewAction` |
| delivery/range, attack roll, ordered damage | Strike |
| step order, per-step modifiers, when a script stops | Sequence |
| save request | Contest / Save |
| executable effect and lifecycle | condition implementation |
| action economy policy | caller/session projects `Definition.Cost` into `Input.Cost`; resolution charges only that supplied runtime cost |

`NewAction` consumes the definition's typed profile but does not infer who pays
or which turn applies. The caller explicitly projects `Definition.Cost` into a
`resolution.Cost` carrying that runtime payer/turn context. A definition's cost
is therefore enforced at the resolution door without making profile dispatch
own action-economy policy.

Actions never activate themselves, subscribe, apply, remove, or manage a
lifecycle. There is no producer-specific compiler in resolution and no generic
monster-action implementation ref.

## Attack profile

`AttackProfile` covers any attack-roll interaction: melee/ranged weapon,
natural, unarmed, and melee/ranged spell attacks. Delivery is an exactly-one
melee/ranged union in feet. Optional ability and weapon fields are static
evidence; precomputed monster stat blocks may omit them honestly.

Damage is ADR-0041's ordered pool collection. A profile may omit damage only
when it declares at least one meaningful on-hit condition.

## Sequence profile

`SequenceProfile` is a script: ordered steps, each naming a component action
`core.Ref` the SAME actor carries plus whatever applies to that step alone.
The SRD's Multiattack is the whole of what it is for today.

`NewAction` resolves the steps against `ActionInput.Components` — the actor's
own definitions, supplied by whoever holds the actor, since this package loads
no sheets — through `actions.ResolveSequence`. A step naming something the
actor does not carry, or naming another sequence, is refused there, while
`Resolve` is still in pure preflight.

The machine owns three things and no rules: **order**, **per-step modifiers**,
and **the stop rule**. Every blow is the existing Strike, reached through
`Request` exactly as a cast reaches a contest per target, so reach, the attack
chain, criticals, damage, on-hit riders and Sanctuary are unchanged and
unduplicated.

A step's declared disadvantage travels as a `StrikeInput.Imposed` source —
`SourceRef` the sequence (the rule), `SourceID` the attacker (the die) — and
folds onto the component's attack chain **before** the fold, so an effect that
grants advantage cancels it exactly as it cancels the long-range source Strike
appends itself. Disadvantage is never a boolean; it reaches `rolls.RollD20` as
an imposed source and lands on the keep record under the reason the content
wrote.

Every step is preflighted before the first is run, because half a multiattack
cannot be taken back.

Each entry in `SequenceOutcome.Steps` names the component that swung beside
the blow it produced, so a consumer writing a beat per step says "scimitar"
rather than walking the definition's step list in parallel and trusting the
two to stay aligned.

**Concentration is recorded per swing**, not per action (Kirk's ruling,
2026-09-20). A concentration check is a roll a defender made against ONE blow,
so it rides that blow's step along with the break it caused, exactly as a lone
strike's rides its own outcome. `Output.ConcentrationChecks` and
`ConcentrationBreaks` are therefore EMPTY for a sequence — one place, so a
consumer cannot record the same save twice by reading both. A fact no swing's
own check explains (a caster dropped to zero by the blow, whose fact carries no
save to match) rides the last step rather than being guessed at.

**The stop rule is about the target, not the blow.** A miss does not end a
sequence — "two attacks with its scimitar" is two swings whatever the first
one rolled. A downed target does, and `SequenceOutcome.Unswung` reports how
many declared steps never happened rather than leaving it to be reconstructed.

## Conditions

`ConditionApplication` contains a condition ref, opaque parameters, and an
optional `SaveGate`. Strike prepares registry behavior during pure Start, before
payment. On hit it processes applications in slice order:

- no save: publish the prepared condition and record `Applied:true`;
- save succeeds: record the Contest and `Applied:false`;
- save fails: publish the prepared condition and record `Applied:true`.

Multiple applications remain ordered data rather than an effects language.

## Driven activations

A monster's abilities are driven, not declared: whoever drives its turn decides
them. So `NewActivation` refuses a monster unless `ActivationInput.Driven` is
set. Resolution lands the ability's outcome on the monster the way the player
arm lands it on a character: the same condition, through `publishCondition`, so
`replaceSameAddress` supplies the one-instance-per-address rule the monster
keeper lacks (rpg-toolkit#1946). The outcome is the player arm's
`ActivationOutcome` shape. Disengage is the only driven ability today
(rpg-toolkit#2032). Any other is refused by name, and a character sent through
the driven door is refused because characters declare. Open question: which
condition a driven ability lands (Disengage to Disengaging) is rules knowledge
held here, and when Hide arrives the rulebook should answer it instead.

### What changed on 2026-11-26 (rpg-project#1131, slice 1b)

This section used to say that a monster's economy is not on its sheet, that
`Driven` meant the driver had already decided and paid, and that a driven
activation sent with an `Input.Cost` was refused at `Input.Validate`. None of
that is true now. The operator ruled that an NPC's turn is owned by its sheet,
as a player's is (rpg-project#1131, 2026-11-26): `monster.Data.Economy` holds an
action, a bonus action and a reaction (nil for a full turn), the sheet gives
them back at its own turn start, and `*monster.Monster` is a `combat.Ledger`
(`rulebooks/dnd5e/v0.238.0`, rpg-project#1191).

- The one door charges a monster as it charges a character: `payAtTheDoorFrom`
  calls `combat.Pay(combat.WithPools(monster, budget), cost.Profile)`. A refusal
  is `ErrCannotPay` wrapping the gate's own error, with the payer's id. A nil
  profile charges nothing. A monster has no pools, so a price naming a pool is
  refused whole unless the second budget owns it.
- A monster is not refreshed at the door. `Cost.Turn` is a character's marker
  and is ignored on a monster's cost.
- `Driven` means "decided by whoever drives the monster's turn", no longer
  "decided and paid for". A driven activation sent with a `Cost` is charged at
  the door before its step runs, like every other machine; one sent with no
  `Cost` charges nothing.
- A monster's cast is still not priced at this door (`payForMachine` refuses it
  with `ErrNoPayer`).

State between two slices: no caller sends a monster a cost yet. The session's
slice is next. Until it lands, encounter's `TurnBudget` still spends the
Disengage's bonus action and counts it beside the sheet, not on it, and the
session sends the driven activation with no `Cost`, so nothing in play changes
(rpg-project#1194).

## Extension rule

> Add a profile arm only when a distinct machine exists. Dispatch by profile
> type, never content identity.

Ranged attacks reuse Strike because their step sequence is unchanged. A
save-area attack or healing action earns a profile only with its own machine.
Multiattack earned one: the Sequence arm above dispatches to a machine that
owns order and a stop rule, and it dispatches on the populated profile without
ever learning the word.
