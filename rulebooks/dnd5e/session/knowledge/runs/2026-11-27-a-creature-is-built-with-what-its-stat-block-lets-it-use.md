# Run — 2026-11-27 — NPC abilities S2d a creature is built with what its stat block lets it use

Slice [rpg-project#1229](https://github.com/KirkDiggler/rpg-project/issues/1229) of story [rpg-project#1131](https://github.com/KirkDiggler/rpg-project/issues/1131). Base: `rpg-toolkit` `main` at [`a5ccde44`](https://github.com/KirkDiggler/rpg-toolkit/commit/a5ccde44).

One line per entry. Every line ends with a link.

## Shipped

- `templateOf` hands a stat block's compiled `lets` to the rules as JSON, nil when none; a placed creature is stored with its kit and a full count — [slice issue](https://github.com/KirkDiggler/rpg-project/issues/1229).
- `CheckContent` returns `Templates []TemplateRefusal` for every stat block whose `lets` the rules refuse, in template id order, at `templates.<id>.lets` — [slice issue](https://github.com/KirkDiggler/rpg-project/issues/1229).
- Pins: `rulebooks/dnd5e` v0.243.0 and `encounter` v0.263.0; `go mod tidy` moved nothing else but eight `go.sum` lines — [slice issue](https://github.com/KirkDiggler/rpg-project/issues/1229).
- Walk site `lieutenant-holds-second-wind-walk.yaml`, uses a longsword (resolves) — [slice issue](https://github.com/KirkDiggler/rpg-project/issues/1229).

## Decided

- A launch with a bad stat-block `lets` is refused by the assembly as `ErrInvalidWorld`; `refuseLaunchContent` has no branch for stat blocks (operator amendment, way 3) — [Amendment 1](https://github.com/KirkDiggler/rpg-project/issues/1229#issuecomment-7207486451).
- The authoring echo is left unchanged — [Amendment 1](https://github.com/KirkDiggler/rpg-project/issues/1229#issuecomment-7207486451).

## Found

- F1 to F4 held (F2 prints four lines, as the first comment says); F6 passed at the old pins and the new — [Rests-on comment](https://github.com/KirkDiggler/rpg-project/issues/1229#issuecomment-7207431570).
- F5: `blockOf` never reads `sheet.Kit` or `sheet.Left`, and `DerivedBlock` has no field for them, so the echo shows nothing a stat block lets its creature use — [gap, not blocking](https://github.com/KirkDiggler/rpg-project/issues/1229#issuecomment-7207431876).
- The stored sheet's kit and counts: `{"kit":{"pools":{"second-wind":{"maximum":1,"restore_on":["short_rest","long_rest"]}},"grants":{"second-wind":{"ability":{"module":"dnd5e","type":"features","id":"second_wind"},"parameters":{"level":1},"pool":"second-wind","uses":1}}},"left":{"second-wind":1}}` — [slice issue](https://github.com/KirkDiggler/rpg-project/issues/1229).
- Refusals, content check: `stat block "gate-lieutenant": lets[0]: \`use\` is "fireball"; it is one of second wind`; `... lets[0] \`uses.max\` is 0; it is a whole number from 1 to 1000`; `... lets[0] \`cost\` is paid in \`charges\`, but the stat block declares no \`charges\``. A launch says the same sentence after `launch: monster "lieutenant": template "gate-lieutenant": invalid encounter data:` — [slice issue](https://github.com/KirkDiggler/rpg-project/issues/1229).
- Proof (a), no `Lets` in `templateOf`: Tests 1 to 3 fail ("Expected value not to be nil", `Grants()` nil, `Left` nil) — [slice issue](https://github.com/KirkDiggler/rpg-project/issues/1229).
- Proof (b), no template check in `CheckContent`: Test 5 fails (three cases, "should have 1 item(s), but has 0"); Test 6 still passes, because the assembly refuses at launch — [Amendment 1](https://github.com/KirkDiggler/rpg-project/issues/1229#issuecomment-7207486451).
- Proof (c) was dropped by the amendment — [Amendment 1](https://github.com/KirkDiggler/rpg-project/issues/1229#issuecomment-7207486451).
- Suite before (old pins) and after the pins: green; at the final head: see the PR body — [slice issue](https://github.com/KirkDiggler/rpg-project/issues/1229).

## Mistaken

- The brief expected a launch to say `ErrUnknownContent` for a bad `lets`, but the launch assembles stat blocks before its content check; the builder stopped and reported it, and the brief was amended — [gap](https://github.com/KirkDiggler/rpg-project/issues/1229#issuecomment-7207479717).
- The first commit holds a test file that did not compile until the next commit; the hook runs the linter only — [slice issue](https://github.com/KirkDiggler/rpg-project/issues/1229).
