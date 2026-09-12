# Cleric creation implementation

## Supported spell acquisition follow-up (2026-09-12, PR #1698)

Creation now asks for Bane, Command, and Cure Wounds through the existing
leveled-spell selection field and persists their refs in KnownSpells. Cleric
finalization seeds the existing two-slot first-level resource from class data,
using the same initialization as Bard. Preparation and domain grants remain
deferred. The shared spell-choice validator rejects repeated picks, including
duplicates split across submissions (#1662).

Tests cover executable spell profiles, draft and character JSON round trips,
slot spending/reload/rest, invalid selections, class replacement, and loading
older finalized sheets without implicit grants. Character/choice tests passed
locally before the final rest/compatibility assertions; Windows Application
Control blocked the updated character test executable, so final execution is
delegated to Linux CI. Vet and dependency hygiene passed. No dependency pins
changed. New or unfinished drafts need the new spell choice; existing finalized
characters are not migrated, and API/UI adoption remains a separate release step.

The first contribution to [rpg-project#406](https://github.com/KirkDiggler/rpg-project/issues/406)
repairs level-one Cleric character creation. [PR #1585](https://github.com/KirkDiggler/rpg-toolkit/pull/1585)
merged as `b2f3b88d` on 2026-09-08 and is released as D&D `v0.150.0`.
The refreshed [plan.md](plan.md) records subsequent integration work.

Clerics receive light/medium armor, shield and simple-weapon proficiencies,
plus one fixed shield. Life's existing subclass data supplies heavy armor.
The selected domain reaches validation, and recording, completeness and final
validation use subclass requirements. Duplicate skills and cantrips are rejected;
Life's starting warhammer requires proficiency from the compiled grants.
Main already supplies spell-choice completeness, so this contribution reuses it.

The Cleric suite verifies draft and character JSON round trips, equipment
quantities and alternatives, background grants, invalid choices, and class/domain
replacement. Reload preserves damaged HP and spent slot data. This is creation
support; prepared spells, runtime casting, domain spells, Disciple of Life and
concentration remain outside this PR. Other domains' automatic grants and
equipment prerequisites are not migrated here.

## Verification

- The full D&D module suite passes on native Windows with no exclusions.
- The existing appearance test assumed consecutive updates cross a clock tick.
  It now uses a reloaded draft's zero timestamp as the baseline, retaining the
  strict update assertion and invalid-input atomicity checks without sleeps or
  production changes. That test passes 100 consecutive native Windows runs.
- `go fmt ./...` and `go mod tidy` ran in the D&D module; no dependency changes.
- Local `golangci-lint` is unavailable. Race testing requires cgo, disabled in
  this environment. The initial creation commit passed CI; CI also validates
  the subsequent timestamp-test change.

The broader playable-Cleric journey remains open; these checks do not establish
API, web or live-game support.

## Sacred Flame content (merged #1592, D&D v0.151.0, 2026-09-08)

Added the level-one Sacred Flame profile to the existing cast table: 60 feet,
one creature, Dexterity save against the supplied DC, 1d8 radiant on failure,
negated on success, with no condition or concentration. Profile tests use the
reviewed 2014 entry in `source-index.json`. A creation/reload regression proves
a Cleric with Wisdom 16 supplies DC 13 and retains all three known cantrips,
while only Sacred Flame among those choices has executable content.

The full D&D module suite passes natively on Windows with no skips. Formatting
and diff checks pass. No other module or repository pins changed. This content
slice is merged; cover semantics, higher-level scaling,
and the shared damage-chain issue #1582 remain outside the validated claim.

## Sacred Flame session acceptance (2026-09-08)

The session module now adopts the published D&D `v0.151.0` content. No production
casting changes or other-repository pin updates were needed. Session acceptance
uses a level-one Cleric sheet with Wisdom 16, Charisma 8, three known cantrips,
and both first-level slots already spent.

The full session module suite passes on native Windows. New cases prove:

- Only Sacred Flame has an executable offer among Sacred Flame, Guidance and Light.
- The skeleton uses Dexterity +2 against the Cleric's Wisdom-based DC 13.
  A total of 12 takes the scripted radiant damage; 13 negates it without a damage roll.
- Both outcomes spend one action and preserve exhausted slots and known cantrips.
- JSON round trips of all repository records and manager recreation preserve
  the exact cast/save/damage story, target HP and spent action.
- Forged selectors, unknown targets, removed spell ownership and repeated casts
  are refused without dice, event publication or repository writes.
- A target at 60 feet is offered; one at 65 feet is visibly out of range and refused.

Formatting and module tidy checks pass. Local race/lint tooling remains unavailable;
CI supplies those checks. Preparation and API/web adoption are not part of this slice.

Cover assessment: session target preflight filters current intel holdings and
range. The resolution save uses the sheet modifier and saving-throw chain, but
does not receive geometric cover; the cast profile has no cover-exception flag.
This establishes neither general cover bonuses nor Sacred Flame's explicit
exception. Visibility/total-cover acceptance remains unverified here. Keep those
rules, higher-level scaling and #1582 open before claiming full Sacred Flame support.


## Cure Wounds development (2026-09-12)

Implemented against main `fd4cdade`, with local module overrides for the combined
walk. No overrides belong in the publication commits.

The root module declares immediate healing on CastProfile, compiles first-level
Cure Wounds with its existing action/slot price, and provides reusable sourced
healing arithmetic. Character.CastDefinition binds the class casting ability and
Life Domain's feature-owned contribution from persisted class/subclass facts.
Healing is never stored as a condition. Monster family can be read from catalogue
identity on older sheets without changing their serialized shape; custom types
survive load/save. Recipient handlers refuse negative incoming healing.

Encounter exposes value-only boundary reads from its live, read-only canvas.
Resolution preflights healing recipients and physical touch, then rolls and
publishes through the existing Gather and healing collector after payment.
Session projects provider target answers and existing healing result records.
Known targets need not currently be seen, and the caster is selectable.

Acceptance now covers sourced ordinary/Life healing, self and another creature,
clamping/full HP, dying/stabilized recovery, paid no-effect undead/constructs,
negative sums, stale access/modifiers, exhausted costs, physical barriers,
darkness, concentration preservation, slot recovery and exact JSON/story reload.
Dice failures persist no changes; encounter-save failures can leave character
writes durable and report that partial state. Stream delivery failure leaves a
recoverable story. A late bus subscriber failure does not undo a live HP write.

The full session, resolution and encounter suites pass in the combined local
workspace. Their fixture readers normalize CRLF only before test string edits;
no game data or production line-ending behavior is normalized. The full root suite also passes independently with `GOWORK=off`; an initial
Windows Application Control block did not recur on the independent run.
Consumer validation against pushed pins and CI evidence follow publication;
local overrides are not the release graph.

### Resolution follow-up after provider releases (2026-09-12)

PR #1695 now consumes rulebook v0.159.0 and encounter v0.76.0. The user corrected
the missing-type policy: an untyped monster remains selectable and receives
healing; only a known matching type triggers the spell's no-effect exclusion.
Regression cases cover missing refs/types, unknown custom refs, explicit and
catalogue undead/construct types, target projection, payment, HP output, and
concentration preservation. The full resolution suite and vet pass with the
released pins and no workspace override. At that resolution handoff, session
#1696 still needed its released resolution pin and untyped-monster acceptance;
those are completed by the follow-up below.

### Session follow-up after resolution release (2026-09-12)

PR #1696 consumes rulebook v0.159.0, encounter v0.76.0, and resolution v0.44.0.
Its target-query call no longer supplies exclusions, because resolution owns
the no-effect decision at cast delivery. Tests verify an untyped custom monster
is selectable and receives healing with one action and one slot spent. Both a
missing ref and an unknown custom ref are covered. Session/encounter/character
records are JSON-round-tripped and a fresh manager replays the exact healing
story with dice disabled, retaining healed monster HP, the spent slot, and the
original missing classification. The full session suite, vet, and tidy checks
pass against released providers without local overrides. Merge/release of the
session PR remains pending; no API/protos/web adoption or pins were changed.

See [healing-rules.md](healing-rules.md) for rules sources, model decisions and
explicit scope limits. Preparation, automatic grants, upcasting, out-of-combat
casting and external repository pins remain deferred.

### Healing Word: same-turn rule provider (2026-09-12)

The user chose to ship the 2014 bonus-action casting restriction with Healing
Word. The first provider checkpoint adds a pure spell-turn rule in `combat`
and atomic `CanPaySpell`/`PaySpell` operations on Character. Casting history is
stored with action economy, using an independent explicit turn identity. A
resource refresh preserves that history; querying or paying on another turn
does not refresh resources. Successful free casts also mark history dirty.

Combat tests pass for both casting orders, the one-action cantrip exception,
reaction spells, bonus-action cantrips, different turn identities, JSON history,
and invalid declarations. Static checks pass for combat and character. Character
payment/persistence tests compile, but Windows Application Control blocks their
execution; Linux CI is required. Formatting and module tidy introduce no source
changes outside this slice and no dependency changes.

This is a provider checkpoint, not live Healing Word support. Spell content,
actual clock identity wiring, ranged healing, session offers and execution
remain to be implemented and verified in the sequence in `plan.md`. No consumer
pins or external repositories have been changed.

### Healing Word content provider (after #1700)

Built on main `412f3dc7` / rulebook `v0.161.0`. Healing Word compiles to a
bonus-action, first-level-slot price and single-creature 60-foot healing profile.
Existing character compilation supplies Wisdom/Charisma and Disciple of Life
modifiers, with the same exclusions as Cure Wounds. No acquisition list changed.

Compiled spells carry explicit level and casting time, separately from price.
Tests cover all eleven executable spells, classification passed to the shared
same-turn rule, ranged healing profile validation, JSON/clone isolation, and
ordinary Cleric, Life Cleric, and Bard healing modifiers. Existing Bard cantrips
remain executable even where the older spell-data catalog has no entry.

Affected spells, actions, character and choices tests pass locally, as do vet
and module tidy. Full module/CI evidence is recorded on the provider PR. This
is content support only; live target validation, spell payment wiring and
session integration remain pending in the sequence in `plan.md`.

### Healing Word resolution integration (after #1701)

Resolution consumes rulebook `v0.162.0` and retains encounter `v0.76.0`.
Costed cast payment now requires profile classification, a matching caster/payer,
and an explicit spell-turn identity. Existing cast fixtures were updated to
supply those facts. Ordinary action payment remains unchanged.

The full resolution suite and vet pass locally. New integration cases cover
both casting orders with JSON reload, one-action cantrips, missing declarations,
free-spell history, same-turn reactions, turn changes without reaction refresh,
failed-payment history preservation, ranged dying recovery, self/unknown-type
target projection, and stale sight/wall/range refusal before payment or RNG.
The existing Cure Wounds and Bard suites remain green. Linux CI evidence is
recorded on the resolution PR.

Session is still pinned to the old resolution release. Its next PR must update
casting plumbing and its pin together, as the user explicitly authorized, and
prove the actual clock and persistence flow before enabling Healing Word.
