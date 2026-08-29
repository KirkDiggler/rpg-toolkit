# Planted escapes

These files never build. `testdata` is invisible to the go tool, and that is
the point: they are the shapes `TestOnlyTheDoorWidensACastMember` exists to
refuse, kept as source so the pin can be run over them and shown to catch
them (`TestTheAliasEscapeIsClosed`).

Do not "fix" them. A file here that stops being an escape stops being evidence.
