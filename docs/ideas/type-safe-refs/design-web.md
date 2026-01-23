# Design: React Components for Type-Safe Damage Sources

## Overview

React component patterns for displaying damage sources using the new `SourceRef` oneof structure.

## Core Pattern: Enum Display Maps

Each proto enum gets a display map:

```typescript
// src/components/enums/displays.ts

import {
  Weapon,
  Ability,
  ConditionId
} from '@kirkdiggler/rpg-api-protos/gen/ts/dnd5e/api/v1alpha1';

export const WEAPON_DISPLAY: Record<Weapon, string> = {
  [Weapon.WEAPON_UNSPECIFIED]: 'Unknown',
  [Weapon.WEAPON_LONGSWORD]: 'Longsword',
  [Weapon.WEAPON_WARHAMMER]: 'Warhammer',
  [Weapon.WEAPON_GREATAXE]: 'Greataxe',
  [Weapon.WEAPON_DAGGER]: 'Dagger',
  // ... all weapons
};

export const ABILITY_DISPLAY: Record<Ability, string> = {
  [Ability.ABILITY_UNSPECIFIED]: 'Unknown',
  [Ability.ABILITY_STRENGTH]: 'STR',
  [Ability.ABILITY_DEXTERITY]: 'DEX',
  [Ability.ABILITY_CONSTITUTION]: 'CON',
  [Ability.ABILITY_INTELLIGENCE]: 'INT',
  [Ability.ABILITY_WISDOM]: 'WIS',
  [Ability.ABILITY_CHARISMA]: 'CHA',
};

export const CONDITION_DISPLAY: Record<ConditionId, string> = {
  [ConditionId.CONDITION_ID_UNSPECIFIED]: 'Unknown',
  [ConditionId.CONDITION_ID_RAGING]: 'Rage',
  [ConditionId.CONDITION_ID_BRUTAL_CRITICAL]: 'Brutal Critical',
  [ConditionId.CONDITION_ID_FIGHTING_STYLE_DUELING]: 'Dueling',
  [ConditionId.CONDITION_ID_FIGHTING_STYLE_TWO_WEAPON_FIGHTING]: 'Two-Weapon Fighting',
  [ConditionId.CONDITION_ID_SNEAK_ATTACK]: 'Sneak Attack',
  [ConditionId.CONDITION_ID_DIVINE_SMITE]: 'Divine Smite',
};
```

## Individual Enum Components

Simple components for each enum type:

```typescript
// src/components/enums/WeaponDisplay.tsx

import { Weapon } from '@kirkdiggler/rpg-api-protos/gen/ts/dnd5e/api/v1alpha1';
import { WEAPON_DISPLAY } from './displays';

interface WeaponDisplayProps {
  weapon: Weapon;
  className?: string;
}

export const WeaponDisplay: React.FC<WeaponDisplayProps> = ({ weapon, className }) => (
  <span className={`weapon-badge ${className ?? ''}`}>
    {WEAPON_DISPLAY[weapon] ?? 'Unknown Weapon'}
  </span>
);
```

```typescript
// src/components/enums/AbilityDisplay.tsx

import { Ability } from '@kirkdiggler/rpg-api-protos/gen/ts/dnd5e/api/v1alpha1';
import { ABILITY_DISPLAY } from './displays';

interface AbilityDisplayProps {
  ability: Ability;
  className?: string;
}

export const AbilityDisplay: React.FC<AbilityDisplayProps> = ({ ability, className }) => (
  <span className={`ability-badge ${className ?? ''}`}>
    {ABILITY_DISPLAY[ability] ?? 'Unknown'}
  </span>
);
```

```typescript
// src/components/enums/ConditionDisplay.tsx

import { ConditionId } from '@kirkdiggler/rpg-api-protos/gen/ts/dnd5e/api/v1alpha1';
import { CONDITION_DISPLAY } from './displays';

interface ConditionDisplayProps {
  condition: ConditionId;
  className?: string;
}

export const ConditionDisplay: React.FC<ConditionDisplayProps> = ({ condition, className }) => (
  <span className={`condition-badge ${className ?? ''}`}>
    {CONDITION_DISPLAY[condition] ?? 'Unknown Condition'}
  </span>
);
```

