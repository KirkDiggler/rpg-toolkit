// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec

import (
	"fmt"
	"sort"
	"strings"

	"github.com/KirkDiggler/rpg-toolkit/mind/behavior"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/tools/spatial"
	"gopkg.in/yaml.v3"
)

// Compiled is an authored dungeon turned into a world, in two halves.
//
// # Why two halves and not one
//
// [Compiled.Field] is finished: hand it to [encounter.NewEncounter] as it
// stands. The rest is NOT finished, and cannot be here — a monster needs a
// sheet, and a sheet needs somebody who knows what "dnd5e:monsters:skeleton"
// resolves to. This package may not know that (design law C1), so the refs
// come out the far end as the same strings that went in.
type Compiled struct {
	// Key and Name are the provider-owned published identity. v3 uses the
	// root key and visual scene name; legacy compilation fills the spec values.
	Key  string
	Name string

	// Field is the compiled world: the regions as authored, the walls and
	// props at their absolute cells, the doors standing in their crossings,
	// and the ways out. Ready for [encounter.NewEncounter].
	//
	// THE EXITS LIVE HERE, at Field.Exits, rather than as a second list on
	// this struct. They are part of the world the composition runs — a
	// TriggerExitedHolding is compared against one — so a copy beside Field
	// would be a second thing to keep true, which is exactly what
	// [Compiled]'s two halves exist to avoid. Prop ids and takeability ride
	// along the same way, on Field.Props.
	Field encounter.FieldInput

	// PartyStart is where the party comes in, best seat first.
	//
	// A LIST, because a party is more than one person and the author declares
	// one cell. The first seat is the cell they wrote; the rest are the start
	// region's other free cells, nearest first, so a caller with four players
	// takes the first four and gets them standing together at the way in.
	// Never empty for a spec that compiled.
	PartyStart []Seat

	// StartFacing is the direction the party is looking when they arrive —
	// one of the eight true-compass names, or empty when the author stated
	// none (rpg-project#374).
	//
	// A CONVENIENCE BESIDE PartyStart, and deliberately not a second source
	// of truth: both are DERIVED at compile time from the one authored
	// `start:`, in the same pass, so there is no window in which they could
	// disagree. It sits here because a host reading the seats to place a
	// party wants the camera direction in the same breath, and reaching into
	// Field.Start for one word would be the awkward half of an otherwise
	// clean read. The field's copy is the one that SURVIVES — see
	// [Compiled.Field].
	StartFacing string

	// StartFacingDegrees is StartFacing LOWERED, once, through
	// [encounter.FacingFromCompass] (rpg-project#582, design R13): the
	// bearing a host hands to [encounter.JoinInput.Facing] when it seats the
	// party. Nil when the author stated none, which the encounter seats as
	// [encounter.UnstatedFacing]. The word stays beside it because it is what
	// the author wrote and what the atlas's start carries.
	StartFacingDegrees *float64 `json:"StartFacingDegrees,omitempty"`

	// Monsters is every authored monster, in the order the author wrote
	// them. The half that still needs a sheet.
	Monsters []MonsterPlacement

	// Intel is every authored knowledge record, in authored order, with its
	// id COMPILED (`<key>/<id>`) exactly as a door's is — see
	// [MonsterPlacement.Holds]. Nil when the file declares none.
	//
	// Also carried on [Compiled.Field] as the composition's intel table,
	// which is what reads a record's reveals when it changes hands. This is
	// the same list, surfaced here for a host that wants to show an author
	// what their dungeon declares without reaching into the field.
	Intel []encounter.IntelRecord

	// Concealments are the secrets this dungeon hides, in the order the
	// field carries them, with ids COMPILED (`<key>/<id>`) exactly as a
	// door's is (rpg-project#490). Nil when the file hides nothing.
	//
	// Also carried on [Compiled.Field] as the composition's own list, which
	// is the copy that survives a save. This is the same list, surfaced here
	// for a host that wants to show an author what their dungeon hides
	// without reaching into the field — [Compiled.Intel]'s reason, one root
	// declaration over.
	Concealments []encounter.ConcealmentInput

	// Factions and Dispositions are the sides this dungeon authored, in
	// authored order, with the predicates compiled to the composition's
	// triggers (rpg-project#375). Nil when the file declares none. Also
	// carried on [Compiled.Field], which is the copy that survives a save;
	// surfaced here for a host that wants to list what the dungeon declares
	// without reaching into the field.
	Factions     []encounter.FactionInput
	Dispositions []encounter.DispositionInput

	// Scenarios is the dungeon's scenario bindings, carried through exactly
	// as authored: scenario id to field key to the id it names
	// (rpg-project#368, design §3.1). Nil when the file binds none.
	//
	// OPAQUE, AND VALIDATED ONLY AS REFERENCES (design law C1). This package
	// checked that every value names something in the file and nothing else;
	// what the keys mean and whether the thing named is the right KIND is
	// the scenario package's own `New(cfg, compiled)`, in form-filler words.
	//
	// Deep-copied, so a caller mutating the map it gets cannot reach back
	// into the spec it was compiled from.
	Scenarios map[string]map[string]string

	// Endings are the endings this dungeon authored in the file itself
	// (rpg-project#375, R10), in authored order, each predicate compiled to
	// the composition's own trigger by [predicateOf] — ready to be declared
	// on [encounter.SetupInput.Endings] beside whatever the bound scenarios
	// declare. Nil when the file authors none.
	//
	// NOT ON THE FIELD, because an ending is not a fact about the building:
	// the composition takes its endings beside its field, and the run
	// persists them beside it too ([encounter.EncounterData.Endings]).
	Endings []encounter.EndingInput

	// Templates are the stat blocks this dungeon authored, keyed by the id
	// the author gave each one, carried through exactly as written
	// (rpg-project#555). Nil when the file declares none.
	//
	// THE HALF THAT STILL NEEDS THE RULEBOOK, for [Compiled.Monsters]'
	// reason. A placement whose ref is `dnd5e:monsters:guard` comes out in
	// Monsters unchanged; whoever turns refs into sheets reads this map to
	// learn that `guard` is a block this file authored. This package never
	// learns what a base resolves to (design law C1).
	//
	// NOT ON THE FIELD, because the composition never sees a stat block: a
	// member arrives already assembled.
	//
	// Deep-copied, so a caller mutating what it gets cannot reach back into
	// the spec it was compiled from.
	Templates map[string]TemplateSpec
}

// Seat is one cell somebody can be placed in, named the way
// [encounter.MemberInput] wants it.
type Seat struct {
	// Region is the ID of the region that owns the cell — derived from the
	// floor, never authored, and carried for a caller that wants to say
	// "she starts in the entrance".
	Region string

	// At is the ABSOLUTE authored cell, offset [col,row]:
	// [encounter.MemberInput.Position].
	At spatial.Position
}

