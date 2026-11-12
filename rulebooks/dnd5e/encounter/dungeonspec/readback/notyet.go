// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package readback

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// notyet.go lists what the file authors and the read-back does not say. It
// walks the AUTHORED TREE, level by level, against the keys each kind of
// mapping is said (or summarised) for, so a key the dialect grows tomorrow
// lands here on its own rather than being skipped silently.
//
// Every kind below is one spec struct; notyet_internal_test.go holds each
// struct's yaml (or json) keys to this table, so a struct that gains a key
// fails the build's tests until the key is said here or acknowledged as not
// yet said. Predicates (`requires`, `pass`, `arrives`, `until`, a row's
// `when`) are a closed grammar the compiler refuses unknown words in, and
// are said through their compiled triggers.

// saidKeys is, for each kind of authored mapping, the keys the read-back
// says or summarises. A key absent from its list is not yet said.
var saidKeys = map[string][]string{
	"root":               {"version", "key", "play", "templates", "bundles", "factions", "dispositions", "intel", "exits", "endings", "room", "interactions", "behaviorDefinitions"},
	"room":               {"version", "id", "name", "coordinateFrame", "scene", "room"},
	"room.room":          {"implicitRegionId", "walls", "regions", "propDeclarations", "arrangementDeclarations", "monsterDeclarations", "partyStart", "monsterBindings", "doorBindings", "propBindings"},
	"monsterDeclaration": {"id", "ref", "startingCell", "appearanceRef", "defaultActivityRef"},
	"monsterBinding":     {"faction", "options", "behaviors", "behaviorState", "blockingStates", "posts", "holds", "arrives", "temper"},
	"faction":            {"id", "mind", "temper", "options"},
	"disposition":        {"between", "stance", "until"},
	"propBinding":        {"holdable", "holds", "options", "arrives"},
	"doorBinding":        {"closed", "locked", "opened"},
	"behaviorDefinition": {"recipe", "table"},
	"interaction":        {"recipe", "approach", "outcomes"},
	"approach":           {"within"},
	"outcome":            {"recognize"},
	"recognize":          {"topic", "holder", "about", "heard_by"},
	"option":             {"id", "say", "loudness", "check", "requires", "advance", "success", "failure"},
	"checkApproach":      {"ability", "tool", "dc"},
	"advance":            {"interaction", "success", "failure"},
	"optionRow":          {"say", "loudness", "fact", "recognize", "activity", "weight"},
	"activity":           {"ref", "cycles"},
	"post":               {"door", "at", "against", "pass"},
	"recipe":             {"ref", "initial", "states"},
	"state":              {"transitions", "terminal"},
	"transition":         {"on", "when", "to", "do", "reason"},
	"act":                {"pick", "wake_in", "set", "clear", "anchor"},
	"tableRow": {"weight", "say", "loudness", "activity", "fact", "flee", "when", "hold", "attack", "toward", "dash", "away",
		"search", "use", "with", "as"},
	"template": {"base", "name", "hitDice", "armor", "abilities", "proficiency", "skills", "actions"},
	"ending":   {"id", "when", "if", "outcome"},
	"exit":     {"id", "cell"},
	"intel":    {"id", "reveals"},
	"region":   {"id", "name", "lighting", "cells", "ground"},
}

