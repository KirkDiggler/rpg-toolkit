// Copyright (C) 2026 Kirk Diggler
// SPDX-License-Identifier: GPL-3.0-or-later

package encounter

// everyoneStanding is the Standing capability the internal tests install.
//
// Nobody is ever down: these tests are about clocks, not about hit points, and
// the capability is required at construction so they have to say so. The
// rulebook's real answer is the production consumer's business.
type everyoneStanding struct{}

func (everyoneStanding) Standing([]MemberID) ([]MemberID, error) {
	return nil, nil
}

func (everyoneStanding) Assess(members []MemberID) (*ParticipationAssessment, error) {
	return testAssessmentFromDown(members, nil), nil
}

func testAssessmentFromDown(members, reported []MemberID) *ParticipationAssessment {
	down := make(map[MemberID]bool, len(reported))
	for _, id := range reported {
		down[id] = true
	}
	assessment := &ParticipationAssessment{}
	for _, id := range members {
		member := MemberParticipation{Member: id, Contact: true, Conscious: true, Turn: TurnParticipationWait}
		if down[id] {
			member.Down = true
			member.Contact = false
			member.Conscious = false
			member.Turn = TurnParticipationRemove
		}
		assessment.Members = append(assessment.Members, member)
	}
	return assessment
}
