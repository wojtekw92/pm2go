# Architecture

## Goal

PM2go gives operators and existing scripts a documented subset of the PM2
interface while delegating process management to systemd. The compatibility
layer translates PM2 concepts; systemd performs supervision and remains the
source of truth.

```text
PM2 CLI and ecosystem input
            |
            v
Compatibility translation and rendering
            |
            v
Systemd backend (systemctl today, D-Bus planned)
            |
            v
Units, targets, cgroups, journald, and systemd runtime state
```

## Components

- **CLI:** parses supported PM2 command shapes and renders compatible output.
- **Systemd manager:** translates normalized application operations into unit
  definitions and systemd lifecycle calls.
- **Agent:** optional outbound control-plane adapter. It observes and operates
  the same local systemd manager.
- **Hub:** optional command queue and observation cache. It does not own local
  desired state.

## State Ownership

| Concern | Authority |
|---|---|
| Application definition | PM2go-owned systemd service unit |
| PM2 ID and name | Unit name and compatibility metadata in the unit |
| Runtime state and PID | systemd |
| Restart behavior | Effective systemd unit properties |
| Startup membership | `<prefix>saved.target.wants/` |
| Signals and lifecycle | systemd manager operations |
| Metrics | systemd/cgroup properties, once the read model migration is complete |
| Remote inventory | Last observation cached by the hub |
| Ecosystem file | One-time input translated into units |

No JSON, SQLite, or in-memory structure may supersede local systemd state.

## Unit Model

Managed services use this identity:

```text
<prefix><pm-id>-<escaped-name>.service
```

The default prefix is `pm2-`. `PM2GO_SERVICE_PREFIX` isolates test or deployment
namespaces.

The unit contains the effective execution definition. PM2-specific metadata
may preserve script/interpreter decomposition, argument boundaries, and a
stable compatibility creation timestamp. Metadata is for rendering only and
must not override effective systemd configuration.

## Save And Resurrect

`pm2go save` captures currently online managed units as symlinks in:

```text
<prefix>saved.target.wants/
```

The saved target is linked to `default.target` for user units and
`multi-user.target` for system units. This is the only saved process set.

`pm2go stop` does not change saved membership. A later `resurrect` may start the
stopped application. Running `save` while it is stopped removes it from the
saved set.

`pm2go delete` removes the unit and its saved-target membership. `resurrect`
never recreates missing unit definitions.

## Consistency Model

- Mutations are serialized by a lock in the systemd unit directory.
- Unit files are written through a temporary sibling and rename.
- Saved target links are updated directly. A multi-unit operation is not an
  all-or-nothing transaction; errors describe incomplete work or a failed reload.
- A failed start keeps its unit definition for inspection or an explicit retry.
- Delete stops each unit successfully before removing its links and definition.
  A reload failure does not restore a deleted definition.
- PM2go has no commit markers, backup registry, or automatic recovery scans.
- Reads are observations of systemd and may change immediately after return.
- Hub observations are eventually consistent and must expose their age.

## Command Flows

| Command | Flow |
|---|---|
| `start` | Validate identity, write unit, reload, start |
| `stop` / `restart` | Resolve unit, request systemd operation |
| `delete` | Stop, remove saved and legacy boot links, remove unit, reload |
| `save` | Observe active state, update saved links, remove legacy direct boot links, reload |
| `resurrect` | Read saved links, start existing valid units, report member failures |

Identity lookup reads unit names rather than querying runtime metrics. `save`
queries only the state needed to select online units. Full inspection happens
through `list`, `describe`, and `jlist`.

Old experimental dump files and deletion backups are not imported, removed, or
recovered automatically. Only existing unit definitions and saved target links
participate in the current model.

## Transport Independence

The current backend invokes `systemctl`. The planned D-Bus backend is a
transport replacement, not a state-model change. It must preserve synchronous
job completion, user/system scope, explicit partial-failure semantics, and the
same unit files and targets.

## Current Transitional Areas

- Process discovery and effective configuration still rely partly on unit-file
  scanning and parsing.
- Script, interpreter, and argument presentation can still come from embedded
  metadata when a unit was changed externally. Drop-ins and transient
  overrides are not yet represented. This is a known implementation gap; the
  effective systemd definition remains the intended authority.
- CPU and memory currently use `/proc`; the target model uses systemd/cgroup
  properties.
- Default logs are currently PM2-style files. The target model uses journald by
  default while retaining explicit systemd file-output options.
- Startup and agent service setup still execute command-line tools directly.

These are migration areas, not permission to introduce another state store.

## Non-goals

- Reimplementing the PM2 daemon or RPC protocol.
- A PM2go application supervisor process.
- Cluster-mode load balancing or zero-downtime reload without explicit systemd
  support and a documented compatibility contract.
- Hub-driven local desired-state reconciliation.
- Full compatibility with the Node.js `pm2` module API.
