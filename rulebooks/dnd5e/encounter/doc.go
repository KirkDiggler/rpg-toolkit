// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package encounter implements the free-roam encounter composition.
//
// An encounter is a composition with an outcome (Setup → play → Outcome).
// This module is the courier between play/clock, mind/perception,
// play/record, and tools/spatial: it hands one perception pass the presences
// and the geometry, lets deciders act on their own holdings, and appends the
// story to record.
// Members exit, encounters close; player activity pumps the clock, the world
// thinks on the tick. Participation is supplied by the rulebook: Down writes
// the story beat, Contact decides sides, and Turn independently retains,
// auto-passes, or removes an initiative slot. Removed members keep their map
// placement and roster entry. Party defeat and whether a one-sided bubble must
// retain turn order are likewise supplied group policy, never thresholds
// inferred here; party defeat takes precedence.
//
// LOCATION KNOWLEDGE IS ENCOUNTER-OWNED. mind/perception holds channel-
// sourced testimony opaquely; this composition gives sight payloads their
// strict Known(position) or Unknown meaning. New payloads are tagged, legacy
// untagged coordinates remain readable as known, and malformed or current-
// unknown sight testimony is refused on load. Other channels' payloads
// remain uninterpreted.
//
// A fight-time [MonsterView] keeps current sight in Seen and held known
// locations in Remembered. Remembered members carry no concealed standing or
// reach fact, are never attackable, and have paths ending on the exact
// remembered cell. Held unknown testimony persists but is not actionable. A
// view is rebuilt after each driven move, so a visible-first driver such as
// behavior.Basic can abandon remembered pursuit for new sight on its next
// call.
//
// Only a successful fight-time driver move performs arrival correction: after
// sight refresh, the composition compares the mover's prior held Intel and
// arrival cell with that lawful complete percept. An absent subject remembered
// at the exact cell becomes Held + Unknown without exposing its concealed live
// position. Encounter-owned [IntelDelta] values surface the correction for
// persistence; public Step and the world's own round do not independently
// correct location testimony.
//
// The composition holds no rules of its own that it could hold instead.
// InitiativeRoller, Participation, and Sight are SUPPLIED at construction and
// consulted during play, never defaulted (rpg-toolkit#1033). The constructor
// fields retain their legacy Standing type, but the concrete value must satisfy
// StandingWithParticipation or construction returns ErrNoParticipation; binary
// Standing is never a fallback. Randomness, life-state participation and light
// remain facts this module asks for rather than facts it knows (C1).
//
// Every member is on exactly one clock (R6). The world tick is the default —
// free roam is not a mode, it is where you are when no fight has pulled you
// elsewhere. A fight is a turn bubble: Form pulls its members off the world
// clock into a caller-rolled order (R7 — initiative arrives from outside),
// Transfer moves a straggler in or out mid-round, EndTurn advances the fight,
// and Dissolve re-homes everyone to the tick. A fight also ENDS ITSELF when a
// supplied Remove leaves it without a Contact side — [ByDefeat], with no
// caller, the mirror of sight starting one. Everyone not in the fight keeps
// free-roaming while it runs; everyone in it is the fight's alone — Step is a
// world-clock verb and will not act for a fight member, and a fight member is
// not on the world clock for the world to think for. Which clock somebody is
// on is always askable, per member, via ClockOf.
//
// # Time on the world clock (rpg-project#465)
//
// The world clock advances ONLY BECAUSE SOMEBODY ACTS. A walk pays one round
// per PACE — every CellsFromFeet(SpeedFeet) cells, the remainder carried on
// the member and persisted. Every verb the turn clock would price as an action
// pays one round for its actor, after its outcome has landed: Intimidate,
// Persuade, Search, Unlock, Interact, Loot, RecordCast, RecordActivation. A
// fight round wrapping pays one round PER BUBBLE MEMBER, each naming itself.
// Standing still is free, said plainly: a party that talks to a goblin and
// waits sees nothing move.
//
// Every advance names its MEMBER as the driver, never a literal "world". The
// leaf accrues by driver as max rather than sum, so four players walking six
// cells together is one round and not four — and a driver called "world" would
// have made a long fight the front runner forever.
//
// When time passes, THE WORLD THINKS, inside the same call that raised the
// high-water and after the verb's own beats: every standing monster on the
// world clock with budget and a table is given one turn's worth of doing per
// unit, rolls its `time` table, and spends one. A creature that walks into an
// opposed member's sight forms or joins a fight by the ordinary path.
//
// # Concealment: the run composes its world (rpg-toolkit#1371, rpg-project#490)
//
// A field may declare CONCEALMENTS — one noun per secret, each with an id,
// the checks that find it, and the cells, doors and props it hides
// ([ConcealmentInput]) — and the composition acts on them. Who knows what is
// a journal of audience-scoped facts folded per member (world v0.3.0's
// journal + graph, seeded from the field at construction, persisted on
// EncounterData.World); two more capabilities arrive SUPPLIED, exactly when a
// concealment exists and never defaulted: [CheckResolver] rolls a find check,
// [Witness] answers who currently perceives a hidden door standing open. The
// laws, each pinned in concealment_test.go, conceal_test.go and
// conceallaw_test.go:
//
//   - [Encounter.Search] sweeps the concealments a region TOUCHES, one roll
//     per secret rather than per hidden thing; success is audienced to the
//     searcher alone, and NO output — not the answer, not the story, not the
//     blob — ever says whether there was something to find.
//   - ONE KNOWLEDGE MOMENT. A door carried its own find check and a region
//     its own flag until rpg-project#490, so finding a door revealed the
//     DOOR and the room behind it arrived only on perceiving that door open.
//     A concealment is one noun: finding it gives you its cells, its doors
//     and its props together. The causes are exemplary and not a closed set
//     — a search, an intel record, crossing a member door, stepping onto its
//     floor, perceiving a member door open, perceiving a creature standing
//     on it. PRESENCE PIERCES: an occupant of hidden floor knows it from
//     frame one.
//   - [Encounter.AtlasFor] and [Encounter.DoorsFor] answer as one member
//     under the never-authored yardstick: an unfound hidden door is absent
//     from every list and masked as an ordinary wall, whichever side or
//     sides of it are hidden (at the neighbouring run's height); hidden
//     cells, the props standing on them and the props the secret names are
//     byte-identical to never authored, and a region entry is TRIMMED to
//     what survives rather than dropped. A FOOTPRINT door may be a member:
//     it is withheld like a prop and the cells its rectangle stands on go
//     with it, which is how a rectangle with no crossing to mask keeps its
//     secret. Its BOUNDARY with visible space is not never-authored
//     (rpg-toolkit#1419): every crossing into hidden space — authored wall,
//     hidden door, or bare unwalled seam — reads as an ordinary wall,
//     because a wall a step cannot cross is the ordinary case and floor that
//     ends in nothing is the anomaly. Only a crossing wholly inside hidden
//     space, bordering no visible cell at all, stays withheld.
//   - The PROBE LAW: everywhere a door or prop id is spoken, a hidden one
//     the actor has not found answers not-found, byte-identical to an id
//     that names nothing. The MOVE LAW: a step stopped by one refuses
//     byte-identical to a wall.
//   - Reveals reach members as ONE recipient-scoped beat
//     ([BeatConcealmentRevealed] — the wire's CONCEALMENT_REVEALED), and a
//     hidden door's own state beats go to its knowers alone. The beat is a
//     PATCH for the recipient's cached atlas, so it carries the secret's own
//     slice AND the walls they did not have a moment ago — the segments
//     newly presented to them, and the cells nobody stands on
//     (rpg-toolkit#1480). A client draws walls from segments, so a reveal
//     without them opens a secret onto a room with no walls. THE FRONTIER
//     STOP (second-round ruling): a step whose destination lies on hidden
//     floor is delivered only to that secret's knowers and the mover — the
//     trail stops at the concealment boundary, resumes on reveal, and is
//     never backfilled. Everything else non-detection stays full data until
//     v1.0 (sight-scoped movement with last-known ghosts is the ruling's own
//     named follow-up).
//   - STATE IS REVERSIBLE; KNOWLEDGE IS NOT (second-round ruling).
//     Concealment never globally ends — there is no [graph.Reveal] in the
//     seeding, only per-member pierces — so a re-closed door is a wall to
//     strangers again, while every member who ever perceived it keeps a
//     visible shut door, and a mapped room stays mapped, forever.
//
// A field that hides nothing requires neither capability and sweeps
// nothing, which keeps every plain dungeon's blob exactly as it was.
//
// # Sides: the run composes ONE world (rpg-project#375)
//
// One journal and one graph exist from Setup and from Load, whether or not
// anything is concealed (world.go). The graph declares every faction — the
// reserved `party` and `monsters`, and the ones the field authors — every
// member belonging to its faction, and a hostile-to or allied-with edge per
// direction between every pair from the declared and default dispositions
// (disposition.go). [MemberKind] stays a kind; WHO IS OPPOSED is the graph's
// answer: formation, fightIsDecided and surprise ask whether a hostile-to
// edge stands between two members' factions, and under the defaults that is
// exactly "a player and a monster", the whole table this module ran on
// before factions existed. Knowledge is facts with audiences in the same
// journal — `known:door:`, `known:region:`, `holds:`, `known:fact:`, and
// `settled:<stance>:<pair>` — each folded per member; a record may reveal a
// FACT, and the flip a fact causes is the graph's own: a Raise on the fact
// and a pair-settling projection, folded as the mind, so a scout who reads
// the letter changes nothing. A fight whose two sides stopped being sides
// ends with [ByStance]; a member holding the letter standing in the chief's
// region teaches the chief (presence transfer, on the same sweep occupancy
// pierces by). Nothing stores a stance: it is derived on every question and
// every load from the declaration plus the facts.
//
// # A pair turns BOTH WAYS, and one law nobody authors (rpg-project#493)
//
// A disposition's `until` is not the hostile-only, fact-only predicate this
// charter once described. It turns a declared pair to THE OTHER of hostile
// and neutral, in whichever direction it was declared: a hostile camp stands
// down when its `until` holds, and a neutral camp turns on you when its own
// does. `until` on `allied` is refused — an allied pair has nothing to
// become — and one disposition carries one `until`, so authoring can never
// oscillate a pair.
//
// FOUR PREDICATE FORMS, TWO GRAINS. `{ fact }` holds when the faction's MIND
// knows it, which is the audience grain above and the only form a mind is
// needed for. `{ down }`, `{ round }` and `{ stance }` are the world's own
// truth, true for everyone the moment they happen, and nobody's mind has to
// learn them (turning.go). A `round` is a FIGHT's round and never holds
// outside one.
//
// THE TURN IS PUBLIC AND IT IS A FACT. `settled:<stance>:<pair>` is what a
// turn writes — one kind per pair per stance, neutral projected before
// hostile so a pair its own `until` stood down can be turned again by an
// attack (the betrayed truce). Stances stay derived: the settled fact joins
// the declaration, it does not replace it, and a blob claiming a turn this
// field cannot produce is refused on load.
//
// AGGRESSION IS A LAW, NOT AN AUTHORED TRIGGER (rpg-project#493 R3, rebound
// by its R5 — not the atomicity rule of the same name below). Hostile
// intent delivered by a member of one faction to a member of a faction it is
// NEUTRAL with turns the pair hostile — faction-wide, publicly, immediately,
// through the same settle and the same `stance` beat an `until` uses. No file
// writes "if attacked, become hostile", and no file can turn it off. Hostile
// intent is an attack roll, hit or miss, OR a cast that asks that member for
// a saving throw against a harmful effect, landed or not (turning.go); both
// land [DeedAttack] on the recipient, so a creature hurt by a spell testifies
// to it exactly as one hit by a sword does. An ALLIED pair is not
// turned: friendly fire is not betrayal in this cut. Because a turn makes
// strangers enemies where a later sight refresh would report only Refreshed,
// the stance site synthesizes that first contact and feeds it through the one
// formation path — precedence, surprise and straggler-join stay one set of
// rules (turning.go).
//
// A [KindWorld] MEMBER IS NOT A TARGET (rpg-project#493 R4). The attack verbs
// refuse one by name, before anything is appended, and say how to author a
// creature that can be attacked instead. It is the ref check in the words this module has:
// a `dnd5e:npcs:*` placed by the session becomes a member of Kind world, and
// this composition may not import the rulebook that would name the ref (C1).
// Nothing else is spared — a player may be attacked, and so may every
// monster. A world member is also in no faction, so no pair turns around it.
//
// # Atomicity, and what R5 does and does not promise
//
// Verbs validate before they mutate, and the first validation failure wins
// (R5). That covers the common case completely: a rejected verb never touched
// anything.
//
// It does NOT mean a verb's MUTATE phase is atomic. Join and Exit each perform
// several fallible steps after their first mutation — refreshSight and
// appendBeat both come after placement and member registration — so a failure
// late in a verb can leave the in-memory encounter partially changed.
//
// The clock verbs are the same: Form moves members off the world clock one at a
// time and Dissolve re-homes them one at a time, so a failure mid-verb can
// leave a member between clocks — a state ClockOf reports as a defect rather
// than guessing (see its on-no-clock check).
//
// Record is the same shape for a reason worth naming, since the verb looks
// atomic: it consults Standing AFTER appending its outcome beat, because that
// beat is the cause the consult reads (see [Encounter.Record]). A rulebook that
// cannot answer therefore leaves an outcome recorded and its consequences
// unworked.
//
// That is safe because of how this module is used, not by accident: every
// caller loads, acts, and saves, so a verb returning an error means the
// encounter is DISCARDED UNSAVED and the persisted world is untouched. There
// is no long-lived encounter to repair. Rolling back individual steps would
// buy nothing and would imply an atomicity the mutate phase does not have.
//
// The obligation this places on a caller is the whole of it: on error, drop
// the encounter. Do not save it, and do not keep using it.
//
// Design contract: docs/ideas/encounter/design.md (composition laws C1–C8).
// This is not a play/ leaf: the module composes published pieces and exposes
// one aggregate persistence pair at the host seam.
//
// Every cell of a field lives in one dungeon-absolute space (docs/ideas/
// encounter-anchoring/design.md; regions since rpg-project#256, ADR-0044),
// governed by these laws:
//
//   - W1 (one geometry per field) — every field is hex, under ONE declared
//     orientation ([CanvasInput.Orientation], required). The square family
//     left with the room chain (#256): a region is painted on a hex grid,
//     and a second family would be a second frame for every coordinate to
//     be wrong in.
//   - W2 (nothing claims a cell twice) — the floor is the union of the
//     regions' cells and [FieldInput.Scenery], and every cell of it is
//     claimed exactly once. A cell listed twice, in one region, across two,
//     in both a region and the scenery, or twice in the scenery, is refused
//     (ErrRegionOverlap); touching is legal, sharing a cell is not.
//     SCENERY IS THE FLOOR THAT BELONGS TO NOBODY (rpg-project#360): a wall
//     stands on it, a prop sits on it, a sightline crosses it whatever
//     [Void] says, it is in every member's map, and nobody's feet touch it.
//     Standing is what an OWNER grants MINUS what a wall takes away: a cell
//     a wall leaves too little of is in [FieldInput.Sealed], and keeps its
//     region, its lighting and its archetype while losing its feet. So a
//     seat, a step and an ending's trigger cell ask STANDABLE, which is
//     owned-and-not-sealed, rather than merely whether there is ground —
//     and the atlas reports [Atlas.Sealed] because membership in a region
//     stopped implying it.
//   - W3 (a door edge joins two adjacent floor cells) — every crossing
//     handed to this module, wall or door, has both endpoints on the floor —
//     a region's or the scenery's — and adjacent under the declared
//     orientation (ErrEdgeOffFloor, ErrEdgeNotAdjacent).
//     The envelope is implied, never written: a crossing from floor into
//     void is a crossing nobody can make, and [Void] already says whether
//     sight crosses it.
//     THE AUTHOR NO LONGER WRITES THESE (rpg-project#360). A wall is a LINE
//     between two picked positions, and the compiler derives the crossings
//     it blocks, the cells it stands on ([SegmentInput.Footprint]) and the
//     cells it seals. What arrives here is still pairs, because pairs are
//     what a canvas registers; what a client DRAWS is [Atlas.Segments], the
//     lines themselves. Both come from one authored line, so they cannot
//     disagree.
//   - W4 (projection is a read) — RETIRED by #1106, and worth stating as
//     history because the whole shape of this module used to follow from it.
//     Rules and verbs stayed room-local and absolute coordinates appeared
//     only where the module REPORTED a cell; that reporting set grew until
//     it was everything, at which point the room-local frame underneath had
//     no readers left. #1106 compiled the authored rooms into one canvas;
//     #256 deleted the rooms. There is one frame, and ONE conversion into
//     it: every authored [col,row] pair goes through [HexCellAt] exactly
//     once, at construction (compileField), and no caller ever adds an
//     origin — the room-local seam (#1139) ceased to exist rather than
//     getting fixed.
//   - W5 (world facts are construction data) — a region's archetype and
//     lighting are authored, REQUIRED, carried unread, and persisted; an
//     archetype NEVER decides a mechanic (ErrRegionArchetypeMissing,
//     ErrRegionLightingMissing). Validated identically at Setup and Load
//     through the one shared compileField.
//   - W6 (the field is one canvas) — the floor's bounding box fits in a
//     single origin-centred hex grid, which always widens to hold it.
//
// A REGION IS A NAMED SET OF CELLS (#1108, #256). It is how a dungeon is
// AUTHORED — [RegionInput] lists the cells it owns — and what the runtime
// answers in: [Encounter.RegionAt] says which region holds a cell,
// [Encounter.MembersIn] says who is standing in one, and [Encounter.Atlas]
// reports every region's cells, archetype and lighting beside the flat lists
// of props, walls and doorways. Membership is DERIVED from a member's cell
// wherever it is reported, never stored beside it.
//
// AND THE CANVAS ITSELF IS READABLE (#1114). Everything above DESCRIBES the
// map; [Encounter.Canvas] hands out the map, to read. It is the live room
// rather than a snapshot, so it goes out behind a view that refuses every
// write by name — see its own doc for why a copy is not an option and why a
// silent no-op would be worse than a refusal.
//
// Field size is bounded (maxFieldCells): a region is its cells, so the bound
// is on how many a field may list, which is also the size of the owner map
// this package keeps and the list the Atlas hands out.
package encounter
