// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// SightArea is a current, observer-visible sight-obscuring footprint. It
// reveals no hidden creator identity and never instructs a client to infer rules.
type SightArea struct {
	ID         string           `json:"id"`
	Name       string           `json:"name"`
	Ref        string           `json:"ref"`
	Center     spatial.Position `json:"center"`
	RadiusFeet int              `json:"radius_feet"`
}

// Areas reports the active sight areas observable by the requested member.
// It shares View's session/member authorization scope; geometry is encounter-owned.
func (m *Manager) Areas(ctx context.Context, in *ViewInput) ([]SightArea, error) {
	if in == nil {
		return nil, fmt.Errorf("areas: %w", ErrNilInput)
	}
	if in.Member == "" {
		return nil, fmt.Errorf("areas: %w", ErrNoMemberID)
	}
	data, err := m.loadSessionData(ctx, in.Session)
	if err != nil {
		return nil, fmt.Errorf("areas: %w", err)
	}
	enc, err := m.loadWorld(ctx, data)
	if err != nil {
		return nil, fmt.Errorf("areas: %w", err)
	}
	// Validate membership through the same owner read as View.
	if _, err := enc.View(&encounter.ViewInput{Member: encounter.MemberID(in.Member)}); err != nil {
		return nil, translate(err)
	}
	areas := enc.SightAreasFor(encounter.MemberID(in.Member))
	out := make([]SightArea, 0, len(areas))
	for _, a := range areas {
		opaque := sha256.Sum256([]byte(a.ID))
		out = append(out, SightArea{ID: hex.EncodeToString(opaque[:16]), Name: a.Name, Ref: a.Ref, Center: a.Center, RadiusFeet: a.RadiusFeet})
	}
	return out, nil
}
