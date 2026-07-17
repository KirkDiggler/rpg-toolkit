package character

import (
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/classes"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/fightingstyles"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
)

// StructurallyPermanentConditionRefs returns the canonical ref strings
// (e.g. "dnd5e:conditions:martial_arts") for every condition Draft.Finalize
// attaches at character creation — every class's Grant.Conditions entry,
// at any level, plus every fighting-style choice — as opposed to a
// condition activated live via Encounter.ActivateFeature (rpg-toolkit#778).
//
// WHY THIS SET MATTERS: these conditions are structurally never
// reachable by the live broker ConditionAppliedEvent path.
// Encounter.ActivateFeature is the encounter package's ONLY bridge from a
// dnd5e-level ConditionAppliedEvent to a broker events.ConditionAppliedEvent,
// and it installs its capture subscriber in a narrow, call-scoped window
// (subscribe, call ActivateAbility, capture, unsubscribe). Draft.Finalize
// publishes ConditionAppliedEvent for every Grant.Conditions entry and
// fighting-style choice too — Character.subscribeToEvents' own
// onConditionApplied handler is what actually appends the condition to
// c.conditions, for BOTH paths uniformly — but Finalize runs at character
// creation, before the character belongs to any encounter, on a bus no
// encounter-level bridge subscriber will ever be attached to. So a
// reconnecting client's live event stream was ALWAYS going to miss these;
// excluding them from a snapshot projection (encounter's ActiveConditions)
// keeps the snapshot and the live stream describing the same world instead
// of the snapshot being a strict superset.
//
// STRUCTURAL INVARIANT THIS DEPENDS ON — read before adding a new
// Grant.Conditions entry or a new live-activatable condition:
// conditions.CreateFromRef (the factory compileConditions calls to build
// every Grant.Conditions entry into a live ConditionBehavior) has exactly
// ONE caller in this rulebook: compileConditions itself. Every genuinely
// live-activated condition (e.g. RagingCondition) is constructed directly
// by its own feature's activation code, never through CreateFromRef. This
// function's correctness depends on that separation holding: if some
// future condition becomes reachable through BOTH a Grant.Conditions entry
// AND a live ActivateFeature call, this function would still exclude it
// from ActiveConditions even on the encounters where it WAS genuinely live
// -activated and IS announced on the broker stream — the fix at that point
// is a real per-instance provenance marker (rpg-toolkit#778's option (b)),
// not this static derivation.
//
// Derived from classes.ClassData + classes.GetGrants + fightingstyles.All()
// rather than a hand-maintained literal, so a newly migrated class's
// Grant.Conditions (or a newly added fighting style) is picked up
// automatically instead of silently missing from the exclusion set —
// exactly the "someone forgot to update a list" failure mode
// rpg-toolkit#767 was about, applied to this new list. Walks
// classes.ClassData (all 12 PHB classes, not classes.All, which is
// deprecated) rather than a hand-maintained class-ID list; GetGrants
// already no-ops (returns nil) for any class not yet migrated to the grant
// system, so unmigrated classes correctly contribute nothing.
func StructurallyPermanentConditionRefs() []string {
	seen := make(map[string]struct{})
	var out []string
	add := func(ref string) {
		if ref == "" {
			return
		}
		if _, ok := seen[ref]; ok {
			return
		}
		seen[ref] = struct{}{}
		out = append(out, ref)
	}

	for classID := range classes.ClassData {
		for _, grant := range classes.GetGrants(classID) {
			for _, condRef := range grant.Conditions {
				add(condRef.Ref)
			}
		}
	}
	for _, style := range fightingstyles.All() {
		if ref := fightingStyleConditionRef(style); ref != nil {
			add(ref.String())
		}
	}

	sort.Strings(out)
	return out
}

// fightingStyleConditionRef returns the canonical condition ref for a
// fighting style, mirroring createFightingStyleCondition's (draft.go)
// dispatch shape — the two must stay in sync: a style createFightingStyleCondition
// knows how to construct a condition for should also have a case here.
func fightingStyleConditionRef(style fightingstyles.FightingStyle) *core.Ref {
	switch style {
	case fightingstyles.Archery:
		return refs.Conditions.FightingStyleArchery()
	case fightingstyles.Defense:
		return refs.Conditions.FightingStyleDefense()
	case fightingstyles.Dueling:
		return refs.Conditions.FightingStyleDueling()
	case fightingstyles.GreatWeaponFighting:
		return refs.Conditions.FightingStyleGreatWeaponFighting()
	case fightingstyles.Protection:
		return refs.Conditions.FightingStyleProtection()
	case fightingstyles.TwoWeaponFighting:
		return refs.Conditions.FightingStyleTwoWeaponFighting()
	default:
		return nil
	}
}
