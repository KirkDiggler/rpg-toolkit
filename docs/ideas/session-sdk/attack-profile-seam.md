# The Attack Profile Seam: One Strike, N Compilers

> **Superseded by ADR-0045 / rpg-toolkit#1198.** The compiler placement and
> resolution-owned profile described below are historical. Current producers
> share `combat/actions.Definition`; monsters author it directly, characters use
> `character.AssembleAttack`, and resolution dispatches with `resolution.NewAction`.

**Date:** 2026-08-15
**Status:** reflection + forward design. The "today" examples are working
code on main (resolution v0.5.0); the "proposed" examples are the shape
#1003 builds and are marked as such.
**Prompted by:** Kirk's review concern on #1002 — *"we made Strike a monster
specific thing"* — which deserves a precise answer, because it is half
true and the half matters.

## The precise claim

The strike is three layers, and they are not equally monster-shaped:

| Layer | Shape | Monster-specific? |
|---|---|---|
| The machine (`NewStrike`) | fold → hit vs AC → damage → gate → contest | **No** — reads only `AttackProfile` |
| The currency (`AttackProfile`) | `{Ref, AttackBonus, DamageDice, DamageType, Gate}` | **No** — derived, never persisted |
| The compilers | `AttackFromMonsterAction` | **Yes — today it is the only one** |

