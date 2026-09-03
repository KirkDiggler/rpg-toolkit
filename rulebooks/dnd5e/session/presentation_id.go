// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session

import "fmt"

// PresentationIDGenerator supplies one host-owned opaque correlation token for
// each accepted Death Save. Tokens are not story sequence numbers and session
// never derives one from the other.
type PresentationIDGenerator interface {
	Generate() string
}

// maxPresentationIDBytes bounds the value copied into persistence, responses,
// and every recipient event. The accepted alphabet is RFC 3986 unreserved,
// making the opaque value safe in JSON, URLs, logs, and metadata fields without
// escaping or interpretation.
const maxPresentationIDBytes = 128

func validatePresentationID(id string) error {
	if id == "" {
		return fmt.Errorf("presentation id is empty")
	}
	if len(id) > maxPresentationIDBytes {
		return fmt.Errorf("presentation id exceeds %d bytes", maxPresentationIDBytes)
	}
	for _, r := range id {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '-' || r == '.' || r == '_' || r == '~' {
			continue
		}
		return fmt.Errorf("presentation id contains non-wire-safe character %q", r)
	}
	return nil
}
