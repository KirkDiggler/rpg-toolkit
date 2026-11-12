// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package session_test

import (
	"context"
	"fmt"

	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/encounter"
	"github.com/KirkDiggler/rpg-toolkit/rulebooks/dnd5e/session"
)

// offeredRevision is the revision a line is offered at to one speaker now
// (rpg-project#924): what a client reads off [session.SpeakOptionRow.Revision]
// and sends back in [session.SpeakInput.Revision]. Zero for a line that
// touches no interaction, or one not offered (the door then refuses it as it
// would have before).
func offeredRevision(
	ctx context.Context, mgr *session.Manager, sess, member, target, prop, option string,
) (uint64, error) {
	enc, err := mgr.LoadWorldForTest(ctx, sess)
	if err != nil {
		return 0, err
	}
	var lines []encounter.OfferedOption
	if prop != "" {
		lines, err = enc.PropOptionsFor(encounter.MemberID(member), encounter.PropID(prop))
	} else {
		lines, err = enc.OptionsFor(encounter.MemberID(member), encounter.MemberID(target))
	}
	if err != nil {
		return 0, nil //nolint:nilerr // the verb under test gives the refusal
	}
	for _, line := range lines {
		if line.ID == option {
			return line.Revision, nil
		}
	}
	return 0, nil
}

// speakAsOffered says a line at the revision it is offered at now: a client
// that read its menu and picked from it.
func speakAsOffered(ctx context.Context, mgr *session.Manager, in *session.SpeakInput) (*session.SpeakOutput, error) {
	revision, err := offeredRevision(ctx, mgr, in.Session, in.Member, in.Target, in.Prop, in.OptionID)
	if err != nil {
		return nil, fmt.Errorf("read the offer: %w", err)
	}
	in.Revision = revision
	return mgr.Speak(ctx, in)
}
