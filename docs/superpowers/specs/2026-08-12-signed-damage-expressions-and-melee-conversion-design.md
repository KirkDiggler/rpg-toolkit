# Signed Damage Expressions and Legacy Melee Conversion Design

## Goal

Let D&D 5e damage pools represent linear dice math accurately, while converting existing monster melee actions such as `1d8+4` into structured damage data without breaking saved content.

## Scope

This work covers the shared D&D damage-expression model and the already-connected monster melee action path. It does not wire RangedAction or BiteAction into the shared combat resolver; those actions will reuse this model in a later, deliberate integration.

## Supported Expression Grammar

The first version supports one linear expression per damage pool:

```text
XdY (+ or - XdY)* (+ or - whole number)?
```

`X` and `Y` are positive whole numbers. Whitespace around `+` and `-` is permitted.

Valid examples:

- `1d8+4`
- `2d6-2`
- `1d6+1d4`
- `1d6-1d4+2`
- `2d6 + 1d4 - 3`

Rules:

- Every `XdY` is a rolled dice term.
- `+` adds the following dice term or whole number.
- `-` subtracts the following dice term or whole number.
- A whole number without a following `d` contributes to the pool's `FlatBonus`.
- Parentheses, multiplication, division, functions, and other non-linear syntax are rejected for now.

## Structured Model

Replace the current single dice string as the canonical representation of a damage pool with signed dice terms plus a flat bonus:

```text
Damage pool: acid
├── + 1d6
├── - 1d4
└── FlatBonus: +2
```

The original normalized expression is retained as display metadata. It is never used to calculate damage after parsing.

Each signed dice term contains:

- dice notation: a pure positive `XdY` group;
- sign: `+1` or `-1`.

The pool owns the signed terms, damage type, flat bonus, properties, and optional save. A pool remains one damage component regardless of how many signed dice terms it contains. This is essential: resistance, vulnerability, and immunity apply to the completed same-type pool, not separately to its individual dice terms.

## Resolution and Critical Hits

The combat resolver rolls every term independently and adds or subtracts each result according to its sign. The flat bonus is added once.

For example:

```text
1d6 - 1d4 + 2 acid
```

with rolls `5` and `2` produces `5 acid`.

On a critical hit, every dice term in a crit-eligible pool doubles, including subtracted dice terms. Flat bonuses never double:

```text
1d6 - 1d4 + 2 acid
critical → 2d6 - 2d4 + 2 acid
```

## Player-Facing Display

The resolved component retains each signed term and its final dice rolls. The formatter shows the actual arithmetic rather than flattening or inventing a label:

```text
1d6 (5) - 1d4 (2) + 2 acid = 5
```

Positive multi-dice expressions similarly show each term:

```text
1d6 (4) + 1d4 (3) + 2 acid = 9
```

## Legacy Melee Conversion

`MeleeConfig` currently has legacy fields:

```text
DamageDice
DamageType
DamageComponents
```

The conversion boundary is `NewMeleeAction` and its serialized loader path.

Precedence and behavior:

1. A valid new structured damage specification is authoritative.
2. Otherwise, each legacy `DamageComponents` entry becomes one structured pool, preserving its damage type and parsing its dice expression.
3. Otherwise, legacy `DamageDice` and `DamageType` become one structured pool.
4. Every converted legacy melee pool is marked crit-eligible, preserving ordinary weapon-attack critical behavior.
5. If a legacy expression is invalid under the supported grammar, construction/loading returns a clear validation error before combat rolls.

Examples:

```text
Legacy Brown Bear Bite
DamageDice: 1d8+4
DamageType: piercing

Structured result
+ 1d8 piercing; FlatBonus +4
```

```text
Legacy mixed Pseudopod
1d6-1 bludgeoning
2d6 acid

Structured result
Pool 1: + 1d6 bludgeoning; FlatBonus -1
Pool 2: + 2d6 acid; FlatBonus 0
```

## Persistence Compatibility

When old data is loaded, preserve its original legacy `DamageDice` and `DamageComponents` text for compatibility and human readability. Also construct and persist the new structured damage specification.

On subsequent loads, the structured specification is authoritative. The legacy fields remain compatibility data and must stay synchronized with the structured model when serializing a newly created melee action.

No existing monster source file must be rewritten merely to make combat work.

## Non-Goals

- Do not add parentheses, multiplication, division, functions, variables, or arbitrary expression evaluation.
- Do not wire RangedAction or BiteAction into the shared resolver in this change.
- Do not create a Pseudopod-only, Brown-Bear-only, or monster-only damage resolver.
- Do not alter damage affinity rules in this change; the signed terms calculate a single typed pool before affinity application.

## Testing

Tests will be written first and include:

- parse/validation for added and subtracted dice terms, flat bonuses, whitespace, and rejected non-linear syntax;
- Brown Bear `1d8+4` conversion and resolved damage;
- a legacy mixed-component action preserving both pools and their damage types;
- `1d6-1d4+2` with deterministic signed rolls and exact player display;
- critical hits doubling positive and negative dice terms but not the flat bonus;
- persistence round-trip retaining legacy text and the structured specification;
- regression coverage proving existing modern Gray Ooze and Ochre Jelly definitions resolve unchanged.

The full D&D 5e suite must pass when dependencies are available.
