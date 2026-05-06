// Package types holds the primitive value types shared across the encounter
// SDK's internal subpackages (events, perception) and the top-level
// encounter package. It exists specifically to keep the internal package
// graph acyclic — events and perception import types; encounter imports
// all three.
//
// These types are intentionally minimal. When Hex/grid logic moves to
// rpg-toolkit/tools/spatial in a future slice, this package will shrink
// or be absorbed.
package types
