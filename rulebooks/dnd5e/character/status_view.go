// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package character

import (
	"sort"

	"github.com/KirkDiggler/rpg-toolkit/core"
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
	"github.com/KirkDiggler/rpg-toolkit/rpgerr"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	dnd5eEvents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/features"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/resources"
)

// HitPointView is the immutable projection of the character's hit points.
type HitPointView struct {
	// Current is the character's current hit points.
	Current int

	// Maximum is the character's maximum hit points.
	Maximum int
}

// FeatureView is the immutable projection of one character feature: its
// canonical ref, never-empty display name, optional server-authored detail,
// and an optional owned resource key. It is a detached value — mutating the
// sheet after a StatusView call does not change a previously returned view.
type FeatureView struct {
	// Ref is the feature's canonical ref.
	Ref core.Ref

	// Name is the feature's display name. Never empty.
	Name string

	// Detail is optional, server/toolkit-composed display text. May be empty.
	Detail string

	// ResourceKey is the stable key of the resource this feature owns or
	// shares, or nil for a feature that owns no resource (Reckless Attack,
	// Deflect Missiles). A pointer so nil is distinguishable from an empty
	// key.
	ResourceKey *coreResources.ResourceKey
}

// ConditionView is the immutable projection of one active condition: its
// canonical ref, never-empty display name, optional detail, and an optional
// source member. The source member is nil for self-owned class conditions
// (fighting styles, Unarmored Defense, Martial Arts, Sneak Attack) and is
// populated when a condition names an external party member as its source.
type ConditionView struct {
	// Ref is the condition's canonical ref.
	Ref core.Ref

	// Name is the condition's display name. Never empty.
	Name string

	// Detail is optional, server/toolkit-composed display text. May be empty.
	Detail string

	// SourceMember is the optional party-member ID a condition names as its
	// source. Nil for self-owned class conditions.
	SourceMember *string
}

// ResourceView is the immutable projection of one owned resource: its stable
// key, rulebook-owned display name, and current/maximum counts.
type ResourceView struct {
	// Key is the stable, opaque core/resources.ResourceKey.
	Key coreResources.ResourceKey

	// Name is the rulebook-owned display name (resources.DisplayName). Never
	// empty for a known key.
	Name string

	// Current is the currently available amount.
	Current int

	// Maximum is the resource's capacity.
	Maximum int
}

// StatusView is the immutable, no-magic projection of a character's status:
// level, hit points, base speed, features, conditions, and resources. It is
// built from the live sheet and feature Status reports — never from
// persistence JSON — and excludes spell slots and legacy class resources by
// construction.
type StatusView struct {
	// Level is the character's level.
	Level int

	// HitPoints is the character's current/maximum hit points.
	HitPoints HitPointView

	// BaseSpeedFeet is the character's base walking speed in feet, before
	// condition modifiers.
	BaseSpeedFeet int

	// Features is the character's features, sorted by Ref.String().
	Features []FeatureView

	// Conditions is the character's active conditions, sorted by Ref.String().
	Conditions []ConditionView

	// Resources is the character's owned resources (owner-owned plus
	// feature-private), sorted by key.
	Resources []ResourceView
}

// StatusViewInput is the input to (*Character).StatusView. Reserved for
// future, server-authored detail; today it carries nothing the projection
// cannot derive from the sheet itself.
type StatusViewInput struct{}

// StatusViewOutput wraps the projected StatusView. A pointer so a future
// caller can tell "projection failed" (nil) from "projection succeeded with
// an empty view" (non-nil).
type StatusViewOutput struct {
	// View is the projected status. Never nil when err is nil.
	View *StatusView
}

