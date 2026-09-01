package npcs

import (
	"github.com/KirkDiggler/rpg-toolkit/world"
	"github.com/KirkDiggler/rpg-toolkit/world/graph"
	"github.com/KirkDiggler/rpg-toolkit/world/journal"
)

const (
	// KindPerson identifies a D&D person-like world entity.
	KindPerson graph.Kind = "person"

	// FactVendorViewed records that someone looked at a vendor.
	FactVendorViewed journal.Kind = "vendor-viewed"

	// ViewVendor is the declared world action for looking at a vendor.
	ViewVendor world.VerbName = "view-vendor"
)

// VendorScenarioConfig provides the world declaration facts for a vendor.
type VendorScenarioConfig struct {
	EntityID   journal.EntityID
	Vendor     *Vendor
	Membership graph.Relation
}

// VendorScenario declares a configured vendor as a visible world entity.
func VendorScenario(config VendorScenarioConfig) (world.Scenario, error) {
	if config.EntityID == "" {
		return world.Scenario{}, ErrNoWorldEntity
	}
	if config.Vendor == nil || config.Vendor.NPC() == nil {
		return world.Scenario{}, ErrNoWorldVendor
	}
	if config.Membership == "" {
		return world.Scenario{}, ErrNoWorldMembership
	}

	return world.Scenario{
		Graph: graph.Config{
			Membership: config.Membership,
			Entities: []graph.Entity{{
				ID:   config.EntityID,
				Kind: KindPerson,
			}},
		},
		Verbs: []world.Verb{{
			Name:      ViewVendor,
			Otherwise: world.Emission{Kind: FactVendorViewed, Witness: world.WitnessTarget},
		}},
	}, nil
}
