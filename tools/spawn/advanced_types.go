package spawn

import "github.com/KirkDiggler/rpg-toolkit/tools/spatial"

// TeamConfig configures team-based spawning
type TeamConfig struct {
	Teams           []Team                `json:"teams"`
	CohesionRules   TeamCohesionRules     `json:"cohesion_rules"`
	SeparationRules SeparationConstraints `json:"separation_rules"`
}

// Team represents a group of entities that should be placed together
type Team struct {
	ID            string            `json:"id"`
	EntityTypes   []string          `json:"entity_types"`
	Formation     *FormationPattern `json:"formation,omitempty"`
	PreferredZone string            `json:"preferred_zone"`
	Cohesion      float64           `json:"cohesion"`
}

// TeamCohesionRules define how teams are kept together
type TeamCohesionRules struct {
	KeepFriendliesTogether bool    `json:"keep_friendlies_together"`
	KeepEnemiesTogether    bool    `json:"keep_enemies_together"`
	MinTeamSeparation      float64 `json:"min_team_separation"`
}

// SeparationConstraints define minimum distances between teams
type SeparationConstraints struct {
	MinTeamDistance float64               `json:"min_team_distance"`
	TeamPlacement   TeamPlacementStrategy `json:"team_placement"`
}

// TeamPlacementStrategy defines how teams are positioned relative to each other
type TeamPlacementStrategy string

const (
	// TeamPlacementCorners places teams in room corners
	TeamPlacementCorners TeamPlacementStrategy = "corners"
	// TeamPlacementOppositeSides places teams on opposite sides
	TeamPlacementOppositeSides TeamPlacementStrategy = "opposite_sides"
	// TeamPlacementRandom places teams randomly with separation
	TeamPlacementRandom TeamPlacementStrategy = "random"
)

// FormationPattern defines a structured arrangement of entities
type FormationPattern struct {
	Name        string                `json:"name"`
	Positions   []RelativePosition    `json:"positions"`
	Scaling     FormationScaling      `json:"scaling"`
	Constraints FormationConstraints  `json:"constraints"`
}

// RelativePosition defines a position relative to formation center
type RelativePosition struct {
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
	Priority int     `json:"priority"`
}

// FormationScaling controls how formations adapt to space constraints
type FormationScaling struct {
	AllowRotation   bool `json:"allow_rotation"`
	AllowStretching bool `json:"allow_stretching"`
	PreserveRatios  bool `json:"preserve_ratios"`
}

// FormationConstraints define requirements for formation placement
type FormationConstraints struct {
	MinSpacing     float64 `json:"min_spacing"`
	RequiredSpace  float64 `json:"required_space"`
	WallClearance  float64 `json:"wall_clearance"`
}

// SpawnZone defines an area where players can choose spawn positions
type SpawnZone struct {
	ID          string              `json:"id"`
	Area        spatial.Rectangle   `json:"area"`
	EntityTypes []string            `json:"entity_types"`
	MaxEntities int                 `json:"max_entities"`
}

// PlayerSpawnChoice represents a player's choice of spawn position
type PlayerSpawnChoice struct {
	PlayerID string            `json:"player_id"`
	ZoneID   string            `json:"zone_id"`
	Position spatial.Position  `json:"position"`
}

// SpatialConstraints define spatial requirements and restrictions
type SpatialConstraints struct {
	MinDistance   map[string]float64 `json:"min_distance"`
	LineOfSight   LineOfSightRules   `json:"line_of_sight"`
	WallProximity float64            `json:"wall_proximity"`
}

// LineOfSightRules define visibility requirements between entities
type LineOfSightRules struct {
	RequiredSight []EntityPair `json:"required_sight"`
	BlockedSight  []EntityPair `json:"blocked_sight"`
}

// EntityPair represents a relationship between two entity types
type EntityPair struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// PlacementRules define how entities should be positioned
type PlacementRules struct {
	MaintainExitAccess bool    `json:"maintain_exit_access"`
	MinPathWidth       float64 `json:"min_path_width"`
	PreferredAreas     []string `json:"preferred_areas"`
}

// ScalingConfig controls adaptive room scaling behavior
type ScalingConfig struct {
	Enabled        bool    `json:"enabled"`
	ScalingFactor  float64 `json:"scaling_factor"`
	PreserveAspect bool    `json:"preserve_aspect"`
	EmitEvents     bool    `json:"emit_events"`
}