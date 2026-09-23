// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"github.com/KirkDiggler/rpg-toolkit/mind/perception"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"math"
)

// previewCastAim uses the same compiled offer and recipient derivation as Cast.
// No persistence, event publication, dice, or resolution occurs here.
func previewCastAim(enc *encounter.Encounter, member string, aim *CastAim, offers []compiledOffer) (*CastAimPreview, error) {
	if aim == nil {
		return nil, nil
	}
	if aim.Cell == nil || math.IsNaN(aim.Cell.X) || math.IsNaN(aim.Cell.Y) || math.IsInf(aim.Cell.X, 0) || math.IsInf(aim.Cell.Y, 0) {
		return nil, ErrBadCast
	}
	var selected *compiledOffer
	for i := range offers {
		if offers[i].declaration.ID == aim.DeclarationID && offers[i].declaration.Verb == VerbCast {
			selected = &offers[i]
			break
		}
	}
	if selected == nil || aim.DeclarationID == "" {
		return nil, ErrStaleDeclaration
	}
	if selected.spell == nil || selected.spell.Cast == nil || selected.spell.Cast.Area == nil || selected.declaration.TargetKind != TargetCell {
		return nil, nil
	}
	profile := selected.spell.Cast
	if profile.Area.ObscuresSight {
		return nil, nil
	}
	cell := *aim.Cell
	out := &CastAimPreview{Aim: CastAim{DeclarationID: aim.DeclarationID, Cell: &cell}, Available: selected.declaration.Available, Why: selected.declaration.Why, AffectedMembers: []string{}}
	if !out.Available {
		return out, nil
	}
	roster, err := enc.Members()
	if err != nil {
		return nil, err
	}
	positions := rosterPositions(roster)
	origin, ok := positions[member]
	if !ok {
		return nil, ErrNoMemberID
	}
	if profile.Area.Footprint.Origin == combatActions.AreaOriginPoint && !enc.PointReachable(origin, *aim.Cell, profile.RangeFeet) {
		return nil, ErrBadCast
	}
	if _, err := castTargets(selected.spell, *selected, nil, castAim{cell: aim.Cell, casterAt: origin}); err != nil {
		return nil, err
	}
	caught, err := deriveAreaMembers(enc, profile, member, roster, aim.Cell)
	if err != nil {
		return nil, err
	}
	holdings, err := enc.View(&encounter.ViewInput{Member: encounter.MemberID(member)})
	if err != nil {
		return nil, err
	}
	out.AffectedMembers = visibleAimMembers(member, caught.resolvable, holdings)
	return out, nil
}

func visibleAimMembers(member string, caught []string, holdings []perception.Holding) []string {
	result := []string{}
	visible := map[string]bool{member: true}
	for _, holding := range holdings {
		if holding.CurrentOn(perception.Sight) {
			visible[string(holding.Subject)] = true
		}
	}
	for _, id := range caught {
		if visible[id] {
			result = append(result, id)
		}
	}
	return result
}
