// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

import "fmt"

// DiscoveryLifetime names how long a character's discovery attempts are retained.
type DiscoveryLifetime string

const (
	// DiscoveryLifetimeCharacter retains attempts across visits to the authored check.
	DiscoveryLifetimeCharacter DiscoveryLifetime = "character"
	// DiscoveryLifetimeRun grants a fresh allowance in a new run, not on reconnect.
	DiscoveryLifetimeRun DiscoveryLifetime = "run"

	defaultDiscoveryMaxAttempts = 1
	defaultDiscoveryResetHexes  = 3
	discoveryReachHexes         = 1
)

// DiscoveryPolicyInput is an authored check's optional attempt configuration.
// Nil numeric fields mean omitted; explicit zero is invalid, not a default.
// Multiple approaches to one concealment share this one policy and allowance.
type DiscoveryPolicyInput struct {
	// MaxAttempts counts all attempts, including the first. Omitted means one.
	MaxAttempts *int
	// ResetHexes is distance from the checked content before a repeat can re-arm.
	// Omitted means three; a value inside the one-hex trigger range is invalid.
	ResetHexes *int
	// Lifetime defaults to character when omitted. Explicit empty is invalid.
	Lifetime *DiscoveryLifetime
}

// DiscoveryPolicy is the resolved, immutable-by-convention authored policy.
// All defaults are explicit in persisted snapshots so a loaded run does not
// acquire a different policy merely because authoring defaults change.
type DiscoveryPolicy struct {
	MaxAttempts int               `json:"max"`
	ResetHexes  int               `json:"reset_hexes"`
	Lifetime    DiscoveryLifetime `json:"lifetime"`
}

// ResolveDiscoveryPolicy fills omitted values and refuses invalid authored
// values with ErrBadConcealment. Nil input is ErrNilInput; an empty non-nil
// input requests the defaults. The result retains no references to the input.
func ResolveDiscoveryPolicy(in *DiscoveryPolicyInput) (*DiscoveryPolicy, error) {
	if in == nil {
		return nil, fmt.Errorf("discovery policy: %w", ErrNilInput)
	}
	out := &DiscoveryPolicy{
		MaxAttempts: defaultDiscoveryMaxAttempts,
		ResetHexes:  defaultDiscoveryResetHexes,
		Lifetime:    DiscoveryLifetimeCharacter,
	}
	if in.MaxAttempts != nil {
		out.MaxAttempts = *in.MaxAttempts
	}
	if in.ResetHexes != nil {
		out.ResetHexes = *in.ResetHexes
	}
	if in.Lifetime != nil {
		out.Lifetime = *in.Lifetime
	}
	if out.MaxAttempts < 1 {
		return nil, fmt.Errorf("discovery attempts.max must be a positive integer: %w", ErrBadConcealment)
	}
	if out.ResetHexes <= discoveryReachHexes {
		return nil, fmt.Errorf("discovery attempts.reset_hexes must be greater than one: %w", ErrBadConcealment)
	}
	if out.Lifetime != DiscoveryLifetimeCharacter && out.Lifetime != DiscoveryLifetimeRun {
		return nil, fmt.Errorf("discovery attempts.lifetime must be character or run: %w", ErrBadConcealment)
	}
	return out, nil
}
