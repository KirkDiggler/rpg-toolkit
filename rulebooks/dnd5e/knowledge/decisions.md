# Decisions — rulebooks/dnd5e

One line per entry, dated and linked. Ingest is the writer; entries arrive through its reviewed PR.
A decision reads `- <date> · <the words> · options: <the options it closed> · because: <why> · <link> · upheld: <links, or none yet>`;
a finding, friction or open line reads `- <date> · <the words> · <link>`. A finding that needs more than a line links its case file in `findings/`.
Friction names both entries and never settles them.

## Decisions

- 2026-08-14 · the bus lives in one place, resolution; session and encounter trade in data · options: every rules package attaches to the bus, resolution alone owns it · because: resolution is the moving part and the composer · [ADR-0038](https://github.com/KirkDiggler/rpg-toolkit/blob/c355102dffe40fa6aa3852d13708ec21c055172d/docs/adr/0038-resolution-owns-the-bus.md) · upheld: none linked yet; reaffirmed by the operator 2026-10-27 ([rpg-project#694](https://github.com/KirkDiggler/rpg-project/issues/694))
- 2026-10-27 · "data only" names what happens at the seams of the resolution machine: everything that crosses a seam is data, and the extensions the machine composes with are data, so each extension is trivially unit-testable; the composable data-only pieces hold no randomness. It is a property of the seams and the extensions, not a claim that the encounter never rolls; where else the pattern works is a question, not a rule · options: data only as a property of the whole encounter, data only as a property of the seams and extensions · because: data at every seam is what makes that section work and each extension testable alone · [rpg-project#694](https://github.com/KirkDiggler/rpg-project/issues/694) · upheld: none yet
- 2026-10-27 · where a die rolls follows what the roll is for: a roll that resolves an interaction (attack, check, save, death save, activation) rolls in resolution, which takes a required roller on every Input; a roll that decides what a creature does or how the world orders itself (a behavior table pick, a search heading, an answer to a spoken line, initiative) rolls in the encounter through the driver. The host supplies both rollers at construction and each is refused by name when absent; the roller stays in the encounter (the operator), and a seeded roller per encounter, so a fight cannot be cheesed by replaying it, needs no new seam because the host supplies it through the existing capability ([encounter/capabilities.go:83](https://github.com/KirkDiggler/rpg-toolkit/blob/c355102dffe40fa6aa3852d13708ec21c055172d/rulebooks/dnd5e/encounter/capabilities.go#L83), [resolution/check.go:55](https://github.com/KirkDiggler/rpg-toolkit/blob/c355102dffe40fa6aa3852d13708ec21c055172d/rulebooks/dnd5e/resolution/check.go#L55)) · options: every roll in resolution, rolls split by what they are for · because: a decision is the driver's, not an interaction · [rpg-project#694](https://github.com/KirkDiggler/rpg-project/issues/694) · upheld: none yet
- 2026-10-27 · who decides what a viewer may see is postponed on purpose. Pre-playtest, everyone sees everything; in-game information and stories are shared across the party so all see the same thing; debug detail travels on its own RPC at its own level of detail. The range held open runs from RAW (you know what you saw or what a teammate literally told you; a trap described to you is never shown to you) to all see everything · options: build a visibility mechanism now, postpone it until a use case brings it · because: no mechanism is built for the range until a use case brings it · [rpg-project#694](https://github.com/KirkDiggler/rpg-project/issues/694) · upheld: none yet
- 2026-08-24 · the top-level encounter/ module was the early prototype, given up on purpose when the composable format arrived; the encounter moved under rulebooks/dnd5e once it was clear it could live outside that release tree, so a design that reaches for the prototype's shape reaches for something abandoned on purpose, not something lost · options: keep the prototype beside the composable encounter, retire it · because: the composable format replaced it · [rpg-toolkit#1220](https://github.com/KirkDiggler/rpg-toolkit/pull/1220) · upheld: none yet

## Findings

- none yet

## Friction

- none yet

## Open

- none yet
