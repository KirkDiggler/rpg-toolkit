// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"

	combatActions "github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/combat/actions"

	"github.com/cyberphone/json-canonicalization/go/src/webpki.org/jsoncanonicalizer"
)

// Declaration selector version and the sealed variant strings for the verbs
// that carry no authored action definition. Any change to these literals, the
// selector document shape, or the verb/slot bytes requires a selector-version
// bump — see docs/ideas/session-combat/experience/design.md.
const (
	declarationDomain    = "session-declaration:v1"
	declarationPrefix    = "v1."
	variantMoveSealed    = "session:move:v1"
	variantEndTurnSealed = "session:end-turn:v1"
)

// ErrDeclarationIDCollision is returned by [indexCompiledOffers] when two
// non-identical compiled offers collide on the same declaration ID. Afford and
// execution fail closed as an internal provider defect rather than selecting
// either offer; the same offer recurring with the same ID is not a collision.
var ErrDeclarationIDCollision = errors.New(
	"declaration id collision between non-identical offers")

// declarationIDInput is the material a declaration selector is built from. It
// is the seam-internal shape the compiled-offer builder feeds to
// [declarationID]; it never crosses the host boundary.
type declarationIDInput struct {
	// Session is the session the declaration belongs to.
	Session string
	// Member is the acting member the declaration is priced for.
	Member string
	// Verb is the seam verb. Must be one of [VerbAttack], [VerbMove],
	// [VerbEndTurn]; any other byte is rejected.
	Verb Verb
	// Slot is the economy shape this declaration would spend. Must be one of
	// [SlotNone], [SlotAction], [SlotBonus], [SlotReaction]; any other byte is
	// rejected.
	Slot Slot
	// Attack is the complete validated [combatActions.Definition] for
	// [VerbAttack]. It must be non-nil for [VerbAttack] and nil for
	// [VerbMove]/[VerbEndTurn], whose selectors use sealed variant strings
	// instead.
	Attack *combatActions.Definition
}

// selectorDocument is the canonical JSON value a declaration ID is derived
// from. Field order is irrelevant: the whole document is canonicalized with
// RFC 8785 before hashing, which fixes object-key ordering, string escaping,
// number rendering, array framing, and whitespace.
type selectorDocument struct {
	Domain  string          `json:"domain"`
	Session string          `json:"session"`
	Member  string          `json:"member"`
	Verb    string          `json:"verb"`
	Slot    string          `json:"slot"`
	Variant json.RawMessage `json:"variant"`
}

// declarationID builds the canonical, full-SHA-256 declaration selector for one
// compiled offer variant.
//
// The selector document carries exactly these keys — domain, session, member,
// verb, slot, variant — with the verb/slot bytes taken from the seam's own
// typed constants. For [VerbAttack] the variant is the JSON value produced by
// serializing the complete validated [combatActions.Definition], so the
// definition's existing presence semantics (nil vs non-nil empty SpendProfile,
// omitempty on nil/empty maps and slices) are the selector material. For
// [VerbMove] and [VerbEndTurn] the variant is a sealed string.
//
// The document is canonicalized with the JSON Canonicalization Scheme
// (RFC 8785) via [jsoncanonicalizer.Transform], hashed with SHA-256, and
// encoded as unpadded base64url after the prefix "v1.". The full digest is
// encoded with no truncation. Numbers outside RFC 8785's interoperable exact
// range are rejected by the canonicalizer rather than approximated.
func declarationID(input declarationIDInput) (string, error) {
	if err := validateDeclarationVerbSlot(input.Verb, input.Slot); err != nil {
		return "", err
	}

	variant, err := selectorVariant(input.Verb, input.Attack)
	if err != nil {
		return "", err
	}

	doc := selectorDocument{
		Domain:  declarationDomain,
		Session: input.Session,
		Member:  input.Member,
		Verb:    string(input.Verb),
		Slot:    string(input.Slot),
		Variant: variant,
	}

	raw, err := json.Marshal(doc)
	if err != nil {
		return "", fmt.Errorf("declaration selector marshal: %w", err)
	}

	canonical, err := jsoncanonicalizer.Transform(raw)
	if err != nil {
		return "", fmt.Errorf("declaration selector canonicalization: %w", err)
	}

	sum := sha256.Sum256(canonical)
	return declarationPrefix + base64.RawURLEncoding.EncodeToString(sum[:]), nil
}

