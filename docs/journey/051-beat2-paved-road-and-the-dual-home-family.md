# Journey 051: Beat 2 — Three Verbs Paved a Road, and a Family of Bugs Introduced Itself

Beat 2's stated goal was small: make Dodge, Help, and Hide *do* something —
turn resolved verbs into real conditions instead of economy-only no-ops
(`rpg-toolkit#699`, `#716`). What actually happened is the three verbs forced
out a reusable spine for every future mechanical effect, and along the way we
tripped over a family of bugs we hadn't gone looking for. This entry is the
shape of both, plus the death-save arc that became the wave's proof-of-life
scene. Full ledger and retro: `rpg-project#75`.

## The three verbs that became a road

Dodge (`#729`) and Help/Hide (`#727`) look unrelated on the surface — one is a
self-buff, the other two are cross-entity effects with a check attached. But
building all three back-to-back exposed the same five-stage shape every time:

1. **Activation** — the verb's `Activate` constructs a condition (Dodge just
   had to construct `DodgingCondition`; the type already existed, incl.
   self-expiry — it simply never got instantiated before this wave).
2. **`ConditionAppliedTopic` publish** — the activation announces the
   condition to the bus.
3. **Registration** — `loader.go`/`factory.go` entries so the condition
   round-trips through `ToJSON`/`LoadJSON` instead of evaporating at the next
   RPC. We hit this directly: Hidden/Helped conditions worked in-memory in
   dev but vanished on reload until the registration was added.
4. **Ticks** — per-turn subscriber wiring so the condition actually expires or
   re-evaluates (see the turn-start section below).
5. **The event pipeline table → render** — toolkit topic → api translate →
   proto field → web dispatch. Every condition needs its own row in this
   table or it's invisible to the player no matter how correct the toolkit
   logic is.

None of this was designed up front as "the paved road" — it fell out of doing
the same five steps three times in one wave and noticing the third time that
we already knew every step before writing it. The road is now the thing a
Bless implementation (the Beat-3 acid test — see `rpg-project#75`'s close-out
comment) walks, not reinvents.

## `checks`: the second propose/resolve spine

Hide needed a Stealth check against observers' passive Perception. Rather than
inventing new roll machinery, `rulebooks/dnd5e/checks` (`checks.go`) mirrors
`rulebooks/dnd5e/saves` almost line for line: same roller + modifier +
chain-sourced advantage/disadvantage/bonus shape, same
`events.NewStagedChain` + `PublishWithChain` + `Execute` sequence, applied to
skill checks instead of saving throws. `MakeAbilityCheck` even fires
`AbilityCheckChain` when no subscriber exists yet this wave — the same "chain
exists so future ones can" bet `SavingThrowChain` already made. This is the
second time the propose/resolve-via-chain shape has been extracted as its own
package rather than folded into the verb that happened to need it first,
which is a decent signal it's a real pattern and not a one-off.

## Waking three dormant subscribers with one publish

`Encounter.seedActorTurn` used to only seed a player's action economy at
turn start. `dnd5eEvents.TurnStartTopic` had exactly one publisher in the
whole codebase (`turn_manager.go`) and **no live subscribers** — dead code
mirroring the already-live `TurnEndTopic`. Reviving that one publish
(`turn_economy.go:61`, mirroring `EndTurn`'s existing `TurnEndTopic` publish
in `combat.go`) woke **three** dormant subscribers at once, not the one we
were there for:

- `DodgingCondition`'s self-removal (the reason we were touching this file).
- Barbarian `RecklessAttackCondition`'s self-removal — untouched by this
  wave's own code, but its lifecycle silently started firing as a side
  effect. Worth a regression check, not a redesign.
- `UnconsciousCondition`'s auto-death-save roll (`unconscious.go`'s
  `onTurnStart`) — this is the mechanism behind the wave's own
  player-driven-to-0-HP playtest beat, and it's what makes the death-save arc
  below real instead of simulated.

The lesson that earned its own comment block in the code
(`turn_economy.go:19-30`): reviving one dead topic publish doesn't just fix
the bug you're looking at. Every condition that ever subscribed to that topic
and got silently ignored wakes up together. That's a feature once you know to
expect it and a landmine if you don't check what else was listening.

## The three-layer badge-clear gap