So the honest state: the *system* is neutral, the *coverage* is monster-only.
That is a designed gap, not an accident — a character's swing is the same
phases with a different compilation — but a design claim is only proven by
its second consumer. This document shows the second consumer in working
detail, and recommends building it next (#1003) so the claim stops being
an assertion.

## Today, working: a wolf's bite (gate and all)

```go
// The stat block is the whole input.
wolf := monsters.NewWolf("wolf-1").ToData()

attack, err := resolution.AttackFromMonsterAction(wolf.Actions[0])
// attack == AttackProfile{
//     Ref:         dnd5e:monster_actions:bite,
//     AttackBonus: 4,
//     DamageDice:  "2d4+2",
//     DamageType:  damage.Piercing,
//     Gate:        &SaveGate{[STR], DCStatic(11), Negated, RecurrenceNone},
// }

out, err := resolution.Resolve(ctx, &resolution.Input{
    World:        worldData, // EncounterData — positions ride in it
    Participants: []resolution.Participant{{Character: heroData}, {Monster: wolf}},
    Machine: resolution.NewStrike(&resolution.StrikeInput{
        AttackerID: "wolf-1", TargetID: "hero", Attack: attack,
    }),
})
// On a hit with a failed save: out.DirtyCharacters[0] has less HP and a
// prone condition; out.Hooks says which effect attached what; the
// StrikeOutcome carries the full breakdown including the contest's.
```

## Today, working: a skeleton's shortsword

Monsters use weapons, and a stat-block weapon is a generic `MeleeAction`
with pre-computed numbers — 5e's own model ("Shortsword. +4 to hit,
1d6+2 piercing" is content, not runtime equipment resolution):

```go
skeleton := monsters.NewSkeleton("sk-1").ToData()

attack, _ := resolution.AttackFromMonsterAction(skeletonMeleeAction)
// AttackProfile{Ref: melee, AttackBonus: 4, DamageDice: "1d6+2",
//               DamageType: Piercing, Gate: nil}   // a plain weapon just hits
```

Same machine, no gate, no contest. Pinned end to end
(`TestASkeletonSwingsItsShortsword`: exactly `1d6[3]+2 = 5`).

## Proposed (#1003): a character's longsword

A character's numbers are not pre-computed — they are *derived* from the
sheet and the equipped weapon. That derivation is the whole of #1003; the
machine does not change by a line:

```go
// AttackFromCharacter compiles an equipped weapon and the sheet holding it
// into the same neutral profile a monster action compiles into.
func AttackFromCharacter(c *character.Character, in *CharacterAttackInput) (AttackProfile, error) {
    slot := c.GetEquippedSlot(in.Slot)             // which weapon
    weapon := weaponFor(slot)                       // weapons.Weapon: dice, type, properties

    // Ability: STR, unless finesse lets DEX win.
    ability := abilities.STR
    if weapon.HasProperty(weapons.PropertyFinesse) &&
        c.GetAbilityModifier(abilities.DEX) > c.GetAbilityModifier(abilities.STR) {
        ability = abilities.DEX
    }
    mod := c.GetAbilityModifier(ability)

    // Proficiency: from the sheet, or absent.
    bonus := mod
    if c.IsProficientWith(weapon) {
        bonus += c.ProficiencyBonus()
    }

    // Versatile: the caller says how many hands; the dice change, nothing else.
    dice := weapon.Damage                           // "1d8"
    if in.TwoHanded && weapon.HasProperty(weapons.PropertyVersatile) {
        dice = weapon.VersatileDamage               // "1d10"
    }

    return AttackProfile{
        Ref:         weapon.Ref(),                  // dnd5e:weapons:longsword
        AttackBonus: bonus,                         // mod + proficiency
        DamageDice:  fmt.Sprintf("%s%+d", dice, mod), // "1d8+3" — ability mod rides the pool
        DamageType:  weapon.DamageType,
        Gate:        nil,                           // weapons declare no rider
    }, nil
}
```

*(Method names are illustrative where the sheet does not expose them yet —
`IsProficientWith`, `VersatileDamage`, `weapon.Ref()` are #1003's small
additions; `GetEquippedSlot`, `GetAbilityModifier`, `ProficiencyBonus`,
`HasProperty` exist today.)*

Then the same call as the wolf, sides swapped:

```go
Machine: resolution.NewStrike(&resolution.StrikeInput{
    AttackerID: "hero", TargetID: "wolf-1",
    Attack:     heroLongsword,
})
```

## Future: the ADR-0039 mirror — a monk knocks a wolf down

Open Hand Flurry is a player-imposed gate — the same declaration the wolf's
bite carries, compiled from class content instead of a stat block:

```go
Attack: AttackProfile{
    Ref: refs.Features.FlurryTechnique(), AttackBonus: 5,
    DamageDice: "1d4+3", DamageType: damage.Bludgeoning,
    Gate: &saves.SaveGate{
        Abilities:  []abilities.Ability{abilities.DEX},  // Flurry is DEX where the bite is STR
        DC:         saves.DCStatic(8 + prof + wisMod),
        OnSuccess:  saves.Negated,
        Recurrence: saves.RecurrenceNone,
    },
}
// Monster savers already work (#988/#994): the wolf rolls its own STR/DEX.
```

## The stress table — where each 5e mechanic lands

The seam's division of labor, stated once so nobody re-litigates it per
mechanic:

| Mechanic | Layer | Why |
|---|---|---|
| Finesse, versatile, proficiency, a +1 weapon | **Compiler** | static facts of weapon + sheet, knowable before the swing |
| Rage's damage bonus, Bless, Sneak Attack's dice | **Chain (bus fold)** | situational — their own predicates decide per swing; the compiler must not know they exist |
| Great Weapon Fighting rerolls | **Chain** | per-die, which is why `FinalDiceRolls` is a per-die contract |
| Advantage/disadvantage (prone, Dodging, Pack Tactics) | **Chain** | same |
| Extra Attack, Action Surge, two-weapon economy, Multiattack | **Above the machine** | turn economy — how many strikes, not what one strike is. Multiattack is a natural future machine that `Request`s N strikes |
| Reactions (Shield, opportunity attacks) | **Wave 5** | windows between phases — the machine's step boundaries are already the suspension points |

The rule of thumb: **static facts compile, situational effects fold,
economy sits above.** If a mechanic seems to need a new profile field, check
this table first — most "missing fields" are chain contributions wearing a
disguise.

## Honest gaps, named

- **Ranged attacks** — refused today. Needs range brackets, long-range
  disadvantage, and prone's *reversed* interaction at distance. Compiler +
  machine both grow; sequenced behind #965 slice 2.
- **Reach/adjacency** — unenforced for bite and weapons alike; the room is
  built and positions exist, so the check is cheap once someone owns the
  rule. Slice 2's list.
- **`AbilityUsed` on the attack chain event** — if an effect ever predicates
  on "attacks using Strength", the folded event will need to carry which
  ability the compiler chose. Additive when needed; not built against
  hypotheticals.
- **Character gate sources** — Flurry's gate rides the profile above, but
  where class content *declares* it (feature data vs computed) is #1003's
  design question, answered by ADR-0039's data-first rule.

## Where we are going — recommended order

1. **#1003 next: `AttackFromCharacter`.** It is the proof of the seam's
   central claim, it is additive (zero machine changes), and it should land
   *before* the divestment reshapes combat underneath — a second consumer is
   exactly the evidence slice 2 wants about which combat helpers are
   vocabulary worth keeping.
2. **#965 slice 2** (reopened after the #1002 squash auto-closed it): the
   measured divestment, now with two attacker kinds of evidence, the Notify
   classification, and the Gather-widening ADR call.
3. **#964, #966** as planned.

— asset-pipeline agent, on behalf of KirkDiggler
