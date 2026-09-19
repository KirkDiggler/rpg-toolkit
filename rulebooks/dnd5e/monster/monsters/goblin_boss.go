// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package monsters

import (
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/abilities"
	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/monster"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/refs"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/shared"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/weapons"
)

// secondScimitarSwing is the goblin boss's own words for why its second blow
// is rolled at disadvantage. It reaches the d20's keep record as the imposing
// source's name, so this string is what a player reads when they ask why the
// low die counted (rpg-project#462).
const secondScimitarSwing = "second attack of a multiattack"

// NewGoblinBoss creates the SRD goblin boss (CR 1): a scimitar, a javelin,
// and the first Multiattack this engine can actually run.
//
// # Why this monster and not another
//
// Its Multiattack line is the smallest one that is not just "swing twice":
// *"The goblin makes two attacks with its scimitar. The second attack has
// disadvantage."* A sequence that only repeated a step could be faked by a
// loop; a second step that rolls differently from the first cannot, so the
// per-step declaration on [combatActions.SequenceStep] has to be real from
// the day it lands.
//
// # Author order is Multiattack, scimitar, javelin
//
// The stat block's order, and it is load-bearing rather than cosmetic: a
// driver reads this list and takes the first entry it can reach with, so the
// boss swings twice when it is standing over somebody. The components stay
// listed on their own beneath it, because a sequence is a SCRIPT OVER A
// REPERTOIRE — [combatActions.ResolveSequence] resolves each step against
// these very entries — and because a boss at range still throws its javelin.
//
// # The javelin lands as its melee half
//
// The SRD line is *"Melee or Ranged Weapon Attack: +2 to hit, reach 5 ft. or
// range 30/120 ft."* and [combatActions.AttackDelivery] is an exactly-one
// union, so one definition cannot be both. The catalog's javelin is a simple
// MELEE weapon that happens to carry the thrown property, so what assembles
// here is the five-foot half. Nothing in the engine throws a melee weapon
// today; when something does, the thrown band is a fact about the catalog
// entry and every thrown weapon gains it at once, rather than this stat block
// growing a hand-typed second definition.
//
// # Deliberately NOT represented
//
//   - Nimble Escape (Disengage or Hide as a bonus action). There is no
//     monster bonus-action economy and no Hide verb to spend one on.
//   - Redirect Attack (swap places with an adjacent goblin as a reaction to
//     being hit). A reaction that retargets a landed blow has no seam: the
//     strike machine's reaction windows are ADR-0027's, unbuilt.
//
// Both are stubs waiting to be written, and neither is stubbed here — a
// declared ability the engine ignores reads exactly like one it honours.
func NewGoblinBoss(id string) *monster.Monster {
	m := monster.New(monster.Config{
		ID:   id,
		Name: "Goblin Boss",
		Ref:  refs.Monsters.GoblinBoss(),
		HP:   21, // 6d6
		AC:   17, // Chain shirt + shield
		AbilityScores: shared.AbilityScores{
			abilities.STR: 10, // +0
			abilities.DEX: 14, // +2
			abilities.CON: 10, // +0
			abilities.INT: 10, // +0
			abilities.WIS: 8,  // -1
			abilities.CHA: 10, // +0
		},
	})

	// FIRST, so the boss's own line is the one a driver reaches for. The
	// steps name the scimitar armed below: a sequence declares what to do
	// with a repertoire, never a new attack of its own.
	mustAddAction(m, combatActions.Definition{
		Ref:  *refs.MonsterActions.GoblinBossMultiattack(),
		Name: "Multiattack",
		Sequence: &combatActions.SequenceProfile{
			Steps: []combatActions.SequenceStep{
				{Action: *refs.Weapons.Scimitar()},
				{Action: *refs.Weapons.Scimitar(), Disadvantage: secondScimitarSwing},
			},
		},
	})

	// Scimitar and javelin off the catalog and this boss's own numbers: DEX
	// 14 carries the finesse blade to the SRD's "+4, 1d6+2", STR 10 carries
	// the javelin to "+2, 1d6" — two abilities, one assembly
	// (rpg-project#448). Melee first among the components, as every other
	// stat block here orders them.
	mustAddWeapon(m, weapons.Scimitar, weapons.Javelin)

	m.SetSpeed(monster.SpeedData{Walk: 30})

	return m
}
