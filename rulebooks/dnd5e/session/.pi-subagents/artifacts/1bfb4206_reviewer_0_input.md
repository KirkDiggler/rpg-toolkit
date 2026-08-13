# Task for reviewer

Independently verify the pivotal findings from the canonical toolkit owner’s review of KirkDiggler/rpg-toolkit PR #924. Review the current GitHub PR head; report if it differs from 41412f57bafb40068c50152d6d0fba9387cec60e. Use a clean temporary archive/worktree, never the long-lived checkout.

Narrow questions:
1. Can a map-present nil Decider pass LoadEncounter and then panic in Pump? Reproduce safely with the smallest targeted test/probe if useful; state correct severity and smallest contract-level fix.
2. Does AxialHexGrid accept fractional Q/R positions that cube conversion truncates? Verify against the exact spatial version #924 consumes and determine whether #924 itself can honestly claim identity mapping to integer cube wire coordinates.
3. Are public IntelDeltas ordering nondeterministic because refreshSight ranges a map and Surveil preserves input order? Determine whether this is observable and whether it blocks API stream projection or is merely an unordered-contract documentation issue.
4. Does #924 add any public total per-cell visible/remembered projection, own-position projection, absolute room embedding, or route-intent owner that the first review missed?
5. Verify the stale #923 plan-vs-design inconsistency and the unresolved GitHub review state only if still current.

Return only evidence-backed agree/disagree/correction per finding and a final merge recommendation. Cite current-head links/file:line and targeted command results. Do not edit persistent files, mutate GitHub, run subagents, or use commands that can rewrite the real repo’s go.mod/go.sum. Delete temporary probe files before completion and report final persistent repo status unchanged.

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