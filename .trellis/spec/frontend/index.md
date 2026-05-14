# Frontend Development Guidelines

> Best practices for frontend development in this project.

---

## Overview

This directory contains taskdaemon frontend guidelines. The frontend app has not been scaffolded yet, so these files describe the decisions already captured in Trellis PRDs and the conventions future implementation should follow.

---

## Guidelines Index

| Guide | Description | Status |
|-------|-------------|--------|
| [Directory Structure](./directory-structure.md) | Module organization and file layout | Filled |
| [Component Guidelines](./component-guidelines.md) | Component patterns, props, composition | Filled |
| [Hook Guidelines](./hook-guidelines.md) | Custom hooks, data fetching patterns | Filled |
| [State Management](./state-management.md) | Local state, global state, server state | Filled |
| [Quality Guidelines](./quality-guidelines.md) | Code standards, forbidden patterns | Filled |
| [Type Safety](./type-safety.md) | Type patterns, validation | Filled |

---

## Pre-Development Checklist

Before frontend implementation:

1. Read [Directory Structure](./directory-structure.md).
2. Read [Component Guidelines](./component-guidelines.md) before creating UI.
3. Read [Hook Guidelines](./hook-guidelines.md) and [State Management](./state-management.md) before adding data fetching or shared state.
4. Read [Type Safety](./type-safety.md) before adding DTOs, forms, schemas, or API client code.
5. Read [Quality Guidelines](./quality-guidelines.md) before writing tests or adding dependencies.

The goal is to keep Web/Desktop UI implementation aligned with the management-console product direction.

---

**Language**: Project working documentation is written in Chinese unless a generated tool requires otherwise.
