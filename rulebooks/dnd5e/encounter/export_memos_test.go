package encounter

// ExportedWorldThinking marks e as inside or outside a wake pass, the
// condition under which [Encounter.participationNow] keeps one assessment.
func (e *Encounter) ExportedWorldThinking(on bool) {
	e.worldThinking = on
	if !on {
		e.wakeParticipation = nil
	}
}
