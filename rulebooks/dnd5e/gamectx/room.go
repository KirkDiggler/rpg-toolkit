// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package gamectx

import (
	"context"
	"errors"

	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// roomContextKey is the key type for storing spatial.Room in context.Context.
type roomContextKey struct{}

// WithRoom wraps a context.Context with the provided spatial.Room.
// Purpose: Enables features and conditions to query entity positions and
// perform spatial calculations during event processing.
//
// Example:
//
//	ctx = gamectx.WithRoom(ctx, combatRoom)
//	// Now features can query positions, check distances, etc.
func WithRoom(ctx context.Context, room spatial.Room) context.Context {
	return context.WithValue(ctx, roomContextKey{}, room)
}

// Room retrieves the spatial.Room from the context.
// Returns the room and true if found, nil and false otherwise.
//
// Purpose: Allows conditions and features to query spatial data when available,
// gracefully handling cases where no Room is present.
//
// Example:
//
//	if room, ok := gamectx.Room(ctx); ok {
//	    entities := room.GetEntitiesInRange(targetPos, 5.0)
//	    // Check if any allies are within 5ft
//	}
func Room(ctx context.Context) (spatial.Room, bool) {
	if room, ok := ctx.Value(roomContextKey{}).(spatial.Room); ok && room != nil {
		return room, true
	}
	return nil, false
}

// RequireRoom retrieves the spatial.Room from the context.
// Returns an error if no Room is present in the context.
//
// Purpose: For code paths that require spatial data to function and need
// explicit error handling rather than silent failures.
//
// Example:
//
//	room, err := gamectx.RequireRoom(ctx)
//	if err != nil {
//	    return c, err
//	}
//	targetPos, _ := room.GetEntityPosition(targetID)
func RequireRoom(ctx context.Context) (spatial.Room, error) {
	room, ok := Room(ctx)
	if !ok {
		return nil, ErrNoRoom
	}
	return room, nil
}

// ErrNoRoom is returned when a required Room is not found in context.
var ErrNoRoom = errors.New("no Room found in context")
