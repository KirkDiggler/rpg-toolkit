# Implementation

War finalization now applies heavy armor and the canonical martial weapon grant.
Tests finalize a native draft, round-trip draft and character JSON, and verify
both longsword and longbow attacks gain +2 accuracy without changing damage.
Switching back to Life drops the martial grant. Full root tests and lint pass.

This corrects newly created characters only. Existing saved character migration,
War Priest and nonproficient-armor penalties are not implemented by this change.
API adoption and persistence coverage follow in a separate PR.
