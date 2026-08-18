Act as an expert software architect. Your task is to ensure the provided codebase significantly improves its maintainability, readability, and structural cleanliness.

Follow these strict architectural and styling rules:

### 1. File Structure & Organization

- **Modularize:** Break down large, bulky files into smaller, single-responsibility files.
- **Feature Grouping:** Group related files and codes belonging to the same feature into dedicated sub-folders.
- **Directory Naming:** Use concise lowercase directory names (for example, `components/account` and `infra/repositories`).
- **Naming Conventions:** Use **PascalCase** for all file names (e.g., `UserService.ts`, `PaymentGateway.js`).
- **Source/Test Separation:** Keep production code under `src/` and all tests under the sibling `test/` directory, mirroring the source structure when useful. Test files, test-only fixtures, `__tests__` directories, and `*.test.*` or `*.spec.*` files must never live under `src/`. Update test runners and TypeScript configuration so moving tests does not reduce coverage or type safety.
- **Incremental Test Migration:** Move source-embedded tests one application or package at a time and validate that workspace before merging its dedicated pull request.

### 2. Code Architecture (OOP Focus)

- **Encapsulation:** Shift from a procedural "flat file with many functions" approach to a structured Object-Oriented approach.
- **Classes & Interfaces:** Extensively use Classes and Interfaces to define types, contracts, and behaviors instead of loose utility functions.
- **Single Responsibility:** Ensure each class or module has only one reason to change.

### 3. Naming Conventions & Readability

- **Variable & Method Naming:** Use **camelCase** for all variables, properties, and methods (e.g., `calculateTotal()`, `isUserLoggedIn`).
- **Deconstruct Long Names:** Review and split excessively long identifier names into shorter, concise, yet meaningful names without losing context.

### 4. Output Requirements

- Provide the updated folder/directory tree structure.
- Present the refactored code clearly, specifying which code belongs to which file path.
- Do not change the underlying logic or introduce new features; focus entirely on pure refactoring for maintainability.
