package actions

// MoveStopReason indicates why movement ended.
// Used by the game server to understand movement results and provide appropriate feedback.
type MoveStopReason string

const (
	// MoveStopReasonCompleted indicates the entity reached its destination successfully
	MoveStopReasonCompleted MoveStopReason = "completed"

	// MoveStopReasonInsufficientMovement indicates there wasn't enough movement remaining
	MoveStopReasonInsufficientMovement MoveStopReason = "insufficient_movement"

	// MoveStopReasonPositionOccupied indicates the destination hex is occupied by another entity
	MoveStopReasonPositionOccupied MoveStopReason = "position_occupied"

	// MoveStopReasonBlockedByWall indicates the path is blocked by a wall or obstacle
	MoveStopReasonBlockedByWall MoveStopReason = "blocked_by_wall"

	// MoveStopReasonInvalidCoordinates indicates the destination is outside grid bounds
	MoveStopReasonInvalidCoordinates MoveStopReason = "invalid_coordinates"
)

// String returns the string representation of the MoveStopReason
func (r MoveStopReason) String() string {
	return string(r)
}

// IsSuccess returns true if the movement completed successfully
func (r MoveStopReason) IsSuccess() bool {
	return r == MoveStopReasonCompleted
}
