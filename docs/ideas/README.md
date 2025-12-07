# Ideas

Self-contained directories for ideas progressing from brainstorm to implementation.

## Structure

Each idea gets its own directory:

```
docs/ideas/<idea-name>/
├── progress.json    # Tracks phases, decisions, reasoning
├── README.md        # Human-readable summary and status
├── brainstorm.md    # Initial exploration
├── use-cases.md     # Concrete scenarios
├── design.md        # Implementation specification
└── ...              # Additional docs as needed
```

## progress.json

The spine of each idea. Tracks:

- **status**: Current phase (brainstorming, designing, implementing, complete)
- **phases**: Each phase with status, date, file, and notes
- **decisions**: Key choices made with reasoning

Phases can be marked as:
- `pending` - Not started
- `in_progress` - Currently working on
- `completed` - Done
- `skipped` - Intentionally skipped (must include reason)

## Philosophy

- Each directory tells the complete story
- JSON tracks what happened and why
- Decisions are captured so they survive context switches
- Phases can be skipped but must explain why

## Current Ideas

| Idea | Status | Summary |
|------|--------|---------|
| [action-economy-history](./action-economy-history/) | Brainstorming | Track what actions were taken, not just how many remain |
