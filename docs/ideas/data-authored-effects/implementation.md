# Implementation and executable evidence

The [accepted design](design.md) and [checked plan](plan.md) belong to
[project #946](https://github.com/KirkDiggler/rpg-project/issues/946).
This is **source implementation evidence, not a release**. Operator fixture view,
one bounded independent review and authorized release/tag adoption remain open.

## Source checkpoints

| Unit | PR | Verified source |
|---|---|---|
| Neutral output observations, supporting #950 | [toolkit #2402](https://github.com/KirkDiggler/rpg-toolkit/pull/2402) | `a1a45e426f416c1be1ec38d70dde93ab3fd0d5e5`; full module race/vet/lint and CI43313094673 pass |
| Rulebook API and executable cookbook, #946 | [toolkit #2401](https://github.com/KirkDiggler/rpg-toolkit/pull/2401) | Current head `6a7256b52623088c5a047c5fea2e5008d10656ff` adds only the package README to runtime head `97cad4a4`; runtime full race/vet/lint, focused cookbook and CI43320346757 pass. Current-head CI is reported on the PR |
| Design, plan and evidence | [toolkit #2399](https://github.com/KirkDiggler/rpg-toolkit/pull/2399) | Documentation, not runtime delivery |

Root consumes the real pushed neutral-provider version
`v0.5.1-0.20261112214314-a1a45e426f41`. It has no replace/go.work/source-copy graph.
An actual release tag must replace that pin during separately authorized merge-out.
`examples/items` is not imported by the root production or test graph; #947 remains
a separate deferred consumer, not an unfinished prerequisite.

## Run the rulebook examples

From the root implementation worktree:

```sh
cd rulebooks/dnd5e
go test -count=1 -v ./conditions -run 'TestAuthoredEffectsCookbookSuite|Example_dataAuthoredEffects'
```

The Go Example executes the real compiler/binder, paid source boundary, keepers,
chains and ending policies. Its captured output is checked exactly against:

```text
rage-ring: owner=A damage=12 physical=4 attacks=0/0 uses=0 ended=true
rage-scroll: owner=B damage=12 physical=4 attacks=0/0 uses=0 ended=true
physical-ward: owner=A damage=10 physical=4 attacks=0/0 uses=0 ended=true
damage-gift: owner=A damage=12 physical=9 attacks=0/0 uses=0 ended=true
kitchen-sausage: owner=B damage=10 physical=9 attacks=1/1 uses=0 ended=true
```

`damage` is the outgoing fold of a base 10 Strength melee weapon hit; `physical`
is settlement of incoming 9 slashing damage. `attacks` counts advantage sources
on the holder's eligible attack / an attack against the holder. These are real
chain contributions, not a claim that native dice, windows or browser animations
ran. The ordinary fixed +2 fixture requires no Barbarian level.

The single canonical content source is
[`conditions/testdata/effects/library.json`](https://github.com/KirkDiggler/rpg-toolkit/blob/97cad4a464faa18254ae2a58365ba60c89b2fa13/rulebooks/dnd5e/conditions/testdata/effects/library.json).
The five primary examples match the approved JSON in [examples.md](examples.md);
the same library adds a fire-ward variation to prove configuration changes without
another handler. The executable
[`Example_dataAuthoredEffects`](https://github.com/KirkDiggler/rpg-toolkit/blob/97cad4a464faa18254ae2a58365ba60c89b2fa13/rulebooks/dnd5e/conditions/authored_example_test.go)
uses the same small source and world helpers as the assertion suites.

## What the tests establish

- **T1:** captured frame values distinguish unknown from known-empty and deeply
  own parameters, guards and amounts; legacy held identity stays comparable.
- **T2:** independent damage/resistance/save-check advantage parts use existing
  meanings, validate supported configuration and own copied declarations.
- **T3:** activity flags, first-end grace and duration are separate; positive
  applied damage and missed attacks sustain only the correct holder. Rest,
  turn-start, zero-HP and actual last-member endings are explicit.
- **T4:** the whole authored library is validated, including unused entries;
  role binding is pure and produces copied application data with distinct local
  addresses even for self-use.
- **T5:** by/against-holder attack effects share assessment with execution;
  copied observer facts do not follow a live target. Missing/wrong authoritative
  configuration or generation refuses execution. Self-attacks do not double-list
  the target-side drawback in actor rows.
- **T6:** preflight catches foreign live and pending address collisions before
  payment; replacement is limited to its exact owner. Pending delivery, orphan
  refusal, reload, frozen declarations and generation-scoped ending remain valid.
- **T7/T8:** all examples run through public rulebook APIs, with a one-use test
  source backed by the existing resource primitive. The ring survives idle target
  turns and target unconsciousness; the scroll's recipient owns activity/grace/
  ten-end duration; source unavailability does not rewrite or refund a grant.
  Real HP application precedes one DamageTaken observation; the source user—not
  another recipient—owes the concentration follow-up. Tests choose a failure and
  deliver its actual consequence, without claiming native save resolution.

Positive and negative checks include unsupported fields/guards/roles, malformed
unused data, mutation isolation, unknown/empty observation facts, replacement,
independent sources, stale removal, real delivery refusal after payment and no
examples-module dependency. New code tests were observed RED before implementation;
T8's initial RED named the absent host-operation helpers. Existing full root tests
remained green after each task; final root race/vet/lint passed.

## Findings and implementation rulings

1. **A neutral observation was missing.** Bound exposed params and evaluated
   answers, not its complete authored guard/amount. S17/#950 adds read-only
   Output.Clone/Bound.Definition in its actual owner rather than duplicating
   descriptor state or weakening configuration checks. Root delegates its original
   copy helper to this API. Cost: a separate provider PR/tag-adoption step.
2. **One test used a ref as meaning.** The old target-held test called any linked
   ref concentration. It now supplies explicit known-empty detail and preserves
   its original no-bearing assertions; unknown configuration stays Depends.
3. **Self-attack information duplicated a drawback.** A RED assertion exposed
   target-side output also appearing in actor rows. Actor and target bearings now
   contribute their own rows once, agreeing with the actual attack chain.
4. **Stale removals are refusals.** The new test initially expected silent success;
   existing binding correctly returns a stale-generation error while preserving
   the new effect. The assertion now pins that existing stronger contract.
5. **The public fixture host was needed in T6.** Its creation moved forward from
   T7 to exercise real keepers during custody tests. It remains external test
   scaffolding, not a production item engine.
6. **Existing last-member proof is reused.** T3 extends the existing
   TestPartsSuite/TestUntilEmptyRequiresActualLastMemberLoss with the neutral
   reason rather than adding a duplicate named wrapper. The exported historical
   Go constant retains its name with the sole current value members_empty; no
   old-format reader or dual reason path is introduced.

7. **Package documentation follows the implementation.** After its content was
   complete, the package README moved from the design PR to the root PR so its
   relative source links open the actual implemented code. The design PR retains
   law, plan and evidence; there is no concurrent README writer.

## Gate limits and remaining work

- The workspace `make pre-commit` target fails in unchanged Core/Events coverage
  extraction/reporting: bc parse errors from mixed package output and reported
  Core coverage 78.0%. Core, Events and Makefile are unchanged from the baseline.
  This target was not bypassed or claimed green; owning root and provider gates
  passed separately, as the repository test guide requires.
- Genre checking remains advisory: 44 findings across 89 scanned files, exit1.
- **Pending:** operator view of the executable examples and public API, then the
  bounded independent joined review of #2401/#2402 and focused closure if needed.
- **Pending, separate authority:** merge/tag neutral provider, adopt the actual
  tag and rerun root gates/CI, then root/doc merge. No tags are predicted or made
  manually; neither implementation PR is merge-ready now.
- No native inventory/studio/API/proto integration, observer disclosure adoption,
  browser QA or production release is claimed. The existing operator walk stack,
  run data and open journal #863 are untouched.
