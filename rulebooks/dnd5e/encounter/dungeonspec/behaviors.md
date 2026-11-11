# Author behavior bindings

A version-5 site declares reusable data under `behaviorDefinitions`. Its ordered
`behaviors` bindings name document-local definitions with `use` and actor-local
attachments with `id`. The compiler materializes these into
`MonsterPlacement.Behaviors`; launch carries that value to the encounter, not the
source definition registry or a second runtime.

See the complete [two-guard site](testdata/behavior-patrol-consumer.yaml).

## Define once, supply values per actor

A finite sequence uses the production `mind/behavior.SequenceDefinition` shape:
`ref`, optional scalar `parameters`, explicit `end` (`repeat` or `stop`), and a
native `body` containing `recipe` and `table`. The body uses the component's
whole-value reference objects: `$item`, `$param`, `$next` and `$done`.

```yaml
behaviorDefinitions:
  patrol:
    ref: example:behaviors:patrol
    end: repeat
    body:
      recipe:
        initial: walk
        states:
          walk:
            transitions:
              - {on: time, do: [{pick: move}]}
              - {on: {$done: move}, to: {$next: true}}
      table:
        move: [{Weight: 1, Toward: {At: {$item: true}}}]
```

Each actor supplies its route without copying that definition:

```yaml
behaviors:
  - id: duty
    use: patrol
    items: [{q: 6, r: 1}, {q: 2, r: 0}]
```

Items are explicit site cells in the same axial frame as the floor. They are
validated and lowered once to canonical component items. Scalar `with` values
are checked by the definition's parameter metadata: no inferred defaults,
sampling or coercion. `false`, `0` and `""` are supplied values. A decimal such
as `1.0` is not an integer parameter.

## Attach a concrete concern

A non-sequence definition contains a native machine `recipe` and an optional
`table` using the existing authored row grammar (`when`, `toward`, `hold`, bundle
`use`/`with`, and source `{q,r}` cells).
Its reusable ref belongs to the recipe; its attachment ID belongs to the binding.
`items` and `with` on a concrete definition are refused, including explicit empty
collections. Definition fields cannot contain an attachment `id`, `blob` or
runtime `progress`.

Recipe fields use the machine JSON spelling. Concrete definition tables reuse the
source row/bundle compiler and retain their source references on round trip. The
opaque finite sequence `body.table` uses the behavior component's native template
spelling (`Weight`, `When`, `Toward`, `Hold`, etc.), as required by ExpandSequence.
There is no automatic grammar detection or runtime script language.

## Inherit or replace

A faction may declare `behaviors`. A placement that omits the field inherits its
faction's whole ordered list; an explicit list replaces it rather than merging by
attachment ID. `behaviors: []` means no roles. Source YAML/JSON round trips retain
that distinction and keep uses/values rather than generated graphs.

Attachment IDs must be unique within one actor. Two actors may use `duty` for the
same reusable definition; their materialized configurations and progress remain
independent.

## Bind physical state

`behaviorState` names the one attachment supplying physical body/post state. A
sole attachment is unambiguous; multiple roles with state-bound permissions must
name the source explicitly. `blockingStates` names its source states, and `posts`
maps those names to the existing post data (`door`, optional `at`, `against`,
`pass`). A `pass` is a bare fact (a password the mover knows) or the
requirement grammar a line's `requires` uses, read with the mover as "me" and
the post's holder as "the other party": `pass: { recognized: admitted }` lets
through whom the holder recognises (rpg-project#928). Door IDs are site-local and cells use `{q,r}`. These fields may accompany
faction or placement bindings. Replacing the behavior list does not inherit a
body binding onto unrelated replacement roles.

Sequence-local state names are lowered through the producer's source map, not by
parsing or authoring generated names. A local state repeated in several items
binds each corresponding materialized state. Other roles never become additional
physical permission authorities.

## Refusals and runtime boundary

The compiler refuses unknown uses, duplicate IDs, invalid scalar values, malformed
native definitions, off-floor targets and unknown body states at source paths.
Old actor `on`, `table` and `states` policy fields are refused, not normalized or
silently ignored. Weighted row-bundle tooling remains separate from attachment
identity and private progress.

World opportunities use `time`; fight opportunities use `turn`. A definition
intended for both names both. Clock notifications are not action opportunities.
Compilation neither rolls nor executes these inputs. Saved encounters retain
original materialized configuration and progress; loading a run does not consult
this source again.
