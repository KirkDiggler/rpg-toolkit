package spatial

// CreateDoorConnection creates a bidirectional door connection between two rooms
func CreateDoorConnection(id, fromRoom, toRoom string, fromPos, toPos Position) *BasicConnection {
	return NewBasicConnection(BasicConnectionConfig{
		ID:           id,
		Type:         "connection",
		ConnType:     ConnectionTypeDoor,
		FromRoom:     fromRoom,
		ToRoom:       toRoom,
		FromPosition: fromPos,
		ToPosition:   toPos,
		Reversible:   true,
		Passable:     true,
		Cost:         1.0,
		Requirements: []string{},
	})
}

// CreateStairsConnection creates a stairway connection between floors
func CreateStairsConnection(id, fromRoom, toRoom string, fromPos, toPos Position, goingUp bool) *BasicConnection {
	cost := 2.0 // Stairs are more expensive to traverse
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
		FromPosition: fromPos,
		ToPosition:   toPos,
		Reversible:   true,
		Passable:     true,
		Cost:         cost,
		Requirements: requirements,
	})
}

// CreateSecretPassageConnection creates a hidden passage that may have requirements
func CreateSecretPassageConnection(
	id, fromRoom, toRoom string, fromPos, toPos Position, requirements []string,
) *BasicConnection {
	if requirements == nil {
		requirements = []string{"found_secret"}
	}

	return NewBasicConnection(BasicConnectionConfig{
		ID:           id,
		Type:         "connection",
		ConnType:     ConnectionTypePassage,
		FromRoom:     fromRoom,
		ToRoom:       toRoom,
		FromPosition: fromPos,
		ToPosition:   toPos,
		Reversible:   true,
		Passable:     true,
		Cost:         1.0,
		Requirements: requirements,
	})
}

// CreatePortalConnection creates a magical portal connection
func CreatePortalConnection(id, fromRoom, toRoom string, fromPos, toPos Position, bidirectional bool) *BasicConnection {
	requirements := []string{"can_use_portals"}

	return NewBasicConnection(BasicConnectionConfig{
		ID:           id,
		Type:         "connection",
		ConnType:     ConnectionTypePortal,
		FromRoom:     fromRoom,
		ToRoom:       toRoom,
		FromPosition: fromPos,
		ToPosition:   toPos,
		Reversible:   bidirectional,
		Passable:     true,
		Cost:         0.5, // Portals are instant
		Requirements: requirements,
	})
}

// CreateBridgeConnection creates a bridge connection that might be destructible
func CreateBridgeConnection(id, fromRoom, toRoom string, fromPos, toPos Position) *BasicConnection {
	return NewBasicConnection(BasicConnectionConfig{
		ID:           id,
		Type:         "connection",
		ConnType:     ConnectionTypeBridge,
		FromRoom:     fromRoom,
		ToRoom:       toRoom,
		FromPosition: fromPos,
		ToPosition:   toPos,
		Reversible:   true,
		Passable:     true,
		Cost:         1.0,
		Requirements: []string{},
	})
}

// CreateTunnelConnection creates an underground tunnel
func CreateTunnelConnection(id, fromRoom, toRoom string, fromPos, toPos Position) *BasicConnection {
	return NewBasicConnection(BasicConnectionConfig{
		ID:           id,
		Type:         "connection",
		ConnType:     ConnectionTypeTunnel,
		FromRoom:     fromRoom,
		ToRoom:       toRoom,
		FromPosition: fromPos,
		ToPosition:   toPos,
		Reversible:   true,
		Passable:     true,
		Cost:         1.5, // Tunnels take a bit longer to traverse
		Requirements: []string{},
	})
}
