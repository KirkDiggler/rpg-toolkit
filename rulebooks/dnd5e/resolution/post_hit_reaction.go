package resolution

// ReactionChoice is an opaque answer to a post-hit reaction window.
type ReactionChoice string

const ReactionDecline ReactionChoice = "decline"

// PostHitReaction is the resolution-side projection of events.PostHitOffer.
// The event package owns the canonical wire shape; this type documents the
// settled facts a generic retaliation machine consumes.
type PostHitReaction struct {
	ReactorID    string `json:"reactor_id"`
	SourceID     string `json:"source_id"`
	ConditionRef string `json:"condition_ref"`
	Option       string `json:"option"`
	SaveAbility  string `json:"save_ability"`
	SaveDC       int    `json:"save_dc"`
	DamageDice   string `json:"damage_dice"`
	DamageType   string `json:"damage_type"`
	HalfOnSave   bool   `json:"half_on_save"`
}
