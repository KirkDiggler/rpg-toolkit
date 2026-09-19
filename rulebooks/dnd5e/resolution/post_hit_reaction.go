package resolution

// ReactionChoice is an opaque answer to a post-hit reaction window.
type ReactionChoice string

const ReactionDecline ReactionChoice = "decline"

// PostHitReaction carries settled rules facts for a generic retaliation.
type PostHitReaction struct {
	ReactorID    string           `json:"reactor_id"`
	SourceID     string           `json:"source_id"`
	ConditionRef string           `json:"condition_ref"`
	Options      []ReactionOption `json:"options"`
	SaveAbility  string           `json:"save_ability"`
	SaveDC       int              `json:"save_dc"`
	DamageDice   string           `json:"damage_dice"`
	DamageType   string           `json:"damage_type"`
}

// ReactionOption is content-authored; resolution does not interpret its key.
type ReactionOption struct {
	Key   ReactionChoice `json:"key"`
	Label string         `json:"label"`
}
