package core_test

// Shared test fixture constants — extracted to satisfy goconst across
// action_test.go, ref_test.go, and typed_ref_test.go. Names carry a
// `test` prefix to avoid shadowing the `core` package import.
const (
	testRage           = "rage"
	testDarkvision     = "darkvision"
	testModuleCore     = "core"
	testTypeFeature    = "feature"
	testModuleCombat   = "combat"
	testTypeEvent      = "event"
	testErrInvalidChar = "invalid characters"
	// testIDPart2 is what a refusal calls the SECOND part of an id, which
	// is the part a trailing separator leaves empty.
	testIDPart2 = "id part 2"
)
