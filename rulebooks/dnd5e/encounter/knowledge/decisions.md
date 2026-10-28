# Decisions — rulebooks/dnd5e/encounter

One line per entry, dated and linked. Ingest is the writer; entries arrive through its reviewed PR.
A decision reads `- <date> · <the words> · options: <the options it closed> · because: <why> · <link> · upheld: <links, or none yet>`;
a finding, friction or open line reads `- <date> · <the words> · <link>`. A finding that needs more than a line links its case file in `findings/`.
Friction names both entries and never settles them.

## Decisions

- 2026-08-10 · the composable encounter is the foundation the rest is built on, and it is protected: the old way, an encounter that holds behaviour and computes answers itself, is not reached for again · options: the behaviour-holding encounter, the composable encounter · because: the composable format is the foundation the rest builds on · [rpg-project#676](https://github.com/KirkDiggler/rpg-project/issues/676) · upheld: the operator and the lead session have each reached for the old way since 2026-08 and were stopped (operator testimony, 2026-10-27); those PRs are not linked yet
- 2026-10-27 · C8 ("no randomness in the composition") is amended in daylight rather than restored: no random at all is too hard a line, and the direction is a roller seeded per encounter, so rolls are deterministic but random (controlled random); the amendment names its date, this ruling and the drift it legitimises, and AGENTS.md and the design say the roller exists · options: restore C8 and remove the roller, amend C8 to a seeded roller · because: the encounter has held an optional dice.Roller since [rpg-project#465](https://github.com/KirkDiggler/rpg-project/pull/465), so the line was already crossed · [rpg-project#694](https://github.com/KirkDiggler/rpg-project/issues/694) · upheld: none yet

## Findings

- 2026-10-28 · encounter AGENTS.md still says of randomness "There is no roller here and no place to add one" ([AGENTS.md L41](https://github.com/KirkDiggler/rpg-toolkit/blob/c355102dffe40fa6aa3852d13708ec21c055172d/rulebooks/dnd5e/encounter/AGENTS.md#L41)), while Capabilities holds `Roller dice.Roller` ([capabilities.go:83](https://github.com/KirkDiggler/rpg-toolkit/blob/c355102dffe40fa6aa3852d13708ec21c055172d/rulebooks/dnd5e/encounter/capabilities.go#L83)); the C8 amendment above is not yet written there
- 2026-10-27 · the audience of a beat is who witnesses it: members witness things, an ally can be seen getting shot, an NPC can protect. That stance is the game's own and tied to no D&D rule; pre-playtest it is "everyone sees everything" · [rpg-project#658](https://github.com/KirkDiggler/rpg-project/issues/658)

## Friction

- 2026-10-28 · where audienceFor came from: the operator's account (it exists because of the concealed door; when one member opened it, the others in the room saw it) and git ([rpg-toolkit#1219](https://github.com/KirkDiggler/rpg-toolkit/pull/1219) added it on 2026-08-23 as the pre-v1 "everyone sees everything" shelf for [rpg-toolkit#940](https://github.com/KirkDiggler/rpg-toolkit/issues/940), and it still returns the whole roster at about two dozen call sites; concealed-door witnessing is the separate Witness capability of [rpg-toolkit#1373](https://github.com/KirkDiggler/rpg-toolkit/pull/1373), and door beats left audienceFor in [rpg-toolkit#1923](https://github.com/KirkDiggler/rpg-toolkit/pull/1923)) disagree. Both accounts agree the witnessing intent is real; in git it lives in the sight-based sites. Both stand until the operator reads the git account

## Open

- 2026-10-27 · audienceFor deserves a look before any slice touches it again: witnessing reads as a general-purpose rule rather than a dnd5e one. Evaluate the other kinds of audience (sight-holders, the party sharing, hearing, the host's debug feed) and what makes audienceFor different from them · [rpg-toolkit#1219](https://github.com/KirkDiggler/rpg-toolkit/pull/1219)
