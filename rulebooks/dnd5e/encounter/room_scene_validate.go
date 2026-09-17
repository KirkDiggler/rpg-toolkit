// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import (
	"fmt"
	"math"
)

// room_scene_validate.go is THE ONE OWNER of room scene presentation
// validation (issue #1753). The typed value validators the v3 source decoder
// grew live here, beside the types they judge, and every caller — the
// decoder, NewEncounter and LoadEncounter through compileField — asks the
// same function, so the runtime cannot accept a presentation the decoder
// would refuse or vice versa (the shared-validator law #929 T2 established
// for fields, applied to the scene).
//
// SCALAR bounds are the editor's own (world-building serialization.ts /
// roomDraft.ts at height head), so a room the editor accepts is never
// refused here and vice versa. The check walks DECODED VALUES — presence and
// scalar kind in the original YAML stay the source decoder's business; an
// id this validator sees as the empty string is a defect wherever the value
// came from, file or blob.
//
// The single accepted presentation version is 1, and any other is refused by
// name rather than read hopefully — the same ruling the field's void and
// orientation words carry (rpg-toolkit#1053/#1068: a shape this build does
// not know is refused, never reinterpreted).

// Scalar bounds mirrored from the editor's own validators, so a room the
// editor accepts is never refused here and vice versa.
const (
	maxRoomSceneItems     = 200
	maxRoomSceneGroups    = 80
	maxRoomSceneText      = 120
	maxRoomSceneAssetRef  = 160
	maxRoomSceneHeight    = 8.0
	maxRoomSceneRotationY = math.Pi * 100
	maxRoomSceneIntensity = 20.0
	minRoomSceneRange     = 0.01
	maxRoomSceneRange     = 24.0 // two workspace radii at the editor's WORLD_LIMIT
	roomSceneOffsetLimit  = 12.0
	minRoomSceneHScale    = 0.25
	maxRoomSceneHScale    = 4.0
)

// Approved source values for the frame and the scene, verbatim from the
// editor's own words.
const (
	roomScenePlaneWorldXZ     = "world-xz"
	roomSceneAxisWorldYUp     = "world-y-up"
	roomSceneUnitWorldScene   = "world-scene-unit"
	roomSceneFootprintLocalXZ = "owner-local-xz"
)

// roomSceneWorkspacePresets are the workspace extents the editor offers. A
// v3 source carries exactly one of them, never an invented pair.
var roomSceneWorkspacePresets = [...]RoomSceneWorkspace{
	{HexRadius: 6, HorizontalLimit: 12},
	{HexRadius: 10, HorizontalLimit: 20},
	{HexRadius: 14, HorizontalLimit: 28},
}

// RoomSceneDefect is one defect in a decoded room scene presentation, at the
// path of the thing that is wrong — relative to the presentation root, so a
// caller that addresses the document around it (`room.` in the v3 source)
// prefixes it.
type RoomSceneDefect struct {
	// Path is the presentation-relative address, e.g.
	// "scene.items[0].pointLight.range".
	Path string

	// Message is what is wrong with it, in the same words the source decoder
	// has always reported.
	Message string
}

// Error renders one defect as "path: message".
func (d RoomSceneDefect) Error() string {
	if d.Path == "" {
		return d.Message
	}
	return d.Path + ": " + d.Message
}

// ValidateRoomScene reports EVERY defect in a decoded room scene
// presentation: the single accepted version, the approved frame words, one
// of the workspace presets, scene identities and counts, item and group
// values, and a live and acyclic parent/support graph. An empty defect list
// means the presentation is one this build carries losslessly.
//
// THE ONE VALIDATOR for the typed shape: the v3 source decoder delegates to
// it, and both construction seams ([NewEncounter], [LoadEncounter] through
// compileField) refuse a presentation it would refuse. It is the same
// walk the decoder has always run — moved here because a validator may not
// live on the far side of the import arrow that points at these types.
func ValidateRoomScene(p *RoomScenePresentation) []RoomSceneDefect {
	if p == nil {
		return nil
	}
	var defects []RoomSceneDefect
	add := func(path, message string) {
		defects = append(defects, RoomSceneDefect{Path: path, Message: message})
	}
	if p.Version != 1 {
		add("version", fmt.Sprintf("unsupported room scene presentation version %d (want 1)", p.Version))
	}
	roomSceneFrameDefects(p.Frame, add)
	if !roomSceneWorkspaceValid(p.Workspace) {
		add("workspace", "unsupported workspace preset")
	}
	roomSceneContents(&p.Scene, p.Workspace.HorizontalLimit, add)

	return defects
}

// roomSceneFrameDefects judges the portable coordinate declaration. The
// approved frame is the one the compiler calibrates to, and hexRadius is
// REQUIRED to be 1 so the source-to-canonical adapter needs no second
// factor.
func roomSceneFrameDefects(f RoomSceneFrame, add func(path, message string)) {
	if f.HorizontalPlane != roomScenePlaneWorldXZ {
		add("coordinateFrame.horizontalPlane", "must be world-xz")
	}
	if f.VerticalAxis != roomSceneAxisWorldYUp {
		add("coordinateFrame.verticalAxis", "must be world-y-up")
	}
	if f.DistanceUnit != roomSceneUnitWorldScene {
		add("coordinateFrame.distanceUnit", "must be world-scene-unit")
	}
	if f.FootprintFrame != roomSceneFootprintLocalXZ {
		add("coordinateFrame.footprintFrame", "must be owner-local-xz")
	}
	if f.HexRadius != 1 || !roomSceneFinite(f.HexRadius) {
		add("coordinateFrame.hexRadius", "must be 1")
	}
}

