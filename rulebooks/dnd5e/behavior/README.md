# rulebooks/dnd5e/behavior — how a monster's mind is configured

This package decides what happens when the fight clock reaches a member
nobody is playing. It holds the turn drivers: the small reference one, and
the one that gives each monster the mind its sheet names.

What a mind is, and the fixed ladder it decides through, is the sibling
page: [mind/behavior](../../../mind/behavior/README.md). This page is
about where the knobs are.

## The two drivers

**`Basic`** is the reference driver and the floor. Attack the closest
standing player if one is in reach; otherwise take one step toward them;
if none is visible, step toward the closest remembered one; otherwise
pass. It never learns what a monster is, and it never holds a grudge.

**`Minded`** gives each member the mind its sheet names and drives it
through `mind/behavior`. It reads the encounter's view of the turn, builds
a situation, lets the mind judge and rank it, climbs the ladder, and maps
the answer onto the encounter's sealed intents (attack, move, pass).

`Minded` falls back to `Basic` on a member whose sheet names no mind, so
the two are not alternatives: `Minded` contains `Basic`. A word `Minded`
has never heard of is not a fallback — it fails loudly with
`ErrUnknownMind`, because a typo that silently produced basic behaviour
would look like a design choice.

`Minded` is stateful: it remembers which mind each member was given and
what each member calls whom. One of them serves one session, and it is not
safe for concurrent use.

## How a monster gets a mind

One word, said in three places, with no shared type between them.

1. **The definition names it.** `monster.Mind` is a field on the monster
   definition, set with `SetMind`. Today that is Go: the skeleton sets
   `MindRetaliator`, the thug `MindBerserker`, the goblin `MindCoward`.
   Every other monster names nothing and gets `Basic`.
2. **The word crosses at placement**, as an opaque string beside
   `Targeting`, onto the encounter's member record, and back out on the
   view a driver reads. Each member carries its own; nothing on the shared
   path invents one, which is why a player who is being auto-driven never
   picks up a monster's grudge.
3. **The driver looks it up** in its own registry, keyed by the same
   string.

The vocabulary is sealed on the definition side. `monster.ParseMind`
accepts exactly `"retaliator"`, `"berserker"` and `"coward"` and rejects
everything else, including the empty string — an author who wrote nothing
never calls it. It is the door for minds authored as data; nothing outside
tests calls it yet, because definitions are still Go.

Because the two sides share no type, `rulebooks/dnd5e/session/mind_test.go`
is the only place that can compare them, and it does: `TestMindWordsAgree`
pins each definition constant against the driver's registry key and checks
that an author may write either. That the driver does not import the
definition package is a rule, not an impossibility, so the test is what
keeps it honest.

## The Retaliator and its profile

There is one authored mind in this package. The three words are three
tunings of it, not three types.

`Retaliator` turns on whoever attacked it while that deed is still worth
answering, keeps whatever room it wants, and otherwise goes for the
closest standing player. Its four judgments: `Judge` attaches an attack
deed to the figure the deed names; `Name` is the member's own id; `Rank`
is the grudge, then live before remembered, then distance; `Keep` is
`Room`.

Its profile is two fields on the mind and one knob on the driver.

- **`Grudge.Patience`** — how many ticks a witnessed attack stays worth
  answering, counting from the tick it was confirmed. A tick is a fight
  round.
- **`Grudge.Excuse`** — what lets a currently *seen* attacker off before
  patience runs out. `ExcuseUnarmed` releases someone the mind can see
  holding nothing that could shoot back. `ExcuseNever` releases nobody.
- **`Room`** — how many steps of space it wants between itself and a live
  creature, read by the ladder's rung 0.
- **`Ranged`**, on the driver rather than on any profile, says whether an
  item id names a weapon that can shoot back. Nil means the rulebook's own
  weapon catalog. What a bow *is* is data about the world; whether a mind
  cares is the profile.

### The three presets

| Word | Patience | Excuse | Room | What it does on the board |
|---|---|---|---|---|
| `retaliator` | 3 | `unarmed` | 0 | Answers the shot for the round it lands and the two after. Lets go the moment it sees the shooter without something ranged in hand. The skeleton. |
| `berserker` | 10 | never | 0 | Comes for whoever shot it and nothing talks it off: swapping weapons does not help, and ten rounds is longer than the fight. The thug. |
| `coward` | 0 (none) | — | 2 | Holds no grudge at all. Backs away from anything that gets adjacent, and fights only when cornered. The goblin. |

