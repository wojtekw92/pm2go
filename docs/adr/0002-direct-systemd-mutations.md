# ADR 0002: Direct Systemd Mutations With Explicit Partial Failures

Status: Accepted

## Context

Deletion commit markers, renamed backups, recovery scans, and saved-directory
exchange/rollback made the wrapper harder to follow and introduced independent
state-recovery logic. The product needs a clear translation layer over systemd,
not another transactional process manager.

## Decision

Use direct, serialized operations on owned units and systemd links. Keep atomic
individual unit writes, input validation, timeouts, and systemd result checks.
Do not keep backup definitions or scan for interrupted transactions.

Failed starts keep their definitions for inspection. Delete stops a service
before unlinking its membership and definition. Save updates the saved target's
links in place. Filesystem and reload failures return explicit errors describing
partial progress; they do not trigger hidden rollback or replay.

## Consequences

- Command flows are short and auditable.
- Multi-unit changes can partially succeed, including when interrupted.
- Operators inspect the existing units and links, fix the cause, and retry the
  appropriate operation. Deleted definitions are never silently recreated.
- Existing experimental dump files and backup artifacts are ignored rather than
  automatically migrated or deleted.
- Tests cover observable filesystem and command outcomes instead of a custom
  transaction-recovery protocol.
