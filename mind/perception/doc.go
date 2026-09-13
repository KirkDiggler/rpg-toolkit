// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

// Package perception is what an observer holds: channel-sourced testimony
// that may be false and may be stale. It never sees the world. It is handed
// what is true as values and cannot ask a question of its own, which is what
// lets it hold a lie. play/intel is its store; callers never see it — except
// in persistence, where that would cost more to hide than to admit: Load's
// errors wrap intel.ErrInvalidData, and Data.Intel is intel.Data verbatim.
//
// A caller assembles one payload per member, once, per pass — encoded before
// the pass, opaque to this package, and identical for every observer who
// perceives it. Observe takes that Pass, loops once per observer, and lands
// one complete percept per observer via intel.Surveil. Fading, re-acquiring,
// and what counts as "changed" are intel's mechanism, applied here, never
// reimplemented: this package supplies the loop and the geometry-free
// contract around it, not the store.
//
// Two verbs write, and the difference between them is what they claim. A
// Pass is a complete statement about one channel at one moment: everything
// delivered, and by omission everything no longer delivered, which is what
// lets a holding fade. A Report is discrete testimony to one observer — what
// it was told — which sustains nothing and retires nothing. A subject known
// only from a Report is current on no channel at all.
//
// Design contract: docs/ideas/mind/perception/design.md (R1–R11). Composition
// module, not a leaf: depends on core and play/intel (play/README.md's leaf
// promise is "depends only on core", which this deliberately is not — it is
// one layer above a play primitive, not another one).
package perception
