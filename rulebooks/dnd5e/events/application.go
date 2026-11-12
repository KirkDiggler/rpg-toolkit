// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package events

import "github.com/KirkDiggler/rpg-toolkit/events"

// ApplicationMembersChangedEvent reports an actual live-membership change at an
// exact coordinator. The application binding publishes it after accepting the
// loss; empty construction is not a loss. Parts may observe it but do not own the
// member set or derive addresses from the cardinalities.
type ApplicationMembersChangedEvent struct {
	ApplicationID string
	Owner         ConditionAddress
	Before        int
	After         int
}

// ApplicationMembersChangedTopic carries scoped membership facts to upkeep parts.
var ApplicationMembersChangedTopic = events.DefineTypedTopic[ApplicationMembersChangedEvent]("dnd5e.application.members_changed")
