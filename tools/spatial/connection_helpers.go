package spatial

// CreateDoorConnection creates a bidirectional door connection between two rooms (ADR-0015: Abstract Connections)
func CreateDoorConnection(id, fromRoom, toRoom string, cost float64) *BasicConnection {
	return NewBasicConnection(BasicConnectionConfig{
		ID:           id,
		Type:         "connection",
		ConnType:     ConnectionTypeDoor,
		FromRoom:     fromRoom,
		ToRoom:       toRoom,
		Reversible:   true,
		Passable:     true,
		Cost:         cost,
		Requirements: []string{},
	})
}

// CreateStairsConnection creates a stairway connection between floors
func CreateStairsConnection(id, fromRoom, toRoom string, cost float64, goingUp bool) *BasicConnection {
	requirements := []string{}

	if goingUp {
		requirements = append(requirements, "can_climb")
	}

	return NewBasicConnection(BasicConnectionConfig{
		ID:           id,
		Type:         "connection",
		ConnType:     ConnectionTypeStairs,
		FromRoom:     fromRoom,
		ToRoom:       toRoom,
		Reversible:   true,
		Passable:     true,
		Cost:         cost,
		Requirements: requirements,
	})
}

// CreateSecretPassageConnection creates a hidden passage that may have requirements
func CreateSecretPassageConnection(id, fromRoom, toRoom string, cost float64, requirements []string) *BasicConnection {
	if requirements == nil {
		requirements = []string{"found_secret"}
	}

	return NewBasicConnection(BasicConnectionConfig{
		ID:           id,
		Type:         "connection",
		ConnType:     ConnectionTypePassage,
		FromRoom:     fromRoom,
		ToRoom:       toRoom,
		Reversible:   true,
		Passable:     true,
		Cost:         cost,
		Requirements: requirements,
	})
}

// CreatePortalConnection creates a magical portal connection
func CreatePortalConnection(id, fromRoom, toRoom string, cost float64, bidirectional bool) *BasicConnection {
	requirements := []string{"can_use_portals"}

	return NewBasicConnection(BasicConnectionConfig{
		ID:           id,
		Type:         "connection",
		ConnType:     ConnectionTypePortal,
		FromRoom:     fromRoom,
		ToRoom:       toRoom,
		Reversible:   bidirectional,
		Passable:     true,
		Cost:         cost,
		Requirements: requirements,
	})
}

// CreateBridgeConnection creates a bridge connection that might be destructible
func CreateBridgeConnection(id, fromRoom, toRoom string, cost float64) *BasicConnection {
	return NewBasicConnection(BasicConnectionConfig{
		ID:           id,
		Type:         "connection",
		ConnType:     ConnectionTypeBridge,
		FromRoom:     fromRoom,
		ToRoom:       toRoom,
		Reversible:   true,
		Passable:     true,
		Cost:         cost,
		Requirements: []string{},
	})
}

// CreateTunnelConnection creates an underground tunnel
func CreateTunnelConnection(id, fromRoom, toRoom string, cost float64) *BasicConnection {
	return NewBasicConnection(BasicConnectionConfig{
		ID:           id,
		Type:         "connection",
		ConnType:     ConnectionTypeTunnel,
		FromRoom:     fromRoom,
		ToRoom:       toRoom,
		Reversible:   true,
		Passable:     true,
		Cost:         cost,
		Requirements: []string{},
	})
}
