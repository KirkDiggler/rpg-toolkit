// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec

import (
	"fmt"
	"math"

	"gopkg.in/yaml.v3"
)

// single_room_lowering.go is THE LOWERING (rpg-project#479, R3): the whole of
// what the engine reads out of the World Builder's authored presentation,
// and the ONE place that read happens.
//
// The presentation is CONTENT. The room's appearance — assets, labels,
// groups, parents, supports, height scales, point lights, the frame's axis
// words, the workspace's drawing limit — belongs to the World Builder, is
// served to the player by dungeon key, and is judged by the codec that owns
// those words (the web's). This package carries the three presentation
// subtrees as the [yaml.Node] they were authored as, walks them for exactly
// the values play depends on, and validates nothing else about them: an
// unknown key inside `scene` is not this decoder's business, and neither is
// a light's colour.
//
// WHAT PLAY DEPENDS ON, by path, and why each one is still read:
//
//	room.coordinateFrame.hexRadius      the adapter's scale
//	                                    k = FeetPerCell/sqrt(3) assumes it is
//	                                    1 (single_room_placement.go pins the
//	                                    arithmetic term by term). At any other
//	                                    value every placement would be
//	                                    silently mis-scaled, so 1 is required
//	                                    by name — the one frame number, and
//	                                    nothing else from the frame.
//	room.workspace.hexRadius            bounds the walkable cells
//	                                    (walkableValues).
//	room.scene.name                     the compiled dungeon's display name
//	                                    ([Compiled.Name]) — the one
//	                                    non-numeric value the lowering reads,
//	                                    and the host's dungeon list is what
//	                                    shows it.
//	room.scene.items[].id               THE JOIN KEY: a `propDeclarations`
//	                                    entry names a scene item, and the
//	                                    pose it places comes from that item.
//	room.scene.items[].transform.x      the placement's origin and facing —
//	                              .z    read ONLY for an item a declaration
//	                              .rotationY  names, because nothing reads the
//	                                    pose of a prop that blocks nothing.
//
// Nothing else is read, required, bounded, or modelled. `transform.y` is a
// height and play is flat; `kind`, `assetRef`, `label`, `parentId`,
// `supportId`, `heightScale`, `pointLight` and the whole `groups` list reach
// no gameplay fact at all.
//
// THE REFUSALS THAT SURVIVED are the ones that guard a gameplay fact rather
// than the editor's own bounds: a number play reads that was never authored
// (a silent zero is a real pose, a real scale and a real floor radius), a
// number that is not finite (a NaN radius compares false against every cell
// and would quietly widen the floor), and a duplicate item id (the join
// would pick one of two poses for the same declaration). The editor's scalar
// bounds, its workspace presets, its frame vocabulary, its item and group
// caps and its parent/support graph left with the presentation.

// scenePose is one scene item's pose AS AUTHORED: the three numbers
// [placedPropFrom] converts into canonical geometry, and nothing else.
type scenePose struct {
	X         float64
	Z         float64
	RotationY float64
}

// roomRead is the engine's whole read of one room's authored presentation —
// every value in this struct is named in this file's header, with the
// gameplay fact that depends on it.
type roomRead struct {
	// Name is `room.scene.name`: what [CompileSingleRoom] carries out as the
	// dungeon's display name.
	Name string

	// WorkspaceHexRadius is `room.workspace.hexRadius`, the floor bound
	// walkable cells are measured against. WorkspaceKnown is false when it
	// was not authored as a finite number — the defect names that, and the
	// floor bound is not applied on top of it, so one missing number reports
	// as one defect rather than as every cell being off the floor.
	WorkspaceHexRadius float64
	WorkspaceKnown     bool

	// ItemIDs is every authored scene item id: the universe a
	// `propDeclarations` entry may name.
	ItemIDs map[string]bool

	// Poses is the pose of every DECLARED item whose three numbers are
	// authored and finite. An item nobody declared is absent from here
	// because nothing would read it.
	Poses map[string]scenePose
}

// errNotANumber is the defect for a presentation scalar play must read as a
// number and that was authored as something else.
const errNotANumber = "must be a number"

