# Observed implementation

Provider implementation: root commit 1a1511f7347f, draft toolkit #1832.
Full root `go test -race ./...` and `golangci-lint run ./...` passed.
The additional monster regression passed after the full run. The condition uses
existing ACChain and concentration custody. Monsters now fold their attached
AC chain over authored stat-block AC; the temporary bonus is never persisted
into the base stat block.

API implementation: commit ff1ffee, draft rpg-api #1011, rooted at dev fb7ed78.
Only root changes to v0.184.1-0.20260918183851-1a1511f7347f; session v0.98.0,
encounter v0.91.0 and resolution v0.55.0 remain. Required pre-commit and ci-check
passed. Focused ShieldOfFaithSuite passed with race detection: native acquisition,
self/ally protection, private AC and authorization, exact live/Story replay,
concentration replacement, both orders of leveled-spell refusal without payment,
legal action cantrip and weapon attack, and hit-to-miss protection for character
and monster recipients after reload.

Local image rpg-api:shield-of-faith runs behind localhost:3001. Browser verification
observed Shield Of Faith among nine native Cleric spell choices. Actual browser
combat acceptance remains pending with the user. No web or proto change needed.
Keep both PRs draft until accepted. Replace API pseudo pin with the published
root release during user-directed merge sequencing; do not merge automatically.
