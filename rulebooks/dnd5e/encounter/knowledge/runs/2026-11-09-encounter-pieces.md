# Run — 2026-11-09 — the rest as pieces, written when their mark moves

Leg (b) of [rpg-project#897](https://github.com/KirkDiggler/rpg-project/issues/897), PR 2 of 4, for story [rpg-project#879](https://github.com/KirkDiggler/rpg-project/issues/879). Base: `rpg-toolkit` `main` after [rpg-toolkit#2361](https://github.com/KirkDiggler/rpg-toolkit/pull/2361), which made an unchanged member encode to the same bytes.

## Shipped

- `SplitPieces` groups `ToCheckpoint`'s rest into pieces by rpg-project#770's shapes, marks each, and returns the head plus every piece whose mark differs from the held marks. A held key the rest no longer has comes out in `Removed`. Nil held marks return every piece. — [rpg-project#897](https://github.com/KirkDiggler/rpg-project/issues/897)
- `JoinPieces` rebuilds the rest from every piece a store holds. Its JSON is the rest's byte for byte. A missing head, roster, knowledge or journal piece fails with `ErrMissingPiece` naming it. A piece that does not decode, a key that names nothing, a member under another member's key, or an observer holding nothing fails with `ErrBadPiece` naming it. — [rpg-project#897](https://github.com/KirkDiggler/rpg-project/issues/897)
- `ErrRestCarriesRecord` refuses a rest, or a stored head, whose log carries entries. The record is the store's segments. — [rpg-project#897](https://github.com/KirkDiggler/rpg-project/issues/897)
- The pieces are `head`, `roster`, `knowledge`, `journal`, `m:<member>` and `o:<observer>`. `PieceKey.Shape` names the shape a key belongs to, so a store routes by it. — [rpg-project#897](https://github.com/KirkDiggler/rpg-project/issues/897)
- `TestMain` compiles the small keep's garrison beside the serial tests when a run selects a test that reads it. — [rpg-project#897](https://github.com/KirkDiggler/rpg-project/issues/897)

## Decided

- **The head is returned on every save, whatever its mark.** It carries the log head, which moves on almost every verb, and the store uses it as the checkpoint's existence sentinel. A store never has to ask whether the head came out. — [rpg-project#897](https://github.com/KirkDiggler/rpg-project/issues/897)
- **The head is the clocks, the log head, the site hash, the endings, the retention and the perception subjects' version.** These are about 3 KB and most move every verb. The doors, sight areas, interactions, reserve, outcome, pause and ever-members are the `roster` piece. Discovery and the sneak verdicts are the `knowledge` piece. Doors alone are 9 KB on the garrison, and they would ride every save in the head. This follows the map's keys, where these are the roster and knowledge hashes' `_` fields. — [rpg-project#897](https://github.com/KirkDiggler/rpg-project/issues/897)
- **An observer's sight sweep rides in its observer piece.** A sweep changes when its observer's holdings do, so the pair is written together. An observer piece exists while the observer holds anything or sweeps. — [rpg-project#897](https://github.com/KirkDiggler/rpg-project/issues/897)
- **The journal piece always exists.** A run with no journal writes the JSON null, so a store that lost the journal is refused by name rather than read as "nothing happened". — [rpg-project#897](https://github.com/KirkDiggler/rpg-project/issues/897)
- **The mark is FNV-1a 64 of the piece's bytes.** It is in the standard library, unseeded, so the same bytes give the same mark in every process and build, which a stored mark needs. `hash/maphash` is seeded per process and xxhash is a new dependency. A collision would skip one write; at 64 bits and about 200 pieces a save it is not a practical risk. — [rpg-project#897](https://github.com/KirkDiggler/rpg-project/issues/897)
- **Pieces are encoded with `encoding/json` v1**, as the host encodes the rest, so a joined rest is the rest's bytes. — [rpg-project#897](https://github.com/KirkDiggler/rpg-project/issues/897)
- **A test holds the split to `EncounterData`'s fields by reflection.** A field added to the rest and to no piece would be lost on every save in silence. `TestPieces_EveryFieldHasAPiece` fails instead. — [rpg-project#897](https://github.com/KirkDiggler/rpg-project/issues/897)
- **The small keep garrison's compile starts in `TestMain`**, as the 10k keep's launch already does. It is about 3 s under `-race`, and the first test to read it would otherwise pay it against its own 5 s budget. — [rpg-project#897](https://github.com/KirkDiggler/rpg-project/issues/897)

## Found

Small keep garrison, sight 8, the store carried forward save by save (`TestPieces_SavesWhatChanged`):

| verb | pieces written |
|---|---|
| launch, no marks | every piece: head, roster, knowledge, journal, 61 members, 61 observers |
| a step back | head, the walker, 7 observers |
| a pace that raised a round | head, 2 members, 21 observers, and the journal when facts were appended |
| a pace that raised no round | head and the walker |
| a step to a new cell | head, the walker, 7 observers |
| a player's exit | head, 45 observers; the leaver's member piece removed |

After every save the pieces the store holds join back to the rest byte for byte, and the pieces written are exactly the oracle's, which reads the two rests' own JSON.

10k garrison, posts asleep, 30 Moves (`BenchmarkSplitPiecesGarrison`):

| | measured | F26's expectation, live sight |
|---|---|---|
| split per save | 4.1 ms | 2.0 ms for the members and observers alone |
| split with no marks | 3.9 ms, 204 pieces, 472 KB | |
| Move with no round | 7.9 KB, 4 pieces: head 2.9 KB, walker, 2 observers | 25–35 KB with the 5.5 KB segment |
| round Move | 250 KB, about 99 pieces: 21 members, 77–78 observers, the journal on some | about 465 KB with the 140 KB segment |

- The split costs more than the map's 2.0 ms because it also encodes the head and the 124 KB journal on every save, and digests every piece. — [rpg-project#897](https://github.com/KirkDiggler/rpg-project/issues/897)
- This keep's sight is the test's own, so its perception is about 30% smaller than live. The non-round Move is under the expectation for that reason too. The api leg measures live. — [rpg-project#897](https://github.com/KirkDiggler/rpg-project/issues/897)

## Mistaken

- Six paces in the keep test cost 6.9 s under `-race`. Three paces still give a pace with a round and one without. — [rpg-project#897](https://github.com/KirkDiggler/rpg-project/issues/897)

## Verification

Mutants, each applied alone and restored:
- Return every piece, skipping the mark comparison: `TestPieces_SavesWhatChanged` fails at the first step, and `TestPieces_UnchangedRestWritesTheHeadAlone` fails. — [rpg-project#897](https://github.com/KirkDiggler/rpg-project/issues/897)
- Drop the sight sweeps from the observer pieces: the join no longer equals the rest at launch. — [rpg-project#897](https://github.com/KirkDiggler/rpg-project/issues/897)
- Never report a removal: the join after the exit no longer equals the rest, and the unchanged-rest test fails. — [rpg-project#897](https://github.com/KirkDiggler/rpg-project/issues/897)
- A stored piece that loses a byte fails the join with `ErrBadPiece` naming it, on the keep and on hand-built pieces. — [rpg-project#897](https://github.com/KirkDiggler/rpg-project/issues/897)

```
go test -race -count=1 -run 'TestPieces_' -v .
go test -run '^$' -bench BenchmarkSplitPiecesGarrison -benchtime 30x .
```
