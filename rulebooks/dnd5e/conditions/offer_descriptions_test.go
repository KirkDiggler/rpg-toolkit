package conditions

// Each die-holding condition authors the prose of the offer it poses, so the
// player reads what taking the die does before committing it (R12).

func (s *InspiredConditionTestSuite) TestInspiredGuidedResistanceOffersDescribed() {
	offers := s.foldOffers("fighter-1")
	s.Require().Len(offers, 1)
	s.Equal(InspiredOfferDescription, offers[0].Description)
	s.NotEmpty(offers[0].Description)
}

func (s *GuidedConditionTestSuite) TestInspiredGuidedResistanceOffersDescribed() {
	offers := s.foldOffers("rogue-1")
	s.Require().Len(offers, 1)
	s.Equal(GuidedOfferDescription, offers[0].Description)
	s.NotEmpty(offers[0].Description)
}

func (s *ResistanceConditionTestSuite) TestInspiredGuidedResistanceOffersDescribed() {
	offers := s.foldOffers("rogue-1")
	s.Require().Len(offers, 1)
	s.Equal(ResistanceOfferDescription, offers[0].Description)
	s.NotEmpty(offers[0].Description)
}
