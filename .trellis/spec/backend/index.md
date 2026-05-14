# Backend Development Guidelines

> Best practices for backend development in this project.

---

## Overview

This directory contains taskdaemon backend guidelines. The project is currently early-stage, so these files combine existing code facts with decisions already captured in Trellis PRDs. When code lands, update the guidelines to reflect the implemented reality.

---

## Guidelines Index

| Guide | Description | Status |
|-------|-------------|--------|
| [Directory Structure](./directory-structure.md) | Module organization and file layout | Filled |
| [Database Guidelines](./database-guidelines.md) | ORM patterns, queries, migrations | Filled |
| [Error Handling](./error-handling.md) | Error types, handling strategies | Filled |
| [Quality Guidelines](./quality-guidelines.md) | Code standards, forbidden patterns | Filled |
| [Logging Guidelines](./logging-guidelines.md) | Structured logging, log levels | Filled |

---

## Pre-Development Checklist

Before backend implementation:

1. Read [Directory Structure](./directory-structure.md).
2. Read the domain-specific guide for the layer you are touching.
3. For scheduler/runner work, read [Quality Guidelines](./quality-guidelines.md).
4. For data/auth/config/API work, read [Database Guidelines](./database-guidelines.md), [Error Handling](./error-handling.md), and [Logging Guidelines](./logging-guidelines.md).
5. Check parent task PRDs under `.trellis/tasks/` when a guide says the behavior is planned but not implemented yet.

The goal is to help AI assistants and new team members match taskdaemon's current decisions without inventing a different architecture.

---

**Language**: Project working documentation is written in Chinese unless a generated tool requires otherwise.
