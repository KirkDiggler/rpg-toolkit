// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package dungeonspec

import (
	"bytes"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// convert_v4.go is the ONE-TIME CONVERSION of a single-room document from the
// walkable-list shape (root version 3 or 4) to the regions shape (version 5,
// rpg-project#725, design R6). There is no dual-dialect period: every content
// file converts once, by this, and the content goldens prove its compiled
// floor did not move.
//
// IT EDITS THE AUTHOR'S TEXT, NOT A RE-EMITTED TREE. The document is parsed
// only to find where each changed key sits; the change is then made to those
// line ranges and every other byte — comments, key order, flow style, quoting
// — is the author's own. A file re-marshalled through yaml.v3 would come back
// re-indented and re-quoted, and the review of a conversion would be a review
// of the marshaller.
//
// What changes:
//
//   - the root `version` becomes 5;
//   - `room.workspace` is deleted (R3: the canvas is unbounded);
//   - `room.room.walkableHexes` is replaced, in place, by `room.room.regions`:
//     one region per explicit `scene.authoringRegions` entry (its id, the text
//     of the map label it links as its name, `lighting.background` or 1 as its
//     intensity, and its cells that are floor), then the implicit region
//     (`implicitRegionId`, the room's name, intensity 1) owning every floor
//     cell no explicit region claims — first in the list;
//   - `scene.authoringRegions` is deleted, and a region's `wallFinish` moves
//     to `scene.regionPresentation.<id>.wallFinish`;
//   - `scene.mapLabels` keeps only the labels no region links: a region's
//     label is its name now.
//
// What it refuses, leaving the file unconverted: an AUTOMATIC authoring region
// (its cells are a witness the converter cannot evaluate, so it is converted
// by hand), a cell two explicit regions both claim, and a region whose id is
// the implicit region's. Comments that DESCRIBE the old keys are prose and
// are the converting author's to edit.

// ConvertNote is something the conversion did that its author should read:
// a region cell that was not floor and was dropped, a linked label that was
// never authored.
type ConvertNote struct {
	Path    string
	Message string
}

// ErrNotConvertible is wrapped by every refusal [ConvertV4] makes.
var ErrNotConvertible = errors.New("not convertible to version 5")

// ConvertV4 rewrites one version 3 or 4 single-room document as version 5.
// It returns the new bytes and the notes the author should read, or an
// error wrapping [ErrNotConvertible] — and no bytes — when the document is not
// one it converts. A document already at version 5 is an error too: there is
// nothing to convert.
func ConvertV4(src []byte) ([]byte, []ConvertNote, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(src, &doc); err != nil {
		return nil, nil, fmt.Errorf("%w: %w", ErrNotConvertible, err)
	}
	root := resolveNode(documentRoot(&doc))
	versionKey, version := mappingEntry(root, "version")
	if version == nil || (version.Value != "3" && version.Value != "4") {
		return nil, nil, fmt.Errorf("%w: the root version is not 3 or 4", ErrNotConvertible)
	}
	_, room := mappingEntry(root, "room")
	workspaceKey, workspace := mappingEntry(room, "workspace")
	_, gameplay := mappingEntry(room, "room")
	_, scene := mappingEntry(room, "scene")
	walkableKey, walkable := mappingEntry(gameplay, "walkableHexes")
	if walkable == nil {
		return nil, nil, fmt.Errorf("%w: room.room.walkableHexes is absent", ErrNotConvertible)
	}
	var floor []RoomCell
	if err := walkable.Decode(&floor); err != nil {
		return nil, nil, fmt.Errorf("%w: room.room.walkableHexes: %w", ErrNotConvertible, err)
	}
	_, implicitID := mappingEntry(gameplay, "implicitRegionId")
	_, roomName := mappingEntry(room, "name")
	if implicitID == nil || roomName == nil {
		return nil, nil, fmt.Errorf("%w: room.room.implicitRegionId and room.name are required", ErrNotConvertible)
	}

	labelsKey, labels := mappingEntry(scene, "mapLabels")
	authoringKey, authoring := mappingEntry(scene, "authoringRegions")
	explicit, linked, presentation, notes, err := explicitRegions(authoring, labels, floor, implicitID.Value)
	if err != nil {
		return nil, nil, err
	}

	claimed := map[RoomCell]bool{}
	for _, r := range explicit {
		for _, c := range r.cells {
			claimed[c] = true
		}
	}
	implicit := convertedRegion{id: implicitID.Value, name: roomName.Value, intensity: 1}
	for _, c := range floor {
		if !claimed[c] {
			implicit.cells = append(implicit.cells, c)
		}
	}

	lines := strings.SplitAfter(string(src), "\n")
	var edits []lineEdit
	edits = append(edits, lineEdit{
		from: versionKey.Line, to: versionKey.Line,
		text: replaceScalar(lines[versionKey.Line-1], version, "5"),
	})
	if workspace != nil {
		edits = append(edits, lineEdit{from: workspaceKey.Line, to: lastLine(lines, workspace)})
	}
	edits = append(edits, lineEdit{
		from: walkableKey.Line, to: lastLine(lines, walkable),
		text: regionsBlock(walkableKey.Column-1, append([]convertedRegion{implicit}, explicit...)),
	})
	if authoring != nil {
		edits = append(edits, lineEdit{
			from: authoringKey.Line, to: lastLine(lines, authoring),
			text: presentationBlock(authoringKey.Column-1, presentation),
		})
	}
	if labels != nil {
		edits = append(edits, labelEdits(lines, labelsKey, labels, linked)...)
	}
	return applyLineEdits(lines, edits), notes, nil
}

// convertedRegion is one region of the converted document.
type convertedRegion struct {
	id, name  string
	intensity float64
	cells     []RoomCell
}

// explicitRegions reads `scene.authoringRegions`: each explicit region with
// its floor cells, the ids of the labels the regions link, and each region's
// wallFinish node keyed by region id. Cells off the floor are dropped with a
// note.
func explicitRegions(authoring, labels *yaml.Node, floor []RoomCell, implicitID string) (
	[]convertedRegion, map[string]bool, []presentationEntry, []ConvertNote, error,
) {
	onFloor := make(map[RoomCell]bool, len(floor))
	for _, c := range floor {
		onFloor[c] = true
	}
	labelText := map[string]string{}
	if labels != nil {
		for _, l := range labels.Content {
			_, id := mappingEntry(l, "id")
			_, text := mappingEntry(l, "text")
			if id != nil && text != nil {
				labelText[id.Value] = text.Value
			}
		}
	}
	var (
		out          []convertedRegion
		notes        []ConvertNote
		presentation []presentationEntry
	)
	linked := map[string]bool{}
	owner := map[RoomCell]string{}
	if authoring == nil {
		return nil, linked, nil, nil, nil
	}
	for i, n := range authoring.Content {
		p := fmt.Sprintf("room.scene.authoringRegions[%d]", i)
		_, id := mappingEntry(n, "id")
		if id == nil || id.Value == "" {
			return nil, nil, nil, nil, fmt.Errorf("%w: %s has no id", ErrNotConvertible, p)
		}
		if id.Value == implicitID {
			return nil, nil, nil, nil, fmt.Errorf("%w: %s is the implicit region's id %q", ErrNotConvertible, p, id.Value)
		}
		_, boundary := mappingEntry(n, "boundary")
		_, kind := mappingEntry(boundary, "kind")
		if kind == nil || kind.Value != "explicit" {
			return nil, nil, nil, nil, fmt.Errorf("%w: %s is not an explicit region; convert it by hand",
				ErrNotConvertible, p)
		}
		var cells []RoomCell
		if _, list := mappingEntry(boundary, "cells"); list != nil {
			if err := list.Decode(&cells); err != nil {
				return nil, nil, nil, nil, fmt.Errorf("%w: %s.boundary.cells: %w", ErrNotConvertible, p, err)
			}
		}
		region := convertedRegion{id: id.Value, intensity: 1}
		if _, label := mappingEntry(n, "labelId"); label != nil {
			linked[label.Value] = true
			text, authored := labelText[label.Value]
			if !authored {
				notes = append(notes, ConvertNote{Path: p + ".labelId",
					Message: fmt.Sprintf("links label %q, which is not authored; the region has no name", label.Value)})
			}
			region.name = text
		}
		if _, lighting := mappingEntry(n, "lighting"); lighting != nil {
			if _, bg := mappingEntry(lighting, "background"); bg != nil {
				v, err := strconv.ParseFloat(bg.Value, 64)
				if err != nil {
					return nil, nil, nil, nil, fmt.Errorf("%w: %s.lighting.background: %w", ErrNotConvertible, p, err)
				}
				region.intensity = v
			}
		}
		for _, c := range cells {
			if !onFloor[c] {
				notes = append(notes, ConvertNote{Path: p + ".boundary.cells",
					Message: fmt.Sprintf("{q: %d, r: %d} is not floor and is dropped", c.Q, c.R)})
				continue
			}
			if first, taken := owner[c]; taken {
				return nil, nil, nil, nil, fmt.Errorf("%w: %s claims {q: %d, r: %d}, which %q already claims",
					ErrNotConvertible, p, c.Q, c.R, first)
			}
			owner[c] = region.id
			region.cells = append(region.cells, c)
		}
		if _, finish := mappingEntry(n, "wallFinish"); finish != nil {
			presentation = append(presentation, presentationEntry{id: region.id, wallFinish: finish})
		}
		out = append(out, region)
	}
	return out, linked, presentation, notes, nil
}

// presentationEntry is a region's look that moves under scene.
type presentationEntry struct {
	id         string
	wallFinish *yaml.Node
}

// regionsBlock writes `regions:` at indent, in the key order the studio
// writes: id, name, lighting, cells.
func regionsBlock(indent int, regions []convertedRegion) string {
	pad := strings.Repeat(" ", indent)
	var b strings.Builder
	b.WriteString(pad + "regions:\n")
	for _, r := range regions {
		fmt.Fprintf(&b, "%s  - id: %s\n", pad, yamlScalar(r.id))
		fmt.Fprintf(&b, "%s    name: %s\n", pad, yamlScalar(r.name))
		fmt.Fprintf(&b, "%s    lighting: { intensity: %s }\n", pad, strconv.FormatFloat(r.intensity, 'f', -1, 64))
		rows := CellRows(r.cells)
		if len(rows) == 0 {
			fmt.Fprintf(&b, "%s    cells: []\n", pad)
			continue
		}
		fmt.Fprintf(&b, "%s    cells:\n", pad)
		for _, row := range rows {
			spans := make([]string, len(row.Q))
			for k, s := range row.Q {
				spans[k] = fmt.Sprintf("[%d, %d]", s[0], s[1])
			}
			fmt.Fprintf(&b, "%s      - r: %d\n%s        q: [%s]\n", pad, row.R, pad, strings.Join(spans, ", "))
		}
	}
	return b.String()
}

// presentationBlock writes `regionPresentation:` at indent for the regions
// that carried a wallFinish, or nothing.
func presentationBlock(indent int, entries []presentationEntry) string {
	if len(entries) == 0 {
		return ""
	}
	pad := strings.Repeat(" ", indent)
	var b strings.Builder
	b.WriteString(pad + "regionPresentation:\n")
	for _, e := range entries {
		finish, err := yaml.Marshal(e.wallFinish)
		if err != nil {
			finish = []byte("{}\n")
		}
		fmt.Fprintf(&b, "%s  %s:\n%s    wallFinish:", pad, yamlScalar(e.id), pad)
		text := strings.TrimRight(string(finish), "\n")
		if !strings.Contains(text, "\n") {
			fmt.Fprintf(&b, " %s\n", text)
			continue
		}
		b.WriteString("\n")
		for _, line := range strings.Split(text, "\n") {
			fmt.Fprintf(&b, "%s      %s\n", pad, line)
		}
	}
	return b.String()
}

// labelEdits deletes every label a region linked; a list left empty is
// written `mapLabels: []`.
func labelEdits(lines []string, key, labels *yaml.Node, linked map[string]bool) []lineEdit {
	var (
		edits []lineEdit
		kept  int
	)
	for _, l := range labels.Content {
		_, id := mappingEntry(l, "id")
		if id == nil || !linked[id.Value] {
			kept++
			continue
		}
		edits = append(edits, lineEdit{from: l.Line, to: lastLine(lines, l)})
	}
	if kept == 0 && len(labels.Content) > 0 {
		return []lineEdit{{
			from: key.Line, to: lastLine(lines, labels),
			text: strings.Repeat(" ", key.Column-1) + "mapLabels: []\n",
		}}
	}
	return edits
}

// yamlScalar spells s as a YAML scalar: plain when it can be, quoted when
// plain would read as something else.
func yamlScalar(s string) string {
	out, err := yaml.Marshal(s)
	if err != nil {
		return strconv.Quote(s)
	}
	return strings.TrimRight(string(out), "\n")
}

// mappingEntry returns the key and value nodes for key in a mapping, or nils.
func mappingEntry(m *yaml.Node, key string) (*yaml.Node, *yaml.Node) {
	m = resolveNode(m)
	if m == nil || m.Kind != yaml.MappingNode {
		return nil, nil
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i], resolveNode(m.Content[i+1])
		}
	}
	return nil, nil
}