// notYetSaid walks the authored tree and lists every key nobody says.
func (b *builder) notYetSaid() []Unsaid {
	out := append([]Unsaid{}, b.unsaid...)
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
			if known[key] {
				continue
			}
			p := key
			if path != "" {
				p = path + "." + key
			}
			out = append(out, Unsaid{Path: p, Kind: kind + "." + key, Why: "the read-back cannot say this yet"})
		}
	}
	// each visits every entry of the list or mapping at n.
	each := func(n *yaml.Node, path string, fn func(child *yaml.Node, childPath string)) {
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
	at := func(n *yaml.Node, key string) *yaml.Node {
		if n == nil || n.Kind != yaml.MappingNode {
			return nil
		}
		for i := 0; i+1 < len(n.Content); i += 2 {
			if n.Content[i].Value == key {
				return n.Content[i+1]
			}
		}
		return nil
	}
	recognize := func(row *yaml.Node, path string) {
		check(at(row, "recognize"), path+".recognize", "recognize")
	}
	options := func(n *yaml.Node, path string) {
		each(n, path, func(o *yaml.Node, op string) {
			check(o, op, "option")
			each(at(o, "check"), op+".check", func(c *yaml.Node, cp string) { check(c, cp, "checkApproach") })
			check(at(o, "advance"), op+".advance", "advance")
			for _, verdict := range []string{"success", "failure"} {
				each(at(o, verdict), op+"."+verdict, func(r *yaml.Node, rp string) {
					check(r, rp, "optionRow")
					recognize(r, rp)
					check(at(r, "activity"), rp+".activity", "activity")
				})
			}
		})
	}
	recipe := func(n *yaml.Node, path string) {
		check(n, path, "recipe")
		each(at(n, "states"), path+".states", func(st *yaml.Node, sp string) {
			check(st, sp, "state")
			each(at(st, "transitions"), sp+".transitions", func(tr *yaml.Node, tp string) {
				check(tr, tp, "transition")
				each(at(tr, "do"), tp+".do", func(a *yaml.Node, ap string) { check(a, ap, "act") })
			})
		})
	}

	root := b.doc.root
	room := at(root, "room")
	gp := at(room, "room")
	check(root, "", "root")
	check(room, "room", "room")
	check(gp, "room.room", "room.room")
	each(at(gp, "regions"), "room.room.regions", func(n *yaml.Node, p string) { check(n, p, "region") })
	each(at(gp, "monsterDeclarations"), "room.room.monsterDeclarations", func(n *yaml.Node, p string) {
		check(n, p, "monsterDeclaration")
	})
	each(at(gp, "monsterBindings"), "room.room.monsterBindings", func(n *yaml.Node, p string) {
		check(n, p, "monsterBinding")
		options(at(n, "options"), p+".options")
		each(at(n, "posts"), p+".posts", func(post *yaml.Node, pp string) { check(post, pp, "post") })
	})
	each(at(root, "factions"), "factions", func(n *yaml.Node, p string) {
		check(n, p, "faction")
		options(at(n, "options"), p+".options")
	})
	each(at(root, "dispositions"), "dispositions", func(n *yaml.Node, p string) { check(n, p, "disposition") })
	each(at(gp, "propBindings"), "room.room.propBindings", func(n *yaml.Node, p string) {
		check(n, p, "propBinding")
		options(at(n, "options"), p+".options")
	})
	each(at(gp, "doorBindings"), "room.room.doorBindings", func(n *yaml.Node, p string) { check(n, p, "doorBinding") })
	each(at(root, "behaviorDefinitions"), "behaviorDefinitions", func(n *yaml.Node, p string) {
		check(n, p, "behaviorDefinition")
		recipe(at(n, "recipe"), p+".recipe")
		each(at(n, "table"), p+".table", func(rows *yaml.Node, kp string) {
			each(rows, kp, func(r *yaml.Node, rp string) {
				check(r, rp, "tableRow")
				check(at(r, "activity"), rp+".activity", "activity")
			})
		})
	})
	each(at(root, "interactions"), "interactions", func(n *yaml.Node, p string) {
		check(n, p, "interaction")
		check(at(n, "approach"), p+".approach", "approach")
		recipe(at(n, "recipe"), p+".recipe")
		each(at(n, "outcomes"), p+".outcomes", func(list *yaml.Node, op string) {
			each(list, op, func(c *yaml.Node, cp string) {
				check(c, cp, "outcome")
				recognize(c, cp)
			})
		})
	})
	each(at(root, "templates"), "templates", func(n *yaml.Node, p string) { check(n, p, "template") })
	each(at(root, "endings"), "endings", func(n *yaml.Node, p string) { check(n, p, "ending") })
	each(at(root, "exits"), "exits", func(n *yaml.Node, p string) { check(n, p, "exit") })
	each(at(root, "intel"), "intel", func(n *yaml.Node, p string) { check(n, p, "intel") })
	return out
}