// roomSceneFinite reports a usable float: NaN and both infinities fail, so a
// NaN can never pass a bound check that follows.
func roomSceneFinite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}

// roomSceneWorkspaceValid reports whether the workspace is exactly one of
// the editor presets. NaN never matches, so a NaN radius cannot sneak
// through.
func roomSceneWorkspaceValid(w RoomSceneWorkspace) bool {
	for _, p := range roomSceneWorkspacePresets {
		if w.HexRadius == p.HexRadius && w.HorizontalLimit == p.HorizontalLimit {
			return true
		}
	}
	return false
}

// roomSceneContents judges the visual scene: its own version, its
// identities, its items and groups inside the editor's bounds, and the one
// acyclic graph they form. limit is the workspace's horizontal limit, the
// frame a transform pose is measured against.
func roomSceneContents(scene *RoomVisualScene, limit float64, add func(path, message string)) {
	if scene.Version != 1 {
		add("scene.version", "unsupported version")
	}
	roomSceneText(scene.ID, "scene.id", add)
	roomSceneText(scene.Name, "scene.name", add)
	if len(scene.Items) > maxRoomSceneItems {
		add("scene.items", fmt.Sprintf("must contain at most %d props", maxRoomSceneItems))
	}
	if len(scene.Groups) > maxRoomSceneGroups {
		add("scene.groups", fmt.Sprintf("must contain at most %d groups", maxRoomSceneGroups))
	}
	itemIDs, groupIDs := roomSceneIdentities(scene, add)
	for i := range scene.Items {
		roomSceneItem(&scene.Items[i], fmt.Sprintf("scene.items[%d]", i), limit, add)
	}
	for i := range scene.Groups {
		roomSceneGroup(&scene.Groups[i], fmt.Sprintf("scene.groups[%d]", i), limit, add)
	}
	roomSceneGraph(scene, itemIDs, groupIDs, add)
}

// roomSceneText rejects empty and oversized identifiers: the editor allows at
// most maxRoomSceneText characters for names and labels.
func roomSceneText(v, p string, add func(path, message string)) {
	if v == "" {
		add(p, "is required")
	}
	if len(v) > maxRoomSceneText {
		add(p, fmt.Sprintf("must be at most %d characters", maxRoomSceneText))
	}
}

// roomSceneIdentities collects item and group IDs in one namespace (the
// editor refuses duplicates across both) and reports every duplicate at the
// second occurrence.
func roomSceneIdentities(scene *RoomVisualScene, add func(path, message string)) (itemIDs, groupIDs map[string]bool) {
	itemIDs = make(map[string]bool, len(scene.Items))
	groupIDs = make(map[string]bool, len(scene.Groups))
	all := make(map[string]bool, len(scene.Items)+len(scene.Groups))
	for i := range scene.Items {
		roomSceneID(scene.Items[i].ID, fmt.Sprintf("scene.items[%d].id", i), all, itemIDs, add)
	}
	for i := range scene.Groups {
		roomSceneID(scene.Groups[i].ID, fmt.Sprintf("scene.groups[%d].id", i), all, groupIDs, add)
	}

	return itemIDs, groupIDs
}

func roomSceneID(id, p string, all, mine map[string]bool, add func(path, message string)) {
	if id == "" {
		add(p, "is required")
	}
	if len(id) > maxRoomSceneText {
		add(p, fmt.Sprintf("must be at most %d characters", maxRoomSceneText))
	}
	if id != "" && all[id] {
		add(p, "duplicate id")
	}
	all[id] = true
	mine[id] = true
}

// roomSceneItem judges one scene prop.
func roomSceneItem(it *RoomSceneItem, p string, limit float64, add func(path, message string)) {
	if it.Kind != RoomSceneKindProp {
		add(p+".kind", "must be prop")
	}
	if it.AssetRef == "" {
		add(p+".assetRef", "is required")
	}
	if len(it.AssetRef) > maxRoomSceneAssetRef {
		add(p+".assetRef", fmt.Sprintf("must be at most %d characters", maxRoomSceneAssetRef))
	}
	roomSceneText(it.Label, p+".label", add)
	if it.HeightScale != nil {
		roomSceneBound(*it.HeightScale, minRoomSceneHScale, maxRoomSceneHScale, p+".heightScale", add)
	}
	roomSceneTransform(it.Transform, p, limit, add)
	if it.PointLight != nil {
		roomSceneLight(*it.PointLight, p+".pointLight", add)
	}
}

