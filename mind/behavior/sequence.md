# Authored finite sequences: one Patrol, two routes

`ExpandSequence` is a **producer tool**, not an interpreter inside `Composer`.
It materializes one data-authored body over a finite supplied list and returns
ordinary `InstanceSpec`, vocabulary, completion mapping and source mapping values.
The [full Patrol source](testdata/sequence-patrol.json) and
[executable journey](sequence_patrol_test.go) are the example's canonical data
and assertions.

## What is authored

The definition supplies its `Ref`, scalar `Parameters`, required `End` policy
(`repeat` or `stop`) and a JSON `Body` containing `recipe` and `table`.
The recipe/table use their existing JSON shapes. The body omits a recipe ref:
the definition owns that identity once.

The Patrol body has one local state, `walk`:

```text
on time:             pick move
on completion(move): go to next item

move: weight supplied by parameter; Toward.At supplied by current item
```

The actual JSON expresses those values using four exact, single-member objects:

| Reference | Supplies |
|---|---|
| `{"$item": true}` | The current whole JSON item |
| `{"$param": "weight"}` | A declared, supplied scalar checked by `CheckParameters` |
| `{"$next": true}` | The next item's initial state; final item follows `End` |
| `{"$done": "move"}` | The completion beat for this attachment/item/local table |

Plain strings such as `"$item"` are literal. Inserted values are not interpreted
again. There is no string interpolation, script, sampler, nested sequence
interpreter or name-based Patrol factory. Structural recipe/state/table maps
remain authored source rather than replaceable program fragments.

## What each binding supplies

Load the source JSON into a `SequenceDefinition`, then call `ExpandSequence` for
each attachment. Both examples use ID `duty`, parameter `weight: 1` and the same
definition. IDs are local to the actor's composer.

| Actor | `Items` — canonical component positions | Expected completed-leg loop |
|---|---|---|
| West | `[{"x":0,"y":0},{"x":4,"y":0}]` | 0 → 4 → 0 |
| Court | `[{"x":10,"y":0},{"x":12,"y":0},{"x":14,"y":0}]` | 10 → 12 → 14 → 10 |

These are canonical `spatial.Position` values, **not studio axial coordinates**.
The existing content/spatial producer still owns coordinate lowering, floor
validation, catalog lookup and explicit defaults. Scalar values retain the
existing metadata/checker's strict kinds, bounds and absence rules. The finite
`Items` collection is supplied explicitly and must be nonempty; this component
does not claim a general domain-item schema or generation-quality framework.

## What expands

For West, the materialized state/table outline is:

```text
initial: item/0/walk

item/0/walk:
  on time: pick item/0/move
  on sequence/duty/item/0/move: to item/1/walk
item/0/move: weight 1, toward (0,0)

item/1/walk:
  on time: pick item/1/move
  on sequence/duty/item/1/move: to item/0/walk
item/1/move: weight 1, toward (4,0)
```

Court has a third emitted pair and its own coordinates. `StateSources` and
`TableSources` map generated names back to the item ordinal and authored local
name. Local names are escaped; state, table and bag references are item-qualified.
Explicit `$next` retains its reference identity even when an authored local name
happens to equal a generated destination. External fact and ordinary beat names
are not rewritten. A stop sequence's final `$next` goes to a compiler-owned
terminal, with no fictional authored source entry.

Each resulting instance retains the shared definition ref but has its own
materialized configuration identity. A `ComposerConfig` can contain it alongside
other ordinary instances in authored precedence order. Expansion validates the
result using the real composer constructor without rolling, reading external
facts, executing actions or advancing progress.

## Selection, actual completion and persistence

1. Give `Instance` and the returned `Vocabulary` to the real composer.
2. On an authorized opportunity, call `Decide` with lawful facts and a supplied die.
3. Execute the **already selected** answer once; selecting a target is not arrival.
4. When that selected work actually completes, find its beat in
   `Completions[pick.Key]` and deliver `ObserveInput{Instance: decision.Instance,
   On: completionBeat}` to the owning actor's composer.
5. Save the original materialized configuration and the composer's `ToData()`
   under their existing owners. Retain the completion mapping/original vocabulary
   contract needed by the caller. Reload with `LoadComposer`, not by expanding
   subsequently edited source again.

An old step's completion cannot advance a different current step. This is **not
an asynchronous job-token or cross-cycle deduplication guarantee**: the caller
must deliver actual current-work completion, not replay old results as new work.
The tool creates no per-item clocks: anchor and wake retain the existing
instance-wide machine semantics, and it performs no implicit resets.

## Paths the component test exercises

- Both loop lengths through production expansion, real machines and the real
  weighted picker, with exact d100 face 1 for each singleton selection.
- Selection versus actual completion; West's completion leaves Court unchanged;
  unknown attachment refusal and inactive-step completion.
- An independent authored investigation takes precedence with exact d200 faces
  **1 and 101**, then returns to West's unfinished `(4,0)` leg.
- No movement budget: hold, zero rolls, unchanged Patrol continuation.
- JSON reload: West resumes `(4,0)`, Court `(14,0)`; incompatible configuration
  is refused and load performs no binding or selection.
- [A second body](testdata/sequence-lines.json) says `alpha`, then `beta`, then
  terminates through the same expander without a movement word.

Run from `mind/behavior`:

```sh
go test -count=1 -v -run '^TestSequencePatrolSuite$' .
```

## Refusals and limits

`ErrBadSequence` names source/value/reference failures and wraps existing
parameter or compiler errors. Nil input returns `ErrNilInput`. Invalid end
policy, empty items, malformed source, unknown/mixed reference objects, missing
scalar values, invalid local references/rows and generated-beat collisions are
refused without partial output or input mutation. Outputs are detached.

Materialized configuration and compilation cost grow with the finite item list.
No performance claim at keep scale follows from these small fixtures. No existing
runtime implementation changes, and no studio/dungeonspec field, API deployment,
physical movement, spatial rejoin policy or live integration is supplied here.
The old position-derived `Selector.Path` remains separate and unchanged.
