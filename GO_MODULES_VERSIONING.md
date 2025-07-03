# Go Module Versioning in Multi-Module Repositories

## Current Approach (Relative Paths)
```go
replace github.com/KirkDiggler/rpg-toolkit/core => ../../core
```

**Problems:**
- Can't version modules independently
- Users must clone entire repository
- Can't `go get` individual modules
- Breaks when published

## Better Approach: Go Workspaces (Go 1.18+)

### 1. Create go.work file at repository root
```bash
go work init
go work use ./core
go work use ./events
go work use ./mechanics/conditions
```

### 2. Remove replace directives from go.mod files
Each go.mod becomes clean:
```go
module github.com/KirkDiggler/rpg-toolkit/mechanics/conditions

go 1.24

require (
    github.com/KirkDiggler/rpg-toolkit/core v0.1.0
    github.com/KirkDiggler/rpg-toolkit/events v0.1.0
)
```

### 3. How Versioning Works

**Option A: Single Version for All Modules**
- Tag releases as `v0.1.0`
- All modules get the same version
- Simpler but less flexible

**Option B: Module-Specific Versions**
- Tag releases as `core/v0.1.0`, `events/v0.2.0`
- Each module can evolve independently
- More complex but more flexible

### 4. For Local Development
The go.work file handles local development:
```
rpg-toolkit/
├── go.work         # Local development only (not committed)
├── core/
│   └── go.mod
├── events/
│   └── go.mod
└── mechanics/
    └── conditions/
        └── go.mod
```

### 5. For Users
Users can install individual modules:
```bash
go get github.com/KirkDiggler/rpg-toolkit/core@v0.1.0
go get github.com/KirkDiggler/rpg-toolkit/events@v0.1.0
```

## Recommended Setup

1. **Use go.work for local development**
2. **Remove replace directives**
3. **Use module-specific version tags**
4. **Document module compatibility matrix**

## Example Compatibility Matrix

| Module | Compatible Core | Compatible Events |
|--------|----------------|-------------------|
| conditions v0.1.0 | core v0.1.0 | events v0.1.0 |
| combat v0.1.0 | core v0.1.0 | events v0.1.0-v0.2.0 |
| dnd5e v1.0.0 | core v0.1.0+ | events v0.2.0+ |