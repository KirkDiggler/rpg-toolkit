# ADR-0041: A sighting carries what its channel knows, typed per channel

Date: 2026-08-22

## Status

Accepted

## Context

`session.Sighting` (and `Report`, the first-contact form of the same fact) hands
a host `Payload []byte`: "what the observer knows about it, encoded by the
composition." Today the only channel is `sight`, and the composition
(`rulebooks/dnd5e/encounter`) encodes `SightPayload{X, Y}` — the sighted
member's dungeon-absolute cell — as JSON. Session hands the bytes through
unread (pinned in `onemap_test.go`: intel's testimony is the composition's
encoding, not session's), and intel stores them opaquely and carries no
version (`encounter/data.go`, the room-local scar of #1044).

The first real client of `View` on the new stack (rpg-dnd5e-web#762, the 3D
route) needs the skeleton's position to draw it. The position is on the wire,
but only as bytes whose shape is a private agreement between one composition
and whoever reverse-engineers it. That is the situation ADR-0040
(atlas-carries-render-layout) already ruled on: *a rule a client cannot read
off the wire gets re-derived by experiment, once per client.* The proposed
shortcut — decode the JSON in the client behind one function and file the
proper change later — was rejected on the same grounds: a client dependency on
a non-contract is drag, and the session SDK's seams exist to stay honest.

More channels are coming (hearing, tremorsense, memory of a last sighting,
lit vs darkvision distinctions inside sight itself). Each tells the observer a
different *kind* of thing. Loose optional fields on `Sighting` would not say
which channel produced them; a single typed `Position` would say nothing about
why it is there or what else that channel knew.

## Decision

`Sighting` and `Report` gain a typed, channel-keyed sub-struct for each channel
the session SDK commits to, starting with sight:

```go
// Seen is what the sight channel knows about a subject. Present exactly when
// the sighting was produced by sight; nil for every other channel. A memory
// (CurrentVia empty) keeps the Seen it last had — that is the last-known cell
// a client draws a faded marker on.
type Seen struct {
    Position spatial.Position // dungeon-absolute; one map, no room
}

type Sighting struct {
    Subject    string
    Seen       *Seen   // sight channel's knowledge, typed
    Payload    []byte  // retained for channels the SDK has not typed
    Channel    string
    At         uint64
    CurrentVia []string
    Status     string
}
```

`Report` carries `Seen` the same way, so first contact and a held sighting
are the same fact in the same shape.

> **Implementation note, added during #1157/PR #1159 (Copilot review):**
> `Sighting.Seen` gates on real channel provenance — `intel.Holding` carries a
> `Channel` field, so `Seen` is nil for anything that is not literally
> sight-channel testimony. `Report.Seen` cannot do the same today:
> `intel.Report`/`intel.SurveilOutput` carry no channel of their own — a
> `Surveil` call is scoped to one channel, but that channel is not threaded
> onto each `Report` it returns — so `Report.Seen` is populated by decoding
> the payload and checking whether it parses as a sight payload, not by
> asking what channel produced it. That is equivalent to the real check only
> because sight is the only channel any composition in this codebase calls
> `Surveil` with today. Closing the gap needs `SurveilOutput` (or the percept
> it is built from) to carry its own channel — a `play/intel` change outside
> this PR's scope, tracked as a known limitation rather than silently
> papered over.

Mechanics follow ADR-0040 and rule S2: the **composition decodes its own
encoding** — `encounter` exposes its perception typed, and session's
projection copies it into `Seen`. Session never unmarshals payload bytes;
intel stays opaque storage. `spatial.Position` is the existing allow-listed
value type, so no inner type crosses the boundary.

> *(Shipped as `encounter.DecodeSightPayload(payload []byte) (spatial.Position,
> bool)` rather than a retyped `View` return — smaller, and `View` keeps
> returning `[]intel.Holding` unchanged. Either satisfies "the composition
> decodes its own encoding"; this is the one PR #1158 built.)*

Future channels add their own sub-struct (`Heard`, `Felt`, …) with the facts
that channel actually conveys. Sight-specific facts that arrive later
(lighting, distance band, partial cover) go inside `Seen`, not on `Sighting`.

The wire mirrors this: protos add `Seen seen` to `Sighting` and `Report`;
rpg-api transcribes; clients render from `seen` and never read `payload`.

## Consequences

### Positive
- The skeleton's cell is a contract, not an experiment. Every client of `View`
  reads the same typed fact.
- Channel provenance is in the type: a nil `Seen` on a `sight` sighting is a
  defect the projection test catches, not a client guess.
- Room for growth without churn: new channels and new sight facts have a home
  that does not touch existing fields.
- Session's passthrough pin survives unchanged; intel's opacity and
  no-version rule are untouched because the composition does the decoding.

### Negative
- One more projected field to keep complete (`convert_test.go`'s projected-pairs
  audit must list `Seen` and justify `Payload`'s retention).
- Three repos move in sequence (toolkit → protos → rpg-api → web) before the
  3D route can draw a monster.

### Neutral
- `Payload` remains on the wire for untyped channels. When every channel has a
  typed sub-struct it becomes baggage and is removed — no back-compat ceremony
  (Kirk's 2026-08-15 ruling).
- The `Sight` capability (range, line of sight, later lighting) is unaffected:
  this ADR is about how knowledge is *reported*, not how it is *gained*.

## Example

A fighter walks through the entrance-hall doorway; the composition's refresh
sees `skeleton-1` at (10,3):

```go
v, _ := mgr.View(ctx, session.ViewInput{Session: s, Member: fighter})
v.Sightings[0].Subject      // "skeleton-1"
v.Sightings[0].Channel      // "sight"
v.Sightings[0].Seen.Position // spatial.Position{X: 10, Y: 3}
```

A hearing channel, when it exists, would leave `Seen` nil and populate
`Heard{Direction, Loudness}` — the client draws a sound cue, not a figure.
