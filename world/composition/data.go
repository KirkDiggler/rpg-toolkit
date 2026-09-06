package composition

import "encoding/json"

// Data the truct that holds a composition for a given world
type Data struct {
	ID      string          `json:"id"`
	WorldID string          `json:"world_id"`
	JSON    json.RawMessage `json:"json"`
}
