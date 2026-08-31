// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package hostagecamp

import (
	"github.com/KirkDiggler/rpg-toolkit/world"
	"github.com/KirkDiggler/rpg-toolkit/world/graph"
	"github.com/KirkDiggler/rpg-toolkit/world/journal"
	"github.com/KirkDiggler/rpg-toolkit/world/quest"
)

// The cast. Three factions took the job, one faction is holding the hostages,
// and one village is missing three of its people.
const (
	// Captors are the bandits holding the hostages.
	Captors journal.EntityID = "hill-bandits"

	// Village is where the hostages are from, and who put the job up.
	Village journal.EntityID = "ashford"

	// PartyA, PartyB and PartyC are the three companies working the same job.
	PartyA journal.EntityID = "party-ember"
	PartyB journal.EntityID = "party-quill"
	PartyC journal.EntityID = "party-thorn"
)

// The hostages: the declared list of names the population is drawn from.
//
// There is no model anywhere generating these. Three names in a slice is the
// whole population system, and a fourth party asking for work is told the job
// is taken rather than handed a person who does not exist.
const (
	// Deryn is the first name off the list.
	Deryn journal.EntityID = "deryn"

	// Moss is the second.
	Moss journal.EntityID = "moss"

	// Tallow is the third.
	Tallow journal.EntityID = "tallow"
)

// The rescuers, one to a company.
const (
	// Wren works for Ember.
	Wren journal.EntityID = "wren"

	// Marek works for Quill.
	Marek journal.EntityID = "marek"

	// Sable works for Thorn.
	Sable journal.EntityID = "sable"
)

// The declared vocabulary.
const (
	// BelongsTo is this world's membership relation.
	BelongsTo graph.Relation = "belongs-to"

	// AlliedWith and HostileTo are the two stances anyone here holds.
	AlliedWith graph.Relation = "allied-with"

	// HostileTo is the other one.
	HostileTo graph.Relation = "hostile-to"

	// Freed is raised on a hostage somebody got out.
	Freed graph.Flag = "freed"

	// Turned is raised on a hostage who threw in with their captors.
	Turned graph.Flag = "turned"

	// Redeemed is raised on a turned hostage talked back. It does not clear
	// Turned — flags only go up — it outranks it, because the projection that
	// reads it is declared later.
	Redeemed graph.Flag = "redeemed"

	// Dead is raised on a turned hostage who was put down instead.
	Dead graph.Flag = "dead"

	// Sworn, Indebted and Talkative are how a rescued hostage turned out.
	Sworn graph.Flag = "sworn"

	// Indebted is the middling one.
	Indebted graph.Flag = "indebted"

	// Talkative is the one that carries word of somebody else's trouble.
	Talkative graph.Flag = "talkative"

	// KindFaction is a group that holds a stance.
	KindFaction graph.Kind = "faction"

	// KindPerson is somebody who can act or be acted on.
	KindPerson graph.Kind = "person"
)

// Fact kinds this content declares.
const (
	// FactRescue is a hostage got out.
	FactRescue journal.Kind = "rescue"

	// FactTurning is a hostage deciding their captors were the better bet.
	FactTurning journal.Kind = "turning"

	// FactGuardsOath, FactRepayment and FactRumour are how a rescue turned out.
	FactGuardsOath journal.Kind = "guards-oath"

	// FactRepayment is the middling disposition.
	FactRepayment journal.Kind = "repayment"

	// FactRumour is word of somebody else's trouble.
	FactRumour journal.Kind = "rumour"

	// FactRedemption is a turned hostage talked back.
	FactRedemption journal.Kind = "redemption"

	// FactSpurned is a redemption that did not take. It changes nothing, which
	// is what makes the attempt repeatable.
	FactSpurned journal.Kind = "spurned"

	// FactExecution is the other way that job ends.
	FactExecution journal.Kind = "execution"
)

// The verbs.
const (
	// Rescue tries to get a hostage out. Failing turns them.
	Rescue world.VerbName = "rescue"

	// Take reads a freed hostage to see how they took it.
	Take world.VerbName = "take-their-measure"

	// Redeem tries to talk a turned hostage back. Repeatable.
	Redeem world.VerbName = "redeem"

	// PutDown ends it the other way.
	PutDown world.VerbName = "put-down"
)

// The jobs.
const (
	// RescueJob is the template three parties claim off.
	RescueJob = "rescue-the-hostage"

	// ReckoningJob is what it becomes when every rescue failed.
	ReckoningJob = "turn-them-back-or-put-them-down"
)

