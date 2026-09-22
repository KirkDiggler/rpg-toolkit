# Level-1 Knowledge Domain grants

Knowledge uses the existing shared Command and Identify spell grants. Identify remains catalog-only until its mechanics are implemented.

Blessings of Knowledge declares two skill choices from Arcana, History, Nature, and Religion. The skill requirement carries `Proficiency: shared.Expert`; finalization applies that rank after ancestry, class, and background proficiencies. Existing skill modifier calculations therefore add twice the proficiency bonus to these two skills, and the rank survives character persistence.

The domain also declares two language choices with explicit standard/exotic options. Languages already learned from ancestry or background cannot consume these picks. Secret class languages are outside this choice list.

## Consumer input

Pass subclass skill/language answers through `SetClassInput.Choices.SubclassChoices` as `choices.Submission` values, preserving the requirement IDs and categories supplied by the toolkit. The IDs are `cleric-knowledge-skills` and `cleric-knowledge-languages`. Values use the existing skill/language IDs. The toolkit stamps `SourceSubclass`, rejects foreign or repeated requirement IDs, copies values, and clears old subclass answers when the class/domain changes.

Keep the ordinary Cleric skills in `ClassChoices.Skills`. Do not combine domain skills with that selection or turn the domain skill grant into a separate expertise pick. Language counts validate per requirement ID, so a Human's additional language does not count against Knowledge's two.

This root change does not update consumer dependency pins or transport adapters. API/web must forward the subclass submissions before the browser can finish Knowledge creation.

## Verification

Cleric finalization tests exercise public SetClass input, Human ancestry alongside the two domain languages, draft serialization, character reload, skill modifiers, shared spell refs, domain switching, invalid selections, and known-language rejection. The full root test suite and golangci-lint pass.
