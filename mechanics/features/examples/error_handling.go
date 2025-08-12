// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package examples

import (
	"errors"
	"log"
	
	"github.com/KirkDiggler/rpg-toolkit/core"
	"github.com/KirkDiggler/rpg-toolkit/mechanics/features"
)

// ExampleErrorHandling demonstrates how to handle feature errors.
func ExampleErrorHandling(character core.Entity, featureRef string) {
	// Example: Activating a feature with proper error handling
	feat := getFeature(featureRef) // Assume this gets a feature
	
	err := feat.Activate(character)
	if err != nil {
		// Check for specific errors
		switch {
		case errors.Is(err, features.ErrAlreadyActive):
			// Feature is already active - this might be OK
			log.Printf("Feature %s is already active", featureRef)
			return
			
		case errors.Is(err, features.ErrNoUsesRemaining):
			// No uses left - player needs to rest
			log.Printf("Cannot activate %s: no uses remaining", featureRef)
			// Could trigger UI to suggest a rest
			return
			
		case errors.Is(err, features.ErrTargetRequired):
			// Need to ask player for a target
			log.Printf("Feature %s requires a target", featureRef)
			// Would trigger target selection UI
			return
			
		case errors.Is(err, features.ErrCannotActivate):
			// Some other condition prevents activation
			log.Printf("Cannot activate %s right now", featureRef)
			return
			
		default:
			// Unexpected error
			log.Printf("Failed to activate feature: %v", err)
			return
		}
	}
	
	log.Printf("Successfully activated %s", featureRef)
}

// ExampleLoadingErrors demonstrates handling loading errors.
func ExampleLoadingErrors(data []byte) {
	feat, err := features.LoadFeatureFromJSON(data)
	if err != nil {
		// Check if it's a load error with context
		var loadErr *features.LoadError
		if errors.As(err, &loadErr) {
			log.Printf("Failed to load feature %s: %v", loadErr.Ref, loadErr.Reason)
			
			// Check the underlying reason
			if errors.Is(loadErr.Reason, features.ErrFeatureNotFound) {
				// This feature type isn't implemented yet
				log.Printf("Feature type %s is not supported", loadErr.Ref)
			}
			return
		}
		
		// Check for unmarshaling errors
		if errors.Is(err, features.ErrUnmarshalFailed) {
			log.Printf("Invalid JSON data for feature")
			return
		}
		
		log.Printf("Unexpected error loading feature: %v", err)
		return
	}
	
	log.Printf("Successfully loaded feature: %s", feat.Name())
}

// ExampleRetryableErrors demonstrates checking if an error might succeed on retry.
func ExampleRetryableErrors(feat features.Feature, owner core.Entity) {
	err := feat.Activate(owner)
	if err != nil {
		if features.IsRetryable(err) {
			log.Printf("Activation failed but might succeed later: %v", err)
			// Could show "try again after rest" message
		} else {
			log.Printf("Activation failed and won't succeed if retried: %v", err)
			// Show definitive error message
		}
	}
}

// getFeature is a placeholder for getting a feature.
func getFeature(ref string) features.Feature {
	// This would actually look up the feature
	return nil
}