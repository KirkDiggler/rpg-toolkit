# Shared Raging — bounded native QA receipt

Recorded after the operator's early look, “raging is acting like normal.” Owning checklist: [S12 #884](https://github.com/KirkDiggler/rpg-project/issues/884). This is pre-playtest QA, not exhaustive certification or merge permission.

## Inputs and method

- Corrected stack: API `f843fdeeaad704e92557fa8e0107224c68976f31`, session `277ed80e`, resolution `e6e26a2d`, root `c00e0d76`; web `156a39bc9340fe0bb28f0cdc4a842f441e417047`; proto release v0.1.259 at generated `7dcb1b2`.
- Isolated `local/raging-composition`, web :3307/API :8307. Operator :3030/:8115 and existing characters were untouched. API restarts retained Redis.
- Fresh identities `raging-qa-20261108-a` and `-b`, created through production draft/update/finalize RPCs. This proves normal server-side character creation, not clicking every creation-wizard control. No active condition was seeded or character reset. Granted greataxe equipped through the public equipment API.
- Two separate browser contexts mounted the real client. Gameplay intents used its real authenticated RPC clients; rendered HUD, action details, logs and reload were inspected. It is not a claim that every intent was mouse-driven.
- Local walk site: `raging-composition-walk`, untracked courtyard/captain cut. Local and served file SHA-256: `660b20dd089b9cc4f7760fd84c267da7a54ec24d8e6c2d3f5a1e264e90c43115`.
- Castle repeat: actual `the-mad-king`; API-head file and container file both SHA-256 `d0ab3f6087e5fc4e5cea8e86ec03b94615eaa60e55392956abed0b603cbb1f2a`.

## Checklist evidence

| S12 check | Result and limit |
|---|---|
| Fresh Barbarian normally created | Both level-1 sheets had native Rage and no active Raging. Normal admission provided 2/2 Rage charges and 14 HP; no hand-authored condition/feature blobs were inserted. |
| Activate and pay normal class cost | Actor charge 2 → 1, observer remained 2. Saved data contained canonical ref, member_id, origin feature ref/entity ID, class binding and runtime. No cached level/bonus at the old top level. |
| Honest Strength melee information | Four distinct rows reached actual wire and rendered DOM. Only outgoing damage applied (+2); incoming resistance/save/check rows explicitly did not apply to the attack. All carried the actor's source ID without rekeying. |
| First boundary, read, reload | State fresh → raging; anchor 1, saw_turn_end true, turns_active 1. A full browser reload retained progress and spent charges. The corrected walk-site run also ended on the following inactive boundary rather than renewing grace. |
| Sustain, then end | An attack attempt, including a miss, set activity; the next boundary retained Rage at turns_active 2. Corrected castle hits also sustained it. Inactivity and explicit combat-end removal were observed; no stale active condition remained. Exact ten-round expiry and dice-dependent damage-only sustain use the named tests below. |
| Rest and separate budgets | Short rest healed the original injured actor 2 → 10, spent its hit die, did not refill its long-rest Rage charge, and did not affect the observer's pool. New-run admission restored 2/2 and 14 HP without recreating an expired effect. In the castle, another activation consumed the last charge; a zero-hit-die short rest removed the active Rage and left charges at 0. |
| Native readers, no client formula | The native HUD uses server ConditionView; action rows render server answers. Raw current condition JSON also crossed the character endpoint unchanged. The otherwise unmounted legacy ConditionBadge has focused component proof, not a claim it appeared in the combat HUD. |

The first walk session `4b770757-0ae5-41de-9699-5028ca878501` exposed the inherited damage-sustain defect [#2312](https://github.com/KirkDiggler/rpg-toolkit/issues/2312). Both clients received the same native Rage activation act `activate:01M6W9QMX5058S0S5WZK2YCDFN`. That pre-fix observation is not erased or relabeled as a pass.

Corrected walk-site session: `0bd650a9-687e-43cd-b2b2-653d42ffd319`. Corrected castle session: `1d601cc6-2f88-455c-b50f-1c4d657033a5`. Normal re-admission through a new lobby/run, not a direct database write, exercised class long-rest recovery.

## Relevant standing castle step

Repeated the affected fight path (standing QA step 4), not unrelated stealth, social and gate-authoring mechanics. The party began outside, neutral, entered through the front opening and attacked gate-1 from free roam. The blow killed gate-1 and formed the fight with the other visible living garrison members and both players. World tick advanced 3 → 4 once. No reserve joined during the two observed rounds; this is not an unlimited-arrival claim.

The real damage record for `attack:01M6WDZ1FM4TW7BNZDWM3TA81M` contains greataxe 7 + Strength 3 + Raging 2 = 12. The Raging modifier names the condition ref and actor source ID. A subsequent captain attack by the player recorded 8 + 3 + 2 = 13 (`attack:01M6WE58AF54MSKQ7D15FC8R99`). The castle uses the same native creator without a content change.

## Dice-dependent proof and focused closure

After the correction, native NPC attacks in the bounded recheck missed. No die outcomes were patched, reset or rerolled to manufacture an incoming hit. Under the walk skill's dice-dependent-row rule, positive damage-only sustain is **test-backed**, not claimed as a post-fix live hit:

- `resolution.TestActivationCalculationSuite/TestNativeDamageSustainsRagingAcrossReload`: real NPC strike, resistance, one HP application (15 → 12), no legacy damage notification, dirty saved hit flag, a new interaction sustaining once, and later inactivity removal. Failed on the old root; passes on c00e0d76.
- `conditions.TestRagingConditionTestSuite/TestRageSurvivesDamageThatDoesNotDropTheBarbarian`: grace consumed first; nil/zero/other-holder notifications do not sustain; positive post-apply DamageTaken does.
- `conditions.TestRagingConditionTestSuite/TestRagingConditionEndsAfter10Rounds` and the integration scenario `Rage ends after 10 turns (1 minute)` retain exact expiry; the lifecycle suite also retains late-anchor, backwards-clock and zero-HP cases. They are not a claim that a level-1 character survived ten live combat rounds.

Focused independent closure is published at [root c00e0d76](https://github.com/KirkDiggler/rpg-toolkit/pull/2299#pullrequestreview-5815981056) and [resolution e6e26a2d](https://github.com/KirkDiggler/rpg-toolkit/pull/2304#pullrequestreview-5815981097). Reviewer temporarily restored the legacy subscription: the targeted regression failed on the missing hit flag and `no_combat_activity` removal; restoring the exact fix made it pass. No new finding. Item fixture/pin adoption was inspected without restarting its review.

## Artifacts and limitations

Screenshots and real RPC transcripts are local under the design worktree's `.planning/`: `raging-qa-active-{0,1}.png`, `raging-qa-reload-0.png`, `raging-qa-castle-sustain-0.png`, `raging-qa-castle-rest-removes-0.png`, with correspondingly named JSON snapshots/results. Screenshots were opened and inspected, not merely captured; they are not uploaded GitHub image links.

The driver retained React StrictMode cleanup AbortError logs. One manually authored movement path was correctly refused for nonadjacent cells, then corrected; another stopped at the wall until the actual front opening was used. No production movement change followed. Some broad action detail text requires scrolling; no broad card redesign is claimed.

Item provenance/storage/transfer and non-Barbarian fixed +3 remain real-chain example proofs, not native inventory delivery. Review/QA did not authorize merging. The operator subsequently authorized the inside-out sequence; [released adoption](implementation.md#released-adoption) records actual tags, consumer pins, head gates and merges. The QA evidence and its limits above are unchanged by that release.