// StatusView projects the character's immutable, no-magic status: level, hit
// points, base speed, features, conditions, and resources. It reads the live
// sheet and each feature's non-mutating Status surface — never persistence
// JSON — and never publishes. Every validation failure (unknown condition ref,
// malformed feature status, conflicting duplicate resource key, negative or
// over-maximum resource) returns an error with no partial output.
func (c *Character) StatusView(_ *StatusViewInput) (*StatusViewOutput, error) {
	featureViews, featureReports, err := c.projectFeatures()
	if err != nil {
		return nil, err
	}

	conditionViews, err := c.projectConditions()
	if err != nil {
		return nil, err
	}

	ownerRows := c.ownerResourceReports()
	resourceViews, err := mergeResources(ownerRows, featureReports)
	if err != nil {
		return nil, err
	}

	view := &StatusView{
		Level:         c.level,
		HitPoints:     HitPointView{Current: c.hitPoints, Maximum: c.maxHitPoints},
		BaseSpeedFeet: c.GetSpeed(),
		Features:      featureViews,
		Conditions:    conditionViews,
		Resources:     resourceViews,
	}
	return &StatusViewOutput{View: view}, nil
}

// projectFeatures builds the sorted FeatureView slice and the parallel
// resource-report slice from a single Status call per feature. A feature
// whose Status returns an error, or whose report has an empty ref or name,
// fails the whole projection.
func (c *Character) projectFeatures() ([]FeatureView, []resourceReport, error) {
	views := make([]FeatureView, 0, len(c.features))
	reports := make([]resourceReport, 0, len(c.features))

	for _, f := range c.features {
		out, err := f.Status(&features.StatusInput{Owner: c})
		if err != nil {
			return nil, nil, rpgerr.Wrapf(err, "feature %s reported malformed status", featureID(f))
		}
		if out == nil || out.Status == nil {
			return nil, nil, rpgerr.Newf(rpgerr.CodeInvalidArgument,
				"feature %s reported no status", featureID(f))
		}
		st := out.Status
		if st.Ref == (core.Ref{}) {
			return nil, nil, rpgerr.Newf(rpgerr.CodeInvalidArgument,
				"feature %s reported an empty ref", featureID(f))
		}
		if st.Name == "" {
			return nil, nil, rpgerr.Newf(rpgerr.CodeInvalidArgument,
				"feature %s reported an empty name", featureID(f))
		}

		var key *coreResources.ResourceKey
		if st.Resource != nil {
			k := st.Resource.Key
			key = &k
			// Validate the feature's own counts before merging so a private
			// resource (Second Wind, Action Surge) is rejected here rather than
			// silently merged with bad values.
			if err := validateReport(resourceReport{
				Key: st.Resource.Key, Current: st.Resource.Current, Maximum: st.Resource.Maximum,
			}); err != nil {
				return nil, nil, err
			}
			reports = append(reports, resourceReport{
				Key:     st.Resource.Key,
				Current: st.Resource.Current,
				Maximum: st.Resource.Maximum,
			})
		}
		views = append(views, FeatureView{
			Ref:         st.Ref,
			Name:        st.Name,
			Detail:      st.Detail,
			ResourceKey: key,
		})
	}

	sort.Slice(views, func(i, j int) bool {
		return views[i].Ref.String() < views[j].Ref.String()
	})
	return views, reports, nil
}

// projectConditions builds the sorted ConditionView slice from each active
// condition's ref, looked up in the rulebook-owned display catalog. An unknown
// ref fails the whole projection with no partial output.
func (c *Character) projectConditions() ([]ConditionView, error) {
	views := make([]ConditionView, 0, len(c.conditions))
	for _, cond := range c.conditions {
		ref := cond.Ref()
		if ref == nil {
			return nil, rpgerr.New(rpgerr.CodeInvalidArgument, "condition reported a nil ref")
		}
		display, ok := conditions.DisplayFor(*ref)
		if !ok {
			return nil, rpgerr.Newf(rpgerr.CodeInvalidArgument,
				"condition ref %s is not in the status-view display catalog", ref.String())
		}
		views = append(views, ConditionView{
			Ref:          *ref,
			Name:         display.Name,
			Detail:       display.Detail,
			SourceMember: conditionSourceMember(cond),
		})
	}

	sort.Slice(views, func(i, j int) bool {
		return views[i].Ref.String() < views[j].Ref.String()
	})
	return views, nil
}

