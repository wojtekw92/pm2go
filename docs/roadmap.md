# Roadmap

## Principles

- Preserve systemd authority.
- Prioritize PM2 behavior visible to operators and scripts.
- Reject unsupported features rather than approximate them silently.
- Treat D-Bus as a backend transport improvement, not a new architecture.
- Prefer direct native operations and explicit partial errors over custom
  transaction or recovery machinery; see ADR 0002.

## Now: Establish The Contract

- Use a systemd target for `save` and `resurrect`.
- Maintain the architecture, ADR, and compatibility documentation.
- Correct broad README claims and document agent/hub boundaries.
- Add target persistence and compatibility regression tests.

## Next: Systemd Read Model

- Introduce a small `UnitController` boundary when the D-Bus implementation
  requires it; avoid a generic filesystem or transaction framework.
- Read status, PID, restart count, activation timestamps, environment, cwd,
  logging destination, and unit-file state through one systemd property model.
- Sort process output deterministically.
- Stop inventing PID paths, restart counters, and unknown runtime facts.
- Use systemd/cgroup memory and CPU counters.

## Next: D-Bus Backend

- Add `github.com/coreos/go-systemd/v22/dbus` behind `UnitController`.
- Migrate read-side properties first.
- Migrate lifecycle calls while waiting for systemd job completion.
- Reuse the same controller for agent service management.
- Keep setup operations involving login1 and privilege escalation separate.

## Next: PM2 Conformance

- Add a machine-readable compatibility manifest pinned to PM2 7.0.4.
- Add a JSON schema for the guaranteed `jlist` subset.
- Align selectors, exit codes, stderr/stdout behavior, and log defaults.
- Make accepted ecosystem fields table-driven and semantically tested.

## Later: Logging

- Make journald the default systemd-native logging backend.
- Implement `logs` as a journal presentation layer with PM2-compatible command
  behavior where practical.
- Retain explicit file output as a systemd unit option, not PM2go-owned state.
- Replace external `tail` use with native Go where file output remains.

## Later: Agent And Hub Hardening

- Make remote inventory explicitly stale-able observation data.
- Separate operator and agent authorization.
- Add durable idempotent command delivery.
- Bound and expire logs and inspection payloads.
- Run the persistent agent only as a systemd service.

## Non-goals

- Reimplementing the PM2 daemon.
- A hidden local desired-state store.
- Full PM2 internals or Node.js API compatibility.
- Claiming cluster or zero-downtime semantics without implementing them.

## Proposal Process

Changes to state ownership, supervision, persistence, or control-plane
authority require an ADR before implementation.
