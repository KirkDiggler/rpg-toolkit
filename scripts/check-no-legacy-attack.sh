#!/usr/bin/env bash
set -euo pipefail

if rg -n 'func (ResolveAttack|ResolveAttackHit|ApplyAttackOutcome)|type (AttackInput|ResolveAttackHitInput|ApplyAttackOutcomeInput)' rulebooks/dnd5e/combat encounter; then
  echo 'legacy attack resolution symbols remain' >&2
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
