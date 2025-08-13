// Copyright (C) 2024 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package core

// TypedRef associates a Ref with a specific type at compile time.
// This enables type-safe event subscriptions and other ref-based operations.
type TypedRef[T any] struct {
	Ref *Ref
}

// String returns the string representation of the underlying ref
func (tr TypedRef[T]) String() string {
	if tr.Ref == nil {
		return ""
	}
	return tr.Ref.String()
}