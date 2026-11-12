# Authored examples

These are **proposed input examples**, not fixtures a released loader can execute.
They make [R4/R5](design.md#rulings) concrete. The item adapter supplies user,
recipient, origin and invocation separately; none are saved into the reusable
library. Test-world setup and assertions also stay outside this content.

## One library, five applications

```json
{
  "parts": {
    "rage-damage": {
      "kind": "damage_bonus",
      "amount": {"fixed": 2},
      "when": {"all": [
        {"fact": {"name": "action.melee", "op": "eq", "value": true}},
        {"fact": {"name": "action.weapon_pool", "op": "eq", "value": true}},
        {"fact": {"name": "action.ability", "op": "eq", "value": "str"}}
      ]}
    },
    "physical-resistance": {
      "kind": "resistance",
      "params": {"types": ["bludgeoning", "piercing", "slashing"]}
    },
    "strength-saves": {
      "kind": "advantage",
      "params": {"on": "saving_throw"},
      "when": {"fact": {"name": "action.ability", "op": "eq", "value": "str"}}
    },
    "strength-checks": {
      "kind": "advantage",
      "params": {"on": "ability_check"},
      "when": {"fact": {"name": "action.ability", "op": "eq", "value": "str"}}
    },
    "concentrate": {"kind": "concentration"},
    "no-members": {"kind": "until_empty"},
    "stay-active": {"kind": "combat_activity", "params": {"skip_first": true}},
    "ten-ends": {"kind": "duration", "params": {"turn_ends": 10, "skip_first": false}},
    "rest": {"kind": "end_on", "params": {"beat": "rest", "reason": "rest"}},
    "combat-end": {"kind": "end_on", "params": {"beat": "combat_end", "reason": "combat_ended"}},
    "zero-hp": {"kind": "end_on", "params": {"beat": "dropped_to_zero", "reason": "unconscious"}},
    "next-turn": {"kind": "end_on", "params": {"beat": "turn_start", "reason": "turn_started"}},
    "reckless-benefit": {
      "kind": "advantage",
      "params": {"on": "attack", "side": "by_holder"},
      "when": {"all": [
        {"fact": {"name": "action.melee", "op": "eq", "value": true}},
        {"fact": {"name": "action.weapon_pool", "op": "eq", "value": true}},
        {"fact": {"name": "action.ability", "op": "eq", "value": "str"}},
        {"fact": {"name": "action.opportunity", "op": "eq", "value": false}}
      ]}
    },
    "reckless-drawback": {
      "kind": "advantage",
      "params": {"on": "attack", "side": "against_holder"}
    }
  },
  "benefit_sets": {
    "rage-benefits": ["rage-damage", "physical-resistance", "strength-saves", "strength-checks"],
    "reckless-tradeoff": ["reckless-benefit", "reckless-drawback"]
  },
  "definitions": {
    "rage-ring": {
      "name": "Maintained rage",
      "coordinator": "user",
      "maintenance": [
        {"key": "concentration", "part": "concentrate"},
        {"key": "empty", "part": "no-members"},
        {"key": "rest", "part": "rest"}
      ],
      "benefits": [{"key": "rage", "set": "rage-benefits"}]
    },
    "rage-scroll": {
      "name": "Raging scroll",
      "coordinator": "recipient",
      "maintenance": [
        {"key": "activity", "part": "stay-active"},
        {"key": "duration", "part": "ten-ends"},
        {"key": "empty", "part": "no-members"},
        {"key": "rest", "part": "rest"},
        {"key": "combat-end", "part": "combat-end"},
        {"key": "zero", "part": "zero-hp"}
      ],
      "benefits": [{"key": "rage", "set": "rage-benefits"}]
    },
    "physical-ward": {
      "name": "Maintained physical ward",
      "coordinator": "user",
      "maintenance": [
        {"key": "concentration", "part": "concentrate"},
        {"key": "empty", "part": "no-members"},
        {"key": "rest", "part": "rest"}
      ],
      "benefits": [{"key": "ward", "part": "physical-resistance"}]
    },
    "damage-gift": {
      "name": "Maintained damage gift",
      "coordinator": "user",
      "maintenance": [
        {"key": "concentration", "part": "concentrate"},
        {"key": "empty", "part": "no-members"},
        {"key": "rest", "part": "rest"}
      ],
      "benefits": [{"key": "damage", "part": "rage-damage"}]
    },
    "kitchen-sausage": {
      "name": "Reckless bite",
      "coordinator": "recipient",
      "maintenance": [
        {"key": "next-turn", "part": "next-turn"},
        {"key": "empty", "part": "no-members"},
        {"key": "rest", "part": "rest"},
        {"key": "zero", "part": "zero-hp"}
      ],
      "benefits": [{"key": "reckless", "set": "reckless-tradeoff"}]
    }
  }
}
```

The `action.opportunity` fact, combat-activity part, attack-side configuration,
additional end-on beats and payload When/Amount fields are proposed additions.
The other words reuse current meanings. The detailed implementation must verify
literal spellings against canonical types; the schema must not introduce aliases
for existing facts or ability identifiers.

## Item grant

Each example uses the same experimental consumer ref and binding contract. Only
its definition key changes:

```json
{
  "ref": {"module": "example", "type": "abilities", "id": "apply_effect_definition"},
  "parameters": {"definition": "rage-ring"},
  "pool": "uses",
  "uses": 1
}
```

The proposed illustrative item pool has maximum 1 and no recovery occurrences.
Ring/scroll/sausage economics are not being balanced here. After a successful use,
remaining availability is zero; the inventory object is not deleted. A spent or
detached item is not itself an ending rule for the delivered grant.

## Observable fixture policies

| Fixture | Proposed observable behavior |
|---|---|
| Ring | User A grants B the four benefits with fixed +2 eligible damage. B can remain idle without losing them. There is no duration cap in this definition. A's concentration loss/replacement, zero HP through concentration, or A's rest ends the application. |
| Scroll | User A grants B the same benefits. B owns upkeep; A need not concentrate or remain a lifetime participant. B's first turn end skips only the activity check. Duration still counts that end: at most ten B turn ends total. A later idle B turn end, B's rest/combat end/zero HP, or actual last-member loss ends it. |
| Ward | Only physical resistance; no damage bonus or Strength advantage. The user maintains it like the ring. A second configured damage-type list must exercise the same primitive without changing a handler. |
| Damage gift | Only +2 on eligible Strength melee weapon damage; no resistance or advantages. A recipient with no Barbarian levels can use the fixed bonus. |
| Sausage | User and recipient are the eater. Strength melee weapon attacks other than opportunity attacks gain advantage; all attacks against the eater gain advantage. It ends on the eater's first subsequently received turn start, rest or zero HP. |

The sausage is deliberately **reckless-style**, not acquisition/activation of the
Barbarian feature. It has no first-attack opt-in and does not claim an own-turn
restriction beyond the explicit opportunity-attack exclusion. Native Reckless
Attack remains unchanged. If a different eligibility or lifetime is wanted, that
is an authored policy choice to settle before fixing the fixture's expectations.

The ring and ward grant selected benefits, not the complete native Raging
condition. Target unconsciousness is not an implicit ring maintenance rule;
concentration belongs to the user. This distinction is part of R5, not an
accidental default.

## What the tests must distinguish

- Changing the selected maintenance changes who sustains the effect, not its
  benefit set. Changing the selected benefit leaves source payment independent.
- A target taking damage does not trigger the user's concentration save; the user
  taking positive applied damage does. No second HP application occurs.
- A scroll target's missed attack sustains activity; unrelated attacks, zero
  damage and another creature's turn end do not.
- Ending one maintained application leaves another source's independent benefits
  intact. Replacement removes only the replaced application's exact recipients.
- The sausage's target-side information and actual attack roll agree, including
  ranged attacks against the eater. An observer with unknown configured details
  gets Depends, not a fabricated drawback or a live-state lookup.
- Saving and reloading preserves spent item availability, concrete provenance,
  grace, counters and pending/live membership. Changing the source library does
  not rewrite an already active effect.
- Invalid data fails before payment; a delivery failure after successful payment
  does not pretend the pure item resolver can roll back arbitrary world effects.
