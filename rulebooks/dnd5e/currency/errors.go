package currency

import "errors"

var (
	// ErrMalformedCost reports a cost string that is not exactly one
	// nonnegative integer amount followed by one denomination token — a
	// compound ("1 gp 5 sp"), a missing or extra token, or a non-integer or
	// negative amount.
	ErrMalformedCost = errors.New("dnd5e currency: malformed cost string")

	// ErrUnknownDenomination reports a cost string whose denomination
	// token is not one of cp, sp, ep, gp, pp.
	ErrUnknownDenomination = errors.New("dnd5e currency: unknown denomination")

	// ErrInsufficientFunds reports a Sub that would leave a negative
	// amount — a purse cannot hold less than nothing.
	ErrInsufficientFunds = errors.New("dnd5e currency: insufficient funds")
)
