// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package events

import "fmt"

// ErrDuplicateRef indicates multiple event types are using the same ref pointer
// This should only happen if packages incorrectly create multiple ref instances
type ErrDuplicateRef struct {
	RefString    string
	ExistingType string
	IncomingType string
}

func (e *ErrDuplicateRef) Error() string {
	return fmt.Sprintf(
		"duplicate event ref %q: already registered by %s, attempted by %s",
		e.RefString,
		e.ExistingType,
		e.IncomingType,
	)
}
