// Package dice provides lazy-evaluated, cryptographically secure dice rolls that show their work.
//
// THE MAGIC: Dice don't roll until needed, then remember what they rolled forever.
//
// Example:
//
//	damage := dice.D6(3)                    // No rolls yet - just potential
//	total := damage.GetValue()              // NOW it rolls: 14
//	desc := damage.GetDescription()         // Shows the journey: "+3d6[6,4,4]=14"
//	again := damage.GetValue()              // Still 14 - dice remember their fate
//
// KEY INSIGHT: Lazy evaluation means dice can travel through your event system
// as potential energy, rolling only when observed - perfect for modifiers that
// might never be needed.
//
// SHOWS ITS WORK: Every roll preserves its history. When a player asks "how did
// I take 47 damage?", the dice remember: "+2d6[5,3]=8" from the sword,
// "+8d6[6,6,5,4,3,2,1,1]=28" from the fireball, "+11" from strength.
//
// CRYPTOGRAPHICALLY SECURE: Uses crypto/rand for true randomness - essential
// for online play where predictability equals cheating.
//
// NEGATIVE DICE: Supports penalties as negative dice: dice.D4(-1) creates
// "-1d4" which might roll "-d4[3]=-3" for damage reduction.
//
// Example - Attack with Sneak Attack:
//
//	// Create dice that haven't rolled yet
//	attack := dice.D20(1)
//	weaponDamage := dice.D8(1)
//	sneakAttack := dice.D6(3)
//
//	// Pass them through your event system - still no rolls!
//	event.AddModifier("weapon", weaponDamage)
//	if isSneak {
//	    event.AddModifier("sneak_attack", sneakAttack)
//	}
//
//	// Later, when damage is calculated:
//	fmt.Println(weaponDamage.GetDescription())   // NOW it rolls: "+d8[6]=6"
//	fmt.Println(sneakAttack.GetDescription())    // "+3d6[5,3,1]=9"
//	// Total damage: 15, with perfect history of how we got there
//
// Example - Testing with Predictable Dice:
//
//	// For tests, inject a mock roller
//	mockRoller := mock_dice.NewMockRoller(ctrl)
//	mockRoller.EXPECT().RollN(ctx, 1, 20).Return([]int{20}, nil)  // Crit!
//
//	roll := dice.NewRollWithRoller(1, 20, mockRoller)
//	attack := roll.GetValue()  // Always 20 for this test
package dice
