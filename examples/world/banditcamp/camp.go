// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package banditcamp

import (
	"github.com/KirkDiggler/rpg-toolkit/examples/world/graph"
	"github.com/KirkDiggler/rpg-toolkit/examples/world/journal"
	"github.com/KirkDiggler/rpg-toolkit/examples/world/quest"
)

// The cast. The camp and the crew are factions; everyone else is a person who
// belongs to one of them.
const (
	// Camp is the bandit camp, addressed as a unit.
	Camp journal.EntityID = "bandit-camp"

	// Party is the guild crew the players run.
	Party journal.EntityID = "guild-crew"

	// Leader is the chief, and starts in the camp's leads slot.
	Leader journal.EntityID = "vosk-the-chief"

	// Bandits are the rank and file, at group grain: they witness as the camp.
	Bandits journal.EntityID = "camp-bandits"

	// Lieutenant is the one bandit at individual grain, so a fact can reach him
	// and nobody else.
	Lieutenant journal.EntityID = "lieutenant-mirek"

	// Rook is a rogue with expertise in Stealth and a talent for lying.
	Rook journal.EntityID = "rook"

	// Brann is a barbarian who is bad at sneaking and may sneak anyway.
	Brann journal.EntityID = "brann"

	// Sela is a paladin who talks.
	Sela journal.EntityID = "sela"
)

// The declared vocabulary. Nothing in journal, graph, or quest knows what any
// of these strings mean.
const (
	// BelongsTo is this world's membership relation: it carries audience down
	// to members and allegiance up to factions.
	BelongsTo graph.Relation = "belongs-to"

	// HostileTo is the stance the objective is about.
	HostileTo graph.Relation = "hostile-to"

	// AlliedWith is what hostility can become.
	AlliedWith graph.Relation = "allied-with"

	// Leads is the camp's one slot.
	Leads graph.Role = "leads"

	// Regard is the camp's opinion of a faction, tallied from parleys.
	Regard graph.Counter = "regard"

	// Alerted is raised on a camp that has heard something.
	Alerted graph.Flag = "alerted"

	// Defeated is raised on a camp that has been beaten.
	Defeated graph.Flag = "defeated"

	// Posture is the derived word behaviour reads to know how a fight starts.
	Posture graph.LabelName = "posture"
)

// Postures a camp can be caught in.
const (
	// FormedUp is the posture of a camp that saw you coming.
	FormedUp = "formed-up"

	// Surprised is the posture of one that did not. It is the default because
	// it is the absence of news, not the presence of a stealth flag.
	Surprised = "surprised"
)

// Fact kinds this camp's content declares.
const (
	// FactApproach is walking up in the open. Nothing folds it — a fact does not
	// have to derive anything to be worth recording.
	FactApproach journal.Kind = "approach"

	// FactAssault is an attack the camp can see.
	FactAssault journal.Kind = "assault"

	// FactRout is the camp being beaten. Combat is out of this spike's scope;
	// its outcome enters as a declared fact.
	FactRout journal.Kind = "rout"

	// FactInfiltration is a stealth attempt, written whether or not it worked.
	FactInfiltration journal.Kind = "infiltration"

	// FactEntry is crossing into the camp.
	FactEntry journal.Kind = "entry"

	// FactKilling is a leader dying quietly.
	FactKilling journal.Kind = "killing"

	// FactScuffle is a killing that went loudly wrong.
	FactScuffle journal.Kind = "scuffle"

	// FactImpersonation is a claim to lead, believed by whoever heard it.
	FactImpersonation journal.Kind = "impersonation"

	// FactUnmasking is that claim coming apart in front of somebody.
	FactUnmasking journal.Kind = "unmasking"

	// FactParley is asking to talk.
	FactParley journal.Kind = "parley"

	// FactPersuasion is an argument landing or not landing.
	FactPersuasion journal.Kind = "persuasion"
)

