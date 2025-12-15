// Package resources provides D&D 5e resource key constants.
// These keys identify class-specific resources that are stored on characters
// and consumed by features.
package resources

import (
	coreResources "github.com/KirkDiggler/rpg-toolkit/core/resources"
)

// Resource key constants for D&D 5e class resources.
// These are used with Character.GetResource() to access resource pools.
const (
	// Ki is the monk's resource pool, equal to monk level.
	// Recovered on short or long rest.
	// Used by: Flurry of Blows, Patient Defense, Step of the Wind, etc.
	Ki coreResources.ResourceKey = "ki"
)
