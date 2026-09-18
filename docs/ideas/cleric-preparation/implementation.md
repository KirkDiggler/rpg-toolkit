# Implementation and verification

Cleric-only preparation uses an explicit progression column; Bard and other
classes retain their existing known-spell behavior. Creation records four chosen
level-one spells and compiles domain spells plus bonus cantrips additively into
the existing persisted spell-access lists. Domain grants are excluded from paid
choices. Light remains non-executable pending object targeting and illumination.
Existing characters are not rewritten; long-rest re-preparation remains deferred.

Provider code commit: 219808f4164def20e620f76ada2b0bab39b80773, draft #1833.
Owning root module: full race suite, lint, formatting and module tidiness passed.
Bard regression covers every supported four-spell combination through finalization
and JSON reload. Cleric tests cover exact counts, grants, duplicate prevention,
Light, persistence and level-two progression.

API draft KirkDiggler/rpg-api#1012 adopts the pushed provider pseudo version,
without go.work or replaces. Full API session race suite, focused Cleric/Bard
acceptance, make pre-commit and make ci-check passed. Real handlers prove
chosen preparations and Life grants reach Cast, unselected spells and Light do
not, and Bard still finalizes with four known spells.

Local browser verification (2026-09-18, localhost:3001):
- Life picker requires four and excludes granted Bless/Cure Wounds.
- Light picker requires three cantrips and excludes the granted Light.
- Bard still offers four-of-five spells and two cantrips; completed creation.
- Completed Light Cleric creation with four preparations and three cantrips.
- Read-only Redis verification confirms the new Light Cleric has six spell refs
  (four selections plus Burning Hands/Faerie Fire) and four cantrips (including
  Light). The new Bard has exactly its four selections and two cantrips.

Local API image rpg-api:cleric-preparation is running with unchanged web code.
The draft preview shows chosen inputs, not automatic grants; the existing sheet
has no full spell list. Persisted grants were verified separately above.
User manual acceptance and provider release/consumer release-pin replacement
remain before merging. No PRs were merged and hosted CI was not awaited.
