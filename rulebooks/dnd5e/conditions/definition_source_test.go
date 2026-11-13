// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later
package conditions_test

import (
	"context"
	"fmt"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/resources"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/conditions"
	dndevents "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/events"
)

// This source is test scaffolding, not an item/offer/refund engine. It supplies
// identity and one use; production rulebook APIs own every effect decision.
type fixtureSource struct {
	ID, UserID, DefinitionID string
	Ref                      core.Ref
	Available                bool
	Budget                   *resources.Resource
}
type fixtureSourceData struct {
	ID, UserID, DefinitionID string
	Ref                      core.Ref
	Available                bool
	Remaining                int
}

func newFixtureSource(id, user, definition string) *fixtureSource {
	return &fixtureSource{ID: id, UserID: user, DefinitionID: definition, Ref: core.Ref{Module: "example", Type: "items", ID: definition}, Available: true, Budget: resources.NewResource("uses", 1)}
}
func (s *fixtureSource) origin() dndevents.ConditionOrigin {
	return dndevents.ConditionOrigin{Ref: s.Ref, EntityID: s.ID}
}
func (s *fixtureSource) data() fixtureSourceData {
	return fixtureSourceData{ID: s.ID, UserID: s.UserID, DefinitionID: s.DefinitionID, Ref: s.Ref, Available: s.Available, Remaining: s.Budget.Current}
}
func restoreFixtureSource(in fixtureSourceData) (*fixtureSource, error) {
	if strings.TrimSpace(in.ID) == "" || strings.TrimSpace(in.UserID) == "" || strings.TrimSpace(in.DefinitionID) == "" || in.Remaining < 0 || in.Remaining > 1 {
		return nil, fmt.Errorf("invalid source identity or remaining uses")
	}
	if err := in.Ref.IsValid(); err != nil {
		return nil, err
	}
	source := newFixtureSource(in.ID, in.UserID, in.DefinitionID)
	source.Ref = in.Ref
	source.Available = in.Available
	source.Budget.SetCurrent(in.Remaining)
	return source, nil
}

type definitionUseInput struct {
	Source                     *fixtureSource
	RecipientID, ApplicationID string
	Definitions                *conditions.CompiledDefinitions
	Participants               []conditions.ParticipantConditions
}
type preparedDefinitionGrant struct {
	Prepared  *conditions.PreparedApplication
	Spec      conditions.ApplicationSpec
	Origin    dndevents.ConditionOrigin
	Remaining int
}

func useDefinition(ctx context.Context, in definitionUseInput) (*preparedDefinitionGrant, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	source := in.Source
	if source == nil || source.Budget == nil || !source.Available || source.Budget.Current < 1 {
		return nil, fmt.Errorf("source is unavailable or exhausted")
	}
	if err := conditions.ValidateDefinitionReference(&conditions.ValidateDefinitionReferenceInput{Definitions: in.Definitions, DefinitionID: source.DefinitionID}); err != nil {
		return nil, err
	}
	origin := source.origin()
	bound, err := conditions.BindDefinition(&conditions.BindDefinitionInput{Definitions: in.Definitions, DefinitionID: source.DefinitionID, UserID: source.UserID, RecipientID: in.RecipientID, ApplicationID: in.ApplicationID, Origin: origin})
	if err != nil {
		return nil, err
	}
	prepared, err := conditions.PrepareApplication(&conditions.PrepareApplicationInput{ID: in.ApplicationID, CoordinatorID: bound.CoordinatorID, Spec: bound.Spec, Origin: origin, Existing: in.Participants})
	if err != nil {
		return nil, err
	}
	if err := source.Budget.Use(1); err != nil {
		return nil, err
	}
	return &preparedDefinitionGrant{Prepared: prepared, Spec: bound.Spec, Origin: origin, Remaining: source.Budget.Current}, nil
}
