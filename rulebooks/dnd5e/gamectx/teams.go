// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package gamectx

import "context"

// TeamID represents a team or faction identifier.
// Characters on the same team are considered allies.
type TeamID string

// Common team constants
const (
	// TeamPlayers is the default team for player characters
	TeamPlayers TeamID = "players"
	// TeamEnemies is the default team for hostile NPCs
	TeamEnemies TeamID = "enemies"
	// TeamNeutral is the default team for neutral NPCs
	TeamNeutral TeamID = "neutral"
)

// teamsContextKey is the key type for storing TeamRegistry in context.Context.
type teamsContextKey struct{}

// TeamRegistry tracks which team each character belongs to.
// Purpose: Enables features like Sneak Attack to determine if a character
// is an ally (same team as attacker) or enemy (different team).
type TeamRegistry interface {
	// GetTeam returns the team ID for a character.
	// Returns empty string if character has no assigned team.
	GetTeam(characterID string) TeamID

	// AreAllies returns true if both characters are on the same team.
	// Returns false if either character has no team or they're on different teams.
	AreAllies(characterID1, characterID2 string) bool

	// AreEnemies returns true if the characters are on different teams.
	// Returns false if either character has no team (unknown relationship).
	AreEnemies(characterID1, characterID2 string) bool
}

// WithTeams wraps a context.Context with the provided TeamRegistry.
// Purpose: Enables features and conditions to query team relationships
// during event processing.
//
// Example:
//
//	ctx = gamectx.WithTeams(ctx, teamRegistry)
//	// Now features can check if characters are allies
func WithTeams(ctx context.Context, teams TeamRegistry) context.Context {
	return context.WithValue(ctx, teamsContextKey{}, teams)
}

// Teams retrieves the TeamRegistry from the context.
// Returns the registry and true if found, nil and false otherwise.
//
// Example:
//
//	if teams, ok := gamectx.Teams(ctx); ok {
//	    if teams.AreAllies(attackerID, nearbyCharID) {
//	        // Apply sneak attack
//	    }
//	}
func Teams(ctx context.Context) (TeamRegistry, bool) {
	if teams, ok := ctx.Value(teamsContextKey{}).(TeamRegistry); ok && teams != nil {
		return teams, true
	}
	return nil, false
}

// BasicTeamRegistry is a simple in-memory implementation of TeamRegistry.
type BasicTeamRegistry struct {
	teams map[string]TeamID
}

// NewBasicTeamRegistry creates a new BasicTeamRegistry.
func NewBasicTeamRegistry() *BasicTeamRegistry {
	return &BasicTeamRegistry{
		teams: make(map[string]TeamID),
	}
}

// SetTeam assigns a character to a team.
func (r *BasicTeamRegistry) SetTeam(characterID string, team TeamID) {
	r.teams[characterID] = team
}

// GetTeam returns the team ID for a character.
// Returns empty string if character has no assigned team.
func (r *BasicTeamRegistry) GetTeam(characterID string) TeamID {
	return r.teams[characterID]
}

// AreAllies returns true if both characters are on the same team.
// Returns false if either character has no team or they're on different teams.
func (r *BasicTeamRegistry) AreAllies(characterID1, characterID2 string) bool {
	team1 := r.teams[characterID1]
	team2 := r.teams[characterID2]

	// Both must have a team and be on the same team
	if team1 == "" || team2 == "" {
		return false
	}
	return team1 == team2
}

// AreEnemies returns true if the characters are on different teams.
// Returns false if either character has no team (unknown relationship).
func (r *BasicTeamRegistry) AreEnemies(characterID1, characterID2 string) bool {
	team1 := r.teams[characterID1]
	team2 := r.teams[characterID2]

	// Both must have a team to be enemies
	if team1 == "" || team2 == "" {
		return false
	}
	return team1 != team2
}
