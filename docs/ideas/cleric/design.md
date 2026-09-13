# Cleric level-one contribution assessment

## Bless delivery decision

Recording handoff: encounter accepts the supplied per-target miss fact and
emits `cast_missed` in target order, carrying only actor, target and spell.
It never decides whether a spell missed. Missed recipients cannot also carry
a save or delivered effects; transaction validation rejects that contradiction
before appending anything. Session will project resolution's miss onto this
record contract when it adopts the released encounter provider.

Bless uses the existing gateless condition delivery, contributed-dice selection,
and source-qualified concentration ownership. It introduces a known-creature
target contract: self and conscious/dying/stabilized creatures, with a known
location in range and a clear path, without requiring current sight. Creature
type does not exclude Bless recipients.

The host explicitly chooses the consequence of outdated location testimony.
`StaleTargetRefuse` rejects the entire cast before payment; `StaleTargetAttempt`
pays once, delivers to recipients still at their remembered locations, and
reports a per-target miss otherwise. These options were requested by the user;
neither is a silent default. Location knowledge remains the encounter's
testimony; resolution interprets those facts without substituting hidden live
positions. Session must carry the same policy through offers and execution and
preserve misses in its record. This policy is separate from the spell's effect.

Status: creation merged in toolkit #1585 on 2026-09-08, released as D&D v0.150.0.
The gameplay milestone remains open. The refreshed [plan.md](plan.md) is based
on merged toolkit main `fd4cdade` (2026-09-12) and supersedes the historical dependency
assumptions below. Sacred Flame content and session acceptance are merged. The
user has authorized the bounded combat Cure Wounds + Life healing slice proposed
in toolkit #1643. Results are in [implementation.md](implementation.md).
No upstream adoption or live-play support is claimed. Acceptance checks are in
[plan.md](plan.md); dated rules evidence is in [source-index.json](source-index.json).

