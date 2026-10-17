# Exam Management System

Class project for WAD.

Rewrite of the legacy faculty exam scheduler using Go/Fiber, PostgreSQL, React/Vite/Tailwind, and Keycloak.

Local planning and report documents are kept in the ignored `development/` directory. Runtime database and identity-provider configuration is versioned under `infrastructure/`.

The original SQL Server application remains unchanged under the local `legacy/` directory. It is excluded from this replacement repository because it contains demo credentials and destructive setup scripts. The replacement does not connect to or modify its database.

Repository: https://github.com/reekta92/wad_project

This managed workspace makes `.git/` read-only. Its local Git metadata is stored in `.git-store/`; use `git --git-dir=.git-store --work-tree=.` here. A normal clone from GitHub uses standard Git commands.