// ownerResourceReports collects the character's owner-owned resource rows
// (every entry in c.resources, including standalone Hit Dice) as reports ready
// for merging. Spell slots and legacy class resources are never read here, so
// they are excluded by construction.
func (c *Character) ownerResourceReports() []resourceReport {
	if c.resources == nil {
		return nil
	}
	rows := make([]resourceReport, 0, len(c.resources))
	for key, r := range c.resources {
		rows = append(rows, resourceReport{
			Key:     key,
			Current: r.Current(),
			Maximum: r.Maximum(),
		})
	}
	return rows
}

// resourceReport is the internal, name-less projection of one resource the
// projection is considering: its key and current/maximum counts. The display
// name is always derived from the key via resources.DisplayName, so a report
// never carries a feature-authored name.
type resourceReport struct {
	Key     coreResources.ResourceKey
	Current int
	Maximum int
}

// mergeResources builds the sorted ResourceView slice from owner rows plus
// feature-reported rows. Owner rows always appear. A feature report whose key
// matches an owner row dedupes against it — keeping the owner row — but must
// agree on current and maximum or the merge fails loudly. A feature report
// whose key is not in the owner rows (Second Wind, Action Surge) is added.
// Negative counts, current > maximum, and conflicts all error with no partial
// output.
func mergeResources(ownerRows []resourceReport, reports []resourceReport) ([]ResourceView, error) {
	byKey := make(map[coreResources.ResourceKey]resourceReport, len(ownerRows))
	order := make([]coreResources.ResourceKey, 0, len(ownerRows)+len(reports))

	// Owner rows first; they define the base set.
	for _, row := range ownerRows {
		if err := validateReport(row); err != nil {
			return nil, err
		}
		if _, exists := byKey[row.Key]; !exists {
			order = append(order, row.Key)
		}
		byKey[row.Key] = row
	}

	// Feature reports: dedupe against owner rows by key, fail on conflict,
	// otherwise add.
	for _, rep := range reports {
		if err := validateReport(rep); err != nil {
			return nil, err
		}
		if owner, exists := byKey[rep.Key]; exists {
			// Same key already present from the owner (or an earlier feature
			// report). The counts must agree or the sheet is lying.
			if owner.Current != rep.Current || owner.Maximum != rep.Maximum {
				return nil, rpgerr.Newf(rpgerr.CodeInvalidArgument,
					"conflicting resource %s: owner reports %d/%d but feature reports %d/%d",
					rep.Key, owner.Current, owner.Maximum, rep.Current, rep.Maximum)
			}
			// Agreeing duplicate: keep the existing row, do not add a second.
			continue
		}
		byKey[rep.Key] = rep
		order = append(order, rep.Key)
	}

	out := make([]ResourceView, 0, len(order))
	for _, key := range order {
		row := byKey[key]
		out = append(out, ResourceView{
			Key:     row.Key,
			Name:    resources.DisplayName(row.Key),
			Current: row.Current,
			Maximum: row.Maximum,
		})
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].Key < out[j].Key
	})
	return out, nil
}

// validateReport rejects negative counts and current > maximum.
func validateReport(r resourceReport) error {
	if r.Current < 0 || r.Maximum < 0 {
		return rpgerr.Newf(rpgerr.CodeInvalidArgument,
			"resource %s has negative counts: current=%d maximum=%d", r.Key, r.Current, r.Maximum)
	}
	if r.Current > r.Maximum {
		return rpgerr.Newf(rpgerr.CodeInvalidArgument,
			"resource %s current %d exceeds maximum %d", r.Key, r.Current, r.Maximum)
	}
	return nil
}

// featureID returns a feature's ID for error attribution, falling back to its
// ref string when GetID is empty.
func featureID(f features.Feature) string {
	if f == nil {
		return "<nil>"
	}
	if id := f.GetID(); id != "" {
		return id
	}
	if ref := f.Ref(); ref != nil {
		return ref.String()
	}
	return "<unknown>"
}

// conditionSourceMember returns the optional party-member ID a condition names
// as its source, or nil for self-owned class conditions. No current condition
// in the four-build set names an external source, so this is nil today; the
// seam exists so a future condition (e.g. Helped) can opt in without changing
// the projection's shape.
func conditionSourceMember(_ dnd5eEvents.ConditionBehavior) *string {
	return nil
}