## SourceRef Component (Discriminated Union)

The main component that handles the oneof:

```typescript
// src/components/combat/DamageSourceBadge.tsx

import { SourceRef } from '@kirkdiggler/rpg-api-protos/gen/ts/dnd5e/api/v1alpha1';
import { WeaponDisplay } from '../enums/WeaponDisplay';
import { AbilityDisplay } from '../enums/AbilityDisplay';
import { ConditionDisplay } from '../enums/ConditionDisplay';

interface DamageSourceBadgeProps {
  source: SourceRef;
  className?: string;
}

export const DamageSourceBadge: React.FC<DamageSourceBadgeProps> = ({ source, className }) => {
  // TypeScript discriminated union pattern
  if (source.weapon !== undefined) {
    return <WeaponDisplay weapon={source.weapon} className={className} />;
  }

  if (source.ability !== undefined) {
    return <AbilityDisplay ability={source.ability} className={className} />;
  }

  if (source.condition !== undefined) {
    return <ConditionDisplay condition={source.condition} className={className} />;
  }

  return <span className={className}>Unknown Source</span>;
};
```

## Damage Breakdown Component

Putting it together for the combat UI:

```typescript
// src/components/combat/DamageBreakdown.tsx

import { DamageComponent } from '@kirkdiggler/rpg-api-protos/gen/ts/dnd5e/api/v1alpha1';
import { DamageSourceBadge } from './DamageSourceBadge';

interface DamageBreakdownProps {
  components: DamageComponent[];
  total: number;
}

export const DamageBreakdown: React.FC<DamageBreakdownProps> = ({ components, total }) => (
  <div className="damage-breakdown">
    <div className="damage-components">
      {components.map((comp, i) => (
        <div key={i} className="damage-component">
          <DamageSourceBadge source={comp.sourceRef!} />
          <span className="damage-value">
            {comp.isCritical && '⚡'}
            +{comp.flatBonus + sumDice(comp.finalDiceRolls)}
          </span>
        </div>
      ))}
    </div>
    <div className="damage-total">
      Total: {total}
    </div>
  </div>
);

// Helper
function sumDice(rolls: number[]): number {
  return rolls.reduce((a, b) => a + b, 0);
}
```

## Example Output

For a raging Barbarian hitting with a Warhammer:

```
┌─────────────────────────────┐
│ Warhammer        +8         │
│ STR              +3         │
│ Rage             +2         │
├─────────────────────────────┤
│ Total: 13                   │
└─────────────────────────────┘
```

## Styling

```css
/* src/components/combat/DamageBreakdown.css */

.damage-breakdown {
  font-family: monospace;
  border: 1px solid #ccc;
  border-radius: 4px;
  padding: 8px;
}

.damage-component {
  display: flex;
  justify-content: space-between;
  padding: 4px 0;
}

.weapon-badge {
  color: #8b4513;
  font-weight: bold;
}

.ability-badge {
  color: #4169e1;
}

.condition-badge {
  color: #dc143c;
  font-style: italic;
}

.damage-total {
  border-top: 1px solid #ccc;
  margin-top: 8px;
  padding-top: 8px;
  font-weight: bold;
}
```

## File Summary

| File | Purpose |
|------|---------|
| components/enums/displays.ts | Enum → display name maps |
| components/enums/WeaponDisplay.tsx | Weapon enum display |
| components/enums/AbilityDisplay.tsx | Ability enum display |
| components/enums/ConditionDisplay.tsx | ConditionId enum display |
| components/combat/DamageSourceBadge.tsx | SourceRef oneof handler |
| components/combat/DamageBreakdown.tsx | Full damage breakdown UI |

## Future Extensions

When new source types are added to the oneof:

```typescript
// Just add another branch
if (source.spell !== undefined) {
  return <SpellDisplay spell={source.spell} className={className} />;
}
```

The TypeScript compiler will catch any missed cases if using exhaustive checks.
