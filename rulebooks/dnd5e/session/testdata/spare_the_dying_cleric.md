# Finalized Cleric fixture

`spare_the_dying_cleric.json` is the unedited `Character.ToData()` JSON produced
by the published D&D root `v0.168.0`, using `Draft.ToCharacter` with character ID
`cleric` and a fresh event bus. Session cannot own or import a bus, including in
tests, so finalization happens outside this module and the saved host input is
checked in. No known spells, resources, features or cast profiles were injected
into the saved character.

To reproduce, use `ClericFinalizeSuite.classInput` and `draft` from the root
`character/cleric_finalize_test.go` at tag `rulebooks/dnd5e/v0.168.0`. Replace the
Light choice with Spare the Dying; leave every other draft choice unchanged.
Finalize with `ToCharacter(context.Background(), "cleric", events.NewEventBus())`
and JSON-marshal `ToData()`. Creation/update timestamps will differ.

The root suite tests live draft finalization and reload. Session's
`TestSpareTheDyingPublicCastPersistsAndReplays` starts from this saved output and
tests public offers/casting, payment, stabilization, persistence, private status,
Story replay and stable turn handling against session's committed dependencies.
