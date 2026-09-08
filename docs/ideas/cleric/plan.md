# Cleric contribution plan

Status: proposed, 2026-09-08. Companion: [design.md](design.md).

Proposal issue: [rpg-project#406](https://github.com/KirkDiggler/rpg-project/issues/406).
Opened at the user's request before toolkit implementation; awaiting discussion.

## Local creation checkpoint — 2026-09-08

The user authorized the first local creation contribution after opening #406.
Implemented in the first creation contribution:

- Cleric base armor/weapon grants and fixed shield now reach finalization.
- Life's existing heavy-armor data is applied. The legacy Life chain-mail
  choice remains accepted. Recording, completeness, and validation use
  subclass-dependent requirements.
- The existing subclass field reaches validation without a second persisted
  selection. The first regression exposed `Divine Domain required`, an earlier
  blocker than the missing grants.
- Completeness uses main's recorded cantrip/spell choices. Shared validators reject
  duplicate skills and fixed-count selections; regression tests demonstrated
  duplicate skills/cantrips previously finalized successfully.
- Life's conditional warhammer choice checks the same compiled weapon grants.
- Tests cover invalid/missing choices, domain/class replacement, equipment
  alternatives and quantities, background coexistence, and draft/sheet JSON
  round-trips. Reload preserves damaged HP and spent slot data.

This does not add domain spells, spellcasting, preparation, Disciple of Life,
concentration, API/proto changes, or live support. Other domains' automatic
grants and equipment prerequisites remain outside this slice. Redundant
chain-mail option IDs remain accepted for stored-draft compatibility. Existing
race grants do not supply dwarf weapon training, so an actual dwarf-warhammer
creation proof remains a separate race-grant dependency.

Verification:

- Focused cleric and existing class/choice checks passed.
- The ordinary full D&D module suite failed only at the existing appearance
  timestamp test, `TestDraftSetAppearanceValidatesAtomicallyAndPreservesClassCarryover`
  (`appearance_test.go:175`). The baseline before cleric edits failed there too.
- The final full module run passed with exactly that baseline test excluded by
  the command's `-skip` flag. No test was disabled or edited in the repository.
- Formatting/diff checks passed; `go mod tidy` made no dependency changes.
- Race mode reported that it requires cgo; this environment has `CGO_ENABLED=0`.
- `golangci-lint` was not found on PATH or in checked local Go/tools locations.
  Race and lint gates remain outstanding; no toolchain configuration changed.

The contribution is updated onto `e894540e`; main now supplies the spell-choice
completeness handling. Current checks are recorded in [implementation.md](implementation.md).
Shared casting remains a dependency for the later gameplay milestone.

## Assessment deliverable

- [x] Compare current class data, choices, grants, finalization, and persistence.
- [x] Read upstream casting and cleric trackers and check open toolkit PRs.
- [x] Record edition-specific evidence for the level-one scope and three spells.
- [x] Identify reusable tabletopdnd lessons and incompatible 2024 mechanics.
- [x] Propose an initial milestone, dependencies, and discriminating checks.

The assessment is complete; the checkpoint above records subsequent creation
work. The gameplay slices below remain proposed. Keep this plan and design current;
see [implementation.md](implementation.md) for observed implementation results.

## First contribution: make a cleric draft compile correctly

Scope: the existing D&D rulebook module's level-one class grants, Life armor,
fixed equipment, and applicable choice legality. Prepared magic and the Life
healing behavior remain explicit dependencies; do not call this full migration
or playable Cleric while those are absent. Any change to the existing
unmigrated-class assertion must document precisely the newly supported scope.

1. Recheck main and the named upstream trackers. Agree the contribution boundary
   with the maintainer before describing it upstream as adopted; no external
   messages beyond the user-requested proposal issue #406 are authorized by this
   local assessment.
2. Write a Human/Life Cleric finalization regression that exposes missing class
   armor/weapon grants and the fixed shield. Use independently stated expected
   results, not values computed from the same grant table under test.
3. Prove the selected subclass is retained and its automatic armor grant reaches
   the finished character. Reuse a single authoritative grant path for both
   legality and compilation; avoid a second Life-specific proficiency table.
4. Cover ordinary and category equipment choices, ammunition quantity, pack
   contents, holy symbol, and background coexistence. Test prerequisite choices
   with and without proficiency from another source.
5. Save/load the draft and final character; verify no lost/duplicate grants.
   Exercise class/domain changes before finalization, and invalid/missing skills,
   cantrips, domain, and equipment through the public draft API.

Completion claim: correctly compiled class creation data for the documented
subset. Report outstanding domain spell/feature work plainly.

## Shared casting proof: candidate Sacred Flame

Connect this design to [rpg-project#243](https://github.com/KirkDiggler/rpg-project/issues/243).
Reconcile a save-damage profile with the current action-definition and save
machine contracts. Do not design the complete caster API to prove one cantrip.

Acceptance checks:

- A known, supported cantrip is offered as a server-authored declaration; a
  forged/unknown declaration cannot execute.
- Range, visibility, and casting prerequisites are validated before payment or
  rolling. Review the general targeting/component rules before implementation.
- Deterministic failing and successful saves distinguish damage from no damage;
  the cover exception is tested without treating it as blanket permission to
  target through obstacles.
- One action is spent; no spell slot is required or consumed. Rejection leaves
  sheets unchanged. Results and dirty sheets survive the session save/load path.
- Toolkit tests establish the provider behavior. Closing the cross-repository
  journey additionally needs API/web work and an actual observed game result;
  those consumer changes are outside this assessment.

## Level-one Life milestone after the cantrip proof

| Candidate slice | Acceptance evidence | Dependency decision |
| --- | --- | --- |
| Preparation and slots | Prepared choices and domain grants remain distinct after reload; overlap does not charge preparation twice; known cantrips are not prepared spells; slot exhaustion rejects without side effects; rest recovers the intended balance; loading does not restore spent slots | Adopt a slot representation and migration policy for #799; review preparation/rest rules and the complete source-bounded spell list |
| Cure Wounds and Disciple of Life | Domain grant is usable without selecting it as a prepared class choice; deterministic healing exposes base roll, ability, and domain contribution; ordinary healing does not receive the spell-only bonus; HP and slot changes persist together; maximum HP and downed revival behave correctly | Add the minimal healing profile/machine and a typed spell-healing context; review healing/death interactions; separately test valid no-effect creature types |
| Bless and concentration | Both domain spells are usable; selected targets receive attack/save contributions, not ability-check bonuses; recipients retain effects after reload; source cancellation, replacement, expiry, and broken concentration remove benefits; cost is paid once for the whole cast | Review general concentration and same-spell stacking rules; decide source links and multi-participant persistence |

Do not reduce the milestone to healing alone: Bless is a level-one domain grant
and brings a real concentration dependency. A staged healing demonstration is
useful, but must be named as partial support.

## Validation and publishing boundaries

Before API/web integration, prepare a contract handoff following the
[protos role guidance](https://github.com/KirkDiggler/rpg-project/blob/main/docs/teams/roles/rpg-api-protos-member/prompt.md):

1. Show one executable toolkit casting example, its rejected-input example,
   and the authoritative state/results the consumer needs.
2. Map that example to existing proto messages and API/web uses with file/line
   evidence. Propose only demonstrated gaps; keep formulas and class branching
   in toolkit. No proto inventory has been completed in this assessment.
3. If a contract change is needed, scope its service/version package and
   compatibility decision with the protos owner. Regenerate and commit matching
   Go/TypeScript SDKs, update affected docs, and migrate consumers to the released
   version without permanent duplicate representations.
4. Require the protos repository's `make test`, `buf lint`, `buf breaking`, and
   self-review gates for that implementation. These are future contract checks,
   not checks required for or executed by this documentation-only change.
5. Report provider and consumer evidence separately. Under that team workflow,
   the director owns the integrated playtest, wave closure, and merge/sign-off;
   do not mark those complete from the toolkit contribution.

For this documentation change: validate source record schema/digests, internal
links, whitespace, and the changed-file scope. No gameplay test results are
claimed from reading existing tests.

For implementation: use focused testify suites for the affected behavior, then
the D&D module's required formatting, tidy, lint, and regression checks before
committing. Read the Windows Go hygiene skill and module test guide at that
point. Do not run dependency updates or tests from an assumed repository-wide
module root.

If a provider module must change, split module contributions and publish
inside-out. Consumer pins use CI-minted versions. Local overrides may aid
development but must not be committed. Update relevant status documentation as
behavior changes; do not mark other modules or the live game supported from a
toolkit test alone.

Next bounded action: implement the first creation regression and use its result
to define the smallest honest grants contribution. Revisit the shared casting
contract before adding runtime spell behavior.
