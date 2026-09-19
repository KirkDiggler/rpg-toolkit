# Divine Favor checkpoint

The root rules module now declares Divine Favor as a self-only level-one bonus
spell, consuming one bonus action and slot and owning ten turn ends of
concentration. The condition adds sourced radiant d4 damage to each weapon hit,
doubles its dice on critical hits, uses the interaction roller after reload,
and leaves spell damage and other attackers unchanged. Factory, JSON loader,
canonical ref, display metadata, and long-rest removal are registered.

Focused condition/cast tests, the full root module tests, and lint pass.
API integration and local browser acceptance remain pending. The other combat
spells and NYI sheet visibility are subsequent work, not delivered by this PR.
