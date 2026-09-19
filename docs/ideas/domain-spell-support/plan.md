# Plan

1. Implement Divine Favor first in the root rules module, then adopt its pushed
   commit in an API draft from dev and test the public cast and local browser.
2. Implement Burning Hands, Faerie Fire and Fog Cloud in subsequent spell slices.
3. Batch NYI sheet visibility last: Animal Friendship, Speak with Animals,
   Charm Person, Disguise Self, Identify, and Light. Every grant remains visible;
   unsupported spells do not pretend to cast. Charm Person is explicitly deferred.
4. Keep one nearest-go.mod module per toolkit PR. Open drafts on meaningful
   pushes; use real pushed pseudo versions, no local overrides.
5. Verify payment, concentration, weapon damage, persistence/replay and browser
   acceptance. Preserve Bard creation and Cleric preparation/grant counts.
6. Domain proficiency and features remain subsequent work. No automatic merges.
