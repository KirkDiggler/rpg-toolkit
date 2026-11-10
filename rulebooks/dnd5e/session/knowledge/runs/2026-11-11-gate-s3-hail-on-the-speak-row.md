# Run — 2026-11-11 — Gate conversation S3 the hail on the Speak row (session leg)

Slice [rpg-project#923](https://github.com/KirkDiggler/rpg-project/issues/923) of story [rpg-project#769](https://github.com/KirkDiggler/rpg-project/issues/769). Base: `rpg-toolkit` `main` at [`ab61a7e5`](https://github.com/KirkDiggler/rpg-toolkit/commit/ab61a7e5). Provider: encounter at [`e7ca9e60`](https://github.com/KirkDiggler/rpg-toolkit/commit/e7ca9e60) (pseudo-version).

## Shipped

- Every walk step passes `Reach: InteractionReach`; a step that reports `Addressed` stops the walk on that cell, drops the rest, and answers `MoveOutput.Addressed` / `MoveRest.Addressed` — [rpg-toolkit#2371](https://github.com/KirkDiggler/rpg-toolkit/pull/2371).
- `Declaration.Address` `{guard, interaction}` on the addressed member's own Speak row while `Encounter.AddressFor` at `InteractionReach` says it stands — [rpg-toolkit#2371](https://github.com/KirkDiggler/rpg-toolkit/pull/2371).
- Launch refuses an interaction whose `approach.within` exceeds `InteractionReach` (`ErrInvalidWorld`, names the interaction) — [rpg-project#923 hail rulings](https://github.com/KirkDiggler/rpg-project/issues/923).
- `addressed` and `address-ended` are record-only beats — [rpg-toolkit#2371](https://github.com/KirkDiggler/rpg-toolkit/pull/2371).

## Decided

- One exported `Address` type serves the Move answer and the Speak row, not two shapes for one fact, because both carry the composition's `Addressed` as written — [rpg-project#923](https://github.com/KirkDiggler/rpg-project/issues/923).
- The Move says why it stopped short on `MoveOutput.Addressed`, beside `Barred` and `Formed`, and the act stream carries nothing new: the guard's line is already a table answer under the walker's Move act, and the address beats stay record-only — [rpg-project#923 How it works](https://github.com/KirkDiggler/rpg-project/issues/923).
- A hail on the walk's last cell reports `MovementCompleted` with `Addressed` set, because every requested cell was walked; earlier it is `MovementStopped` — [rpg-project#923](https://github.com/KirkDiggler/rpg-project/issues/923).
- `Addressed` is read before `Outcome` and `Formed`, so a hail whose answer also ends the run or forms a fight is still reported — [rpg-project#923](https://github.com/KirkDiggler/rpg-project/issues/923).
- `StopReason` stays empty for a hail: it is the text of an obstruction, and a hail is not one — [rpg-project#923](https://github.com/KirkDiggler/rpg-project/issues/923).
- The approach is refused at Launch only, the one place session hands authored interactions to the encounter; a stored run was refused at its own launch — [rpg-project#923 hail rulings](https://github.com/KirkDiggler/rpg-project/issues/923).
- The tests open the castle with the hail written in by exact-once text edits (front gate `approach`, the captain's `on: approached` row and line), because the castle the encounter ships at e7ca9e60 has neither; the edits fail loudly once the castle carries them — [rpg-project#923 Castle](https://github.com/KirkDiggler/rpg-project/issues/923).

## Found

- Walking up from the party start, the front gate's nearest carrier is the captain, not gate-1: he hails alice at (-6,10), standing at (-4,8) — [rpg-project#923](https://github.com/KirkDiggler/rpg-project/issues/923).
- The captain's `answered` beat for the hail reaches every recipient in the run, the cleric watching the road included, so bystanders listen in with no session change — [rpg-project#923](https://github.com/KirkDiggler/rpg-project/issues/923).
- One cell back down the road from the hail cell is past the approach (2) and inside Speak reach (3); without `Reach` on the step the next sweep ends the address there — [rpg-project#923](https://github.com/KirkDiggler/rpg-project/issues/923).

## Mistaken

- none observed
