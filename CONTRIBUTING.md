# Contributing to Gumi

Thank you for your interest in contributing to Gumi.

Gumi is a local-first AI runtime designed to make local models more stable, observable, and production-ready.

---

## Core Rules

Before contributing, please understand these rules:

1. Keep Gumi local-first.
2. Do not add cloud providers to V1.
3. Do not add billing or hosted accounts to V1.
4. Do not bypass the Pipeline Engine.
5. Keep provider adapters thin.
6. Do not store full prompts by default.
7. Do not store full responses by default.
8. Do not send external telemetry by default.
9. Keep the runtime modular.
10. Update documentation when behaviour changes.

---

## Architecture Source of Truth

Read the files in `docs/` before making major changes.

Important files:

```text
docs/00-vision-and-positioning.md
docs/02-runtime-architecture.md
docs/04-api-specification.md
docs/07-pipeline-specification.md
docs/14-implementation-roadmap.md