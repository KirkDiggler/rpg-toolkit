// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package readback

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// notyet.go lists what the file authors and the read-back does not say. It
// walks the AUTHORED TREE against the keys each place is said or summarised
// for, so a key the dialect grows tomorrow lands here on its own rather than
// being skipped silently.

// saidKeys is, for each kind of authored mapping, the keys the read-back
// says or summarises. A key absent from its list is not yet said.
var saidKeys = map[string][]string{
	"root":               {"version", "key", "play", "templates", "bundles", "factions", "dispositions", "intel", "exits", "endings", "room", "interactions", "behaviorDefinitions"},
	"room":               {"version", "id", "name", "coordinateFrame", "scene", "room"},
	"room.room":          {"implicitRegionId", "walls", "regions", "propDeclarations", "arrangementDeclarations", "monsterDeclarations", "partyStart", "monsterBindings", "doorBindings", "propBindings"},
	"monsterDeclaration": {"id", "ref", "startingCell", "appearanceRef", "defaultActivityRef"},
	"monsterBinding":     {"faction", "options", "behaviors", "behaviorState", "blockingStates", "posts", "holds", "arrives", "temper"},
	"faction":            {"id", "mind", "temper", "options"},
	"propBinding":        {"holdable", "holds", "options", "arrives"},
	"doorBinding":        {"closed", "locked", "opened"},
	"behaviorDefinition": {"recipe", "table"},
	"interaction":        {"recipe", "approach", "outcomes"},
	"option":             {"id", "say", "check", "requires", "advance", "success", "failure"},
	"optionRow":          {"say", "fact", "recognize", "activity", "weight"},
	"template":           {"base", "name", "hitDice", "armor", "abilities", "proficiency", "skills", "actions"},
}

// notYetSaid walks the authored tree and lists every key nobody says.
func (b *builder) notYetSaid() []Unsaid {
	out := append([]Unsaid{}, b.unsaid...)
	miss := func(path, kind string) {
		out = append(out, Unsaid{Path: path, Kind: kind, Why: "the read-back cannot say this yet"})
	}
	check := func(n *yaml.Node, path, kind string) {
		if n == nil || n.Kind != yaml.MappingNode {
			return
		}
		known := map[string]bool{}
		for _, k := range saidKeys[kind] {
			known[k] = true
		}
		for i := 0; i+1 < len(n.Content); i += 2 {
			key := n.Content[i].Value
			if !known[key] {
				p := key
				if path != "" {
					p = path + "." + key
				}
				miss(p, kind+"."+key)
			}
		}
	}
	each := func(path string, fn func(child *yaml.Node, childPath string)) {
		n := b.doc.node(path)
		if n == nil {
			return
		}
		switch n.Kind {
		case yaml.SequenceNode:
			for i, c := range n.Content {
				fn(c, fmt.Sprintf("%s[%d]", path, i))
			}
		case yaml.MappingNode:
			for i := 0; i+1 < len(n.Content); i += 2 {
				fn(n.Content[i+1], path+"."+n.Content[i].Value)
			}
		}
	}
	options := func(path string) {
		each(path, func(o *yaml.Node, op string) {
			check(o, op, "option")
			for _, verdict := range []string{"success", "failure"} {
				each(op+"."+verdict, func(r *yaml.Node, rp string) { check(r, rp, "optionRow") })
			}
		})
	}

	check(b.doc.root, "", "root")
	check(b.doc.node("room"), "room", "room")
	check(b.doc.node("room.room"), "room.room", "room.room")
	each("room.room.monsterDeclarations", func(n *yaml.Node, p string) { check(n, p, "monsterDeclaration") })
	each("room.room.monsterBindings", func(n *yaml.Node, p string) {
		check(n, p, "monsterBinding")
		options(p + ".options")
	})
	each("factions", func(n *yaml.Node, p string) {
		check(n, p, "faction")
		options(p + ".options")
	})
	each("room.room.propBindings", func(n *yaml.Node, p string) {
		check(n, p, "propBinding")
		options(p + ".options")
	})
	each("room.room.doorBindings", func(n *yaml.Node, p string) { check(n, p, "doorBinding") })
	each("behaviorDefinitions", func(n *yaml.Node, p string) { check(n, p, "behaviorDefinition") })
	each("interactions", func(n *yaml.Node, p string) { check(n, p, "interaction") })
	each("templates", func(n *yaml.Node, p string) { check(n, p, "template") })
	return out
}
