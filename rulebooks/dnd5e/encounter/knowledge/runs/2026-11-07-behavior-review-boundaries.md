# Run — 2026-11-07 — behavior review boundaries

Follow-up within [toolkit #2293](https://github.com/KirkDiggler/rpg-toolkit/pull/2293), part of [live adoption #866](https://github.com/KirkDiggler/rpg-project/issues/866).

## Shipped

- Prepared corrections to held-claim load validation and notification selection ingress; the PR remains held and unmerged — [independent review](https://github.com/KirkDiggler/rpg-toolkit/pull/2293#pullrequestreview-5814805013).

## Decided

- Validate held work after restoring the pause, through the existing behavior claim owner, because a check against an uninitialized pause cannot protect the load boundary; no changed resume authority or deleted-claim inference — [load finding](https://github.com/KirkDiggler/rpg-toolkit/pull/2293#discussion_r4503620670).
- Reject Pick outside `time`, `turn` and the explicitly authorized `barred` reaction during shared canonical configuration validation, because only encounter knows action authority; generic Composer validation cannot make a notification executable — [ingress finding](https://github.com/KirkDiggler/rpg-toolkit/pull/2293#discussion_r4503620675).
- Keep runtime Observe refusal unchanged, never retry as Decide, and rebuild the old custom-key fixture so its clock notification prepares state and its next paid opportunity selects the custom key — [R6 review](https://github.com/KirkDiggler/rpg-toolkit/pull/2293#discussion_r4503620675).
- Defer nonblocking unused-definition diagnostics and stale entry-point prose to bounded follow-ups rather than expand the blocking fix pass — [unused definitions #2302](https://github.com/KirkDiggler/rpg-toolkit/issues/2302), [documentation #2303](https://github.com/KirkDiggler/rpg-toolkit/issues/2303).

## Found

- Eight fresh corruptions of a real held snapshot originally loaded without error: attachment, definition, state, key, completion beat, changed goal, absent goal and wrong completion kind; they now refuse before resumption while the original repeated-pause scenario still completes once — [load finding](https://github.com/KirkDiggler/rpg-toolkit/pull/2293#discussion_r4503620670).
- Six observation-only beats originally accepted action requests in direct construction and both concrete/expanded source; the new tests refuse all of those while accepting world, turn and barred selection — [ingress finding](https://github.com/KirkDiggler/rpg-toolkit/pull/2293#discussion_r4503620675).

## Mistaken

- The load guard was placed before `pauseFrom`, so a nil runtime pause made it unreachable; normal saved-walk tests exercised successful restoration, not corrupt-claim admission — [load finding](https://github.com/KirkDiggler/rpg-toolkit/pull/2293#discussion_r4503620670).
- Treating generic Composer construction as complete encounter validation missed the caller's notification/opportunity distinction; a source could compile and fail its first paid step — [ingress finding](https://github.com/KirkDiggler/rpg-toolkit/pull/2293#discussion_r4503620675).
