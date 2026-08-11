# Future cleanup list

## Shared condition-immunity implementation

The shared condition-immunity implementation currently lives in the
`monstertraits` code area because that is where it was first introduced. Its
rules are now shared by monsters, characters, and future features. During a
deliberate architecture cleanup, move it into a general effects location
without changing its behavior or the canonical list of fifteen standard D&D
conditions.

## Rename the broad effect lifecycle interface

During an intentional architecture cleanup, rename the broad technical
mechanism from `ConditionBehavior` to `EffectBehavior`. Provide compatibility
support so existing toolkit code and external users can migrate safely. This
rename must not imply that every effect is a D&D condition.
