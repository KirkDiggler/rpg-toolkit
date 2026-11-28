# Run — 2026-11-28 — second ability 5b-2 an ability that is only its price

Slice [rpg-project#1258](https://github.com/KirkDiggler/rpg-project/issues/1258) of story [rpg-project#1131](https://github.com/KirkDiggler/rpg-project/issues/1131). Base: `rpg-toolkit` `main` at [`4ed327b5`](https://github.com/KirkDiggler/rpg-toolkit/commit/4ed327b5).

## Shipped

- The gift machine (`gift.go`): `NewAction` returns it for a description that is `OnlyItsPrice`; its step lands nothing and reports `GrantedCapacity` and one `capacity-granted` effect naming the actor — [rpg-project#1258](https://github.com/KirkDiggler/rpg-project/issues/1258).
- `Input.validateOnlyItsPrice` in `resolve.go` refuses a payment that is not the description's own gift, as `ErrBadAction`, before anything loads — [rpg-project#1258](https://github.com/KirkDiggler/rpg-project/issues/1258).
- Nine tests through `Resolve` in `gift_test.go` (a monster, a character, all or none, the six refused payments, a free gift, Test 5's three refusals, a profile still winning, naming nobody else, given once per payment) — [rpg-project#1258](https://github.com/KirkDiggler/rpg-project/issues/1258).
- Two pins of `rulebooks/dnd5e`, v0.243.0 to v0.245.0 and then to v0.246.0; `go mod tidy` moved only that line and its two go.sum lines each time — [rpg-project#1258](https://github.com/KirkDiggler/rpg-project/issues/1258).
- ARCHITECTURE.md and AGENTS.md say what "only its price" means, why its payment is mandatory and what it reports — [rpg-project#1258](https://github.com/KirkDiggler/rpg-project/issues/1258).

## Decided

- The machine is `giftMachine` in `gift.go`, named for what it reports (a gift); `NewAction` asks `in.Definition.OnlyItsPrice()` and this module restates no condition — [rpg-project#1258 amendment 1](https://github.com/KirkDiggler/rpg-project/issues/1258#issuecomment-7231054332).
- The door's order for this machine: `start` at `resolve.go:421`, `payForMachine` at `:427`, `driveStep` at `:431`; a refused payment returns before any step, so nothing is given — [first comment](https://github.com/KirkDiggler/rpg-project/issues/1258#issuecomment-7231020656).
- Refusals, all `ErrBadAction`: "<ref> is only its price and carries none"; "<ref> is only its price and its price charges nothing"; "<ref> is paid by the one who uses it: <actor> uses it, <payer> was named to pay"; "<ref> gives <words> and its payment gives <words>" — [rpg-project#1258](https://github.com/KirkDiggler/rpg-project/issues/1258).
- The two-uses test builds its own captain with a pool of 2, because the shared fixture's pool holds at most 1 and loading refuses 2 — [rpg-project#1258](https://github.com/KirkDiggler/rpg-project/issues/1258).

## Found

- F6 at the first pin: build, vet and the whole suite pass; no ledger fake lives in this module — [comment](https://github.com/KirkDiggler/rpg-project/issues/1258#issuecomment-7231020656).
- F7: the only non-test hits are `cost.go:88` (builds a price) and `chargesSomething` (`resolve.go`, need not read GrantSlots: a gift is not a charge) — [comment](https://github.com/KirkDiggler/rpg-project/issues/1258#issuecomment-7231020656).
- `NewAction` validated the description first and the rules refused any with no profile ("action must populate exactly one profile") at v0.245.0, so the issue's "has no supported profile" was unreachable; fixed in the rules at v0.246.0 — [gap](https://github.com/KirkDiggler/rpg-project/issues/1258#issuecomment-7231032428).
- Test 6: an effect whose cost also gives an action runs as an effect: the heal lands (11 hit points), the action is given at the door (2 left), and the outcome reports `GrantedCapacity` empty and one `healing-applied` effect — [rpg-project#1258](https://github.com/KirkDiggler/rpg-project/issues/1258).
- Proofs: (a) validation switched off, the free-gift test and all six payment cases fail (each reported a gift not given, or failed to be refused); (b) an empty `GrantedCapacity`, the monster and character tests fail; (c) `NewAction` accepting any no-profile description changes no test, because the rules' own validation refuses those first, so the backstop is unobservable from here — [rpg-project#1258](https://github.com/KirkDiggler/rpg-project/issues/1258).
- Before the production change, every new test that runs the machine failed with "definition ... has no supported profile"; the suite passed whole before (1.0s) and passes whole at the final head — [rpg-project#1258](https://github.com/KirkDiggler/rpg-project/issues/1258).

## Mistaken

- The brief said a no-profile description reaches "has no supported profile"; the lead had not traced who refuses first, and the first build stopped on it — [gap](https://github.com/KirkDiggler/rpg-project/issues/1258#issuecomment-7231032428).
