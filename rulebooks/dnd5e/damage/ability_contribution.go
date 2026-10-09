package damage

// AbilityModifierInput contains known facts for an attack's base damage rule.
// Zero is a real modifier, not missing evidence. Callers validate unknown facts
// before asking; conditional effects are not inputs to the base rule.
type AbilityModifierInput struct {
	Modifier int
	OffHand  bool
}

// IncludesAbilityModifier answers whether an ability-marked damage pool takes
// its declared modifier before effects. The two-weapon bonus attack omits a
// nonnegative modifier; a negative one remains. A fighting style may restore
// an omitted contribution during the existing effect fold, not here.
func IncludesAbilityModifier(in AbilityModifierInput) bool {
	return !in.OffHand || in.Modifier < 0
}
