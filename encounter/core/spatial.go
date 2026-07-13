package core

import "github.com/KirkDiggler/rpg-toolkit/tools/spatial"

// ToCube converts a Hex to spatial's CubeCoordinate. Both are cube
// coordinates under the hood — this is a field rename, not a projection.
func (h Hex) ToCube() spatial.CubeCoordinate {
	return spatial.CubeCoordinate{X: h.Q, Y: h.R, Z: h.S}
}

// HexFromCube converts a spatial CubeCoordinate back to a Hex.
func HexFromCube(c spatial.CubeCoordinate) Hex {
	return Hex{Q: c.X, R: c.Y, S: c.Z}
}

// ToPosition converts a Hex to a spatial.Position (offset coordinates).
// Pointy-top orientation — the only orientation environments.QuickRoom
// builds (D&D 5e standard) — so wave 1 hardcodes it here rather than
// threading an orientation parameter through every call site.
func (h Hex) ToPosition() spatial.Position {
	return h.ToCube().ToOffsetCoordinateWithOrientation(spatial.HexOrientationPointyTop)
}

// HexFromPosition converts a spatial.Position (offset coordinates,
// pointy-top orientation) back to a Hex.
func HexFromPosition(pos spatial.Position) Hex {
	return HexFromCube(spatial.OffsetCoordinateToCubeWithOrientation(pos, spatial.HexOrientationPointyTop))
}