// MonsterPlacement is one authored monster: where it stands, and every word
// about it this package is not allowed to interpret.
type MonsterPlacement struct {
	// Ref is content's identifier, exactly as authored.
	Ref string

	// AppearanceRef is optional visual content carried unread to observation.
	// It never changes Ref, sheet assembly, faction or placement. Legacy
	// dialects without this authoring field leave it empty.
	AppearanceRef string `json:"AppearanceRef,omitempty"`

	// Region is the ID of the region whose floor it stands on — derived.
	Region string

	// At is its ABSOLUTE authored cell, offset [col,row].
	At spatial.Position

	// Facing is the direction the monster faces when it arrives, one of the
	// eight true-compass names (n|ne|e|se|s|sw|w|nw), or empty when the
	// author stated none. CARRIED VERBATIM (rpg-toolkit#1899): the authored
	// word, as written. A v2 placement cannot author one (its `facing` is
	// refused there), so the field is empty for the legacy dialect.
	Facing string `json:"Facing,omitempty"`

	// FacingDegrees is Facing LOWERED, once, through
	// [encounter.FacingFromCompass] (rpg-project#582, design R13): the bearing
	// a host hands to [encounter.MemberInput.Facing] when it spawns the
	// sheet. Facing stopped being only a render concern when cones began to
	// read it, so the dialect converts the word and no host keeps a second
	// table. Nil when the author stated none, which the encounter seats as
	// [encounter.UnstatedFacing], 180.
	FacingDegrees *float64 `json:"FacingDegrees,omitempty"`

	// Targeting is the author's word for how it picks a target, or empty.
	// CARRIED, NEVER INTERPRETED.
	Targeting string

	// Actions is what this monster can do, in the author's order
	// ([PlaceSpec.Actions], rpg-project#448) — for a host to hand to the
	// session's SpawnInput.Actions when it spawns the sheet. Nil when the
	// author armed it with nothing, which means the definition keeps the
	// arms its stat block gives it.
	//
	// VERBATIM, and in the authored order, for Targeting's reason and one
	// more: the order IS the instruction. Both drivers take the first action
	// whose target is in reach, so sorting this list or deduplicating it
	// would quietly rewrite what the author said the monster does when you
	// close on it.
	Actions []string `json:"Actions,omitempty"`

	// AddActions appends weapons at the session assembly seam, after any
	// resolved action replacement. It remains authored carrier data here.
	AddActions []string `json:"AddActions,omitempty"`

	// DropActions removes weapons at the session assembly seam. It remains
	// authored carrier data here so dungeonspec never interprets rulebook arms.
	DropActions []string `json:"DropActions,omitempty"`

	// Boss is whether this is the monster whose death ends things.
	Boss bool

	// ID is the author's name for this placement, or empty when they gave it
	// none ([PlaceSpec.ID]).
	ID string

	// MemberID is the id this monster joins the encounter under — minted by
	// the compile ([mintMemberIDs]), never by a host, so every host that
	// launches this dungeon spawns the same members under the same names.
	//
	// THE AUTHOR'S ID, VERBATIM, WHEN THEY GAVE ONE: a placement named
	// `chief` is the member `chief`, which is what lets `factions[].mind:
	// chief` and `{ down: chief }` mean the same member in the run.
	//
	// OTHERWISE THE REF'S OWN ID PLUS A PER-REF ORDINAL — skeleton-1,
	// skeleton-2 — legible in a log, a beat and a client, which see nothing
	// else of a monster. Numbered PER REF so adding a prop or another kind of
	// creature cannot renumber one nothing about it changed, and an ordinal
	// is spent on EVERY placement of the ref, named or not, so naming one
	// skeleton `scout` does not renumber the one after it.
	MemberID string

	// Holds is the intel records this monster carries, by record id
	// ([PlaceSpec.Holds]) — for a host to hand to
	// [encounter.MemberInput.Holds] when it spawns the sheet. Nil when the
	// monster holds nothing.
	//
	// COMPILED RECORD IDS, not the author's, for the reason the compiled
	// door ids this replaced were: [intelOf] mints `<key>/<id>` so two
	// dungeons in one process cannot collide, and a holder carrying the raw
	// authored id would name a record the composition does not have.
	Holds []string

	// Faction is the faction this monster was placed in, AS AUTHORED
	// ([PlaceSpec.Faction]): empty when the author wrote none, which is the
	// reserved `monsters` faction — for a host to hand to
	// [encounter.MemberInput.Faction], whose empty means the same thing.
	// Verbatim, not key-prefixed: a faction is a word a member shows on the
	// roster and a scenario binds by name, like a region rather than a door.
	// Omitted from the committed pictures when empty, so every dungeon
	// authored before factions existed pictures byte-identically.
	Faction string `json:"Faction,omitempty"`

	// Options is the lines a character can say to this monster
	// ([PlaceSpec.Options], [RoomMonsterBinding.Options], rpg-project#569) —
	// for a host to hand to [encounter.MemberInput.Options] when it spawns
	// the sheet. Nil when the author wrote none, which means the monster
	// cannot be spoken to (R2). Compiled by [optionsOf]: every omitted
	// weight resolved, and `requires` compiled to the composition's own
	// trigger.
	Options []encounter.Option `json:"Options,omitempty"`

	// Table is this monster's authored policy — what it does, keyed by what
	// happened ([PlaceSpec.On] laid over [FactionSpec.On]) — for a host to
	// lay the rulebook's default for the monster's kind UNDER and hand to
	// [encounter.MemberInput.Table] when it spawns the sheet. Nil when
	// neither the placement nor its faction authored anything.
	//
	// TWO OF THE THREE LAYERS (design §1). The dungeon knows the author's
	// orders and not the rulebook's defaults — resolving a ref is a
	// rulebook's job and this package may not import one (C1) — so the host
	// completes the stack with one more [encounter.Layer] call.
	//
	// COMPILED, NOT CARRIED: an omitted weight is resolved to 1 here
	// ([tableOf]), so what the host hands over is a table every entry of
	// which states its own share.
	Table encounter.Table `json:"Table,omitempty"`

	// Temper is this monster's temperament: the word the author wrote on the
	// placement, else the faction's word, else the faction's MIX for the
	// composition to deal one from ([TemperSpec], design §3) — for a host to
	// fill the profiles on and hand to [encounter.MemberInput.Temper].
	//
	// THE PROFILES ARE NOT HERE. What a word MEANS is rulebook content, and
	// this package cannot import a rulebook; the host looks the numbers up
	// and fills [encounter.Temper.Profile] (or Profiles, for a mix) on the
	// way in.
	Temper encounter.Temper `json:"Temper,omitzero"`

	// Arrives is the predicate that brings this monster into the run
	// ([PlaceSpec.Arrives]), compiled to the composition's own trigger by
	// [predicateOf] — for a host to hand to [encounter.MemberInput.Arrives]
	// when it spawns the sheet. Nil when the monster stands there from the
	// first frame, and omitted from the committed pictures then, for
	// Faction's reason. A `{ down }` names the placement id, which is the
	// member id the host spawns that placement under.
	Arrives encounter.Trigger `json:"Arrives,omitempty"`
}

// Load decodes, validates and compiles a dungeon in one call.
//
// The three steps are separately available ([Decode], [Validate], [Compile])
// because their errors are about different things, and a tool that wants to
// lint a file without building a world should not have to build one. A
// validation failure is returned as a [*ValidationError] carrying every
// defect, and is an [ErrBadSpec].
//
// THE VERSION DECIDES THE DIALECT, AND THE SINGLE ROOM OWNS 3 AND ABOVE.
// Versions 1 and 2 are the region-chain dialect and stay on [Decode]; a
// version >= 3 is a site document and goes to [DecodeSingleRoom]. The version
// decides and the shape does not, which is what buys the refusal: a
// SITE-SHAPED document carrying a version this build does not speak is
// refused in the single room's own words ("unsupported version 5 (want 3 or
// 4)") rather than misread as a malformed v2 dungeon. This dispatch is
// deliberately BROADER than [acceptedRootVersions]: what the decoder accepts
// is the version seam's business, and routing the rest here is what makes the
// refusal about the version instead of about the shape.
//
// The rule has two measured edges, and each is the rule working rather than a
// hole in it:
//
//   - A v2-SHAPED document relabelled 4 is routed here too, and meets a
//     [SingleRoomSpec] shape error ("field name not found in type
//     dungeonspec.SingleRoomSpec") instead of the v2 decoder's "version 4,
//     which this build does not speak". A file whose version and shape
//     contradict each other has earned a shape error: the version is the
//     claim the document made, and it is held to it.
//   - A document with NO usable root version — the key missing, or 0 — has
//     made no site-document claim at all, so it still meets the v2 decoder
//     and fails in v2's words. The dispatch buys nothing for a file that
//     never said which dialect it is written in.
func Load(raw []byte) (Compiled, error) {
	var version struct {
		Version int `yaml:"version"`
	}
	if err := yaml.Unmarshal(raw, &version); err == nil && version.Version >= 3 {
		decoded, err := DecodeSingleRoom(SingleRoomDecodeInput{Source: raw})
		if err != nil {
			return Compiled{}, err
		}
		return CompileSingleRoom(CompileSingleRoomInput{Spec: decoded.Spec})
	}
	spec, err := Decode(raw)
	if err != nil {
		return Compiled{}, err
	}
	if errs := Validate(spec); len(errs) > 0 {
		return Compiled{}, &ValidationError{Errors: errs}
	}
	return Compile(spec)
}

