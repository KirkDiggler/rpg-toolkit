#!/usr/bin/env bash
set -euo pipefail

production_roots=(
  rulebooks/dnd5e/combat
  rulebooks/dnd5e/events
  rulebooks/dnd5e/conditions
  rulebooks/dnd5e/gamectx
  rulebooks/dnd5e/monster
  rulebooks/dnd5e/resolution
)
# Top-level encounter/ is frozen on an old provider pin pending removal and is
# deliberately excluded from this provider/resolution guard.
legacy_symbols='(^|[^[:alnum:]_])(ResolveAttackHit|ResolveAttack|ApplyAttackOutcome|ResolveAttackHitInput|ApplyAttackOutcomeInput|AttackInput|AttackOutcome|CombatResolver|PhasedCombatResolver|PhasedAttackContext|WeaponForActionRef|MeleeWeaponProvider)([^[:alnum:]_]|$)'

# Blank comments and string literals before scanning. This keeps historical
# prose from tripping the guard while preserving line numbers and all
# executable declarations, calls, field/type references, and inline code.
go_code_without_comments() {
  perl -0pe 's{(?:"(?:\\.|[^"\\])*"|\x27(?:\\.|[^\x27\\])*\x27|`[^`]*`)|//[^\n]*|/\*.*?\*/}{my $text=$&; $text =~ s/[^\n]/ /g; $text}gsex' "$1"
}

scan_go_code() {
  local pattern=$1
  shift
  local file match
  local found=1

  while IFS= read -r -d '' file; do
    while IFS= read -r match; do
      printf '%s:%s\n' "$file" "$match"
      found=0
    done < <(go_code_without_comments "$file" | rg -n -- "$pattern" || true)
  done < <(rg --files -0 "$@" --glob '*.go' --glob '!**/*_test.go')

  return "$found"
}

# Search declarations, calls, and field/type references in provider/resolution
# production Go. The old check only looked for a few top-level declarations, so
# a method or a caller could leave the tree falsely clean after resolver files
# vanished.
if scan_go_code "$legacy_symbols" "${production_roots[@]}"; then
  echo 'legacy attack resolution symbols or callers remain in production' >&2
  exit 1
fi

if scan_go_code '^[[:space:]]*func[[:space:]]*(\([^)]*\)[[:space:]]*)?(ResolveAttack|ResolveAttackHit|ApplyAttackOutcome|TakeActionPhased|CompleteTakeAction)\b' "${production_roots[@]}"; then
  echo 'legacy attack-specific methods remain in production' >&2
  exit 1
fi

if rg -n 'damage_dice|damage_type|damage_bonus|KnockdownDC|Scimitar(Config|Action)' rulebooks/dnd5e/monster --glob '*.go' --glob '!**/*_test.go'; then
  echo 'legacy damage fields remain' >&2
  exit 1
fi

if sed -n '/^type DamageChainEvent struct {/,/^}/p' rulebooks/dnd5e/events/events.go | rg -n '^[[:space:]]*(WeaponDamage|DamageType)[[:space:]]'; then
  echo 'legacy event-wide damage metadata remains' >&2
  exit 1
fi