Getting a condition to *apply* and render was one path. Getting it to
*clear* — the badge disappearing when Dodging expires or Hidden breaks —
turned out to be a separate, transient-vs-permanent subscription problem
(`rpg-toolkit#734`, api `#622`/`#623`). Every other encounter-package bridge
(`subscribeAttacks`, `subscribeConditions`, `installTriggerBuffer`) is a
**transient capture window**: install a subscriber right before one verb
call, collect what fires synchronously, unsubscribe. That pattern doesn't
work for removal — a condition can self-remove at *any* future turn start,
unrelated to whoever applied it, or on a healing action targeting a downed
ally turns later. There's no single verb call to wrap. The fix
(`condition_removed_bridge.go`) is the encounter package's first **permanent**
bus subscription: installed once at construction (both `New` and
`LoadFromData`, so there's exactly one implementation), living for the
encounter's lifetime.

The three-layer part: even with the bridge in place, `StatusApplied` and
`StatusRemoved` for the *same* condition arrived with the ref in two
different string formats — one fully namespaced
(`"dnd5e:conditions:dodging"`), one a short keyword (`"dodging"`) — because
the two events were built from different call sites that had each picked
their own convention independently. A client matching removal to application
by ref would silently fail to correlate them. `normalizeConditionRef` exists
solely to make both sides agree. The lesson: an event *pipeline* (toolkit
topic → api translate → proto → web) isn't verified by checking each hop
compiles and fires — you have to check that the same logical value looks the
same on both ends of the pipeline, because nothing forces two independently
written publishers to agree on a wire format for the "same" ref.

## From ghost events to a rendered nat-20 revival

The death-save arc is the wave's best example of a feature that was real in
the toolkit for a while before it was real for a player watching the screen.
The sequence, in the order it became true:

1. `rpg-api#612` — player snapshot HP was never seeded at `AddPlayer` and
   never reconciled with the character store, so the HP bar and death gate
   were reading a value that didn't move. Fixed by seeding + reconciling
   against the *held* character (the same instance the `LoadFromData`
   cascade hydrates — see Journey 050), not a fresh snapshot read.
2. The turn-start revival above made `UnconsciousCondition.onTurnStart` fire
   for real, rolling death saves and firing `dnd5eEvents.DeathSaveRolledEvent`
   on the rulebook bus — but at this point that event was a **ghost**: it
   existed, it fired, and nothing on the broker side translated it. A death
   save could resolve a character back to 1 HP and the client would never
   know it happened.
3. `#742` gave death-save resolution a first-class broker event stream:
   `encounter/events/death_save_rolled.go`'s `DeathSaveRolledEvent` is the
   cause/narrative event, bridged from the rulebook event the same way
   condition-removed is bridged — per-viewer projection, published through
   the broker, given its own proto wire shape (`api#624`, `protos#175`).
4. `#733`/`#739` closed the loop on the other side of the gate: a downed
   player gets zeroed action economy at turn start (`char.EndTurn`, gated on
   the *live* `char.GetHitPoints()`, not the encounter's HP snapshot — a
   nat-20 revival can change the held character's HP synchronously during
   the very `TurnStartTopic` publish that triggered the roll, so reading the
   snapshot would still see the pre-revival value and wrongly zero a
   just-revived actor's turn).

End state, playtest-verified: a downed player's next turn start rolls a death
save live, a nat-20 heals them to 1 HP and un-zeroes their economy in the same
publish, and the whole arc — roll, result, stabilize-or-die, revival — is
visible on the client instead of being toolkit-internal state nobody
projected. Getting here took four separate PRs across two repos because each
layer (seed the HP, wake the roll, bridge the event, gate the economy
correctly against live vs. snapshot state) was hiding behind the one before
it.

## The family we didn't expect: snapshot vs. live

That last death-save fix — "read the live character, not the snapshot" — is
not a one-off. Beat 2 surfaced the same shape twice more, unprompted:

- `rpg-toolkit#736` — `Character.ArmorClass` is a static field that coexists
  with `EffectiveAC()`, an equipment-driven recompute. Two representations of
  the same fact, no single source of truth.
- `rpg-toolkit#740` — NPC targeting's downed-check reads
  `PlayerData.HP`, a snapshot that only refreshes at `ToData()` time, so a
  nat-20-revived player stays flagged "skipped" until the next full
  round-trip — the exact same snapshot-lags-live class the death-save gate
  above had to route around by reading `char.GetHitPoints()` directly.
- `rpg-api#612`'s original bug (unseeded, unreconciled snapshot HP) is the
  same class from the api side.

None of these three were filed as one investigation — they were found
independently while chasing Beat 2's actual goal, and only in retrospect do
they read as one family: **combat-mutable state has more than one home, and
every consumer has to independently discover which one is live.** This is the
same shape `rpg-api#596` (parked earlier this chapter) already named at the
architecture level. Beat 2 didn't resolve it — it just found three more
concrete instances of it in the wild, which is why the snapshot-vs-live
family is queued as a structural window for after the next planning session
rather than patched instance-by-instance again.

## What's left

The paved road (activation → topic → registration → tick → pipeline → render)
is proven three times but not written down as a how-to yet — the next class
implementation (Bless) is also the road's first real test of whether it holds
up outside dodge/help/hide's shape. The snapshot-vs-live family has three
known members and an open architectural question (`rpg-api#596` / parked
design PR `rpg-project#72`) about whether the fix is "pick one
representation" per instance or a structural rule. Both are deliberate
hand-offs to the next session, not oversights.