// Compile turns a VALIDATED spec into a world.
//
// It assumes [Validate] has already passed and does not re-run it: the checks
// the composition makes at construction are the same rules, and a spec that
// skipped Validate is refused there instead — with the composition's words
// rather than the file's paths.
//
// # Nothing is generated
//
// Version 1 derived a layout (chambers in a row), a seam wall per chamber
// pair, and a doorway row. Version 2 derives nothing: every region, wall,
// door and prop is carried to [encounter.FieldInput] as the author wrote it,
// and the ONE place this package converts a coordinate — a door's edges,
// which [encounter.DoorInput] takes absolute axial — goes through
// [encounter.HexCellAt], the same conversion the composition runs.
func Compile(spec *Spec) (Compiled, error) {
	orientation, ok := orientations[spec.Orientation]
	if !ok {
		return Compiled{}, fmt.Errorf("orientation %q reached the compiler: %w", spec.Orientation, ErrBadSpec)
	}
	void, err := voidOf(spec)
	if err != nil {
		return Compiled{}, err
	}

	// THE GEOMETRY IS RUN ONCE, HERE, and everything downstream is handed the
	// answers (design C9, plan §0). The runtime never embeds a hex: it is
	// given the crossings as pairs, the sealed cells as a list, and the walls
	// as segments it draws and never measures.
	names := authoredOf(spec)
	derived := deriveWalls(spec, orientation, floorOf(spec, orientation), nil)
	doorEdges := doorCrossings(spec, orientation)
	// WHICH CROSSINGS ARE WAYS, derived once from the walls beside them —
	// what the concealment lowering asks to find a secret SUITE
	// (concealments.go).
	ways := waysOf(derived, doorEdges)

	field := encounter.FieldInput{
		Canvas:   encounter.CanvasInput{Void: void, Orientation: orientation},
		Regions:  regionsOf(spec),
		Scenery:  sceneryOf(spec),
		Props:    propsOf(spec),
		Walls:    wallsOf(derived, names, doorEdges),
		Segments: segmentsOf(derived, names, doorEdges),
		Sealed:   sealedOf(spec, orientation, derived),
		Doors:    doorsOf(spec, orientation),
		Exits:    exitsOf(spec.Exits, v2Exit),
		Intel:    intelOf(spec, orientation, ways),
		// WHAT THIS DUNGEON HIDES, lowered from the two words this dialect
		// spells it with (concealments.go, rpg-project#490). THE MERGE RULES
		// ARE THIS ADAPTER'S — a suite of hidden rooms is one secret because
		// one noun holds each cell once, and deciding that is the v2 reader's
		// job rather than the primitive's.
		Concealments: concealmentsOf(spec, orientation, ways),
		// The sides ride the FIELD too (rpg-project#375): the graph is seeded
		// from them at every Setup and Load, so they have to be where the
		// field is.
		Factions:     factionsOf(spec.Factions, placedMembers(spec)),
		Dispositions: dispositionsOf(spec.Dispositions),
		// The way in rides the FIELD, so it survives being stored: a start
		// kept only on Compiled would be lost the moment the dungeon was
		// saved, and a live session's map could never answer for it
		// (rpg-project#374). Authored frame, converted once by the
		// composition like every other authored cell.
		Start: &encounter.FieldStart{
			At:     spatial.Position{X: float64(spec.Start.At[0]), Y: float64(spec.Start.At[1])},
			Facing: spec.Start.Facing,
		},
	}

	start, err := seatsOf(spec, orientation)
	if err != nil {
		return Compiled{}, err
	}

	monsters, err := mintMemberIDs(monstersOf(spec, orientation))
	if err != nil {
		return Compiled{}, err
	}
	if monsters, err = lowerFacings(monsters); err != nil {
		return Compiled{}, err
	}
	var startFacing *float64
	if spec.Start.Facing != "" {
		if startFacing, err = lowerFacing(spec.Start.Facing); err != nil {
			return Compiled{}, &ValidationError{Errors: []FieldError{{Path: "start.facing", Message: err.Error()}}}
		}
	}

	return Compiled{
		Key:         spec.Key,
		Name:        spec.Name,
		Field:       field,
		PartyStart:  start,
		StartFacing: spec.Start.Facing,
		// The same word, lowered (StartFacingDegrees).
		StartFacingDegrees: startFacing,
		Monsters:           monsters,
		Scenarios:          scenariosOf(spec.Scenarios),
		Endings:            endingsOf(spec.Endings),
		Intel:              field.Intel,
		Concealments:       field.Concealments,
		Factions:           field.Factions,
		Dispositions:       field.Dispositions,
	}, nil
}

// endingsOf carries the authored endings through, each `when` compiled to
// the composition's own trigger by [predicateOf] (rpg-project#375, R10). Nil
// when none.
//
// ONE FUNCTION, BOTH DIALECTS (rpg-project#488, slice 1). `endings:` is the
// v2 root key reaching the single-room dialect unchanged — the same
// [EndingSpec] off the same shape, so there is nothing about an ending for a
// second dialect to spell differently. It takes the SLICE rather than a
// [Spec] for exactly that reason: a helper that names one dialect's root can
// only ever serve one dialect.
func endingsOf(endings []EndingSpec) []encounter.EndingInput {
	var out []encounter.EndingInput
	for _, e := range endings {
		out = append(out, encounter.EndingInput{Key: e.ID, Trigger: predicateOf(e.When)})
	}
	return out
}

// intelOf carries the authored records through, with ids compiled the way a
// door's is (`<key>/<id>`) so two dungeons in one process cannot collide.
//
// WHAT A v2 RECORD REVEALS IS A CONCEALMENT NOW, and the author still writes
// `door:` (rpg-project#490, E5). The engine's target moved from the door to
// the secret holding it, so this dialect resolves the authored door id to the
// concealment its own lowering put that door in — which is the whole of the
// retarget, because "the way into the vault" always meant the vault.
//
// A FACT IS NOT PREFIXED. It is a word a disposition's `until` and a
// record's `reveals` agree on within one file, and the composition matches
// the two by that word; a fact belongs to the story, not to a table.
func intelOf(spec *Spec, o encounter.Orientation, ways crossingWays) []encounter.IntelRecord {
	holding := concealmentHoldingDoor(spec, o, ways)

	return intelRecordsOf(spec.Key, spec.Intel, func(rev RevealsSpec) encounter.ConcealmentID {
		return holding[rev.Door]
	})
}

// intelRecordsOf is the minting itself, shared by BOTH DIALECTS
// (rpg-project#488): a record is a root declaration in either document and
// `<key>/<id>` is the one id the composition's tables are keyed by, so a
// second spelling of it is exactly the drift one grammar exists to prevent.
// Nil when the file declares none.
//
// WHICH CONCEALMENT A RECORD REVEALS IS THE DIALECT'S HALF, handed in as
// `conceal`: v2 resolves an authored `door:` to the concealment its lowering
// put that door in, and the single-room dialect mints its authored
// `concealment:` id the way it mints every other root declaration. Both
// answer the empty string for a record that reveals a fact instead, which is
// the zero value meaning exactly what it says.
func intelRecordsOf(
	key string, records []IntelSpec, conceal func(RevealsSpec) encounter.ConcealmentID,
) []encounter.IntelRecord {
	var out []encounter.IntelRecord
	for _, rec := range records {
		r := encounter.IntelRecord{ID: encounter.IntelID(key + "/" + rec.ID)}
		r.Reveals.Concealment = conceal(rec.Reveals)
		r.Reveals.Fact = rec.Reveals.Fact
		out = append(out, r)
	}
	return out
}

// intelHoldingsOf is the compiled record ids one holder carries — the same
// `<key>/<id>` minting, for [MonsterPlacement.Holds]. Nil when it holds
// nothing, so a creature that carries none pictures as it always did.
func intelHoldingsOf(key string, holds []string) []string {
	var out []string
	for _, id := range holds {
		out = append(out, key+"/"+id)
	}
	return out
}