Every number there is a feel number, tuned by a walk and derived from
nothing.

Three zero values carry meaning, and each is a claim rather than an
accident:

- **Patience is a span, not a maximum age.** Zero answers no deed at all,
  which is what makes `Grudge{}` an honest way to say "holds no grudge". A
  maximum age of zero would still answer a deed landed this very tick.
- **`ExcuseNever` is the zero value.** An author who set a grudge and named
  no release rule wrote down no way out of it. Nothing validates the field,
  so an unrecognised excuse excuses nobody, which fails closed on the
  grudge and is still a surprise if you expected the weapon rule.
- **A negative `Room` is no room.** Rung 0 keeps a creature it measures at
  fewer steps than this, and nothing is fewer than zero steps away, so a
  negative value behaves exactly as zero.

Because the profile a word means lives in this package and not on the
definition, tuning a monster is a new word and a toolkit release. That
cost is the argument for authoring minds as data, and it is being paid a
few times first on purpose, so the fields a profile has are known before
any format is chosen.

## What is not a profile field

How far a monster takes something. A leash — "stop chasing after thirty
feet" — and "narrow the attack to one target so a grudge is chased past
the fighter in front" are the ladder abandoning one rung for another, and
only a claim on the ladder may say that. A profile decides *who* a monster
cares about, never *where that sits* in the order of rungs.

Provocation is likewise unpaid. "An attack on me" is hardcoded today; an
attack on an ally, or a damage-only trigger, has no mind asking for it
yet. See
[`docs/ideas/mind/behavior/scenarios.md`](../../../docs/ideas/mind/behavior/scenarios.md),
which keeps the line between a profile and a claim.

## Authoring your own mind

`NewMindedInput.Minds` is the door. A caller adds to the vocabulary rather
than replacing it, and an entry under a built-in word wins — which is how
a game tries its own tuning without waiting for a toolkit release.

```go
// Watcher keeps three steps of room and otherwise takes the board's order.
type Watcher struct{}

func (Watcher) Judge(*mind.JudgeInput) (*mind.JudgeOutput, error) {
	return &mind.JudgeOutput{}, nil // it never bundles two holdings into one contact
}

func (Watcher) Name(in *mind.NameInput) (*mind.NameOutput, error) {
	for _, h := range in.Contact.Holdings {
		if h.Channel == perception.Sight {
			return &mind.NameOutput{Name: mind.Name(h.Subject), Named: true}, nil
		}
	}
	return &mind.NameOutput{}, nil // no word for it, so the ladder will not aim at it
}

func (Watcher) Rank(in *mind.RankInput) (*mind.RankOutput, error) {
	return &mind.RankOutput{Ranked: in.Situation.Contacts}, nil
}

func (Watcher) Keep(*mind.KeepInput) (*mind.KeepOutput, error) {
	return &mind.KeepOutput{Steps: 3}, nil
}

driver, err := behavior.NewMinded(&behavior.NewMindedInput{
	Minds: map[string]mind.Mind{"watcher": Watcher{}},
})
```

`mind` there is `mind/behavior`. Naming off the sight holding rather than
the first one matters: a figure first met as a ghost plus a deed would
otherwise be called by the deed's filing handle, which matches no member
the encounter will ever offer.

One limit to know: a host wiring through `session.Minded` cannot pass its
own minds today. That constructor takes an input that configures nothing
and builds the driver with the built-ins. A caller that wants its own mind
constructs `NewMinded` here.

## How to walk one

The local stack manifest is `envs/local/mind.env` in `game-dev`, which
carries the branch list and the step-by-step walk script for each of the
three words. The dungeon is rpg-api's `content/reference-minds.yaml`,
"The Three Minds": one monster per chamber so each mind can be met alone —
the goblin in the entrance, the thug in the hall, the skeleton in the tomb.

## Known walk finding

A fleeing coward orbits its pursuer instead of leaving the room
([rpg-toolkit#1758](https://github.com/KirkDiggler/rpg-toolkit/issues/1758)).
The mind is not at fault: rung 0 says back away and the board says where,
and the board's "where" is one greedy adjacent cell, which in an open room
is a circle. The fix is the encounter's, not this package's.