// readRoom lowers one room source's authored presentation to the values play
// depends on, reporting every value it needed and could not read at the
// source path it sits at.
//
// It is the only reader of [RoomSource.CoordinateFrame],
// [RoomSource.Workspace] and [RoomSource.Scene]. Both seams go through it:
// [validateSingleRoom] (which both [DecodeSingleRoom] and
// [CompileSingleRoom] run) and [RoomSource.CanonicalPlacedProps], so a
// source the decoder accepts is one the placement adapter can read and vice
// versa.
func readRoom(r *RoomSource) (roomRead, []FieldError) {
	var errs []FieldError
	add := func(p, m string) { errs = append(errs, FieldError{Path: p, Message: m}) }
	out := roomRead{ItemIDs: map[string]bool{}, Poses: map[string]scenePose{}}

	// The frame's one number. It is not carried anywhere: the adapter's
	// arithmetic is calibrated to 1, so any other value is refused rather
	// than folded in as a second factor nobody asked for.
	if v, ok := sceneNumber(resolveNode(&r.CoordinateFrame), "hexRadius", "room.coordinateFrame", add); ok && v != 1 {
		add("room.coordinateFrame.hexRadius", "must be 1")
	}

	if v, ok := sceneNumber(resolveNode(&r.Workspace), "hexRadius", "room.workspace", add); ok {
		out.WorkspaceHexRadius, out.WorkspaceKnown = v, true
	}

	scene := resolveNode(&r.Scene)
	if n := requireString(scene, "name", "room.scene", add); n != nil {
		if n.Value == "" {
			add("room.scene.name", errRequired)
		}
		out.Name = n.Value
	}
	readSceneItems(scene, r.Gameplay.PropDeclarations, &out, add)

	return out, errs
}

// readSceneItems walks `room.scene.items` for the join key and, for the items
// a declaration names, the three numbers that place them.
//
// An absent `items` is not a defect here: with no items, every declaration is
// refused by [errLiveSceneProp] naming the declaration that has no owner,
// which is the honest sentence. An `items` that is authored as something
// other than a list is named, because a silent zero items would report as a
// pile of dangling declarations instead.
func readSceneItems(scene *yaml.Node, declared map[string]RoomPropDeclaration, out *roomRead, add errSink) {
	items := childNode(scene, "items")
	if items == nil || isNull(items) {
		return
	}
	if items.Kind != yaml.SequenceNode {
		add("room.scene.items", errNotAList)
		return
	}
	for i, raw := range items.Content {
		p := fmt.Sprintf("room.scene.items[%d]", i)
		it := resolveNode(raw)
		if it == nil || isNull(it) {
			add(p, errNotNull)
			continue
		}
		if it.Kind != yaml.MappingNode {
			add(p, errNotAMapping)
			continue
		}
		idNode := requireString(it, "id", p, add)
		if idNode == nil {
			continue
		}
		id := idNode.Value
		if id == "" {
			add(p+".id", errRequired)
			continue
		}
		// THE JOIN MUST BE UNAMBIGUOUS. Two items under one id would hand a
		// declaration one of two poses, chosen by authored order — a
		// gameplay fact decided by a coincidence. The editor's wider rule
		// (ids unique across items AND groups) was its own; this is the half
		// that reaches geometry.
		if out.ItemIDs[id] {
			add(p+".id", errDuplicateID)
			continue
		}
		out.ItemIDs[id] = true
		if _, wanted := declared[id]; !wanted {
			continue
		}
		t := childNode(it, "transform")
		x, okX := sceneNumber(t, "x", p+".transform", add)
		z, okZ := sceneNumber(t, "z", p+".transform", add)
		yaw, okYaw := sceneNumber(t, "rotationY", p+".transform", add)
		if okX && okZ && okYaw {
			out.Poses[id] = scenePose{X: x, Z: z, RotationY: yaw}
		}
	}
}

// sceneNumber reads one authored number out of a presentation node.
//
// Every failure is named, because every one of them would otherwise arrive
// as a silent zero that is a legal value of the thing being read: an absent
// `transform.x` is a prop at the origin, an absent `hexRadius` is a floor of
// one cell, and a NaN radius is a bound that no cell is ever outside.
func sceneNumber(m *yaml.Node, key, p string, add errSink) (float64, bool) {
	n := childNode(m, key)
	if n == nil {
		add(fieldPath(p, key), errRequired)
		return 0, false
	}
	if isNull(n) {
		add(fieldPath(p, key), errNotNull)
		return 0, false
	}
	var v float64
	if err := n.Decode(&v); err != nil {
		add(fieldPath(p, key), errNotANumber)
		return 0, false
	}
	if math.IsNaN(v) || math.IsInf(v, 0) {
		add(fieldPath(p, key), "must be finite")
		return 0, false
	}

	return v, true
}
