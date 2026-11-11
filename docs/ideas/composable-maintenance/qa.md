# Composable maintenance — early look and QA

Owning checklist: [S3 #706](https://github.com/KirkDiggler/rpg-project/issues/706). This file distinguishes boot verification, operator viewing and later native QA.

## Scoped stack

- Environment: `local/composable-maintenance`; web `http://localhost:3042`, API `http://localhost:8142`.
- Detached served worktrees: API `a8463915`, web `970625f9`. No shared writable source checkout is served.
- Served API binary module information verifies root `678e9d74`, resolution `6618456d`, session `ccb082c9`, effects `de822f5e`, encounter `v0.196.0`, generated SDK `876f5202eb86`.
- Walk site: **`composable-maintenance-walk` / Composable Maintenance Walk**. Local-only cut of the castle front cells, guard template and appearances, with two stationary practice guards. No injected active effects.
- Source and in-container `/content` SHA-256 agree:
  - walk site: `459c4b747e524958195df7762eec41cc8cd9410ab3d65488736dfe4a075e63d4`;
  - served castle: `fc657abeef915f1ee942f61fc4fbbd9973554af73d9626d128efce2b26596aaa`.
- Browser landing rendered, HTTP 200, zero page errors. The real ListDungeons response includes the walk site and castle.
- `dev`, `bench` and unrelated local environments were not restarted or removed. Disposable stack startup's standard sandbox seeds are not acceptance evidence; use fresh identities below.

## Operator early look

Use `http://localhost:3042/?playerId=maintenance-cleric` for a fresh Life cleric (Bless comes from the normal domain grant). A second browser identity can use `?playerId=maintenance-bard` for a fresh Bard selecting Bane normally.

1. Create/finalize normally; inspect the actual acquired spell rows and private character view.
2. Create/join a lobby and select **Composable Maintenance Walk**. The guards are nearby; no castle navigation is needed.
3. Cast Bless on self/another creature, or Bane on the practice guards. Inspect provider-owned source/maintenance information and the private condition rail; reload and check it remains readable.
4. The castle remains selectable for the later standing fight/regression walk.

This is the early-look handoff, not a request to approve merges. Operator viewing precedes the joined independent review and full walk.

## Checklist state

- [x] Scoped stack boot and exact served source/dependency verification.
- [x] Walk and castle content hashes checked inside the running API container.
- [x] Browser landing and real dungeon catalog render without page errors.
- [ ] Operator early look.
- [ ] Joined independent review and explicit finding dispositions.
- [ ] Bounded two-client creation/acquisition/cast/private-read/reload/replacement/rest/departure QA.
- [ ] Relevant standing QA castle fight path.
- [ ] Explicit merge authority, provider tag verification and consumer re-pins.

Dice-sensitive damage/concentration outcomes have real resolver tests, including self-Bless before cleanup, one follow-up for two recipients and HP applied once. Later walk receipts must distinguish what the browser actually showed from those test-backed branches; no forced live rolls or repeated rolls for screenshots.
