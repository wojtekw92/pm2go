# ADR 0001: Systemd Is Authoritative

Status: Accepted

## Context

PM2go exists to preserve familiar PM2 operator workflows while reducing the
need for a separate process-manager daemon and its private state. Recreating a
PM2-style registry or dump database would duplicate systemd and create
conflicting sources of truth.

## Decision

Systemd units are the durable application definitions. Systemd runtime
properties are authoritative for lifecycle state. Startup membership is stored
as native systemd target dependencies.

PM2go may store PM2-only compatibility metadata inside its managed units. It
may also keep transient presentation caches and remote observations, but those
must never restore, reconcile, or override local systemd state.

`save` updates a PM2go-owned saved target. `resurrect` starts existing members
of that target. Deleting a service removes both its unit and saved membership.

## Consequences

- External systemd changes are visible without import or synchronization.
- Missing units cannot be recreated by `resurrect`.
- PM2 dump-file semantics are intentionally not reproduced.
- D-Bus and `systemctl` are interchangeable transports over the same model.
- Features requiring another local registry or supervisor need a new ADR.
- Some exact PM2 behavior may be rejected where it conflicts with systemd
  authority; the compatibility documentation must state the difference.

## Rejected Alternatives

- A JSON or SQLite local process registry.
- A restorable copy of complete unit definitions.
- A PM2go daemon supervising application processes.
- Hub state acting as local desired state.
