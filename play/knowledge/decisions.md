# Decisions — play

One line per ruling that binds the play leaves together: date · the words · the options · the link. Ingest is the writer; entries arrive through its reviewed PR.

## Decisions

- 2026-10-28 · record is data only: the id is a string the caller supplies on append (required, an empty one refused), record orders by its own append order and never by the id, and the composition receives ids from a host-supplied minter the same way it receives its die; the leaf promise stands unamended, a leaf contains no randomness, not even an id mint · options: amending the leaf promise to "no game randomness" and keeping the ULID mint in record · because: a leaf that holds entropy and a clock can never be made deterministic, and the controlled-random direction (a seeded roller per encounter so a run can be replayed) needs every piece of a run reproducible; the desire for the sequence came from our usage, the wire ordering key, which the host keeps minting (the operator, 2026-10-28) · [rpg-toolkit#2138](https://github.com/KirkDiggler/rpg-toolkit/issues/2138) · upheld: none yet

## Open

- 2026-10-28 · a play leaf using a tool that is itself a primitive (tools/spatial, tools/selectables) is the operator's direction and reads as healthy to them, while leaf rule R1 forbids it today (a leaf depends only on core; spatial named) with the because that a flat graph has no diamonds and isolation makes versioning safe; held until a leaf actually wants a tool, when that use case decides whether R1 is amended and the diamond is measured · [rpg-project#694, play leaves](https://github.com/KirkDiggler/rpg-project/issues/694#issuecomment-6462010434)