// The verbs. Any actor may attempt any of them.
const (
	// Approach walks up to the camp in the open.
	Approach VerbName = "approach"

	// Assault attacks where the camp can see it.
	Assault VerbName = "assault"

	// Defeat records the camp losing the fight.
	Defeat VerbName = "defeat"

	// Sneak tries to move unseen. Success and failure differ only in audience.
	Sneak VerbName = "sneak"

	// Enter crosses into the camp, quietly.
	Enter VerbName = "enter"

	// Assassinate tries to kill somebody without being heard.
	Assassinate VerbName = "assassinate"

	// Impersonate claims somebody else's authority.
	Impersonate VerbName = "impersonate"

	// Parley asks for a hearing.
	Parley VerbName = "parley"

	// Persuade argues a case.
	Persuade VerbName = "persuade"
)

// ConversionThreshold is how much regard turns hostility into alliance.
const ConversionThreshold = 3

// Approaches this camp asks its resolver about. They are the names a D&D 5e
// resolver happens to know as skills; the kernel sees three opaque strings.
const (
	viaStealth    journal.Approach = "stealth"
	viaDeception  journal.Approach = "deception"
	viaPersuasion journal.Approach = "persuasion"
)

// Declaration is the camp as data: who exists, how they stand to each other,
// which role can change hands, and how facts fold into the present.
//
// Read the reducers and projections as sentences. A camp that hears an assault
// is alerted. Whoever holds the leads slot lends the camp their faction's
// stance. Enough regard turns hostility into alliance. A beaten camp stops
// being hostile. None of them mentions a route in, because routes are not
// declared here — there is no list of ways to take this camp anywhere in this
// file, and that is the whole claim being tested.
func Declaration() graph.Config {
	return graph.Config{
		Membership: BelongsTo,
		Entities: []graph.Entity{
			{ID: Camp, Kind: "faction"},
			{ID: Party, Kind: "faction"},
			{ID: Leader, Kind: "person"},
			{ID: Bandits, Kind: "person", Grain: graph.GrainGroup},
			{ID: Lieutenant, Kind: "person", Grain: graph.GrainIndividual},
			{ID: Rook, Kind: "person"},
			{ID: Brann, Kind: "person"},
			{ID: Sela, Kind: "person"},
		},
		Edges: []graph.Edge{
			{From: Leader, Rel: BelongsTo, To: Camp},
			{From: Bandits, Rel: BelongsTo, To: Camp},
			{From: Lieutenant, Rel: BelongsTo, To: Camp},
			{From: Rook, Rel: BelongsTo, To: Party},
			{From: Brann, Rel: BelongsTo, To: Party},
			{From: Sela, Rel: BelongsTo, To: Party},

			{From: Camp, Rel: HostileTo, To: Party},

			// The crew is allied with itself. This is what a slot occupant from
			// the party has to lend: allegiance follows the leader, and the
			// leader's allegiance has to be written down somewhere.
			{From: Party, Rel: AlliedWith, To: Party},
		},
		Slots: []graph.Slot{
			{Role: Leads, Of: Camp, Occupant: Leader},
		},
		Reducers: []graph.Reducer{
			graph.Raise{On: FactApproach, Flag: Alerted},
			graph.Raise{On: FactAssault, Flag: Alerted},
			graph.Raise{On: FactScuffle, Flag: Alerted},
			graph.Raise{On: FactInfiltration, When: graph.Failed, Flag: Alerted},
			graph.Raise{On: FactRout, Flag: Defeated},

			graph.Vacate{On: FactKilling, Role: Leads},
			graph.Occupy{On: FactImpersonation, When: graph.Succeeded, Role: Leads},
			graph.Vacate{On: FactUnmasking, Role: Leads},

			graph.Count{On: FactPersuasion, When: graph.Succeeded, Into: Regard, By: 1},
			graph.Count{On: FactPersuasion, When: graph.Failed, Into: Regard, By: -1},
		},
		Projections: []graph.Projection{
			graph.FollowSlot{Role: Leads, Relations: []graph.Relation{HostileTo, AlliedWith}},
			graph.Threshold{Counter: Regard, At: ConversionThreshold, From: HostileTo, To: AlliedWith},
			graph.Retire{OnFlag: Defeated, Relations: []graph.Relation{HostileTo}},
			graph.Label{Name: Posture, Of: "faction", WhenFlag: Alerted, Then: FormedUp, Else: Surprised},
		},
	}
}