Proposal tracking: [rpg-project#406 — Play a Level-One Life Cleric (2014)](https://github.com/KirkDiggler/rpg-project/issues/406),
opened at the user's request on 2026-09-08 as a proposed Shaping journey linked
to #243. Opening the issue does not establish adoption or implementation ownership.

## Original assessment (historical baseline: `feb8e2da`)

The sections below retain the original assessment and source boundaries.
Current implementation tasks and consumer release status are in [plan.md](plan.md).

## Recommendation

Contribute a 2014-rules level-one Cleric with Life Domain, built on the current
session/resolution architecture. Treat this as several reviewable contributions.
The first cleric-specific contribution should repair creation and finalization;
the first shared casting proof should be one cantrip under the existing casting
journey. A playable Life cleric then needs preparation, live slots, healing,
domain behavior, and concentration.

I'm making an assumption here that the contribution should follow this
rulebook's existing 2014 class model. The domain-at-level-one data and upstream
SRD 5.1 initiative support that choice. A 2024 cleric would need a separate
edition decision, not substitutions inside this implementation.

Life is the proposed domain because its level-one mechanics exercise ordinary
proficiency grants, always-prepared spells, and a healing modifier. Its Bless
grant also makes concentration an explicit dependency. It is not a shortcut
around the shared magic work.

## Current code versus the advertised choices

These are findings from code and existing tests, not a newly executed runtime
proof. Paths are relative to the repository root.

| Area | Evidence on reviewed main | Consequence |
| --- | --- | --- |
| Class identity | `classes/data.go` contains Cleric stats, skill pool, casting ability, slots, and domains under `rulebooks/dnd5e/` | Reuse the identity; an enum or class row is not the missing contribution. |
| Class grants | `classes/grant.go:GetGrants` handles five classes, excluding Cleric; `classes/starting_equipment_test.go:TestUnmigratedClassesGrantNothingAtAll` deliberately asserts nil grants | Cleric needs a complete migration of its level-one grants. An equipment-only row would violate the test's stated intent. |
| Finalization | `character/draft.go:compileProficiencies`, `compileInventory`, and `compileFeatures` consume class grants | Cleric class armor/weapon proficiencies, fixed shield, and features do not arrive through that path. Race/background contributions may still appear. |
| Choice validation | `character/choices/requirements.go:getClericRequirements`, `validation.go`, and `class_comprehensive_test.go` already cover requirements and generic validation | Reconcile remaining gaps with the real validator rather than adding the older issue's proposed parallel class validator. |
| Domain data | `character/choices/subclass_modifications.go` already contains Life heavy armor and domain spell tables | Corrects the older tracker: the lists exist. `ApplySubclassModifications` changes choices but does not apply automatic grants to the finished sheet; finalization does not consume those fields. |
| Known spells | `character/draft.go:compileKnownSpells` converts selected spells/cantrips to canonical refs; `character/data.go` and `known_spells_test.go` cover persistence | The old claim that these fields do not exist is stale. Selection storage does not supply prepared-spell management, automatic domain grants, or source-aware casting semantics. |
| Slots | `character/draft.go:compileSpellSlots` still creates Cleric slot data; its comment explicitly describes the absent casting path | Slot numbers are persisted but not usable magic. Do not present them as playable support. |
| Spell catalog | `spells/data.go:Data` stores ID, level, name, and description; `refs/spells.go` supplies refs | Descriptions cannot supply executable range, components, targeting, effects, or concentration rules. |
| Life behavior | No Disciple of Life implementation found in the rulebook search | A rulebook-owned healing contribution and its persistence/attachment proof are needed. |

### Creation details needing explicit checks

- Base equipment offers warhammer and chain mail with descriptive proficiency
  caveats. The generic equipment validator checks selections, not those caveats.
  Verify legality using the character's combined proficiency sources. A Life
  cleric can gain a weapon proficiency elsewhere; do not hardcode domain alone.
- Life adds another chain-mail option. Resolve duplicate choices without
  duplicating inventory, and preserve any stored choice identity deliberately.
- `ClericSecondaryShortbow` is a misleading constant name: the current payload
  is correctly a light crossbow with bolts. Do not report a wrong weapon bug
  merely from that name.
- The cantrip options include Toll the Dead and Word of Radiance, omit Mending,
  and are not an independently verified SRD-only list. Define the allowed source
  boundary and audit the complete expected list before claiming catalog coverage.
- Domain choice changes must replace dependent choices/grants; round-tripping
  must not duplicate armor, equipment, spells, or features.

## Upstream reconciliation

GitHub issue bodies/comments and open toolkit PRs were read on 2026-09-08.
This is a dated snapshot, not an ownership claim or a full Project 19 board audit.

| Tracker | Observed state | How this proposal relates |
| --- | --- | --- |
| [rpg-project#243: Cast a Spell in Play](https://github.com/KirkDiggler/rpg-project/issues/243) | Open; body says unadopted Shaping, one at-will cantrip, slot architecture undecided | Parent context for shared casting. Propose Sacred Flame as a candidate; do not invent a competing casting framework. |
| [rpg-toolkit#799: orphaned slots](https://github.com/KirkDiggler/rpg-toolkit/issues/799) | Open, with owner confirmation of the gap | Must be resolved for leveled spells. Resource migration is a proposal, not an accepted slot contract. |
| [rpg-toolkit#299: domain spells](https://github.com/KirkDiggler/rpg-toolkit/issues/299) | Open; body says tables are absent | Tables are now present; integration and always-prepared semantics remain. Do not recreate the tables from the issue text. |
| [rpg-toolkit#271: cleric validation](https://github.com/KirkDiggler/rpg-toolkit/issues/271) | Open; names an older class-validator path | Generic validation exists. Prove remaining creation gaps before replacing or closing this tracker. |
| [rpg-toolkit#146: known spells](https://github.com/KirkDiggler/rpg-toolkit/issues/146) | Open; body predates current fields | Persistence is partly implemented. Preparation, domain grants, source identity, and coverage still need reconciliation. |
| [rpg-toolkit#431: combat spellcasting](https://github.com/KirkDiggler/rpg-toolkit/issues/431) | Open; #243 identifies its older architecture | Historical scope, not an implementation blueprint. |
| [rpg-project#231: four-player level-three dungeon](https://github.com/KirkDiggler/rpg-project/issues/231) | Open; explicitly excludes spellcasting | Cleric is a proposed additional effort, not an already adopted requirement of that initiative. |

The open toolkit PR listing contained only #1395 (monster pursuit release
documentation), with no cleric/casting PR. This does not exclude unpublished
branches or another contributor's plans. The initial audit made no upstream
changes; the user subsequently requested proposal issue #406. No PR was opened.

## Cross-repository contract handoff

The [rpg-api-protos member role](https://github.com/KirkDiggler/rpg-project/blob/main/docs/teams/roles/rpg-api-protos-member/prompt.md),
read on 2026-09-08, clarifies how the later consumer work should be divided.
It is role/process guidance, not evidence that a particular casting message
already exists or that this contribution has been assigned that role.

Toolkit owns preparation legality, casting costs, domain behavior, healing,
and effect outcomes. Protos owns the wire representation of the references,
choices, and computed results that API and web actually need. API handles
host orchestration; web presents the authoritative choices and results.
Do not make consumers calculate a cleric rule from a formula in a message.

Before proposing a new RPC or message, inventory the current proto definitions
and their actual API/web uses. Present the protos owner with one concrete
casting example: the request, authoritative outcome, changed state, and failure
case. Identify which existing messages suffice and cite any specific missing
field. Neither a new Cleric service nor a comprehensive spell-management API is
justified by this assessment alone.

Any required contract change needs a named service/version package, reuse of
existing message shapes, synchronized generated Go/TypeScript SDKs, and an
explicit consumer migration. Follow the role's compatibility checks and version
policy; do not leave duplicate old/new representations after migration. Its
release discipline includes a bounded transition and deliberate per-service
versioning for stable-contract breaks, not blanket permission to remove fields.

The initial toolkit creation contribution does not yet demonstrate a need to
change protos. Contract work is a separately scoped follow-up once a real
consumer example exposes a gap. Provider checks and director-owned integrated
playtest/sign-off remain separate completion claims.

## Reusing tabletopdnd lessons

Read-only reference: tabletopdnd's `PROJECT-HANDOFF.md`, `AGENTS.md`,
`cleric-2024.js`, and `cleric-progression-2024.js`. Its guided cleric progression
covers 1 to 5 with Trickery; its handoff does not claim guided Cleric creation.

Transfer the contracts: independently sourced expected content; distinct grant
origins; explicit preparation/replacement timing; spent-resource preservation;
and clear separation between content, tested behavior, and live support.
Translate those into toolkit Go tests and typed data. Tabletopdnd's application
save/retry/ledger workflow is useful test inspiration, not a new persistence
layer to add to this engine.

Do not copy 2024 preparation counts, domain timing, Channel Divinity rules, or
spell formulas into the 2014 module. The saved source records below identify
the edition of the proposed rules. Full spell-list and general casting-rule
reviews remain a named follow-up, not an implied completed audit.

## Proposed milestone and dependency boundaries

The level-one Life milestone should prove creation, reload, spell preparation,
one at-will save cantrip, domain healing, Bless, slot exhaustion, and recovery.
It is an initial supported spell subset, not complete level-one Cleric support.
Unimplemented valid spell choices must remain distinguishable from executable
ones; do not shrink the source list to make coverage appear complete.

Proposed sequence: creation grants; Sacred Flame casting proof; preparation and
slots; Cure Wounds with Disciple of Life; Bless with concentration. These are
candidate merge boundaries, not adopted issues or a promise that each fits one
PR. Full level-one support additionally requires the complete agreed spell
catalog, ritual casting, component/focus rules, and remaining spell behaviors.
Channel Divinity, leveling, other domains, multiclassing, and 2024 rules are
outside this milestone.

The shared casting decision must address:

- Typed action declarations authored by rules and interpreted by machines,
  following [ADR-0045](../../adr/0045-actions-are-data.md). Add profile arms only
  when an implemented machine needs them; a save cantrip is not a weapon Strike.
- The per-interaction bus and dirty participant results in
  [ADR-0038](../../adr/0038-resolution-owns-the-bus.md) and
  `rulebooks/dnd5e/resolution/doc.go`. The host supplies repositories; it does
  not choose spell legality, healing formulas, or concentration behavior.
- Slot ownership and persisted-data compatibility, including existing
  `SpellSlots`. Avoid a second active balance or a silent reset during loading.
- Separate known cantrips, prepared class choices, and always-prepared domain
  grants, retaining source and casting-ability identity for overlapping grants.
- Preflight before cost/dice/mutation, then one cost payment and authoritative
  results. Define valid no-effect casts separately from invalid declarations;
  an immune target is not automatically an invalid target.
- Concentration source/beneficiary links, duration, interruption, replacement,
  cleanup across participants, and reattachment after persistence.

The older [spell-system journey](../../journey/011-spell-system-design.md) and
[spellcasting journey](../../journey/023-spellcasting-system-design.md) provide
historical questions. Current accepted ADRs and package contracts govern the
implementation shape.

## Assessment limits

Source review covers the level-one class/domain facts and three candidate spell
entries, with saved digests. It does not certify a complete SRD catalog, general
spellcasting/chapter exceptions, concentration rules, or higher-level support.
Source review is complete for this assessment's stated entries; implementation
must review the remaining dependencies before claiming their behavior.

This contribution can start with a focused creation regression and grants fix.
Playable cleric remains a substantial shared magic effort. No reliable calendar
estimate is justified until the casting and slot contracts are settled.
