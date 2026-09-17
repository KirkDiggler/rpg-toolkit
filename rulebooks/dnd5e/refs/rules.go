// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package refs

import "github.com/KirkDiggler/rpg-toolkit/core"

// ruleUntrained is the house rule that a character without proficiency in the
// skill a verb uses rolls that check at disadvantage (rpg-project#457 R2).
var ruleUntrained = &core.Ref{Module: Module, Type: TypeRules, ID: "untrained"}

// rulesNS is the namespace for refs that name a RULE rather than a piece of
// content — a thing that modifies a roll and is not a feature, a condition or
// a spell anybody carries.
//
// Its members are deliberately few. A rule earns a ref here only when it has
// to appear in a roll's source list, because that list names WHO changed the
// roll and "nobody, it is just how we play" is an answer a player cannot
// read. Everything else a rule does is arithmetic nobody needs a handle for.
type rulesNS struct{}

// Rules is the namespace for rule refs. Type refs.Rules.<tab> to discover them.
//
// Methods return singleton pointers enabling identity comparison
// (ref == refs.Rules.Untrained()).
var Rules = rulesNS{}

// Untrained is the untrained-skill rule: a character with no proficiency in
// the skill a verb rolls takes disadvantage on that check.
//
// A HOUSE RULE, AND A DIVERGENCE FROM THE 2014 LETTER, ruled by Kirk on
// rpg-project#457 and scoped to SKILL VERBS (Intimidate, Persuade, later
// Deceive). RAW puts no penalty on an untrained check. The divergence exists
// so that taking Intimidation at character creation changes something at the
// table.
//
// IT IS NAMED SO THE LOG CAN NAME IT. A check that silently rolls two dice
// tells a player their luck was bad; a disadvantage source called "Untrained"
// tells them what to take next level. The rule itself lives in ONE function
// in rulebooks/dnd5e/resolution, and returning to the letter is deleting that
// function — this ref is the handle it hangs its source on, nothing more.
func (n rulesNS) Untrained() *core.Ref { return ruleUntrained }
