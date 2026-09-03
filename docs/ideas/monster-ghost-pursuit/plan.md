# Monster Ghost Pursuit Implementation Plan

**Status:** Completed. The observed delivery, mainline integration releases,
and verification evidence are in [implementation.md](implementation.md).

**Goal:** Let a fight-time monster pursue the closest reachable remembered
player position when nobody is visible, correct that position to explicitly
unknown on arrival, and stop pursuing the resolved ghost without receiving
concealed world truth.

**Architecture:** Encounter owns the backward-compatible tagged location
payload, held-known view projection, and arrival correction through opaque
`intel.Report`. `behavior.Basic` is visible-first and uses remembered positions
only as a fallback. Session mirrors the additive view and delta contracts while
preserving load-act-save persistence. `play/intel` does not change.

**Spec:** [design.md](design.md)

## Locked constraints

- [SRD 5.1](https://media.wizards.com/2016/downloads/DND/SRD-OGL_V5.1.pdf#page=76)
  is authoritative; [Roll20 2024](https://roll20.net/compendium/dnd5e/Free%20Basic%20Rules%20%282024%29)
  material is clarification/reference only.
- `play/intel` remains payload-opaque and gains no geometry, truth access, or
  position API.
- `MonsterView.Seen` remains current-only; remembered positions are additive.
- A remembered target cannot be attacked, contains no hidden standing state,
  and has a path ending at the exact remembered cell.
- Visible standing targets always take priority. The nearest visible target
  retains the visible branch even if that branch passes; remembered fallback
  applies only when no visible standing target exists.
- Among reachable remembered targets, equal distances temporarily break by
  subject ID. Multi-target sentience remains deferred.
- Correction occurs only after a driven fight-time monster reaches the exact
  remembered cell. Public `Step`, free-roam `Pump`, and correction merely from
  viewing a remembered cell remain deferred.
- A newly visible player interrupts remembered pursuit on the next driver call.
- Intel corrections are encounter-owned deltas and persist; they are never
  silent mutations.
- No committed `replace`, `go.work`, or `go.work.sum` is permitted.

## Delivered sequence

1. **Strict encounter location testimony.** Added tagged
   `Known(position) | Unknown`, legacy coordinate decoding, and strict load
   validation. `(0,0)` remains valid; malformed, contradictory, and current
   unknown Sight testimony is rejected.
2. **Encounter-owned Intel deltas.** Replaced raw `intel.SurveilOutput` output
   projections with `IntelDelta{FirstContact, Refreshed, Faded, Corrected}` and
   propagated them through enclosing driven paths.
3. **Remembered turn view.** Projected held-known positions into a separate
   `MonsterView.Remembered` collection with shortest exact-cell paths. Held
   unknown remains persisted but produces no actionable entry.
4. **Arrival correction.** After a driven move, encounter uses the mover's own
   held Intel, arrival cell, and complete refreshed percept to report held
   unknown corrections for absent exact-cell subjects. It never reads a hidden
   live position.
5. **Behavior fallback.** Preserved nearest-visible selection and added nearest
   reachable remembered movement only when no visible standing player exists.
6. **Session projection and persistence.** Mirrored remembered data and
   correction identifiers by value; save failure rolls back correction and
   publishes nothing.
7. **End-to-end proof.** Added the double-door pursuit/correction proof and a
   visible-target interruption proof through the real session seam.
8. **Sources of truth.** Recorded the decision in
   [ADR-0047](../../adr/0047-encounter-owns-location-knowledge.md), reconciled
   ADR-0043 status, and recorded observed release evidence.

## API and compatibility decisions

- Canonical known JSON is `{"state":"known","x":0,"y":0}`; unknown is
  `{"state":"unknown"}`. Legacy `{"x":0,"y":0}` remains known input.
- `DecodeSightPayload` is the known-position compatibility wrapper; state-aware
  code uses `DecodeLocationPayload`.
- Session projects explicit location state for sight holdings and rejects
  malformed sight testimony before projection rather than treating it as
  unknown.
- Correction uses Sight, the mover as observer, and clock high-water while
  preserving held status.
- The scoped Session projection uses flat `[]IntelCorrection`; the full
  `IntelDelta` → `Discovery`, `EndTurnOutput.Discovered`, and converter
  field-audit question remains deferred.

## Proof obligations and remaining scope

The session double-door fixture proves: sight at carpet; sight loss behind the
second door; held known-at-carpet without hidden truth; exact-cell pursuit;
held-unknown correction at lawful arrival; no repeated pursuit. The companion
proof makes another player visible during remembered movement and verifies that
the visible branch wins next call.

Not included: route-utility scoring; persistent commitment; correction on
observation instead of arrival; freshness/reliability policy; hiding,
invisibility, sound, deception, search after unknown position, or general
planning. `rpg-project#305` and PR #306 remain design input, not toolkit
authority. The local-game observation named in [issue-201-plan-update.md](issue-201-plan-update.md)
remains before rpg-project#201 closes.
