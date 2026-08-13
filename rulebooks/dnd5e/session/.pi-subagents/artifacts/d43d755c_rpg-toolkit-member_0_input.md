# Task for rpg-toolkit-member

Review KirkDiggler/rpg-toolkit PR #924 as the canonical toolkit standing owner and report to the director. This is review-only; do not implement or mutate GitHub.

Primary target: https://github.com/KirkDiggler/rpg-toolkit/pull/924, full diff, commits, CI/checks, reviews/comments, linked issue #922, design PR #923, and the nearest canonical encounter transition docs/tests. Determine exact base/head SHAs from GitHub and inspect the committed PR head, not an incidental local checkout.

Context from the prior API assessment: rpg-api #793 was blocked because v0.1 used room-local SquareGrid/2D data while the wire/web are absolute cube hex; View lacked per-cell visible/remembered projection; multi-room traversal was unreleased; API persistence needs typed coexistence; reads must not Pump; API may only perform pure route adaptation and reattach decider providers, never own topology/pathing/behavior.

Questions to answer:
1. Does #924 correctly and completely deliver its own stated traversal/decider/grid-shape contract, preserving toolkit ownership and LoadFromData/ToData determinism? Find correctness, aliasing, validation, ordering, reload, or boundary defects with concrete evidence.
2. Which #793 blockers does #924 actually close, partially close, or leave open? In particular verify HexGrid support versus merely identifying shape, public projection/View support for total per-cell VISIBLE/REMEMBERED state, stable local↔absolute coordinate/room mapping, Traverse intent/path adaptation, production decider contribution/re-registration, and event/delta seams.
3. Is v0.2.0 the honest gorelease classification, and is the PR ready for Kirk to merge? Distinguish CI green from behavioral readiness.
4. After #924, what is the thinnest honest rpg-api seam and what still requires toolkit/proto/web work or an explicit Kirk decision?

Authority: strictly read-only. Do not edit files, run commands that mutate go.mod/go.sum, create branches, comment/review on GitHub, commit, push, merge, tag, publish, or update role context. Use temporary/clean inspection as needed and avoid running broad commands unless necessary.

Return a concise evidence-backed report: blockers/high findings first, then what is solid, a #793 blocker matrix, merge recommendation, API implications, and links/file:line references. Separate verified facts from inference.

## Acceptance Contract
Acceptance level: attested
Completion is not accepted from prose alone. End with a structured acceptance report.

Criteria:
- criterion-1: Return concrete findings with file paths and severity when applicable

Required evidence: review-findings, residual-risks

Finish with a fenced JSON block tagged `acceptance-report` in this shape:
Use empty arrays when no items apply; array fields contain strings unless object entries are shown.
`criteriaSatisfied[].status` must be exactly one of: satisfied, not-satisfied, not-applicable.
`commandsRun[].result` must be exactly one of: passed, failed, not-run.
`manualNotes` and `notes` are optional strings; an empty string means no note and does not satisfy `manual-notes` evidence.
```acceptance-report
{
  "criteriaSatisfied": [
    {
      "id": "criterion-1",
      "status": "satisfied",
      "evidence": "specific proof"
    }
  ],
  "changedFiles": [
    "src/file.ts"
  ],
  "testsAddedOrUpdated": [
    "test/file.test.ts"
  ],
  "commandsRun": [
    {
      "command": "command",
      "result": "passed",
      "summary": "short result"
    }
  ],
  "validationOutput": [
    "validation output or concise summary"
  ],
  "residualRisks": [
    "none"
  ],
  "noStagedFiles": true,
  "diffSummary": "short description of the diff",
  "reviewFindings": [
    "blocker: file.ts:12 - issue found, or no blockers"
  ],
  "manualNotes": "anything else the parent should know"
}
```