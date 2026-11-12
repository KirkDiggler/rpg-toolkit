# Run — 2026-11-10 — Gate S2 castle copy: the castle's gate remembers

processed: 2026-11-12T165555Z

Slice [rpg-project#922](https://github.com/KirkDiggler/rpg-project/issues/922) of story [rpg-project#769](https://github.com/KirkDiggler/rpg-project/issues/769). Base: `rpg-toolkit` `main` at [`3cc83edf`](https://github.com/KirkDiggler/rpg-toolkit/commit/3cc83edf). Head: branch `feat/769-gate-s2-castle-copy`.

## Shipped

- The toolkit castle carries the api's gate content: root `front-gate` interaction, `guest` before `troupe` on the captain, gate-1 and gate-2, list `requires`, `advance` per verdict, warning lines on failure — [rpg-api#1218 at 9edb0fe9](https://github.com/KirkDiggler/rpg-api/pull/1218).
- The committed castle picture `world-builder-v4-castle.compiled.json` regenerated with `CONTENT_GOLDEN=write`; no other golden drifted — [plan on #922](https://github.com/KirkDiggler/rpg-project/issues/922).
- Castle routes `TestMadKingTheGateRemembersTheFailedClaim` and `TestMadKingTwoFailedClaimsArrestTheEntrance` cover checklist rows 1–4 on the compiled castle, through a reload — [#922 checklist](https://github.com/KirkDiggler/rpg-project/issues/922).

## Decided

- The toolkit file is the api file byte for byte, comments included, because the api copy was cut from this file and its hand-edit comments already cite #922 in the toolkit's style — [rpg-api#1218](https://github.com/KirkDiggler/rpg-api/pull/1218).
- `TestMadKingTroupeBlown` is rewritten as `TestMadKingTroupeWarned`: a failed troupe claim is now a warning that leaves the entrance `warned-troupe` with the guest claim open and nobody recognized, because the content no longer teaches `party-recognized` there; recognition at the gate stays proven by the failed `stand-aside` routes — [plan Task 7 on #922](https://github.com/KirkDiggler/rpg-project/issues/922).
- The arrest route asserts only the record and the menu, and that the garrison stays civil, because what `arrested` does to posts and bodies is Gate S5 — [#922 body](https://github.com/KirkDiggler/rpg-project/issues/922).

## Found

- Only one castle route depended on a failed troupe claim teaching `party-recognized`; every other castle test passed on the new content unchanged — [rpg-project#922](https://github.com/KirkDiggler/rpg-project/issues/922).
- The admitted hero still crosses gate-1's post on `troupe-admitted` after a warned guest claim, so the post's pass holds through the interaction — [rpg-project#922](https://github.com/KirkDiggler/rpg-project/issues/922).

- The two entrance routes run the whole castle gate with a reload, which puts them over the CI test budget (14.4 s and 8.1 s on CI, 5.2 s and 3.3 s locally); they are allowlisted with that reason in `.github/testbudget/rulebooks/dnd5e/encounter.allow`, sized by the castle rather than the case — [toolkit #2369](https://github.com/KirkDiggler/rpg-toolkit/pull/2369).

## Mistaken

- none observed — [rpg-project#922](https://github.com/KirkDiggler/rpg-project/issues/922).
