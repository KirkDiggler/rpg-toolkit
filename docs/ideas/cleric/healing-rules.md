# Cure Wounds rules and implementation decisions

Reviewed 2026-09-12. This slice uses **2014**, not the revised spell.

- [Cure Wounds, Basic Rules (2014), p.230](https://www.dndbeyond.com/spells/2056-cure-wounds): one action, touch, instantaneous, d8 plus casting modifier. Undead and constructs receive no benefit. This implementation uses a first-level slot; printed higher-slot scaling is deferred.
- [2014 spellcasting: targets and range](https://www.dndbeyond.com/sources/dnd/basic-rules-2014/spellcasting): a creature-targeted spell can select its caster unless its text excludes them. Targets require a clear path. Cure Wounds does not require seeing the recipient.
- Disciple of Life's reviewed class entry and source are retained in [source-index.json](source-index.json). Its contribution applies to healing with a spell of level one or higher; it adds two plus that level. It is not a bonus to every healing source.

The following are explicit implementation decisions, not additional printed spell text:

- Touch uses one five-foot grid step and the encounter's physical obstruction facts. Known targets remain selectable without a current sighting; this does not discover unknown creatures.
- Undead/construct declarations are valid paid casts. They roll no healing dice and record zero requested/applied healing with a source label explaining the immunity.
- Living, full-HP targets remain valid and pay normally. Their requested roll is retained even when zero HP is restored.
- Ordinary healing accepts dying or stabilized characters but does not revive dead characters or defeated monsters, following this toolkit's life-state model.
- Negative healing sums restore zero. The trace retains the negative casting modifier and an explicit adjustment to zero; recipient handlers also reject negative incoming amounts.
- Catalogue monster family is derived from its canonical ref when an older sheet lacks a type. Custom sheets can declare a type. Per the user's decision, missing/unknown family data does not match an exclusion: the creature remains selectable and healing proceeds normally. Only a known excluded type produces the paid no-effect result. Loading does not rewrite old records just to add derived metadata. Broad creature-type adoption is outside the Cleric slice.

Components/focus enforcement, preparation, upcasting, world-clock casting and other-repository adoption remain outside this slice.