// Buckets a hostage can be counted in. Order is precedence, not partition.
const (
	// BucketRedeemed is a turned hostage talked back. Asked first, because a
	// redeemed hostage still carries the flag that says they turned.
	BucketRedeemed = "redeemed"

	// BucketDead is a turned hostage put down.
	BucketDead = "dead"

	// BucketTurned is a hostage on the captors' side.
	BucketTurned = "turned"

	// BucketRescued is a hostage somebody got out.
	BucketRescued = "rescued"

	// BucketCaptive is a hostage nothing has happened to yet. The catch-all,
	// and a real answer rather than an absence of one.
	BucketCaptive = "captive"
)

// Approaches this content asks its resolver about.
const (
	viaStealth    journal.Approach = "stealth"
	viaInsight    journal.Approach = "insight"
	viaPersuasion journal.Approach = "persuasion"
)

// Hostages is the declared population, in the order the board hands them out.
func Hostages() []journal.EntityID {
	return []journal.EntityID{Deryn, Moss, Tallow}
}

// Witnesses is everyone who hears about something done in the open here.
//
// The composer takes bystanders from the caller — there is no sight seam in
// this example — so a scenario that wants a thing known publicly says who
// "publicly" means.
func Witnesses() []journal.EntityID {
	return []journal.EntityID{Captors, Village, PartyA, PartyB, PartyC}
}

// Declaration is the camp as data.
//
// Read the projections as sentences. A hostage who turns stands where their
// captors stand; one who is redeemed stands where their village stands; the
// dead hold no stance at all. Redemption is declared after turning, and that
// order is the whole of why it wins — the flag that says they turned is still
// raised and always will be.
func Declaration() graph.Config {
	return graph.Config{
		Membership: BelongsTo,
		Entities: []graph.Entity{
			{ID: Captors, Kind: KindFaction},
			{ID: Village, Kind: KindFaction},
			{ID: PartyA, Kind: KindFaction},
			{ID: PartyB, Kind: KindFaction},
			{ID: PartyC, Kind: KindFaction},
			{ID: Deryn, Kind: KindPerson, Grain: graph.GrainIndividual},
			{ID: Moss, Kind: KindPerson, Grain: graph.GrainIndividual},
			{ID: Tallow, Kind: KindPerson, Grain: graph.GrainIndividual},
			{ID: Wren, Kind: KindPerson},
			{ID: Marek, Kind: KindPerson},
			{ID: Sable, Kind: KindPerson},
		},
		Edges: []graph.Edge{
			{From: Wren, Rel: BelongsTo, To: PartyA},
			{From: Marek, Rel: BelongsTo, To: PartyB},
			{From: Sable, Rel: BelongsTo, To: PartyC},

			{From: Deryn, Rel: BelongsTo, To: Village},
			{From: Moss, Rel: BelongsTo, To: Village},
			{From: Tallow, Rel: BelongsTo, To: Village},

			// The two sides, and the stance each hostage starts with.
			{From: Village, Rel: AlliedWith, To: Village},
			{From: Village, Rel: HostileTo, To: Captors},
			{From: Captors, Rel: AlliedWith, To: Captors},
			{From: Captors, Rel: HostileTo, To: Village},

			{From: Deryn, Rel: AlliedWith, To: Village},
			{From: Deryn, Rel: HostileTo, To: Captors},
			{From: Moss, Rel: AlliedWith, To: Village},
			{From: Moss, Rel: HostileTo, To: Captors},
			{From: Tallow, Rel: AlliedWith, To: Village},
			{From: Tallow, Rel: HostileTo, To: Captors},
		},
		Reducers: []graph.Reducer{
			graph.Raise{On: FactRescue, Flag: Freed},
			graph.Raise{On: FactTurning, Flag: Turned},
			graph.Raise{On: FactRedemption, Flag: Redeemed},
			graph.Raise{On: FactExecution, Flag: Dead},

			graph.Raise{On: FactGuardsOath, Flag: Sworn},
			graph.Raise{On: FactRepayment, Flag: Indebted},
			graph.Raise{On: FactRumour, Flag: Talkative},
		},
		Projections: []graph.Projection{
			graph.AdoptStance{
				OnFlag: Turned, From: Captors,
				Relations: []graph.Relation{AlliedWith, HostileTo},
			},
			graph.AdoptStance{
				OnFlag: Redeemed, From: Village,
				Relations: []graph.Relation{AlliedWith, HostileTo},
			},
			graph.Retire{OnFlag: Dead, Relations: []graph.Relation{AlliedWith, HostileTo}},
		},
	}
}

