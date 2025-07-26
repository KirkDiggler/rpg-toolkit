# Room Rendering Implementation Roadmap

This document captures the plan for implementing room rendering across the rpg ecosystem.

## Goal
Enable rpg-api to generate rooms using rpg-toolkit and return them via gRPC for client rendering.

## Current Status (2025-07-26)
- ✅ Reviewed spatial tools implementation
- ✅ Analyzed character/draft Data pattern
- ✅ Designed and implemented GameContext pattern (PR #108)
- ✅ Applied GameContext to character (PR #110)
- ✅ Created issues for all work items

## Implementation Plan

### Phase 1: Spatial Data Layer (rpg-toolkit)
**Issue**: #107
**Tasks**:
1. Create `RoomData` struct for room persistence
2. Add `ToData()` method to BasicRoom
3. Add `LoadRoomFromData()` function
4. Update to use GameContext pattern for loading

**Starting Code Structure**:
```go
// In spatial/types.go
type RoomData struct {
    ID       string    `json:"id"`
    Type     string    `json:"type"`
    Width    int       `json:"width"`
    Height   int       `json:"height"`
    GridType string    `json:"grid_type"` // "square", "hex", "gridless"
}

// In spatial/room.go
func (r *BasicRoom) ToData() RoomData {
    // Convert room to data
}

// Using GameContext pattern
func LoadRoomFromContext(ctx context.Context, gameCtx game.Context[RoomData]) (*BasicRoom, error) {
    // Load room with event bus integration
}
```

### Phase 2: Proto Definitions (rpg-api-protos)
**Repository**: KirkDiggler/rpg-api-protos
**Issue**: #31
**Tasks**:
1. Define Room proto message matching RoomData
2. Define encounter service with DungeonStart RPC
3. Define request/response messages

**Proto Structure**:
```proto
// room.proto
message Room {
    string id = 1;
    string type = 2;
    int32 width = 3;
    int32 height = 4;
    string grid_type = 5;
}

// encounter.proto  
service EncounterService {
    rpc DungeonStart(DungeonStartRequest) returns (DungeonStartResponse);
}

message DungeonStartRequest {
    string character_id = 1;
    string difficulty = 2;
}

message DungeonStartResponse {
    Room starting_room = 1;
    repeated Entity entities = 2;
}
```

### Phase 3: Service Implementation (rpg-api)
**Repository**: KirkDiggler/rpg-api
**Issue**: #131
**Tasks**:
1. Implement EncounterService
2. Use rpg-toolkit to generate rooms
3. Convert room data to proto format
4. Return via gRPC

**Service Structure**:
```go
func (s *encounterService) DungeonStart(ctx context.Context, req *pb.DungeonStartRequest) (*pb.DungeonStartResponse, error) {
    // 1. Generate room using toolkit
    room := spatial.GenerateBasicRoom(...)
    roomData := room.ToData()
    
    // 2. Convert to proto
    protoRoom := &pb.Room{
        Id:       roomData.ID,
        Type:     roomData.Type,
        Width:    int32(roomData.Width),
        Height:   int32(roomData.Height),
        GridType: roomData.GridType,
    }
    
    // 3. Return response
    return &pb.DungeonStartResponse{
        StartingRoom: protoRoom,
    }, nil
}
```

## Next Session Focus
1. Start with rpg-toolkit issue #107
2. Implement basic RoomData and ToData()
3. Test round-trip conversion
4. Consider GameContext integration

## Future Enhancements
- Add entity positions to room data
- Add wall/obstacle data
- Add visual themes/tilesets
- Support multi-room connections
- Add fog of war data

## Success Metrics
- [ ] Can generate a room in rpg-api
- [ ] Can return room data via gRPC
- [ ] Client can render the room
- [ ] Data survives round-trip conversion