// factionsOf carries the authored factions through (rpg-project#375): the id
// as the author wrote it, and the mind as the PLACEMENT id — a member id at
// the composition's seam, which is what a host that spawns the placement
// under that id makes it. Nil when none.
//
// THE SINGLETON DEFAULT IS DECLARED HERE. A faction of one has its member as
// its mind (design §2), and it is the COMPILER that says so, once, at
// declaration: the run never infers a mind from whoever happens to be
// standing in a faction, so a declared mind that falls or leaves is a
// faction that cannot learn (R7 — accidental succession is still
// succession). See [singletonMind].
func factionsOf(factions []FactionSpec, cast members) []encounter.FactionInput {
	var out []encounter.FactionInput
	for _, fa := range factions {
		mind := fa.Mind
		if mind == "" {
			mind = singletonMind(cast, fa.ID)
		}
		out = append(out, encounter.FactionInput{ID: fa.ID, Mind: encounter.MemberID(mind)})
	}
	return out
}

// singletonMind is the id of a faction's one creature, or "" when the faction
// has none, several, or one with no id to name.
func singletonMind(cast members, faction string) string {
	only := ""
	n := 0
	for _, m := range cast.all {
		if !m.isMonster() || m.side() != faction {
			continue
		}
		only = m.id
		n++
	}
	if n != 1 {
		return ""
	}
	return only
}

// dispositionsOf carries the authored dispositions through, each until
// compiled to the composition's own trigger by [predicateOf]. Nil when none.
func dispositionsOf(dispositions []DispositionSpec) []encounter.DispositionInput {
	var out []encounter.DispositionInput
	for _, d := range dispositions {
		di := encounter.DispositionInput{Between: d.Between, Stance: encounter.Stance(d.Stance)}
		if d.Until != nil {
			di.Until = predicateOf(d.Until)
		}
		out = append(out, di)
	}
	return out
}

// predicateOf is THE ONE PLACE the designer's spelling becomes the engine's
// type (rpg-project#375, design §2): every form of the grammar compiles to
// one of the composition's sealed triggers, for an `until`, an `arrives` and
// an ending's `when` alike. A `{ down }` names a placement id, carried as the
// member id the host spawns that placement under.
//
// The nil case is unreachable after Validate — a decoded predicate has
// exactly one form — and returned rather than panicked for voidOf's reason.
func predicateOf(p *PredicateSpec) encounter.Trigger {
	if p == nil {
		return nil
	}
	switch p.Form() {
	case predicateRound:
		return encounter.TriggerRound{Round: *p.Round}
	case predicateDown:
		return encounter.TriggerMemberDown{Member: encounter.MemberID(p.Down)}
	case predicateFact:
		return encounter.TriggerFact{Fact: p.Fact}
	case predicateStance:
		return encounter.TriggerStance{Between: p.Stance.Between, Stance: encounter.Stance(p.Stance.Is)}
	case predicateHolds:
		// An option's `requires` only — [grammar.predicate] refuses it
		// everywhere else, so this arm is reached from [optionsOf] alone.
		return encounter.TriggerHolds{Prop: encounter.PropID(p.Holds)}
	default:
		return nil
	}
}

// exitsOf carries the authored ways out through as they were written — an id
// and a cell each, in authored order. Nothing is derived: `start` is not one
// of these, by ruling (design §3.1).
//
// ONE FUNCTION, BOTH DIALECTS, and `lower` is the one half of an exit a
// dialect owns (rpg-project#488 R2): v2 authors an absolute `at: [col,row]`
// in the orientation its document declares, the single room authors
// `cell: {q, r}` in its own axial frame ([RoomExit]), and each spends its own
// conversion on the way in. It is [doorStateOf]'s split one key over — the
// state is shared and the geometry is the dialect's — with the difference
// that everything else about an exit IS shared, so everything else is here.
func exitsOf[E any](exits []E, lower func(E) (string, spatial.Position)) []encounter.FieldExit {
	var out []encounter.FieldExit
	for _, ex := range exits {
		id, at := lower(ex)
		out = append(out, encounter.FieldExit{ID: id, At: at})
	}
	return out
}

// v2Exit lowers one v2 exit: the authored [col,row] pair, converted the way
// every other absolute cell in that dialect is ([authored]).
func v2Exit(ex ExitSpec) (string, spatial.Position) { return ex.ID, authored(ex.At) }

// scenariosOf deep-copies the scenario bindings so a caller cannot reach back
// into the spec through the map it is handed. Nil in, nil out: a file that
// binds no scenario compiles to a dungeon that carries none, rather than to
// one carrying an empty map somebody has to tell apart from none.
//
// ONE FUNCTION, BOTH DIALECTS, for [endingsOf]'s reason: `scenarios:` is the
// v2 root key verbatim, down to the Go type, so it takes the MAP rather than
// a [Spec].
func scenariosOf(scenarios map[string]map[string]string) map[string]map[string]string {
	if len(scenarios) == 0 {
		return nil
	}
	out := make(map[string]map[string]string, len(scenarios))
	for id, bindings := range scenarios {
		copied := make(map[string]string, len(bindings))
		for k, v := range bindings {
			copied[k] = v
		}
		out[id] = copied
	}
	return out
}

// voidOf turns the author's word into the declaration the canvas requires.
// The default case is unreachable after Validate and returns an error rather
// than panicking, because an unreachable branch that can only crash is worse
// than one that can only explain itself.
func voidOf(spec *Spec) (encounter.Void, error) {
	switch spec.Void {
	case "opaque":
		return encounter.VoidIsOpaque(), nil
	case "transparent":
		return encounter.VoidIsTransparent(), nil
	default:
		return nil, fmt.Errorf("void %q reached the compiler: %w", spec.Void, ErrBadSpec)
	}
}

// regionsOf carries the regions verbatim, flattening each one's rows.
func regionsOf(spec *Spec) []encounter.RegionInput {
	out := make([]encounter.RegionInput, 0, len(spec.Regions))
	for _, r := range spec.Regions {
		var cells []spatial.Position
		for _, row := range r.Cells {
			for _, at := range row {
				cells = append(cells, authored(at))
			}
		}
		// Copied, not aliased: a compiled field that pointed at the spec's
		// own float would change under an author who edited the spec
		// afterwards.
		intensity := *r.Lighting.Intensity
		out = append(out, encounter.RegionInput{
			ID: r.ID, Name: r.Name, Cells: cells, Archetype: r.Archetype,
			Lighting: &encounter.Lighting{Intensity: intensity},
		})
	}
	return out
}

// sceneryOf flattens the authored scenery rows into the flat cell list
// [encounter.FieldInput.Scenery] takes, in the authored frame the regions use.
// Nil for a dungeon that authors none, which is the same fact the absent key
// is.
func sceneryOf(spec *Spec) []spatial.Position {
	var cells []spatial.Position
	for _, row := range spec.Scenery {
		for _, at := range row {
			cells = append(cells, authored(at))
		}
	}

	return cells
}

// propsOf is every prop, with both blocking answers copied rather than
// aliased ([encounter.PropInput] holds them as pointers), and Facing/Offset
// carried through exactly as authored — this package validates the word and
// the bounds; it does not reinterpret them (rpg-project#261).
func propsOf(spec *Spec) []encounter.PropInput {
	var out []encounter.PropInput
	for _, p := range spec.Place {
		kind, _ := refKind(p.Ref)
		if !isSceneryRefType(kind) {
			continue
		}
		blocksMovement, blocksLoS := *p.BlocksMovement, *p.BlocksLoS
		var holds []encounter.IntelID
		for _, id := range p.Holds {
			holds = append(holds, encounter.IntelID(spec.Key+"/"+id))
		}
		prop := encounter.PropInput{
			ID: p.ID, Holdable: p.Holdable != nil && *p.Holdable, Holds: holds,
			Ref: p.Ref, At: authored(p.At),
			BlocksMovement: &blocksMovement, BlocksLineOfSight: &blocksLoS,
			Facing: p.Facing,
			// The predicate that brings it in, compiled like an until's
			// (rpg-project#375, R6); nil for a prop on the floor from the
			// first frame.
			Arrives: predicateOf(p.Arrives),
			// The lines a character can say to it (rpg-project#569).
			Options: optionsOf(p.Options, orientations[spec.Orientation]),
		}
		// Validate has already confirmed len(p.Offset) is 0, 2 or 3 by the
		// time Compile runs; anything else is unreachable here. A missing
		// third component is height 0 — on the floor — by design.
		if len(p.Offset) >= 2 {
			prop.Offset = [3]float64{p.Offset[0], p.Offset[1], 0}
			if len(p.Offset) == 3 {
				prop.Offset[2] = p.Offset[2]
			}
		}
		out = append(out, prop)
	}
	return out
}