// Verbs is what anyone here may try.
func Verbs() []world.Verb {
	return []world.Verb{
		{
			// Getting them out, or losing them to the other side. One attempt,
			// two ways it can land, and no branch anywhere that knows which.
			Name:       Rescue,
			Approach:   viaStealth,
			Difficulty: 12,
			Outcomes: []world.Band{
				{Emission: world.Emission{
					Kind: FactRescue, Witness: world.WitnessTargetAndBystanders,
				}},
			},
			Otherwise: world.Emission{
				Kind: FactTurning, Witness: world.WitnessTargetAndBystanders,
			},
		},
		{
			// Three results from one roll, graded by how well it went. A
			// boolean could not carry this, which is why verbs grade by margin.
			Name:       Take,
			Approach:   viaInsight,
			Difficulty: 10,
			Outcomes: []world.Band{
				{AtLeast: 8, Emission: world.Emission{
					Kind: FactGuardsOath, Witness: world.WitnessTargetAndBystanders,
				}},
				{AtLeast: 3, Emission: world.Emission{
					Kind: FactRepayment, Witness: world.WitnessTargetAndBystanders,
				}},
			},
			Otherwise: world.Emission{
				Kind: FactRumour, Witness: world.WitnessTargetAndBystanders,
			},
		},
		{
			// Failing writes a fact that folds to nothing, so the attempt costs
			// a roll and leaves the door open. That is what repeatable means.
			Name:       Redeem,
			Approach:   viaPersuasion,
			Difficulty: 13,
			Outcomes: []world.Band{
				{Emission: world.Emission{
					Kind: FactRedemption, Witness: world.WitnessTargetAndBystanders,
				}},
			},
			Otherwise: world.Emission{
				Kind: FactSpurned, Witness: world.WitnessTargetAndBystanders,
			},
		},
		{
			Name: PutDown,
			Otherwise: world.Emission{
				Kind: FactExecution, Witness: world.WitnessTargetAndBystanders,
			},
		},
	}
}

// Contract is the job three parties claim off, and what it becomes.
//
// Its population is three names. A claim takes one off the board and mints the
// claiming party's own instance about that person; nothing ever puts one back,
// because somebody is not available again just because a company stopped
// trying.
//
// The follow-up opens when the population settles into a shape nobody
// authored a branch for: nobody still captive, and everybody turned.
func Contract() quest.Template {
	return quest.Template{
		ID:       RescueJob,
		Name:     "Rescue the Hostage",
		Subjects: Hostages(),
		Objectives: []quest.Objective{{
			ID:        "hostage-is-free",
			Predicate: quest.Flagged{Flag: Freed, Of: quest.InstanceSubject},
		}},
		Failure: &quest.Objective{
			ID:        "hostage-has-turned",
			Predicate: quest.Flagged{Flag: Turned, Of: quest.InstanceSubject},
		},
		Buckets:    buckets(),
		Successors: []quest.Successor{reckoning()},
	}
}

func buckets() []quest.Bucket {
	return []quest.Bucket{
		{Name: BucketRedeemed, Predicate: quest.Flagged{Flag: Redeemed, Of: quest.InstanceSubject}},
		{Name: BucketDead, Predicate: quest.Flagged{Flag: Dead, Of: quest.InstanceSubject}},
		{Name: BucketTurned, Predicate: quest.Flagged{Flag: Turned, Of: quest.InstanceSubject}},
		{Name: BucketRescued, Predicate: quest.Flagged{Flag: Freed, Of: quest.InstanceSubject}},
		{Name: BucketCaptive, Predicate: quest.Anything{}},
	}
}

// reckoning is the job the failure turns into. Its subjects are left empty on
// purpose: they are whoever ended up in the turned bucket, and those names are
// not known until the population settles.
func reckoning() quest.Successor {
	return quest.Successor{
		When: quest.Every{
			quest.NoneIn{Bucket: BucketCaptive},
			quest.AllIn{Bucket: BucketTurned},
		},
		SubjectsFrom: BucketTurned,
		Opens: quest.Template{
			ID:   ReckoningJob,
			Name: "Turn Them Back — or Put Them Down",
			Objectives: []quest.Objective{{
				ID: "settled-one-way-or-the-other",
				Predicate: quest.Any{
					quest.Flagged{Flag: Redeemed, Of: quest.InstanceSubject},
					quest.Flagged{Flag: Dead, Of: quest.InstanceSubject},
				},
			}},
			Buckets: buckets(),
		},
	}
}

// Scenario is what this content package hands the composer: everything
// declared, and nothing injected.
func Scenario() world.Scenario {
	return world.Scenario{
		Graph:  Declaration(),
		Verbs:  Verbs(),
		Quests: []quest.Template{Contract()},
	}
}
