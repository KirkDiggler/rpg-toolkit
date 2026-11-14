// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package effects

import (
	"fmt"
	"reflect"
)

type projectionInput struct {
	Definition Definition
	Active     map[*Composition]bool
}

func projectDefinition(in *projectionInput) (DefinitionData, error) {
	if in.Definition == nil {
		return DefinitionData{}, fmt.Errorf("definition is required")
	}
	value := reflect.ValueOf(in.Definition)
	if value.Kind() == reflect.Pointer && value.IsNil() {
		return DefinitionData{}, fmt.Errorf("definition is nil")
	}
	switch definition := in.Definition.(type) {
	case Composition:
		return projectMembers(&projectMembersInput{Members: definition.Members, Active: in.Active})
	case *Composition:
		if in.Active[definition] {
			return DefinitionData{}, fmt.Errorf("cyclic Go composition")
		}
		in.Active[definition] = true
		defer delete(in.Active, definition)
		return projectMembers(&projectMembersInput{Members: definition.Members, Active: in.Active})
	default:
		return in.Definition.DefinitionData()
	}
}

type projectMembersInput struct {
	Members []Member
	Active  map[*Composition]bool
}

func projectMembers(in *projectMembersInput) (DefinitionData, error) {
	members := make([]MemberData, 0, len(in.Members))
	for _, member := range in.Members {
		data, err := projectDefinition(&projectionInput{Definition: member.Definition, Active: in.Active})
		if err != nil {
			return DefinitionData{}, fmt.Errorf("member %q: %w", member.Key, err)
		}
		members = append(members, MemberData{Key: member.Key, Name: member.Name, Description: member.Description, Definition: data})
	}
	return DefinitionData{Composition: &CompositionData{Members: members}}, nil
}