// wallsOf is THE MECHANICAL TRUTH the runtime is handed: every crossing the
// authored lines block, as the pairs [encounter.WallInput] has always taken,
// minus the ones a door opens.
//
// A DOOR STANDS IN A WALL and the door's crossing is subtracted here, exactly
// as it was under the pair form: the engine still sees walls and doors
// disjoint, and a wall drawn straight through a doorway compiles to the hole
// its door makes. Two walls that block the same crossing — the corner case,
// literally — emit it once, at the first one's height.
func wallsOf(
	derived wallDerivation, names map[spatial.Position][2]int, doors map[[2]spatial.Position]encounter.DoorID,
) []encounter.WallInput {
	var out []encounter.WallInput
	seen := map[[2]spatial.Position]bool{}
	for _, w := range derived.Walls {
		for _, c := range w.Crossings {
			if seen[c] || doors[c] != "" {
				continue
			}
			seen[c] = true
			wall := encounter.WallInput{Boundary: spatial.Boundary{
				From: authored(names[c[0]]), To: authored(names[c[1]]),
				BlocksMovement: true, BlocksLineOfSight: true,
			}}
			if w.Height != nil {
				wall.Height = *w.Height
			}
			out = append(out, wall)
		}
	}

	return out
}

// segmentsOf is WHAT A CLIENT DRAWS: each authored wall as the line it is, in
// fractional axial, with the floor it stands on and the doors that open in it.
//
// PRESENTATION, not mechanics — the crossings above are the mechanics, and the
// two are derived from the same line so they cannot disagree. Before this, a
// client had to guess a wall's shape by chaining the crossings back into runs,
// with a straightness tolerance that regrouped walls the author had drawn; the
// line was in the file all along, and now it reaches the far end intact.
func segmentsOf(
	derived wallDerivation,
	names map[spatial.Position][2]int, doors map[[2]spatial.Position]encounter.DoorID,
) []encounter.SegmentInput {
	var out []encounter.SegmentInput
	for _, w := range derived.Walls {
		seg := encounter.SegmentInput{
			Name: w.Name,
			From: encounter.AxialPointF{Q: w.Start.Q, R: w.Start.R},
			To:   encounter.AxialPointF{Q: w.End.Q, R: w.End.R},
		}
		if w.Height != nil {
			seg.Height = *w.Height
		}
		for _, c := range w.Footprint {
			seg.Footprint = append(seg.Footprint, authored(names[c]))
		}
		for _, c := range w.Crossings {
			if id := doors[c]; id != "" {
				seg.DoorIDs = append(seg.DoorIDs, id)
			}
		}
		out = append(out, seg)
	}

	return out
}

// sealedOf is every OWNED cell a wall leaves too little of to stand on
// (C10) — the cells that keep their region and lose their feet, which is why
// the runtime cannot work them out from region membership any more.
//
// Scenery a wall cuts is deliberately absent: it was never standable, and
// saying so twice would give the runtime two lists to disagree about.
func sealedOf(spec *Spec, o encounter.Orientation, derived wallDerivation) []spatial.Position {
	owner := ownerOf(spec, o)
	var out []spatial.Position
	for cell := range derived.Sealed {
		if _, owned := owner[cell]; !owned {
			continue
		}
		out = append(out, cell)
	}
	names := authoredOf(spec)
	for i, c := range out {
		out[i] = authored(names[c])
	}
	sort.Slice(out, func(i, j int) bool { return cellBefore(out[i], out[j]) })

	return out
}

// doorCrossings is every crossing a door opens, to the compiled door's id.
// Absolute axial, which is the frame the derivations answer in.
func doorCrossings(spec *Spec, o encounter.Orientation) map[[2]spatial.Position]encounter.DoorID {
	g := geometryOf(o)
	out := map[[2]spatial.Position]encounter.DoorID{}
	for _, d := range spec.Doors {
		step, ok := g.stepAt(d.At.Offset)
		if !ok {
			continue
		}
		here := encounter.HexCellAt(o, d.At.Cell[0], d.At.Cell[1])
		there := spatial.Position{X: here.X + float64(step[0]), Y: here.Y + float64(step[1])}
		out[normalizedCrossing(here, there)] = encounter.DoorID(spec.Key + "/" + d.ID)
	}

	return out
}

// floorOf is every floor cell — a region's or scenery's — absolute axial. What
// the wall derivations cut against (C2, C8).
func floorOf(spec *Spec, o encounter.Orientation) map[spatial.Position]bool {
	out := map[spatial.Position]bool{}
	for _, r := range spec.Regions {
		for _, row := range r.Cells {
			for _, at := range row {
				out[encounter.HexCellAt(o, at[0], at[1])] = true
			}
		}
	}
	for _, row := range spec.Scenery {
		for _, at := range row {
			out[encounter.HexCellAt(o, at[0], at[1])] = true
		}
	}

	return out
}

// authoredOf is the reverse of the cell conversion: every floor cell back to
// the [col,row] pair the author wrote, so a derived crossing or footprint can
// be handed on in the frame [encounter.FieldInput] speaks.
func authoredOf(spec *Spec) map[spatial.Position][2]int {
	o := orientations[spec.Orientation]
	out := map[spatial.Position][2]int{}
	for _, r := range spec.Regions {
		for _, row := range r.Cells {
			for _, at := range row {
				out[encounter.HexCellAt(o, at[0], at[1])] = at
			}
		}
	}
	for _, row := range spec.Scenery {
		for _, at := range row {
			out[encounter.HexCellAt(o, at[0], at[1])] = at
		}
	}

	return out
}

// doorsOf mints each door as `<key>/<id>`, standing in THE ONE CROSSING its
// position is the midpoint of (F11), in the state the file gives it.
//
// ONE DOOR, ONE CROSSING. The pair form let a door list any number of edges,
// which made a two-cell gate and a mistake look identical; a wider doorway is
// now two doors, and the compiler has one less thing to be wrong about.
//
// THE ID IS PREFIXED BY THE DUNGEON'S KEY so two dungeons in one process
// cannot collide, and a door's STATE persists under its ID
// (rpg-toolkit#1123), so the id the author wrote is what the state is keyed
// by — renaming a door in the file is what loses a party's progress through
// it, and nothing else does.
func doorsOf(spec *Spec, o encounter.Orientation) []encounter.DoorInput {
	g := geometryOf(o)
	var out []encounter.DoorInput
	for _, d := range spec.Doors {
		step, ok := g.stepAt(d.At.Offset)
		if !ok {
			continue
		}
		here := encounter.HexCellAt(o, d.At.Cell[0], d.At.Cell[1])
		there := spatial.Position{X: here.X + float64(step[0]), Y: here.Y + float64(step[1])}
		// NORMALIZED, because the SAME door can be written from either of the
		// two cells it stands between — [-0.5,0] of one is [0.5,0] of the
		// other — and two files describing one dungeon must compile to one
		// door, not to two orderings of it.
		crossing := normalizedCrossing(here, there)
		out = append(out, encounter.DoorInput{
			ID:     encounter.DoorID(spec.Key + "/" + d.ID),
			Edges:  []encounter.DoorEdge{{From: crossing[0], To: crossing[1]}},
			State:  doorStateOf(d.Locked, d.Closed),
			Opened: openedFactOf(d.Opened),
		})
	}

	return out
}

