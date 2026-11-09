# Resumable initial delivery

## Shipped

- Add a native resolver characterization: Bane's first recipient is persisted before a later recipient answers a saving-throw offer; this is a checkpoint on released root v0.214.0, not linked-runtime adoption — [S14](https://github.com/KirkDiggler/rpg-project/issues/904).

## Decided

- Stop the affected native lifecycle migration rather than bypass complete-link validation, postpone already-visible effects, or retain a historical concentration runtime; the open design decision belongs on the existing design PR — [design](https://github.com/KirkDiggler/rpg-toolkit/pull/2323).

## Found

- TestResistancePoseSuite/TestEarlierBaneRecipientIsPersistedBeforeLaterTargetAnswers passes on the existing resolver: target zero fails Bane and its dirty monster sheet carries Bane while target one is paused for Resistance — [S14](https://github.com/KirkDiggler/rpg-project/issues/904).
- The new root ApplicationBinding.CheckComplete refuses provisional recipients without a sealed owner, while dynamic post-seal recruitment is explicitly excluded; stable invocation identity alone cannot reconcile that with incremental native delivery — [checked plan](https://github.com/KirkDiggler/rpg-toolkit/pull/2323).

## Mistaken

- The checked provider/consumer plan named continuation identity but missed the persisted partially delivered application at a native pause boundary; source-local gates did not exercise that join — [design](https://github.com/KirkDiggler/rpg-toolkit/pull/2323).
