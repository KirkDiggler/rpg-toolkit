# Run — 2026-11-28 — one way to pay S7d resolver pins the described Rage

Slice [rpg-project#1280](https://github.com/KirkDiggler/rpg-project/issues/1280) of story [rpg-project#1131](https://github.com/KirkDiggler/rpg-project/issues/1131). Base: `rpg-toolkit` `main` at [`07d7fe8f`](https://github.com/KirkDiggler/rpg-toolkit/commit/07d7fe8f).

One line per entry. Every line ends with a link: a PR, an issue comment, a commit, or a design section anchor. This file rides on the work PR, under the `knowledge/runs/` of the lowest directory that contains everything the PR changed. The ingest reads it once and adds a `processed:` line at the top; nobody else edits it after merge.

## Shipped

- The resolver pins `rulebooks/dnd5e v0.251.0`, where Rage is described and its own code retired; no production file changed — [pin commit](https://github.com/KirkDiggler/rpg-toolkit/commit/b0c819cf).
- A barbarian's own Rage resolves at the door from her sheet's description (`barbarian_rage_test.go`: charges 2 to 1, bonus action spent, raging on the saved sheet, one condition-applied fact; already raging gets no description; no charge is refused; level 20 spends none; the feature path refuses Rage) — [tests commit](https://github.com/KirkDiggler/rpg-toolkit/commit/e8b1bce2).
- The feature path's tests take Step of the Wind on a monk fixture beside `s.barbarian(n)` — [tests commit](https://github.com/KirkDiggler/rpg-toolkit/commit/e8b1bce2).

## Decided

- Suite result: before the pin green; after the pin alone red with one failing suite and eight failing tests, all by the feature path's refusal: `TestRagingReachesTheSheetThatWouldBeSaved`, `TestTheChargeIsSpentOnTheSameSheet`, `TestTheAbilitySpendsItsOwnSlot`, `TestAnAbilityThatRefusesIsAnErrorNotADoneInteraction`, `TestOffTheBusTheSameCallSucceedsAndAppliesNothing`, `TestNativeDamageSustainsRagingAcrossReload`, `TestRagingCompositionSurvivesInteractionReload`, `TestTheDoorTellsTheSameStoryAsTheOldRage`; at the final head green — [first comment](https://github.com/KirkDiggler/rpg-project/issues/1280#issuecomment-7234600725).
- Case (a), about raging, ride the description of her own sheet and assert what they asserted: `TestNativeDamageSustainsRagingAcrossReload`, `TestRagingCompositionSurvivesInteractionReload` — [first comment](https://github.com/KirkDiggler/rpg-project/issues/1280#issuecomment-7234600725).
- Case (b), about the feature path, move to Step of the Wind with no assertion weakened: the pool is `ki`, the condition is Disengaging, the sentence is "no ki points remaining". `TestAConditionAnAbilityPublishesReachesTheSheetThatWouldBeSaved` (renamed from `TestRagingReachesTheSheetThatWouldBeSaved`: its point is the publish reaching the saved sheet, not raging), `TestThePoolIsSpentOnTheSameSheet` (renamed from `TestTheChargeIsSpentOnTheSameSheet`), `TestTheAbilitySpendsItsOwnSlot`, `TestAnAbilityThatRefusesIsAnErrorNotADoneInteraction`, `TestOffTheBusTheSameCallSucceedsAndAppliesNothing`, and three that still passed but no longer held what they are about, `TestARefusalIsNotAMalformedActivation` (now also requires `ErrActivationRefused`), `TestActivationSuccessPreservesCollectorCleanupError`, `TestActivationErrorJoinsCollectorCleanupError` — [first comment](https://github.com/KirkDiggler/rpg-project/issues/1280#issuecomment-7234600725).
- F3's answer: Patient Defense puts no condition on its user (its event has no subscriber, toolkit#1986), Flurry of Blows banks capacity and publishes nothing, Reckless Attack is free with no pool and refuses off the bus, Bardic Inspiration conditions another; Step of the Wind has the pool, the bonus action, the condition on its user, the off-bus success and the refusal — [first comment](https://github.com/KirkDiggler/rpg-project/issues/1280#issuecomment-7234600725).
- Case (c): `TestTheDoorTellsTheSameStoryAsTheOldRage` retires; `TestTheDoorTellsWhoPutTheStatusOnAndWhy` carries what only it asserted as literals: the bus fact's source kind is `feature`, the saved condition's origin is Rage and her id — [first comment](https://github.com/KirkDiggler/rpg-project/issues/1280#issuecomment-7234600725).
- Test 3's error with no charge is `resolution: cost cannot be paid: "hero": rage_charges: 1 needed, 0 left` — [tests commit](https://github.com/KirkDiggler/rpg-toolkit/commit/e8b1bce2).
- Proofs, each restored with `git checkout -- <file>`: expecting her charges at 2 in Test 1 failed with expected 2, actual 1; expecting no error in Test 5 failed with "Received unexpected error: resolution: ability refused to activate: this ability is used from its description: build the action with Definition and pay its cost at the gate" — [tests commit](https://github.com/KirkDiggler/rpg-toolkit/commit/e8b1bce2).
- `ragesAtTheDoor` gains one sibling, `ragesAtTheDoorIn`, taking the world and cast, so her own rage and the 7b door share one way to run the door — [tests commit](https://github.com/KirkDiggler/rpg-toolkit/commit/e8b1bce2).

## Found

- Eight tests fail at the pin, not the twelve the issue counted; the other four resolve the feature path to a refusal and pass whether or not the ability worked — [first comment](https://github.com/KirkDiggler/rpg-project/issues/1280#issuecomment-7234600725).
- The feature path's refusal crosses the machine's boundary as a string wrapped in `ErrActivationRefused`: `errors.Is(err, features.ErrUsedFromItsDescription)` is false, so the tests hold its sentence — [tests commit](https://github.com/KirkDiggler/rpg-toolkit/commit/e8b1bce2).
- The new tests passed on first run, because the production they hold already exists at the pin; the proofs are what show they bite — [tests commit](https://github.com/KirkDiggler/rpg-toolkit/commit/e8b1bce2).

## Mistaken

- The first draft of Test 5 asserted `ErrorIs` with the feature's sentinel and failed on the string crossing; the assertion is now the sentence, as the Second Wind and Action Surge refusal tests hold it — [tests commit](https://github.com/KirkDiggler/rpg-toolkit/commit/e8b1bce2).
