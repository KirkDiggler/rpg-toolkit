// Package currency provides D&D 5e coinage arithmetic.
//
// currency is the domain ("things a vendor can be paid in"); Money is its
// first concrete type. This package is deliberately dependency-free — a bare
// string in (ParseCost), Money out — so it can be used by anything that
// prices something without pulling in the equipment catalog, a character, or
// a session. equipment.PriceOf (a later wave) is what connects a catalog
// item's opaque Cost string to a resolved Money value; this package never
// reaches for the catalog itself.
//
// If a second currency kind is ever a real requirement, it lands as a
// sibling type here, not a reshape of Money into something generic — no
// interface is extracted ahead of a second concrete type to extract it from.
package currency
