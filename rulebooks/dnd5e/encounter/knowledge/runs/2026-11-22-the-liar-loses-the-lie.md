# Run — 2026-11-22 — machine story the liar loses the lie

Slice [rpg-project#1110](https://github.com/KirkDiggler/rpg-project/issues/1110) of story [rpg-project#989](https://github.com/KirkDiggler/rpg-project/issues/989). Base: `rpg-toolkit` `main` at [`8018b6ae`](https://github.com/KirkDiggler/rpg-toolkit/commit/8018b6ae2d130ece8cc0946d68a4b4ab0d1a1f6b).

One line per entry. Every line ends with a link.

## Shipped

- The castle's `turn` line (the Ashford claim) requires `king-burned-village` and NOT `caught-lying`, so the one the captain caught lying is no longer offered it — [rpg-project#1110](https://github.com/KirkDiggler/rpg-project/issues/1110).
- The castle test and the read-back test say it; the compiled golden and content hash pin follow — [rpg-project#1110](https://github.com/KirkDiggler/rpg-project/issues/1110).

## Decided

- The straggler assertion was written: the straggler, whom he caught at nothing, is still offered `turn` from `backAtTheGate`, and the helpers read it — [rpg-project#1110 step 2](https://github.com/KirkDiggler/rpg-project/issues/1110).
- The castle hunk carries a `# Hand edit (rpg-project#989)` comment in the file's style — [rpg-project#1110 design](https://github.com/KirkDiggler/rpg-project/issues/1110).

## Found

- F1 to F6 each held at `8018b6ae` (F2 printed four lines: two requirement uses, the outcomes topic, one comment) — [rests-on comment](https://github.com/KirkDiggler/rpg-project/issues/1110#issuecomment-7084453080).
- The compiled golden diff is one change: `monsters[2].Options[5].Requires` goes from one entry to two, the second `Inner: {Topic: caught-lying}` — [rpg-project#1110 step 3](https://github.com/KirkDiggler/rpg-project/issues/1110).
- The content hash pin moves from `d3df3a4bfdf3b243c83c4e456ab668dee8e4159cad3fea0c9b03023095bc4650` to `1f93ad7bc9ecbe9613a93aa1cb474676bc166fe7792224d0a16dbbc21a85e32b` — [rpg-project#1110 step 4](https://github.com/KirkDiggler/rpg-project/issues/1110).
- Proof: before the hunk the new `backAtTheGate` assertion failed ("nor the Ashford line"); with the `not` removed again both it and the read-back test fail; restored, both pass — [rpg-project#1110 step 7](https://github.com/KirkDiggler/rpg-project/issues/1110).
- No existing test changed beyond the content hash pin constant; no castle test repeats the Ashford claim after failing it — [rpg-project#1110](https://github.com/KirkDiggler/rpg-project/issues/1110).
- Race run times: RemembersWhoLiedToHim 1.00s, TheCaptainGoesIn 1.49s (slowest), StrangersLie 0.53s, HoldsTheGateAgain 0.47s, LeavesTheHall 0.52s, each under the 5 s target — [rpg-project#1110](https://github.com/KirkDiggler/rpg-project/issues/1110).
- Whole module suite `go test ./... -count=1 -timeout 15m` passed before and after; `golangci-lint run ./...` reports 0 issues — [rpg-project#1110](https://github.com/KirkDiggler/rpg-project/issues/1110).

## Mistaken

- none observed — [rpg-project#1110](https://github.com/KirkDiggler/rpg-project/issues/1110).