// Verbs is what anyone at this camp may try.
//
// Nine declarations, no prerequisites, and no route in sight. The front door,
// the back way, the changeling, the parley, and the disguise coming apart are
// all compositions of these — the paths are what a player does with the verbs,
// not entries in a list somebody authored.
func Verbs() []Verb {
	return []Verb{
		{
			Name:      Approach,
			OnSuccess: Emission{Kind: FactApproach, Subject: SubjectTarget, Witness: WitnessTarget},
		},
		{
			Name:      Assault,
			OnSuccess: Emission{Kind: FactAssault, Subject: SubjectTarget, Witness: WitnessTarget},
		},
		{
			Name:      Defeat,
			OnSuccess: Emission{Kind: FactRout, Subject: SubjectTarget, Witness: WitnessTarget},
		},
		{
			// The whole of stealth: the same fact, told to different people.
			Name:       Sneak,
			Approach:   viaStealth,
			Difficulty: 13,
			OnSuccess:  Emission{Kind: FactInfiltration, Subject: SubjectTarget, Witness: WitnessNobody},
			OnFailure:  Emission{Kind: FactInfiltration, Subject: SubjectTarget, Witness: WitnessTarget},
		},
		{
			Name:      Enter,
			OnSuccess: Emission{Kind: FactEntry, Subject: SubjectTarget, Witness: WitnessNobody},
		},
		{
			// Landing it leaves a corpse nobody heard about. Botching it leaves
			// a fight everybody did.
			Name:       Assassinate,
			Approach:   viaStealth,
			Difficulty: 15,
			OnSuccess:  Emission{Kind: FactKilling, Subject: SubjectTarget, Witness: WitnessNobody},
			OnFailure:  Emission{Kind: FactScuffle, Subject: SubjectTarget, Witness: WitnessTarget},
		},
		{
			// On failure the fact is about the impostor, not the camp: somebody
			// stopped holding a role, which is the same thing an assassination
			// does and folds through the same declaration.
			Name:       Impersonate,
			Approach:   viaDeception,
			Difficulty: 12,
			OnSuccess:  Emission{Kind: FactImpersonation, Subject: SubjectTarget, Witness: WitnessTarget},
			OnFailure:  Emission{Kind: FactUnmasking, Subject: SubjectActor, Witness: WitnessBystanders},
		},
		{
			Name:      Parley,
			OnSuccess: Emission{Kind: FactParley, Subject: SubjectTarget, Witness: WitnessTarget},
		},
		{
			// Both branches write the same kind. The two Count declarations
			// split on the outcome, so a bad argument costs you ground.
			Name:       Persuade,
			Approach:   viaPersuasion,
			Difficulty: 13,
			OnSuccess:  Emission{Kind: FactPersuasion, Subject: SubjectTarget, Witness: WitnessTarget},
			OnFailure:  Emission{Kind: FactPersuasion, Subject: SubjectTarget, Witness: WitnessTarget},
		},
	}
}

// Contract is the guild's job, and the one thing that decides whether a run
// worked.
//
// The objective is read in the camp's own view, because the camp's own view is
// what the camp acts on. A camp following an impostor is not hostile, however
// much the party knows about the body behind the tent.
func Contract() quest.Template {
	return quest.Template{
		ID:   "quiet-the-bandit-camp",
		Name: "Quiet the Bandit Camp",
		Objectives: []quest.Objective{{
			ID:        "camp-no-longer-hostile",
			Observer:  Camp,
			Predicate: quest.NoEdge{From: Camp, Rel: HostileTo, To: Party},
		}},
	}
}