// openedFactOf is the fact an authored door teaches when it opens
// ([OpenedSpec], rpg-project#569 R12), or none — written once for both
// dialects, verbatim, like every other fact id.
func openedFactOf(opened *OpenedSpec) encounter.FactID {
	if opened == nil {
		return ""
	}

	return encounter.FactID(opened.Fact)
}

// doorStateOf is the state an authored door starts in, from the two keys that
// say so — v2's [DoorSpec] and the single room's [RoomDoorBinding] carry the
// same pair and mean the same thing by it (rpg-project#485, R2).
//
// LOCKED OUTRANKS CLOSED, because a locked door is shut by definition and
// [DoorSpec.Closed] says `closed` is ignored beside it; neither is an open
// doorway. Written once so the two dialects cannot come to disagree about
// what `locked: [...]` with no `closed:` means.
func doorStateOf(locked CheckSpec, closed bool) encounter.DoorState {
	switch {
	case locked != nil:
		return encounter.DoorIsLocked(encounter.Lock{Approaches: approachesOf(locked)})
	case closed:
		return encounter.DoorIsClosed()
	default:
		return encounter.DoorIsOpen()
	}
}

// answersOf carries the authored answer table to the composition's shape,
// nil staying nil so a placement that authored none pictures exactly as it did
// before this key existed.
//
// THIS IS WHERE AN OMITTED WEIGHT BECOMES 1. The authoring dialect lets an
// author leave it out, and the composition takes every entry with its own
// number — so the default is resolved once, here, and nothing downstream has
// to know what "omitted" meant. That is the same move every other compiled
// default in this file makes.
func tableOf(on map[string][]AnswerSpec, o encounter.Orientation) encounter.Table {
	if on == nil {
		return nil
	}
	out := make(encounter.Table, len(on))
	for key, entries := range on {
		rows := make([]encounter.Answer, 0, len(entries))
		for _, entry := range entries {
			rows = append(rows, answerOf(entry, o))
		}
		out[encounter.AnswerKey(key)] = rows
	}

	return out
}

// rootTableOn is the table a binding NAMED at the root, or nil when it named
// none or when no such table is declared (rpg-toolkit#1897).
//
// THE ID IS LOOKED UP, NEVER INTERPRETED. An id this document does not
// declare is refused by name before any compile runs ([bindingTable], asked
// from [siteGrammar]), so the nil branch here is the honest "named
// nothing" case rather than a swallowed error: a binding with no `table:`
// and a binding whose table the validator already refused both order from
// their own `on:` alone.
func rootTableOn(tables map[string]TableSpec, id string) map[string][]AnswerSpec {
	if id == "" {
		return nil
	}

	return tables[id]
}

// layerSpecs returns `over` laid on `base`, in the AUTHORING dialect's own
// type: for each key the nearer layer wins WHOLESALE, which is
// [encounter.Layer]'s rule said in `AnswerSpec` terms.
//
// This exists so a named root table can be stacked under a binding's own
// `on:` WITHOUT the compile site leaving the dialect's type. The engine's
// [encounter.Layer] is what actually decides the merged table, in [ordersOf]
// — this only folds two authored sources into the one `On` that function
// expects, so there is still exactly ONE layering rule in the package and
// one place it is applied.
//
// NIL IS CONTAGIOUS IN THE RIGHT DIRECTION: two nils give nil, so a creature
// that names no table and writes no `on:` carries nil — the same value it
// carried before this key existed, and the reason every document authored
// earlier pictures byte-identically.
func layerSpecs(base, over map[string][]AnswerSpec) map[string][]AnswerSpec {
	if len(base) == 0 {
		return over
	}
	if len(over) == 0 {
		return base
	}
	out := make(map[string][]AnswerSpec, len(base)+len(over))
	for key, entries := range base {
		out[key] = entries
	}
	for key, entries := range over {
		out[key] = entries
	}

	return out
}

// answerOf compiles one authored entry: the weight resolved, the condition
// and the selectors turned into the composition's own shapes, any cell they
// name resolved through the document's orientation o.
func answerOf(entry AnswerSpec, o encounter.Orientation) encounter.Answer {
	weight := 1
	if entry.Weight != nil {
		weight = *entry.Weight
	}
	if entry.Use != nil {
		return groupOf(entry, weight, o)
	}
	fact := ""
	if entry.Fact != nil {
		fact = *entry.Fact
	}
	var search *encounter.Search
	if entry.Search != nil {
		search = &encounter.Search{Distance: DefaultSearchDistance, Heading: DefaultSearchHeading}
		if entry.Search.Distance != nil {
			search.Distance = *entry.Search.Distance
		}
		if entry.Search.Heading != nil {
			search.Heading = encounter.SearchHeading(*entry.Search.Heading)
		}
	}

	return encounter.Answer{
		Weight: weight,
		Say:    entry.Say,
		When:   whenOf(entry.When),
		Fact:   encounter.FactID(fact),
		Flee:   entry.Flee != nil,
		Hold:   entry.Hold != nil,
		Attack: selectorOf(entry.Attack, o),
		Toward: selectorOf(entry.Toward, o),
		Dash:   selectorOf(entry.Dash, o),
		Away:   selectorOf(entry.Away, o),
		Search: search,
	}
}

// groupOf compiles a resolved `use:` row (rpg-project#670): the group arm,
// its rows compiled as rows, and the bundle's name and parameters carried for
// the record. A word, a line or a fact on the row is refused at the form
// before a compile runs, so none is carried here.
func groupOf(entry AnswerSpec, weight int, o encounter.Orientation) encounter.Answer {
	out := encounter.Answer{
		Weight: weight,
		When:   whenOf(entry.When),
		Rows:   rowsOf(entry.Rows, o),
		From:   entry.From,
	}
	if entry.As != nil {
		out.As = *entry.As
	}
	if len(entry.Params) > 0 {
		out.Params = make(map[string]string, len(entry.Params))
		for k, v := range entry.Params {
			out.Params[k] = v
		}
	}

	return out
}

// optionsOf compiles a binding's lines (rpg-project#569): each option's check
// through [approachesOf], its `requires` through [predicateOf], and both
// verdicts' rows through [answerOf], every omitted weight resolved to 1.
//
// NIL FOR NONE, an empty list included, so a creature nobody wrote a line for
// compiles exactly as it did before options existed and the committed
// pictures omit the key.
func optionsOf(specs []OptionSpec, o encounter.Orientation) []encounter.Option {
	if len(specs) == 0 {
		return nil
	}
	out := make([]encounter.Option, 0, len(specs))
	for _, opt := range specs {
		out = append(out, encounter.Option{
			ID: opt.ID, Say: opt.Say, Check: approachesOf(opt.Check),
			Requires: predicateOf(opt.Requires),
			Success:  rowsOf(opt.Success, o), Failure: rowsOf(opt.Failure, o),
		})
	}

	return out
}

// rowsOf compiles one verdict's rows, nil staying nil.
func rowsOf(entries []AnswerSpec, o encounter.Orientation) []encounter.Answer {
	if len(entries) == 0 {
		return nil
	}
	out := make([]encounter.Answer, 0, len(entries))
	for _, entry := range entries {
		out = append(out, answerOf(entry, o))
	}

	return out
}

// whenOf compiles a row's conditions, nil staying nil: an entry with no
// `when` is a standing order and always on the table, and compiles exactly
// as it did before the list existed.
func whenOf(list WhenListSpec) []encounter.When {
	if len(list) == 0 {
		return nil
	}
	out := make([]encounter.When, 0, len(list))
	for i := range list {
		out = append(out, oneWhenOf(&list[i]))
	}

	return out
}

