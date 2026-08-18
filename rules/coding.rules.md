Act as an expert software architect. Your task is to ensure the `decionis/docker` codebase significantly improves its maintainability, readability, and structural cleanliness.

This monorepo hosts two toolchains with distinct idioms. Apply the shared rules everywhere and the per-language rules within their trees:

- **TypeScript / React** — the Docker Desktop extension UI (`extension/ui`).
- **Go** — the Decionis Docker daemon, authority proxy, CLI, and Dev Container helper (`cmd/`, `internal/`).

Follow these strict architectural and styling rules:

### 1. File Structure & Organization

- **Modularize:** Break down large, bulky files into smaller, single-responsibility files.
- **Feature Grouping:** Group related files and code belonging to the same feature into dedicated sub-folders (for example, `extension/ui/src/features/decisions` and `internal/presence`).
- **Directory Naming:** Use concise lowercase directory names in both trees.
- **TypeScript Naming:** Use **PascalCase** for all TypeScript file names (e.g., `DecisionFeed.tsx`, `DossierService.ts`).
- **Go Naming:** Follow the Go toolchain's conventions, not the TypeScript ones: lowercase package and file names (`client.go`, `eventhub.go`), `gofmt` and `golangci-lint` clean, exported identifiers in PascalCase, unexported in camelCase.
- **Source/Test Separation (TypeScript):** Keep production code under `src/` and all tests under the sibling `test/` directory, mirroring the source structure when useful. Test files, test-only fixtures, `__tests__` directories, and `*.test.*` or `*.spec.*` files must never live under `src/`. Update test runners and TypeScript configuration so moving tests does not reduce coverage or type safety.
- **Source/Test Separation (Go):** Go tests stay colocated as `_test.go` files in their package, as the toolchain requires; shared fixtures live in the package's `testdata/` or under `internal/testutil`.

### 2. Code Architecture

- **Protocol boundary (overriding rule):** The Decionis protocol/API is the stable boundary. This repository is the Docker integration surface, not "the Go implementation of Decionis." Components here request, transport, enforce, and display authority decisions; they never re-implement the policy evaluator, verdict semantics, or dossier issuance. Evaluation happens only in the containerized `@decionis/mcp` evaluator or the hosted control plane. A capability that needs evaluator changes is an upstream change to `decionis/decionis`, consumed here at a pinned version.
- **Encapsulation (TypeScript):** Shift from a procedural "flat file with many functions" approach to a structured object-oriented approach. Extensively use classes and interfaces to define types, contracts, and behaviors instead of loose utility functions.
- **Composition (Go):** Prefer small interfaces, composition, and the standard Go project layout over class-hierarchy emulation.
- **Single Responsibility:** Ensure each class, package, or module has only one reason to change.

### 3. Naming Conventions & Readability

- **Variable & Method Naming (TypeScript):** Use **camelCase** for all variables, properties, and methods (e.g., `calculateDrift()`, `isPresenceVerified`).
- **Deconstruct Long Names:** Review and split excessively long identifier names into shorter, concise, yet meaningful names without losing context.
- **One Vocabulary:** Verdict and outcome identifiers use the protocol's published vocabulary; never coin repo-local synonyms.

### 4. Output Requirements

- Provide the updated folder/directory tree structure.
- Present the refactored code clearly, specifying which code belongs to which file path.
- Do not change the underlying logic or introduce new features; focus entirely on pure refactoring for maintainability.
