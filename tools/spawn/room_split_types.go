package spawn

import (
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
)

// RoomSplit represents a room splitting recommendation from environment package
// Purpose: Local copy of environment package types to avoid import cycles
type RoomSplit struct {
	Reason                        string               `json:"reason"`                          // Why split is recommended
	SplitType                     string               `json:"split_type"`                      // Split type
	Dimensions                    []spatial.Dimensions `json:"dimensions"`                      // Room dimensions
	Benefits                      []string             `json:"benefits"`                        // Split advantages
	Complexity                    float64              `json:"complexity"`                      // Complexity rating
	SuggestedSize                 spatial.Dimensions   `json:"suggested_size"`                  // Split room dimensions
	ConnectionPoints              []spatial.Position   `json:"connection_points"`               // Connection points
	RecommendedEntityDistribution map[string]int       `json:"recommended_entity_distribution"` // Entity distribution
	RecommendedConnectionType     string               `json:"recommended_connection_type"`     // Connection type to use
	SplitReason                   string               `json:"split_reason"`                    // Reason for split
	EstimatedCapacityImprovement  float64              `json:"estimated_capacity_improvement"`  // Expected improvement
}