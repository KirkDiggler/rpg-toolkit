# Level-one Cleric domain spell support

## Scope

Audit the seven Cleric domains currently exposed at creation before implementing
missing granted spells. Keep 2014 spell/domain semantics; the adopted 2024
preparation counts do not change spell editions. Domain feature/proficiency work
is a later pass. Only level-one grants are in scope. Death Domain is present in
the provider modification table but is not in the seven-domain creation catalog.

## Audited baseline

Provider main: 31865887dcfb0f698a673594723db13653499bc6 (root v0.186.0).
Grant authority: rulebooks/dnd5e/character/choices/subclass_modifications.go.
Executable definition authority: rulebooks/dnd5e/spells/cast.go, HasCastProfile.
A catalog entry or grant is not evidence of executable behavior.

| Domain | Grant | Current executable Cast definition |
| --- | --- | --- |
| Life | Bless | Yes |
| Life | Cure Wounds | Yes |
| Light | Burning Hands | No |
| Light | Faerie Fire | No |
| Nature | Animal Friendship | No |
| Nature | Speak with Animals | No |
| Tempest | Fog Cloud | No |
| Tempest | Thunderwave | Yes |
| Trickery | Charm Person | No |
| Trickery | Disguise Self | No |
| War | Divine Favor | No |
| War | Shield of Faith | Yes |
| Knowledge | Command | Yes |
| Knowledge | Identify | No |

There are fourteen distinct grants, five with definitions and nine without.
Light (the bonus cantrip) remains explicitly deferred until object targeting and
illumination exist. The existing inert Thaumaturgy should not be presented as
new executable spell support.

## Implementation dependencies

- Divine Favor: reuse bonus-action spell payment, concentration and the damage
  component chain. Add radiant dice only to qualifying weapon hits, including
  ranged weapons; preserve critical rules, attribution, reload and teardown.
- Burning Hands: existing area declarations support radius and box, not a cone.
  Needs a cone customer across targeting/coverage and consumer previews, plus
  Dexterity save/half damage. Do not substitute a cube or chosen-creature list.
- Faerie Fire: a chosen-point cube is not supported by the current caster-only
  area origin declaration. Requires placement, saves, concentration-linked
  persistent recipients, attack advantage and its visibility interactions.
- Fog Cloud: requires a persistent positioned volume affecting sight and target
  eligibility, including entering/leaving, concentration teardown and recovery.
  Existing sight-blocking geometry alone is not a complete spell implementation.
- Animal Friendship and Charm Person: require the appropriate eligibility and
  charmed/social relationship behavior, including hostility and break rules.
  They are not cosmetic effects; defer explicitly if these interactions are
  outside the current wave.
- Speak with Animals, Disguise Self, Identify: require communication, disguise
  recognition, or object identification interactions. Keep granted access but
  do not invent successful no-op casts.

## Availability presentation

The user permits visibly deferred spells. Render "Not yet implemented" from
provider/API-authored availability, including grant rows. Never infer support
from the spell name, domain, or absence of a transient combat declaration.
Not implemented is distinct from temporarily unavailable (no slot, no action,
wrong range or no target). Deferred entries retain their grants and use no
resources; executable Cast behavior must be absent/refused.

Existing available spell refs remain the selection authority; automatic grants
remain outside selected-preparation counts. Bard creation must stay unchanged.
