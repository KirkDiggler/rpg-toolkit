# Run — 2026-11-16 — studio today S7 records and secrets as hubs

processed: 2026-11-25T190427Z

Slice [rpg-project#973](https://github.com/KirkDiggler/rpg-project/issues/973) of story [rpg-project#966](https://github.com/KirkDiggler/rpg-project/issues/966). Base: `rpg-toolkit` `main` at [`f3deb20f`](https://github.com/KirkDiggler/rpg-toolkit/commit/f3deb20f).

## Shipped

- Two hub sections after Topics and Facts, "Records" and "Secrets", one group per record and per concealment, About `{record|concealment, id}`, with kinds `record.reveal`, `record.hold`, `record.tell`, `record.lift`, `secret.hide`, `secret.reveal`, `secret.find`, `secret.bar`, `secret.need`; both folds say each as its own sentence and a short table under "Held by / Told by / Lifts the hidden bar of / Revealed by / Found by / Bars / Needed by" — [rpg-project#973](https://github.com/KirkDiggler/rpg-project/issues/973).
- `place.intel` connects the secret it reveals (`{concealment, id, reveals}`) and `place.concealment` the doors and props it hides (`{door|prop, id, hides}`) — [rpg-project#973](https://github.com/KirkDiggler/rpg-project/issues/973).

## Decided

- A record that reveals a FACT gains no `reveals` connection: its existing `{fact, id, teaches}` already is the link, and a second fact connection would list the record twice in the fact's hub; the record hub links to the fact by that connection, and the fact hub's "Taught by: holding the record" entry links back — [rpg-project#973](https://github.com/KirkDiggler/rpg-project/issues/973).
- A secret connects only the doors and props it hides; the joins it bars connect to it with `hidden-by` and are listed under `secret.bar`, and cells have no id and are counted in the sentence — [rpg-project#973](https://github.com/KirkDiggler/rpg-project/issues/973).
- A hub groups its statements by kind in a fixed order, reading order within a kind, and the hubs' statements are not registered as topic or fact hub entries — [rpg-project#973](https://github.com/KirkDiggler/rpg-project/issues/973).
- Empty sections are said as the existing ones are: the hail site's golden gains `{"title": "Records", "groups": null}` and the same for Secrets — [rpg-project#973](https://github.com/KirkDiggler/rpg-project/issues/973).

## Found

- On the castle, the existing statements change in exactly two places (the `kitchen-passage` concealment and the `kitchen-way` record gain one connection each), checked by diffing the read-back JSON at main and at the head with the new sections removed; statements 683 → 711, JSON 421,818 → 435,861 bytes, `readback.Of` 198 → 200 ms (noise) — [rpg-project#973](https://github.com/KirkDiggler/rpg-project/issues/973).

## Mistaken

- Review of rpg-toolkit#2447: the hub repeats of ways kept `{join, through}`, `{bar, lifts}` and the DC-bearing methods, so a consumer finding ways by `through` saw each grate passage with 7 ways and a method total double counted; the told ways were listed as revealers beside the record; the search was listed once per join; and a mutant dropping the door `hides` connection survived. A way repeated under a record or secret hub is now a pointer (`{way, <id>, is}`), `secret.reveal` is the records alone, `secret.find` is said once from the concealment's own checks, and an internal test holds the door connection (the dialect cannot author a hidden door: `concealments.<id>.doors` is refused). Fact and topic hubs never repeated a way: ways are built outside the hub registration — [review](https://github.com/KirkDiggler/rpg-toolkit/pull/2447#pullrequestreview-5914676770).

- The first fold named a way by its hub kind ("the way `record.lift`"); the repeated statement now keeps the kind it was first said as — [rpg-project#973](https://github.com/KirkDiggler/rpg-project/issues/973).
