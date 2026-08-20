# ADR-0040: The Atlas Carries a Render Layout, Named Apart From the Authoring Orientation

**Date:** 2026-08-21
**Status:** Accepted (Kirk, 2026-08-21 — "go with option 1")

## Context

`session.Atlas` is one flat map: a `Grid` family, every cell in axial
coordinates, props, boundaries, doorways. The composition's
`encounter.Atlas.Orientation` reaches the projection — it is what
`regionCells` uses to turn each authored rectangle into cells — and was then
deliberately dropped, with the reason recorded in `convert_test.go`'s
omission audit: *"this seam sends the CELLS, so a client never performs that
conversion."*

That reasoning is correct about **coordinates** and silent about **drawing**.
Axial `(q,r)` fixes the topology — the same six neighbours either way — but
not the picture: the same cell set laid out pointy-top and flat-top gives two
different images, roughly one rotated from the other. A client putting pixels
on a screen has to pick one, and nothing on the wire told it which.

The first client to try (rpg-dnd5e-web#758, reading the reference tomb from
a live server) guessed from the content and drew three chambers as a
**diagonal staircase**. Worse, the guess that *looked* right was wrong: the
tomb is authored `orientation: pointy` and, at the time, drawing it pointy
was incorrect — because `tools/spatial` had the two orientations running each
other's offset schemes, swapped identically in both directions so every
round-trip cancelled the error (rpg-toolkit#1140, root-caused and fixed as
#1141 → #1143 → #1145). With that fixed, the authored word and the correct
render layout finally agree. But the wire still did not say it. `GridKind`
is `square` or `hex` and stops.

## Options considered

1. **`Atlas.Layout HexLayout`** — a new string enum, `pointy_top` /
   `flat_top`, present exactly when `Grid` is hex, projected from the
   composition's `Orientation.Kind()`. Additive. `Grid` stays orthogonal.
2. **Fold it into `GridKind`** — `square` / `hex_pointy` / `hex_flat`. One
   field, no inconsistent state expressible. Rejected: it conflates the
   distance family with the drawing, turns every client's "is it hex" check
   into a two-value test, and changes a proto enum the web already switches on.
3. **Keep it off the wire and document the rule.** Rejected by the evidence:
   the web found its answer by measuring a bounding box. A rule a client
   cannot read off the wire gets re-derived by experiment, once per client.
4. **Change the projection so the words agree.** Already done (#1141). It does
   not close the gap — the wire still said nothing.

## Decision

Option 1. `Atlas` gains

```go
Layout HexLayout `json:"layout,omitempty"`   // "pointy_top" | "flat_top" | absent
```

projected by `projectLayout(encounter.Orientation)`: pointy → `pointy_top`,
flat → `flat_top`, nil (a square field, which declares no orientation by law)
or an unrecognised kind → empty, for the reason `projectGrid` gives — a guess
turns an impossible state into a wrong picture.

**The name is the decision.** The composition keeps `Orientation`: the frame
an author typed offset cells in, which this seam consumes and never hands
out. The wire carries `Layout`: what a client does with the cells it
receives. Same two values, a different question — and the staircase happened
precisely because the two questions shared a word. They now do not.

The convert audit (`TestEveryInnerFieldIsCarriedOrJustified`) learns the
difference between a rename and a drop: a `renamed` map records the inner
field, the outer field it became, and why the name changed, and asserts the
target field exists. A rename recorded as an omission would be a decision
hiding behind the wrong label.

## Consequences

### Positive
- A client can draw a hex atlas correctly from the wire alone. The next
  client does not repeat rpg-dnd5e-web's bounding-box experiment.
- Additive: compatible for every existing host; `gorelease` clean.
- The composition's law — hex must declare, square must not — is visible at
  the wire as "layout present iff grid is hex".

### Negative
- One more field a transport must mirror: rpg-api-protos gains a `HexLayout`
  enum on `GetAtlasResponse`, rpg-api translates it, and the web replaces its
  hardcoded `DEFAULT_LAYOUT = 'flat'` (which was pinned to the *bug's*
  answer and inverts now that the projection is correct).

### Neutral
- Two vocabularies for one underlying fact, by design. Anyone tempted to
  unify them should reread the Context.

## Rule

**A field on the wire is named for the question the receiver asks, not for
the one the author answered.** When the same value serves two questions at
two seams, give it two names so a reader cannot mistake one for the other.
