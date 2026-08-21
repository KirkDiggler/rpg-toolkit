# Architecture Decision Records

This directory contains Architecture Decision Records (ADRs) for the RPG Toolkit project.

## What is an ADR?

An ADR documents a significant architectural decision made in the project, including:
- The context and problem statement
- The decision made
- The consequences of that decision

## ADR Format

Each ADR follows this structure:
1. **Title**: ADR-NNNN: Brief description
2. **Date**: When the decision was made
3. **Status**: Proposed, Accepted, Deprecated, Superseded
4. **Context**: Why we needed to make this decision
5. **Decision**: What we decided to do
6. **Consequences**: The results of this decision (positive, negative, neutral)

## Current ADRs

**See [DECISIONS.md](DECISIONS.md)** — the cliffnotes digest of every decision,
one or two lines each, with the rule it generalises to.

The accepted composable attack-damage rules are recorded in [ADR-0041](0041-composable-attack-damage.md),
which supersedes ADR-0036's selective-critical proposal.

Read that instead of this directory. Thirty-plus ADRs is a large context load and
much of it is history: superseded shapes, proposals never built, and narrative
that made sense at the time. Open a full ADR when you are about to contradict one,
or need the reasoning behind a specific trade-off.

This section used to hold a hand-maintained list. It drifted to naming 7 of 37 and
nothing failed, which is why the digest is enforced by
`scripts/check-decisions.sh` in CI instead of maintained by memory. A list nobody
is forced to update is a list that quietly stops being true.

## Creating a New ADR

1. Copy the template from `template.md`
2. Number it sequentially — **check for collisions first**; this corpus already
   has two `0006`s and two `0019`s, and one of those is titled "ADR-0014"
   internally. `ls docs/adr` before choosing.
3. Fill in all sections. **Record the options you rejected and why**, including
   the ones that were tempting. A decision reads as obvious in hindsight; the
   alternatives are what a future reader needs in order to reopen it honestly.
4. Set status to "Proposed"
5. After team review, update status to "Accepted"
6. **Add a one-line entry to [DECISIONS.md](DECISIONS.md)** — the decision, and
   the rule it generalises to. CI fails until you do
   (`scripts/check-decisions.sh`), because that file is what people actually
   read.
