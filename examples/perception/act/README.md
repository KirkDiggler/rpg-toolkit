# act — writing a behaviour

A behaviour is one method.

```go
type Decider interface {
    Decide(s Situation) (Intent, bool)
}
```

It is handed one actor's own holdings and returns what that actor means to do.
Returning `false` is a decision too — it means this one does nothing, which is
what most things do most of the time.

## The constraint, and why it is the whole point

**A decider sees no world.** No roster, no positions, no truth surface, not even
another observer's beliefs. This package imports none of that and should not
start.

A decider that could see the world would act on the world, and every wrong
belief underneath it would stop mattering — the illusion, the stale memory, the
noise you never worked out. Those are the interesting cases, and they only stay
interesting if behaviour cannot see past them.

**You can only aim at a name.** `Intent.Target` is a `belief.Name`, never an
entity id, because an actor that could name an id would be reaching past its own
senses to do it. A target therefore exists only once the actor has perceived
something *and* worked out what to call it. `Situation.Named()` is that set.

The consequence worth having: a name that turns out to have been an illusion
binds to nothing when the composition resolves it, so the swing lands on empty
air. Nobody wrote a rule for that.

## What a Situation carries

Each `Held` is one track this actor holds, plus what they call it:

| field | means |
|---|---|
| `Name` / `Named` | what they call it. `Named` false is common and is not a defect |
| `Current` | a channel is delivering this **right now**. False is a memory — a ghost, or something carried in from an earlier run |
| `Channel` | which sense it came in on. Heard and not seen is a real and different state |
| `Locus.Where` | where that channel could place it. Empty means it could not — known to be there, not known where |
| `Observed` / `Confirmed` | when it was first perceived, and when it was last seen to still say the same thing. Staleness is your arithmetic |
| `Payload` | what the channel reported, opaque here — decode it with `content` |

`Situation.Named()` and `Situation.Current()` are the two filters nearly every
behaviour wants. `Contacts` is how the actor has grouped its own tracks: two
tracks in one contact means *this actor believes they are one thing*.

**Nothing in a Situation can tell you whether any of it is true.** That is not a
missing feature.

## Testing one

A behaviour is a pure function of holdings, so a test needs nothing but
holdings — no game, no projection, no world, no journal. `decider_test.go` is
the worked example: build `act.Held` values by hand and call `Decide`.

That is the cheapest part of this design and the reason to write behaviours
here rather than inside a composition.

## The worked ones

| decider | shows |
|---|---|
| `Idle` | doing nothing is returned, not inferred |
| `Aggressive` | attacks what it can perceive and name, however it found out |
| `Cautious` | will not commit to something it has only *heard* — one field's difference |
| `Wary` | attacks what it sees, goes to *look at* what it only remembers |
| `Cowardly` | counts only what it can currently perceive; flees with no target, because fleeing is not aimed at anything |

Read them together: they are the same situation reaching different answers, and
every difference is something the actor holds rather than something it was told.

## Deliberately absent

There is no disposition, no threat score, no memory of having been hurt, no
group awareness, and no pathing. None of it has been paid for yet.

If a behaviour needs something a `Situation` does not carry, that is worth
saying out loud before adding a field — the question is whether the actor could
plausibly *know* it, and a surprising number of behaviour wants turn out to be
requests to see past the senses.
