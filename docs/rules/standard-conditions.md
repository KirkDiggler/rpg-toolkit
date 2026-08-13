# D&D 5e standard condition capability catalog

This catalog describes the 15 conditions in Appendix PH-A of the D&D 5e SRD
5.1 and records what the toolkit supports at commit `577ae68`. It is a scope
and dependency map, not an implementation specification.

Primary rules source: [Systems Reference Document 5.1, Appendix PH-A:
Conditions](https://media.wizards.com/2016/downloads/DND/SRD-OGL_V5.1.pdf#page=357).
Rules below are paraphrased. The SRD remains authoritative.

## How to read this catalog

- **Type** is the identifier in `rulebooks/dnd5e/events/events.go`.
- **Ref** is the canonical identity exposed by
  `rulebooks/dnd5e/refs/conditions.go`.
- **Coverage** distinguishes a declared name from executable behavior.
- A rule that says a creature "cannot" do something needs an eligibility
  check, not merely an event subscriber that reacts after the action.
- Source-specific duration, repeat saves, immunity, and removal belong to the
  applying feature unless the condition itself defines them.

## Capability summary

| Condition | Type | Ref ID | Main shared capabilities | Current coverage | Current monster relevance |
| --- | --- | --- | --- | --- | --- |
| Blinded | `ConditionBlinded` | `blinded` | perception, attack modifiers | identity only | none identified |
| Charmed | `ConditionCharmed` | `charmed` | target eligibility, social checks, source identity | identity only | none identified |
| Deafened | `ConditionDeafened` | `deafened` | perception | identity only | none identified |
| Exhaustion | `ConditionExhaustion1`–`6` | `exhaustion` | leveled state, checks, movement, attacks/saves, HP, death | conflicting identities only | none identified |
| Frightened | `ConditionFrightened` | `frightened` | source identity/visibility, checks, attacks, movement | identity only | none identified |
| Grappled | `ConditionGrappled` | `grappled` | movement speed, forced movement, source relationship | identity only | none identified |
| Incapacitated | `ConditionIncapacitated` | `incapacitated` | action/reaction eligibility | identity only | dependency of several conditions |
| Invisible | `ConditionInvisible` | `invisible` | perception/targeting, attack modifiers | identity only | none identified |
| Paralyzed | `ConditionParalyzed` | `paralyzed` | incapacitated, movement, speech, saves, attacks, critical hits | identity only | Ghoul's Claws intends to apply it |
| Petrified | `ConditionPetrified` | `petrified` | incapacitated, movement, awareness, attacks/saves, affinities | identity only | none identified |
| Poisoned | `ConditionPoisoned` | `poisoned` | attack/check modifiers, immunity | identity only | undead immunities will need it |
| Prone | `ConditionProne` | `prone` | movement modes, attack modifiers, standing | identity only | Wolf's Bite stores knockdown DC 11 |
| Restrained | `ConditionRestrained` | `restrained` | movement speed, attacks, Dexterity saves | identity only | none identified |
| Stunned | `ConditionStunned` | `stunned` | incapacitated, movement, speech, saves, attacks | identity only | none identified |
| Unconscious | `ConditionUnconscious` | `unconscious` | incapacitated, movement, awareness, prone, attacks, critical hits | partial behavior | zero-HP character lifecycle |

“Identity only” means constants and refs exist, but no standard-condition
behavior is loaded or applied. Monster relevance identifies a present content
hook, not proof that the condition works end to end.

## Conditions

### Blinded

- **SRD effects:** The creature cannot see and automatically fails checks that
  require sight. Attacks against it have advantage; its attacks have
  disadvantage.
- **Boundaries:** “Requires sight” is supplied by the check or effect being
  attempted. The condition owns the attack modifiers, while the perception or
  targeting subsystem must expose whether sight is required.
- **Duration/removal:** Defined by the source applying Blindness.
- **Dependencies:** sensory capability and check eligibility; attack modifier
  chain.
- **Toolkit:** `ConditionBlinded` and `refs.Conditions.Blinded()` exist. No
  `BlindedCondition` behavior exists.

### Charmed

- **SRD effects:** The charmed creature cannot attack the charmer or target the
  charmer with harmful abilities or magical effects. The charmer has advantage
  on social-interaction ability checks involving the creature.
- **Boundaries:** The condition must retain the charmer's identity. “Harmful”
  requires action/target classification from the attempted effect.
- **Duration/removal:** Defined by the source applying Charmed.
- **Dependencies:** source-target relationship, action and target eligibility,
  ability-check modifiers.
- **Toolkit:** `ConditionCharmed` and `refs.Conditions.Charmed()` exist. No
  `CharmedCondition` behavior exists.

### Deafened

- **SRD effects:** The creature cannot hear and automatically fails checks that
  require hearing.
- **Boundaries:** The attempted check or effect identifies whether hearing is
  required.
- **Duration/removal:** Defined by the source applying Deafened.
- **Dependencies:** sensory capability and check eligibility.
- **Toolkit:** `ConditionDeafened` and `refs.Conditions.Deafened()` exist. No
  `DeafenedCondition` behavior exists.

### Exhaustion

- **SRD effects:** Exhaustion has six cumulative levels: disadvantage on
  ability checks; speed halved; disadvantage on attacks and saves; hit point
  maximum halved; speed reduced to zero; then death. A creature suffers every
  effect at its current level and below.
- **Boundaries:** Exhaustion is one leveled condition, not six unrelated
  conditions. Gaining exhaustion increases its level; effects that remove it
  normally reduce the level by the amount stated.
- **Duration/removal:** Source and recovery rules change the level. Removing all
  levels removes the condition.
- **Dependencies:** leveled/stacked persistence, check and attack modifiers,
  saving throws, movement speed, maximum hit points, death.
- **Toolkit:** The canonical ref is the single ID `exhaustion`, while event
  types are `ConditionExhaustion1` through `ConditionExhaustion6`. No behavior
  reconciles or executes these representations.

### Frightened

- **SRD effects:** While the source of fear is within line of sight, the
  creature has disadvantage on ability checks and attacks. It cannot willingly
  move closer to that source.
- **Boundaries:** The condition must retain the fear source's identity. The
  visibility and distance clauses are evaluated from current encounter state.
- **Duration/removal:** Defined by the source applying Frightened.
- **Dependencies:** source-target relationship, line of sight, check and attack
  modifiers, voluntary movement eligibility.
- **Toolkit:** `ConditionFrightened` and `refs.Conditions.Frightened()` exist.
  No `FrightenedCondition` behavior exists.

### Grappled

- **SRD effects:** Speed becomes zero and cannot benefit from speed bonuses.
  Grappled ends if the grappler is incapacitated or an effect moves either
  creature beyond the other's reach.
- **Boundaries:** The condition must retain the grappler's identity and reach.
  The rules for initiating or escaping a grapple are separate from this
  condition's effects.
- **Duration/removal:** Removal reacts to grappler incapacitation and separation;
  an applying feature may provide escape rules.
- **Dependencies:** source-target relationship, effective speed calculation,
  reach and forced-movement resolution, Incapacitated.
- **Toolkit:** `ConditionGrappled` and `refs.Conditions.Grappled()` exist. No
  `GrappledCondition` behavior exists.

### Incapacitated

- **SRD effects:** The creature cannot take actions or reactions.
- **Boundaries:** This must be checked before resources are spent or an action
  is resolved. Bonus actions are actions for this restriction.
- **Duration/removal:** Defined by the source applying Incapacitated.
- **Dependencies:** centralized action and reaction eligibility.
- **Toolkit:** `ConditionIncapacitated` and
  `refs.Conditions.Incapacitated()` exist. No `IncapacitatedCondition`
  behavior exists. Paralyzed, Petrified, Stunned, and Unconscious depend on
  this capability.

### Invisible

- **SRD effects:** Without magic or a special sense, the creature cannot be
  seen. For hiding it is heavily obscured, though its location can still be
  revealed by sound or tracks. Its attacks have advantage and attacks against
  it have disadvantage.
- **Boundaries:** Invisible is not automatically hidden or untargetable.
  Detection and location knowledge belong to perception and encounter state.
- **Duration/removal:** Defined by the source applying Invisible.
- **Dependencies:** senses, perception and hiding; attack modifier chain.
- **Toolkit:** `ConditionInvisible` and `refs.Conditions.Invisible()` exist. No
  `InvisibleCondition` behavior exists.

### Paralyzed

- **SRD effects:** The creature is incapacitated, cannot move or speak, and
  automatically fails Strength and Dexterity saves. Attacks against it have
  advantage. A hit from an attacker within 5 feet is critical.
- **Boundaries:** The applying feature owns its save, duration, and immunity
  clauses. Automatic critical hits depend on the resolved attacker's distance,
  not whether the attack was classified as melee.
- **Duration/removal:** Defined by the source; often includes repeat saves.
- **Dependencies:** Incapacitated, movement and speech eligibility, saving
  throws, attack modifiers, distance, critical-hit resolution.
- **Toolkit:** `ConditionParalyzed` and `refs.Conditions.Paralyzed()` exist. No
  `ParalyzedCondition` behavior exists. Ghoul Claws currently contains only a
  comment describing a Constitution save and paralysis intent.

### Petrified

- **SRD effects:** The creature and its nonmagical carried or worn gear become
  solid substance; weight increases tenfold and aging stops. The creature is
  incapacitated, cannot move or speak, and is unaware; attacks against it have
  advantage; it automatically fails Strength and Dexterity saves; it resists
  all damage and is immune to poison and disease, although existing poison or
  disease is suspended rather than removed.
- **Boundaries:** Transformation details and suspension of existing effects are
  state changes beyond ordinary roll modifiers.
- **Duration/removal:** Defined by the source applying Petrified; suspended
  effects resume when appropriate.
- **Dependencies:** Incapacitated, movement/speech and awareness, saving throws,
  attack modifiers, damage resistance, poison/disease immunity and suspension.
- **Toolkit:** `ConditionPetrified` and `refs.Conditions.Petrified()` exist. No
  `PetrifiedCondition` behavior exists.

### Poisoned

- **SRD effects:** The creature has disadvantage on attacks and ability checks.
- **Boundaries:** Poison damage and the Poisoned condition are distinct. A
  creature may resist poison damage, be immune to the condition, both, or
  neither.
- **Duration/removal:** Defined by the source applying Poisoned.
- **Dependencies:** attack and ability-check modifier chains; condition
  immunity at application time.
- **Toolkit:** `ConditionPoisoned` and `refs.Conditions.Poisoned()` exist. No
  `PoisonedCondition` behavior or standard condition-immunity gate exists.

### Prone

- **SRD effects:** The creature may only crawl unless it stands, which ends the
  condition. Its attacks have disadvantage. Attacks against it have advantage
  when the attacker is within 5 feet and disadvantage otherwise.
- **Boundaries:** Standing is a movement choice with a movement cost; forced
  movement does not stand a creature. The attack modifier depends on actual
  distance, not melee/ranged labels.
- **Duration/removal:** Persists until the creature stands or another rule
  removes it.
- **Dependencies:** persisted condition state, movement modes and budget,
  standing, attack modifier chain, spatial distance.
- **Toolkit:** `ConditionProne` and `refs.Conditions.Prone()` exist. No
  `ProneCondition` behavior exists. Wolf Bite already carries `KnockdownDC: 11`
  but does not execute the Strength save or apply Prone. This is the selected
  first vertical slice after saving throws are exposed consistently.

### Restrained

- **SRD effects:** Speed becomes zero and cannot benefit from speed bonuses.
  Attacks against the creature have advantage; its attacks have disadvantage;
  it has disadvantage on Dexterity saves.
- **Boundaries:** The applying feature owns any escape procedure. Speed changes
  must compose with other movement rules without overwriting base speed.
- **Duration/removal:** Defined by the source applying Restrained.
- **Dependencies:** effective speed, attack modifier chain, saving throws.
- **Toolkit:** `ConditionRestrained` and `refs.Conditions.Restrained()` exist.
  No `RestrainedCondition` behavior exists.

### Stunned

- **SRD effects:** The creature is incapacitated, cannot move, and can speak
  only falteringly. It automatically fails Strength and Dexterity saves, and
  attacks against it have advantage.
- **Boundaries:** “Faltering” speech is descriptive unless a consuming rule
  requires a more precise decision.
- **Duration/removal:** Defined by the source applying Stunned.
- **Dependencies:** Incapacitated, movement and speech eligibility, saving
  throws, attack modifier chain.
- **Toolkit:** `ConditionStunned` and `refs.Conditions.Stunned()` exist. No
  `StunnedCondition` behavior exists.

### Unconscious

- **SRD effects:** The creature is incapacitated, cannot move or speak, is
  unaware, drops held items, and falls prone. It automatically fails Strength
  and Dexterity saves; attacks against it have advantage; a hit from an
  attacker within 5 feet is critical.
- **Boundaries:** The zero-hit-point/death-save procedure is related gameplay,
  not the complete definition of Unconscious. Not every unconscious creature
  necessarily uses player-character death saves.
- **Duration/removal:** Defined by its source. In the existing zero-HP flow,
  healing removes it and death-save outcomes can change recovery state.
- **Dependencies:** Incapacitated, movement/speech and awareness, inventory
  handling, Prone, saving throws, attack modifiers, distance, critical hits.
- **Toolkit:** `UnconsciousCondition` is implemented, persisted, and loaded. It
  handles turn-start death saves, damage failures, stabilization, critical
  recovery, and removal on healing. It does not yet enforce the general SRD
  action, movement, speech, awareness, dropped-item, Prone, save, attack, or
  proximity-critical effects.

## Shared capability order

The catalog suggests implementing reusable seams before multiplying condition
types:

1. Expose the existing saving-throw engine consistently to all combatants.
2. Implement persisted Prone state, attack modifiers, and standing/movement
   behavior.
3. Connect Wolf Bite's existing DC 11 knockdown data to a Strength save and
   successful Prone application.
4. Use that completed vertical slice to shape later condition work; do not add
   the other 14 behaviors as part of the Wolf/Prone integration.

This order makes the first integration prove the entire path—monster content,
attack outcome, saving throw, condition application, persisted behavior, and
removal—without treating the catalog itself as an implementation mandate.