// oneWhenOf compiles one condition, a `not`'s inner condition included.
func oneWhenOf(when *WhenSpec) encounter.When {
	out := encounter.When{
		Enemy: encounter.EnemyWord(when.Enemy), Deed: when.Deed, Within: when.Within,
		// The scope rides with the deed (rpg-toolkit#1883): the condition's
		// own reading of WHOSE deed it is about.
		Scope: when.Scope,
		// `who` rides with the deed too (rpg-project#716): the fall's one
		// member, by the placement id the dialect sealed.
		Who:    encounter.MemberID(when.Who),
		Fact:   nameOf(when.Fact),
		Threat: behavior.ThreatWord(when.Threat),
	}
	if when.Heard != nil {
		out.Heard = &encounter.HeardWhen{Kind: nameOf(when.Heard.Kind), Within: when.Heard.Within}
	}
	if when.Not != nil {
		inner := oneWhenOf(when.Not)
		out.Not = &inner
	}

	return out
}

// nameOf reads a `heard` kind or a `fact` as the author wrote it
// (rpg-project#670, R16): `/…/` is a pattern over authored names, anything
// else the literal name it always was. The empty string is the zero name,
// and `//` is refused at the form ([patternOf]) before a compile reads it.
func nameOf(written string) behavior.Name {
	if pattern, ok := patternOf(written); ok {
		return behavior.Name{Pattern: pattern}
	}

	return behavior.Name{Literal: written}
}

// patternOf is the pattern a written name carries between its slashes, and
// whether it is written as one.
func patternOf(written string) (string, bool) {
	if len(written) >= 2 && strings.HasPrefix(written, "/") && strings.HasSuffix(written, "/") {
		return written[1 : len(written)-1], true
	}

	return "", false
}

// selectorOf compiles a selector, nil staying nil. THE CELL IS CONVERTED HERE,
// once, through [encounter.HexCellAt] under the document's orientation o:
// what reaches the composition is a dungeon-absolute axial cell, never an
// authored pair, because a selector's cell is the one cell the composition
// does NOT convert at construction ([encounter.Selector.At] is documented
// dungeon-absolute). Passing the raw pair here was wrong on every odd row of
// a pointy-top map — the walk went to a different cell than the one drawn —
// for `at` before `path` existed, and for both until rpg-toolkit#2044's review.
func selectorOf(sel *SelectorSpec, o encounter.Orientation) *encounter.Selector {
	if sel == nil {
		return nil
	}
	out := &encounter.Selector{Word: encounter.SelectorWord(sel.Word)}
	if sel.At != nil {
		at := encounter.HexCellAt(o, sel.At[0], sel.At[1])
		out.At = &at
	}
	for _, waypoint := range sel.Path {
		out.Path = append(out.Path, encounter.HexCellAt(o, waypoint[0], waypoint[1]))
	}
	// THE SINGLE ROOM'S SPELLING NEEDS NO ORIENTATION: its axial `{ q, r }`
	// already is the dungeon-absolute cell (rpg-project#605). A validated
	// document never carries both spellings — each dialect refuses the other's.
	if sel.AtHex != nil {
		at := hexCell(*sel.AtHex)
		out.At = &at
	}
	for _, waypoint := range sel.PathHex {
		out.Path = append(out.Path, hexCell(waypoint))
	}
	out.Facings = waypointFacings(sel, len(out.Path))
	out.Waits = waypointWaits(sel, len(out.Path))

	return out
}

// waypointFacings lowers a route's pause facings (rpg-project#624) to the
// per-waypoint bearings the encounter's [encounter.Selector.Facings] carries,
// through the placement's own [lowerFacing], or nil when no waypoint states
// one — an unfaced route compiles to the bytes it always did. The axial
// waypoints follow any offset ones in [selectorOf], so their facings do too;
// a validated document never holds both spellings.
//
// A word the dialect would refuse never reaches here (grammar.entryRoute);
// one that does anyway lowers to no facing rather than a guessed one.
func waypointFacings(sel *SelectorSpec, waypoints int) []*float64 {
	if len(sel.PathHexFacing) == 0 {
		return nil
	}
	out := make([]*float64, waypoints)
	first := waypoints - len(sel.PathHex)
	for i, word := range sel.PathHexFacing {
		if word == "" || first+i >= waypoints {
			continue
		}
		if degrees, err := lowerFacing(word); err == nil {
			out[first+i] = degrees
		}
	}

	return out
}

// waypointWaits is a route's rounds to stand at each waypoint
// (rpg-project#624), as the encounter's [encounter.Selector.Waits] carries
// them, or nil when no waypoint states one, for [waypointFacings]' reason.
func waypointWaits(sel *SelectorSpec, waypoints int) []int {
	if len(sel.PathHexWait) == 0 {
		return nil
	}
	out := make([]int, waypoints)
	first := waypoints - len(sel.PathHex)
	for i, wait := range sel.PathHexWait {
		if first+i < waypoints {
			out[first+i] = wait
		}
	}

	return out
}

// temperOf is a placement's temperament: the word it named, else its
// faction's word, else its faction's mix.
//
// THE PLACEMENT'S WORD WINS AND THE MIX IS NOT DEALT FOR IT (design §3). An
// author who named this creature's temperament has already answered the
// question the mix exists to ask, and dealing anyway would overwrite them.
func temperOf(word string, faction TemperSpec) encounter.Temper {
	if word != "" {
		return encounter.Temper{Word: word}
	}
	if faction.Word != "" {
		return encounter.Temper{Word: faction.Word}
	}
	if len(faction.Mix) == 0 {
		return encounter.Temper{}
	}
	mix := make(map[string]int, len(faction.Mix))
	for word, share := range faction.Mix {
		mix[word] = share
	}

	return encounter.Temper{Mix: mix}
}

// CompileTable compiles a bare `on:` block — the text a rulebook ships its
// default table for a monster kind as — through the SAME decoder and the SAME
// validator an authored dungeon's block goes through (design §1, layer 1).
//
// ONE VALIDATOR, TWO CALLERS, which is the whole reason this is exported. The
// rulebook's defaults are content, written in the same dialect an author
// writes; compiling them by hand in Go would be a second grammar that could
// drift from this one, and the first thing to drift would be a refusal the
// author sees and the rulebook does not.
//
// The source is the CONTENTS of an `on:` mapping, not a whole dungeon:
//
//	time:
//	  - { when: { attacked: { within: 3 } }, attack: attacker, weight: 3 }
//	  - { when: { enemy: seen },             attack: enemy }
//	  - { hold: {} }
//
// A cell or route selector is refused here rather than checked against a
// floor: a rulebook's default table belongs to a KIND and not to a map, so
// there is no floor to check it against, and `toward: { at: … }` and
// `toward: { path: … }` are an authored dungeon's words.
func CompileTable(source string) (encounter.Table, error) {
	var on map[string][]AnswerSpec
	if err := yaml.Unmarshal([]byte(source), &on); err != nil {
		return nil, fmt.Errorf("compile table: %w: %w", ErrBadSpec, err)
	}

	// THE CELL REFUSAL COMES FIRST, before the shared validator runs, and it
	// has to: the validator checks a cell against the floor, and a kind's
	// table has no floor to check against. Refusing the word outright is the
	// honest answer, and doing it here keeps the validator's own floor rule
	// from being asked a question it cannot have.
	for key, entries := range on {
		for _, entry := range entries {
			if _, sel := entrySelectorOf(entry); sel != nil && (sel.At != nil || len(sel.Path) > 0 || sel.AtHex != nil || len(sel.PathHex) > 0) {
				return nil, fmt.Errorf(
					"compile table: %w: %s: a default table belongs to a kind and not to a map, "+
						"so it cannot name a cell or a route (line %d)",
					ErrBadSpec, key, sel.Line)
			}
		}
	}

	// ONE GRAMMAR, TWO CALLERS, and here a third with no document under it at
	// all: a kind's default table names no creature and stands on no floor, so
	// the cast is empty and the frame is never reached — the loop above
	// refused every `at:` before this line.
	var errs []FieldError
	g := newGrammar(grammarInput{
		Add:     func(path, message string) { errs = append(errs, FieldError{Path: path, Message: message}) },
		Members: newMembers(nil),
		Cells:   floorCells{orientation: orientations["pointy"], owner: map[spatial.Position]int{}},
		Facts:   factsIn(onLists(on)),
	})
	g.placeOn("table", on)
	if len(errs) > 0 {
		return nil, fmt.Errorf("compile table: %w: %s", ErrBadSpec, errs[0])
	}

	return tableOf(on, orientations["pointy"]), nil
}

