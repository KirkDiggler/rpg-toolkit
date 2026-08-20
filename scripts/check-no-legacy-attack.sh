#!/usr/bin/env bash
set -euo pipefail

production_roots=(rulebooks/dnd5e/combat rulebooks/dnd5e/events rulebooks/dnd5e/conditions rulebooks/dnd5e/gamectx rulebooks/dnd5e/monster encounter)
legacy_symbols='(^|[^[:alnum:]_])(ResolveAttackHit|ResolveAttack|ApplyAttackOutcome|ResolveAttackHitInput|ApplyAttackOutcomeInput|AttackInput|AttackOutcome|CombatResolver|PhasedCombatResolver|PhasedAttackContext|WeaponForActionRef|MeleeWeaponProvider)([^[:alnum:]_]|$)'

# Search declarations, calls, and field/type references in production Go. The
# old check only looked for a few top-level declarations, so a method or a
# caller could leave the tree falsely clean after the resolver files vanished.
if rg -n "$legacy_symbols" "${production_roots[@]}" --glob '*.go' --glob '!**/*_test.go'; then
  echo 'legacy attack resolution symbols or callers remain in production' >&2
  exit 1
fi

if rg -n '^[[:space:]]*func[[:space:]]*(\([^)]*\)[[:space:]]*)?(ResolveAttack|ResolveAttackHit|ApplyAttackOutcome|TakeActionPhased|CompleteTakeAction)\b' "${production_roots[@]}" --glob '*.go' --glob '!**/*_test.go'; then
  echo 'legacy attack-specific methods remain in production' >&2
  exit 1
fi

if rg -n 'damage_dice|damage_type|damage_bonus|KnockdownDC|Scimitar(Config|Action)' rulebooks/dnd5e/monster --glob '*.go' --glob '!**/*_test.go' encounter/seed_monsters.go; then
  echo 'legacy damage fields remain' >&2
  exit 1
fi

if sed -n '/^type DamageChainEvent struct {/,/^}/p' rulebooks/dnd5e/events/events.go | rg -n '^[[:space:]]*(WeaponDamage|DamageType)[[:space:]]'; then
  echo 'legacy event-wide damage metadata remains' >&2
  exit 1
fi
