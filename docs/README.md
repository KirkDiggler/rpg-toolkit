# RPG Toolkit Documentation

This directory contains the primary documentation for the RPG Toolkit project.

## Structure

### Active Documentation

- **[adr/](adr/)** - Architectural Decision Records
  - Formal records of significant architectural decisions
  - Numbered ADRs documenting the "what" and "why" of major design choices
  - See [adr/README.md](adr/README.md) for the index

- **[journey/](journey/)** - Development Journey Logs
  - Chronological exploration and discovery logs
  - Documents the thinking process, experiments, and evolution of ideas
  - Captures the "how we got here" story
  - See [journey/README.md](journey/README.md) for the index

### Archive

- **[archive/](archive/)** - Historical Documentation
  - Older design documents, planning materials, and examples
  - Preserved for reference but not actively maintained
  - Can be selectively restored if needed
  - Includes: design docs, planning materials, issues analysis, examples, guides, diagrams, and reference materials

## Documentation Philosophy

Per the project's CLAUDE.md guidelines:

1. **Journey Docs** (`journey/`): Tell the story of exploration and decisions
2. **ADRs** (`adr/`): Formal architectural decisions
3. **READMEs**: Concise summaries of what exists

## Contributing

When adding new documentation:

- **ADRs**: Use for formal architectural decisions that affect the whole system
- **Journey logs**: Use for exploration, design thinking, and development narratives
- **Archive**: Don't add new docs here - this is for historical preservation only

For active feature planning and ideas, consider creating an `ideas/` directory when needed.