// approachesOf carries an authored check's approaches to the composition's
// shape — copied, not aliased, for regionsOf's reason — with nil staying nil:
// a door that was never concealed carries nothing, which is the zero value
// telling the truth. Uninterpreted, every field.
func approachesOf(check CheckSpec) []encounter.CheckApproach {
	if len(check) == 0 {
		return nil
	}
	out := make([]encounter.CheckApproach, 0, len(check))
	for _, a := range check {
		out = append(out, encounter.CheckApproach{Ability: a.Ability, Tool: a.Tool, DC: a.DC})
	}
	return out
}

// inherited is what each declared faction hands the creatures in it: the
// answer table its members lay their own over, and the temperament they fall
// back to. Indexed once per compile, by [inheritedOrders], and read by
// [ordersOf] per creature.
type inherited struct {
	on     map[string]map[string][]AnswerSpec
	temper map[string]TemperSpec
}

// inheritedOrders indexes the declared factions' orders. The two maps are
// keyed by faction id, and a creature on a side nobody declared simply finds
// nothing — which is the zero value telling the truth, not a defect: the
// reserved `monsters` side is a side whether or not a block declares it.
func inheritedOrders(factions []FactionSpec, tables map[string]TableSpec) inherited {
	out := inherited{
		on:     make(map[string]map[string][]AnswerSpec, len(factions)),
		temper: make(map[string]TemperSpec, len(factions)),
	}
	for _, fa := range factions {
		// A NAMED TABLE IS THE FACTION'S BASE, and its own `on:` is laid over
		// it (rpg-toolkit#1897) — [layerSpecs]' order, the same one a binding
		// uses against the table it names. A faction that names nothing falls
		// through to its `on:` alone, which is what every faction authored
		// before this key does.
		out.on[fa.ID] = layerSpecs(rootTableOn(tables, fa.Table), fa.On)
		out.temper[fa.ID] = fa.Temper
	}

	return out
}

// creatureOrders is one creature as the orders compile reads it: who it is,
// which side it is on, and the three things a faction would otherwise supply.
//
// THE DIALECT DECIDES WHERE THESE COME FROM and the compile does not care: in
// the v2 document all three are keys on the placement itself, and in the
// single room they come off the creature's orders block, keyed by its id.
// What they MEAN is the same, which is why there is one function below and
// not one per dialect (rpg-project#484, design §2).
type creatureOrders struct {
	ID      string
	Ref     string
	Faction string
	On      map[string][]AnswerSpec
	Temper  string
	Actions []string
}

// ordersOf is a creature's compiled orders: its faction's answer table with
// its own laid over KEY BY KEY and the nearer layer winning wholesale, its own
// temperament word beating its faction's word or mix, and its arms verbatim in
// the author's order.
//
// The returned placement carries NOTHING GEOMETRIC — no region and no cell.
// Each dialect fills those in its own frame, which is the whole reason this
// function can be shared.
func ordersOf(c creatureOrders, from inherited, o encounter.Orientation) MonsterPlacement {
	side := sideOf(c.Faction)

	return MonsterPlacement{
		ID: c.ID, Ref: c.Ref, Faction: c.Faction,
		Actions: append([]string(nil), c.Actions...),
		Table:   encounter.Layer(tableOf(from.on[side], o), tableOf(c.On, o)),
		Temper:  temperOf(c.Temper, from.temper[side]),
	}
}

// monstersOf is every authored monster, in authored order, each naming the
// region whose floor it stands on.
func monstersOf(spec *Spec, o encounter.Orientation) []MonsterPlacement {
	owner := ownerOf(spec, o)
	// NIL, AND THAT IS THE HONEST ANSWER FOR THIS DIALECT. The v2 `Spec` has no
	// root `tables:` key — root tables are the single-room dialect's
	// (rpg-toolkit#1897) — so a v2 faction's `table:` resolves to nothing and
	// its orders are its own `on:` alone, exactly as before this field existed.
	//
	// A v2 FILE CARRYING `table:` IS REFUSED rather than read as absent:
	// `KnownFields(true)` rejects a key this shape does not have, so an author
	// learns the spelling is not this dialect's instead of silently getting a
	// faction with no orders.
	from := inheritedOrders(spec.Factions, nil)
	var out []MonsterPlacement
	for _, p := range spec.Place {
		if kind, _ := refKind(p.Ref); kind != typeMonsters {
			continue
		}
		targeting := ""
		if p.Targeting != nil {
			targeting = *p.Targeting
		}
		mp := ordersOf(creatureOrders{
			ID: p.ID, Ref: p.Ref, Faction: p.Faction,
			On: p.On, Temper: p.Temper, Actions: p.Actions,
		}, from, o)
		mp.Region = owner[encounter.HexCellAt(o, p.At[0], p.At[1])]
		mp.At = authored(p.At)
		mp.Targeting = targeting
		mp.Boss = p.Boss
		mp.Holds = intelHoldingsOf(spec.Key, p.Holds)
		mp.Options = optionsOf(p.Options, o)
		mp.Arrives = predicateOf(p.Arrives)
		out = append(out, mp)
	}
	return out
}

// seatsOf resolves the authored start cell to its region and orders that
// region's free cells around it, nearest first.
//
// Seat 0 is the cell they wrote, and the rest are ordered by how far they are
// from it, so four players take the first four and arrive standing together.
// Ties break on column then row, which is arbitrary and deterministic — and
// determinism is the point. Cells with something already standing on them are
// left out: a party member cannot share a cell with a pillar.
func seatsOf(spec *Spec, o encounter.Orientation) ([]Seat, error) {
	owner := ownerOf(spec, o)
	from := encounter.HexCellAt(o, spec.Start.At[0], spec.Start.At[1])
	region, ok := owner[from]
	if !ok {
		return nil, fmt.Errorf("the party starts at [%d,%d], which is not floor: %w", spec.Start.At[0], spec.Start.At[1], ErrBadSpec)
	}

	taken := make(map[[2]int]bool, len(spec.Place))
	for _, p := range spec.Place {
		taken[p.At] = true
	}

	var seats []Seat
	dist := map[[2]int]float64{}
	for _, r := range spec.Regions {
		if r.ID != region {
			continue
		}
		for _, row := range r.Cells {
			for _, at := range row {
				if taken[at] {
					continue
				}
				dist[at] = adjacencyGrid.Distance(from, encounter.HexCellAt(o, at[0], at[1]))
				seats = append(seats, Seat{Region: region, At: authored(at)})
			}
		}
	}
	sort.Slice(seats, func(i, j int) bool {
		ci := [2]int{int(seats[i].At.X), int(seats[i].At.Y)}
		cj := [2]int{int(seats[j].At.X), int(seats[j].At.Y)}
		if dist[ci] != dist[cj] {
			return dist[ci] < dist[cj]
		}
		if ci[0] != cj[0] {
			return ci[0] < cj[0]
		}
		return ci[1] < cj[1]
	})
	if len(seats) == 0 {
		return nil, fmt.Errorf("the party starts at [%d,%d], where something already stands: %w", spec.Start.At[0], spec.Start.At[1], ErrBadSpec)
	}
	return seats, nil
}

// ownerOf is every OWNED cell, absolute axial, to the ID of the region that
// owns it — read once per compile. Scenery is floor too and is deliberately
// absent here: this map exists to answer which region a seat or a monster
// stands in, and scenery's answer to that is that nobody stands on it.
func ownerOf(spec *Spec, o encounter.Orientation) map[spatial.Position]string {
	owner := map[spatial.Position]string{}
	for _, r := range spec.Regions {
		for _, row := range r.Cells {
			for _, at := range row {
				owner[encounter.HexCellAt(o, at[0], at[1])] = r.ID
			}
		}
	}
	return owner
}

// authored is a [col,row] pair as the composition's authored-frame Position.
func authored(at [2]int) spatial.Position {
	return spatial.Position{X: float64(at[0]), Y: float64(at[1])}
}
