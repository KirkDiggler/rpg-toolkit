package dungeon

import "github.com/KirkDiggler/rpg-toolkit/rpgerr"

// Validation errors for dungeon generation.
// These errors are returned by Generate when the input is invalid.
var (
	// ErrNilInput is returned when Generate receives a nil input.
	ErrNilInput = rpgerr.New(rpgerr.CodeInvalidArgument, "input cannot be nil")

	// ErrInvalidTheme is returned when the theme ID is empty.
	ErrInvalidTheme = rpgerr.New(rpgerr.CodeInvalidArgument, "theme cannot be empty")

	// ErrInvalidCR is returned when the target challenge rating is not positive.
	ErrInvalidCR = rpgerr.New(rpgerr.CodeInvalidArgument, "target CR must be positive")

	// ErrInvalidRoomCount is returned when the room count is less than 1.
	ErrInvalidRoomCount = rpgerr.New(rpgerr.CodeInvalidArgument, "room count must be at least 1")
)
