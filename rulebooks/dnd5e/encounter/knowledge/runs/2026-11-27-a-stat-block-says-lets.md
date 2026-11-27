# Run — 2026-11-27 — NPC ability 2c a stat block says lets

Slice [rpg-project#1228](https://github.com/KirkDiggler/rpg-project/issues/1228) of story [rpg-project#1131](https://github.com/KirkDiggler/rpg-project/issues/1131). Base: `rpg-toolkit` `main` at [`d9a8cc1b`](https://github.com/KirkDiggler/rpg-toolkit/commit/d9a8cc1b).

## Shipped

- A `templates:` entry accepts `lets`, carried compiled in the author's words and left out of both tags when empty — [rpg-project#1228](https://github.com/KirkDiggler/rpg-project/issues/1228).
- The site check says it: kind `place.template.lets`, about `{template, <id>}`, one `{ability, <use>, lets}` connection per entry — [rpg-project#1228](https://github.com/KirkDiggler/rpg-project/issues/1228).
- Site `dungeonspec/testdata/template-lets/site.yaml` and tests `template_lets_test.go` in both packages — [rpg-project#1228](https://github.com/KirkDiggler/rpg-project/issues/1228).

## Decided

- The shape is judged at `templates.<id>.lets`, `[i]` and `[i].use`; the words are not — [rpg-project#1228](https://github.com/KirkDiggler/rpg-project/issues/1228).
- A value JSON cannot carry is refused at the entry's path `lets[i]` — [rpg-project#1228](https://github.com/KirkDiggler/rpg-project/issues/1228).
- The sentence is the issue's; the last sentence is removed when a behaviour can use an ability — [rpg-project#1228](https://github.com/KirkDiggler/rpg-project/issues/1228).

## Found

- F1 to F6 held at `d9a8cc1b`; F4: a template's said keys are the `"template"` entry of the key table in readback/notyet.go, to which `lets` is added — [first comment](https://github.com/KirkDiggler/rpg-project/issues/1228#issuecomment-7207119525).
- F5 printed the three castle pictures; none changed — [first comment](https://github.com/KirkDiggler/rpg-project/issues/1228#issuecomment-7207119525).
- The castle content hash MOVED, from `a354acad8480f3a9e56abc7175d6bdd2ea87069f2776842c9530eccacc1d1040` to `e337704d5b0373a5bf586a93004b3f9b51f1379f0982e66dc707321a9be32ae6`, because the hash encodes every field of the compiled types by name — [gap comment](https://github.com/KirkDiggler/rpg-project/issues/1228#issuecomment-7207166680).
- `TestSingleRoomActionAugmentStrictDecode` pins the list of a stat block's keys; the one word `lets` was added by amendment — [amendment](https://github.com/KirkDiggler/rpg-project/issues/1228#issuecomment-7207213566).
- The suite took 1m15s before any change and 1m08s at the final head, all passing — [first comment](https://github.com/KirkDiggler/rpg-project/issues/1228#issuecomment-7207119525).
- An empty stat block name prints no parentheses in the lets sentence — [rpg-project#1228](https://github.com/KirkDiggler/rpg-project/issues/1228).
- Proofs: no `omitempty` fails `TestEveryContentFileCompilesToItsCommittedPicture` for the 3 castle pictures and `AStatBlockWithNoLetsLeavesNoKeyInItsPicture`; not copying `Lets` fails `ACompiledStatBlocksLetsAreNotTheSpecs`; removing `lets` from the said keys fails `TestNotYetSuite/TestEveryKeyIsClassified` and `AStatBlocksLetsAreNotUnsaid` — [rpg-project#1228](https://github.com/KirkDiggler/rpg-project/issues/1228).

## Mistaken

- The brief did not list the test pinning a stat block's key list, so the first run stopped on a blocking gap — [gap comment](https://github.com/KirkDiggler/rpg-project/issues/1228#issuecomment-7207166680).
