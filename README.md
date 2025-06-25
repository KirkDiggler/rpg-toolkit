# RPG Toolkit

A modular toolkit for building RPG game engines, bots, and applications. Build once, use everywhere - from Discord bots to Unity games.

## Vision

RPG Toolkit provides reusable components for creating role-playing game experiences across any platform. Whether you're building a Discord bot, web app, or game engine, our modular architecture lets you pick and choose the systems you need.

## Architecture

```
rpg-toolkit/
├── core/                 # Foundation for all RPG systems
│   ├── entities/         # Players, NPCs, monsters
│   ├── state/            # Game state management
│   ├── events/           # Event system
│   └── storage/          # Persistence
├── systems/              # Generic RPG mechanics
│   ├── combat/           # Base combat interfaces
│   ├── inventory/        # Item management
│   ├── progression/      # Leveling, XP
│   ├── world/            # Maps, locations, dungeons
│   ├── dialogue/         # Conversations, choices
│   └── quests/           # Quest tracking
└── games/                # Game-specific implementations
    ├── d20/              # D&D, Pathfinder (d20 system)
    │   ├── dnd5e/        # D&D 5th Edition
    │   └── pathfinder/   # Pathfinder specific
    ├── pbta/             # Powered by the Apocalypse games
    ├── fate/             # FATE system games
    └── custom/           # User-created systems
```

## Core Principles

- **Modular**: Use only what you need
- **Platform Agnostic**: Works with Discord, Unity, web apps, CLI tools
- **Game System Flexible**: Support multiple RPG rulesets
- **TypeScript First**: Full type safety and great DX
- **Well Tested**: Comprehensive test coverage

## Getting Started

```bash
npm install rpg-toolkit
```

```typescript
import { DungeonGenerator, Character } from 'rpg-toolkit';
import { DnD5e } from 'rpg-toolkit/games/dnd5e';

// Create a character
const hero = new Character(DnD5e.templates.fighter);

// Generate a dungeon
const dungeon = new DungeonGenerator().generate({
  size: 'medium',
  difficulty: 'moderate',
  theme: 'ancient_ruins'
});
```

## Roadmap

### Phase 1: Core Foundation
- [ ] Entity system (characters, monsters, NPCs)
- [ ] State management
- [ ] Event system
- [ ] Basic storage interfaces

### Phase 2: Essential Systems
- [ ] Combat engine
- [ ] Inventory management
- [ ] World/dungeon generation
- [ ] Quest system

### Phase 3: Game Implementations
- [ ] D&D 5e ruleset
- [ ] Generic d20 system
- [ ] Custom game builder

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for details on our code of conduct and the process for submitting pull requests.

## License

MIT

## Links

- [GitHub Discussions](https://github.com/KirkDiggler/rpg-toolkit/discussions) - Design decisions, ideas
- [Project Board](https://github.com/KirkDiggler/rpg-toolkit/projects) - Track development progress
- [Wiki](https://github.com/KirkDiggler/rpg-toolkit/wiki) - Documentation, guides