// lastLine is the last source line n occupies: the deepest line any of its
// nodes starts on, extended over lines that only close a flow collection.
func lastLine(lines []string, n *yaml.Node) int {
	last := n.Line
	var walk func(*yaml.Node)
	walk = func(x *yaml.Node) {
		if x.Line > last {
			last = x.Line
		}
		for _, c := range x.Content {
			walk(c)
		}
	}
	walk(n)
	for last < len(lines) {
		next := strings.TrimSpace(lines[last])
		if next == "" || strings.Trim(next, "]}, ") != "" {
			break
		}
		last++
	}
	return last
}

// replaceScalar rewrites the scalar v on its line to value.
func replaceScalar(line string, v *yaml.Node, value string) string {
	at := v.Column - 1
	return line[:at] + value + line[at+len(v.Value):]
}

// lineEdit replaces source lines from..to (1-based, inclusive) with text,
// which is empty for a deletion and carries its own newlines.
type lineEdit struct {
	from, to int
	text     string
}

// applyLineEdits applies non-overlapping edits from the bottom of the file
// up, so an earlier edit's line numbers stay true.
func applyLineEdits(lines []string, edits []lineEdit) []byte {
	sort.Slice(edits, func(i, j int) bool { return edits[i].from > edits[j].from })
	out := append([]string(nil), lines...)
	for _, e := range edits {
		tail := append([]string{e.text}, out[e.to:]...)
		out = append(out[:e.from-1], tail...)
	}
	var b bytes.Buffer
	for _, l := range out {
		b.WriteString(l)
	}
	return b.Bytes()
}