// roomSceneGroup judges one scene group.
func roomSceneGroup(g *RoomSceneGroup, p string, limit float64, add func(path, message string)) {
	if g.Kind != RoomSceneKindGroup {
		add(p+".kind", "must be group")
	}
	roomSceneText(g.Label, p+".label", add)
	roomSceneTransform(g.Transform, p, limit, add)
}

// roomSceneBound reports v when it is non-finite or outside [lo, hi]. The
// finite check comes first, so a NaN is reported once.
func roomSceneBound(v, lo, hi float64, p string, add func(path, message string)) {
	if !roomSceneFinite(v) {
		add(p, "must be finite")
		return
	}
	if v < lo || v > hi {
		add(p, fmt.Sprintf("must be between %g and %g", lo, hi))
	}
}

// roomSceneTransform keeps a pose inside the editor bounds: X and Z within
// the workspace limit, Y within the editor's 0..8 window, rotation within
// ±100π radians.
func roomSceneTransform(t RoomSceneTransform, p string, limit float64, add func(path, message string)) {
	roomSceneBound(t.X, -limit, limit, p+".transform.x", add)
	roomSceneBound(t.Y, 0, maxRoomSceneHeight, p+".transform.y", add)
	roomSceneBound(t.Z, -limit, limit, p+".transform.z", add)
	roomSceneBound(t.RotationY, -maxRoomSceneRotationY, maxRoomSceneRotationY, p+".transform.rotationY", add)
}

// roomSceneLight judges one authored point light: offset, color and the
// three scalars, each inside the editor's bounds.
func roomSceneLight(l RoomSceneLight, p string, add func(path, message string)) {
	roomSceneOffset(l.Offset, p, add)
	if !roomSceneHexColor(l.Color) {
		add(p+".color", "must be a six-digit hex color")
	}
	roomSceneBound(l.Intensity, 0, maxRoomSceneIntensity, p+".intensity", add)
	roomSceneBound(l.Range, minRoomSceneRange, maxRoomSceneRange, p+".range", add)
}

// roomSceneOffset keeps a light inside the editor's fixed world limit in
// every direction, independent of the room's own workspace.
func roomSceneOffset(o RoomSceneOffset, p string, add func(path, message string)) {
	roomSceneBound(o.X, -roomSceneOffsetLimit, roomSceneOffsetLimit, p+".offset.x", add)
	roomSceneBound(o.Y, -roomSceneOffsetLimit, roomSceneOffsetLimit, p+".offset.y", add)
	roomSceneBound(o.Z, -roomSceneOffsetLimit, roomSceneOffsetLimit, p+".offset.z", add)
}

func roomSceneHexColor(s string) bool {
	if len(s) != 7 || s[0] != '#' {
		return false
	}
	for _, c := range s[1:] {
		isDigit := c >= '0' && c <= '9'
		isLower := c >= 'a' && c <= 'f'
		isUpper := c >= 'A' && c <= 'F'
		if !isDigit && !isLower && !isUpper {
			return false
		}
	}

	return true
}

// roomSceneGraph checks the parent/support graph in one pass: every
// reference names a target of the right kind, and the whole graph is
// acyclic at any length.
func roomSceneGraph(scene *RoomVisualScene, itemIDs, groupIDs map[string]bool, add func(path, message string)) {
	type dep struct {
		to   string
		path string
	}
	deps := make(map[string][]dep, len(scene.Items)+len(scene.Groups))
	order := make([]string, 0, len(scene.Items)+len(scene.Groups))
	for i := range scene.Items {
		it := &scene.Items[i]
		p := fmt.Sprintf("scene.items[%d]", i)
		if it.ParentID != "" && !groupIDs[it.ParentID] {
			add(p+".parentId", "must reference a group")
		}
		if it.SupportID != "" && !itemIDs[it.SupportID] {
			add(p+".supportId", "must reference a prop")
		}
		if it.ID != "" {
			order = append(order, it.ID)
			if it.ParentID != "" && groupIDs[it.ParentID] {
				deps[it.ID] = append(deps[it.ID], dep{to: it.ParentID, path: p + ".parentId"})
			}
			if it.SupportID != "" && itemIDs[it.SupportID] {
				deps[it.ID] = append(deps[it.ID], dep{to: it.SupportID, path: p + ".supportId"})
			}
		}
	}
	for i := range scene.Groups {
		g := &scene.Groups[i]
		p := fmt.Sprintf("scene.groups[%d]", i)
		if g.ParentID != "" && !groupIDs[g.ParentID] {
			add(p+".parentId", "must reference a group")
		}
		if g.ID != "" {
			order = append(order, g.ID)
			if g.ParentID != "" && groupIDs[g.ParentID] {
				deps[g.ID] = append(deps[g.ID], dep{to: g.ParentID, path: p + ".parentId"})
			}
		}
	}
	visiting := make(map[string]bool, len(order))
	done := make(map[string]bool, len(order))
	var visit func(id string)
	visit = func(id string) {
		if done[id] {
			return
		}
		visiting[id] = true
		for _, d := range deps[id] {
			if visiting[d.to] {
				add(d.path, "must not form a cycle")
				continue
			}
			visit(d.to)
		}
		delete(visiting, id)
		done[id] = true
	}
	for _, id := range order {
		visit(id)
	}
}
