# Player-Facing Damage Breakdown Design

## Goal

Show players an exact, readable explanation of each resolved damage component and the final attack-damage total.

## Scope

This is presentation of already resolved damage. It does not change damage rules, dice rolling, critical-hit rules, resistance, vulnerability, immunity, or attack definitions.

## Architecture

Keep `events.DamageComponent` as the canonical resolved record. It already holds the final dice rolls, per-component `FlatBonus`, damage type, and total calculation. Add `DiceNotation` as read-only display metadata, copied from the attack's declared damage pool when dice are rolled. It is never used to calculate damage.

Add a shared formatter in the D&D 5e combat package:

- A component formatter creates a line such as `1d6 (4) + 2 acid = 6`.
- A breakdown formatter joins component lines with `; ` and appends `Total: <n> damage.`

The formatter reads resolved components only. It does not perform or alter game math. Every caller, including monster natural attacks and character weapon attacks, can use the same formatter.

## Formatting Rules

- `DiceNotation` identifies the declared dice: `1d6` or `2d6`. The component's final die values are shown in parentheses: `1d6 (4)` or `2d6 (5 + 3)`.
- A positive flat bonus is rendered as ` + N`; a negative bonus as ` - N`; zero is omitted.
- The damage type follows the roll and bonus.
- The final component value uses `DamageComponent.Total()`.
- The whole-breakdown total is the supplied resolved total, rather than a second independent calculation.

Example:

`1d6 (4) + 2 acid = 6; 2d6 (5 + 3) + 3 bludgeoning = 11. Total: 17 damage.`

## Error Handling

The formatter is intentionally best-effort presentation. Components with no dice but a flat bonus remain representable, for example `+ 3 slashing = 3`. A component with final dice rolls but no `DiceNotation` is formatted from its values without inventing dice sizes; normal combat resolution always provides the notation.

## Tests

Create a focused combat test using deterministic rolls. It must assert the exact player-facing string for separate acid and bludgeoning components and the final total:

- Acid: `1d6`, roll `4`, flat bonus `+2`, component total `6`.
- Bludgeoning: `2d6`, rolls `5 + 3`, flat bonus `+3`, component total `11`.
- Combined display: `1d6 (4) + 2 acid = 6; 2d6 (5 + 3) + 3 bludgeoning = 11. Total: 17 damage.`

The existing full D&D 5e suite must remain green.
