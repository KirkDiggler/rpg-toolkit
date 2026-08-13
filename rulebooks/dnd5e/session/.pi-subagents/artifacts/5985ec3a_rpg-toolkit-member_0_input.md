# Task for rpg-toolkit-member

Goal: advise whether any rpg-toolkit implementation is required for the approved contributor sandbox, and define the toolkit boundary/test expectations.
Target: /home/kirk/game-dev/rpg-toolkit, reason from origin/main/current canonical docs without changing checkout state. Read-only: do not edit, checkout, fetch, commit, push, tag, or publish.
Approved direction: local rpg-api build replaces rulebooks/dnd5e; rpg-api seed command creates Protection Fighter and Barbarian through the running real CharacterService draft/requirements/choices/finalize APIs; web authors dungeon YAML and starts through real lobby APIs. No direct Redis data construction and no new generic toolkit fixture framework.
Confirm how Protection and Barbarian choices should be selected via existing toolkit-driven APIs, what module tests a future monster owner should run, whether the dungeon spec dependency on encounter creates a second local-override need, and any module/version pitfalls. Push back on boundary violations.
Return a concise advisory brief with exact evidence paths and explicit yes/no on toolkit code changes for MVP. No edits or file dumps.

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