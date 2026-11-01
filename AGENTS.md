# Repository Guidance

## Product Direction

PM2go is a PM2 usage-compatibility layer for Linux operators and existing
scripts. It translates supported PM2 commands and configuration into native
systemd units and operations. It does not reimplement the PM2 daemon.

## Architectural Invariants

- systemd is the sole durable authority for local process definitions,
  runtime state, restart supervision, signals, and startup membership.
- Do not add a local process registry, desired-state database, or restorable
  copy of unit definitions.
- `pm2go save` records membership in the PM2go saved systemd target.
- `pm2go resurrect` starts existing units in that target. It never recreates a
  deleted unit.
- Starting an application must not enable it or add it to saved membership;
  only `save` controls `<prefix>saved.target.wants/`.
- PM2-only identity and presentation metadata may be embedded in a unit, but
  effective systemd properties win if metadata and the unit disagree.
- Ecosystem files are import inputs, not continuously reconciled desired state.
- Hub inventory is a timestamped observation cache, never authoritative state.
- A D-Bus backend may replace `systemctl` transport without changing these
  semantics.
- D-Bus mutations must wait for the systemd job result and must not be replayed
  through another backend after an ambiguous failure.
- `design-dbus-extension.md` is superseded reference material, not an
  implementation specification.
- Unsupported PM2 behavior must fail explicitly. Never accept an option and
  silently approximate or ignore it.

## Implementation Style

- Keep command flows direct: validate, translate, update units or links, reload,
  then request the systemd operation where needed.
- Report partial failures clearly. Do not add commit markers, backup registries,
  recovery scans, or automatic rollback/replay protocols around systemd.
- Keep path validation, mutation locking, atomic individual unit writes, bounded
  commands, and checks of systemd results.
- Introduce abstractions only for a concrete implementation need, not speculative
  future backends or filesystem frameworks.

## Compatibility Policy

Compatibility is defined per command, option, ecosystem field, exit code, and
machine-readable output field in `docs/compatibility.md`. Human-readable table
formatting is not a scripting API.

## Change Checklist

- Preserve systemd as the source of truth.
- Update `docs/compatibility.md` for user-visible behavior changes.
- Add Go tests for pure translation and filesystem logic.
- Add Linux/systemd tests for lifecycle and effective unit behavior.
- Keep README claims no broader than automated coverage.
- Record changes to state ownership or component boundaries in an ADR.

## Validation

```bash
go test ./...
go test -race ./...
go vet ./...
```

Run `make test` on Linux with systemd and Bats for end-to-end validation.
