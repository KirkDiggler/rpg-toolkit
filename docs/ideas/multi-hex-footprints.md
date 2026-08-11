# Deferred: Multi-Hex Creature Footprints

D&D 5e creature size categories are represented now, but every creature
currently occupies one 5-foot hex. This keeps existing placement, movement,
pathfinding, line-of-sight, spawning, and combat behavior unchanged.

Before adding multi-hex occupancy, decide and document:

- the footprint shape for each size on a hex grid;
- the anchor hex used for movement and range measurement;
- collision, rotation, squeezing, and pathfinding rules; and
- how multi-hex creatures interact with doors, walls, and opportunity attacks.

This is intentionally deferred work, not a supported behavior today.