// validateDeclarationVerbSlot rejects any verb or slot byte outside the seam's
// closed enum, so a new verb/slot string cannot silently produce a selector
// under the current version without an explicit bump.
func validateDeclarationVerbSlot(verb Verb, slot Slot) error {
	switch verb {
	case VerbAttack, VerbMove, VerbEndTurn:
	default:
		return fmt.Errorf("unsupported declaration verb %q", verb)
	}
	switch slot {
	case SlotNone, SlotAction, SlotBonus, SlotReaction:
	default:
		return fmt.Errorf("unsupported declaration slot %q", slot)
	}
	return nil
}

// selectorVariant builds the variant JSON value for one verb. Move and EndTurn
// use sealed strings; Attack serializes the complete validated definition.
func selectorVariant(verb Verb, attack *combatActions.Definition) (json.RawMessage, error) {
	switch verb {
	case VerbMove:
		if attack != nil {
			return nil, fmt.Errorf("move declaration must not carry an attack definition")
		}
		return json.RawMessage(`"` + variantMoveSealed + `"`), nil
	case VerbEndTurn:
		if attack != nil {
			return nil, fmt.Errorf("end-turn declaration must not carry an attack definition")
		}
		return json.RawMessage(`"` + variantEndTurnSealed + `"`), nil
	case VerbAttack:
		if attack == nil {
			return nil, fmt.Errorf("attack declaration requires an attack definition")
		}
		// Validate the complete definition before serialization: this is the
		// gate that rejects an unvalidated profile and malformed embedded raw
		// JSON (condition parameters) upstream of canonicalization.
		if err := attack.Validate(); err != nil {
			return nil, fmt.Errorf("attack definition is invalid: %w", err)
		}
		raw, err := json.Marshal(attack)
		if err != nil {
			return nil, fmt.Errorf("attack definition marshal: %w", err)
		}
		// Defensive: encoding/json embeds [json.RawMessage] fields verbatim
		// without parsing them, so a malformed embedded blob could slip past
		// Validate into a malformed selector document. Reject it here rather
		// than letting the canonicalizer be the only guard.
		if !json.Valid(raw) {
			return nil, fmt.Errorf("attack definition marshal produced malformed JSON")
		}
		return json.RawMessage(raw), nil
	default:
		// Unreachable after validateDeclarationVerbSlot; kept for completeness.
		return nil, fmt.Errorf("unsupported declaration verb %q", verb)
	}
}

// indexCompiledOffers is the collision guard offer compilation uses when projecting
// compiled offers. It keys offers by their declaration ID — computed through
// an injected ID function — and fails closed when two non-identical offers
// collide on the same ID. Identical offers may share an ID (recurrence), which
// is not a collision.
//
// The primitive is generic over the offer identity so compilation can use its
// own compiledOffer type without reshaping this guard; it owns no rules and
// holds no offer beyond the ID index.
type indexCompiledOffers[T any] struct {
	entries map[string]T
	id      func(T) (string, error)
	equal   func(a, b T) bool
}

// newIndexCompiledOffers creates an empty collision guard. id computes the
// declaration ID for an offer; equal reports whether two offers are the same
// offer, so a recurring offer does not count as a collision.
func newIndexCompiledOffers[T any](
	id func(T) (string, error),
	equal func(a, b T) bool,
) *indexCompiledOffers[T] {
	return &indexCompiledOffers[T]{
		entries: make(map[string]T),
		id:      id,
		equal:   equal,
	}
}

// add records offer under its declaration ID. It returns [ErrDeclarationIDCollision]
// when the ID is already present and the existing offer is not equal to offer.
// An ID-function error propagates unchanged.
func (idx *indexCompiledOffers[T]) add(offer T) error {
	id, err := idx.id(offer)
	if err != nil {
		return err
	}
	if existing, ok := idx.entries[id]; ok {
		if idx.equal(existing, offer) {
			return nil
		}
		return ErrDeclarationIDCollision
	}
	idx.entries[id] = offer
	return nil
}
