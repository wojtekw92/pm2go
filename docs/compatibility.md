# PM2 Usage Compatibility

## Definition

PM2go targets usage compatibility for common PM2 operator commands and scripts
on Linux with systemd. It is not a drop-in implementation of every PM2 feature
or internal API.

Compatibility is evaluated against pinned, observed upstream PM2 behavior. The
initial conformance target is PM2 7.0.4. Human-readable formatting may differ;
documented command semantics, exit status, and machine-readable fields are the
contract.

Do not globally alias or symlink `pm2` to `pm2go` until every command and option
used by your automation is covered here and validated in your environment.

## Compatibility Tiers

- **Must match:** scripts may rely on the documented behavior.
- **May differ:** equivalent operator intent with systemd-native internals or
  different human formatting.
- **Unsupported:** command or option must fail clearly and return nonzero.

## Current Command Matrix

| Surface | Tier | Current contract |
|---|---|---|
| `start <script>` | Must match | Registers and starts one fork-mode application |
| `--name`, `--interpreter`, `--cwd` | Must match | Translated to unit identity and execution properties |
| `--output/-o`, `--error/-e`, `--log/-l` | Must match | Translated to systemd output directives |
| Arguments after `--` | Must match | Preserved as an argument vector, including empty values |
| `stop`, `restart`, `delete` by ID/name | Must match | Missing targets fail; stop retains registration; delete removes it |
| `list`, `describe`, `env`, `pid` | Partial | Field provenance and exact PM2 edge cases remain under conformance work; external drop-ins are not fully reflected yet |
| `jlist` | Partial | Valid JSON and core PM2 fields; full PM2 schema and effective-property parity are not promised |
| `save` | May differ | Saves online units as systemd target membership, not a PM2 dump file |
| `resurrect` | May differ | Starts existing saved units; never restores deleted definitions |
| `logs` | Partial | Current default is file-based and non-streaming unless `--follow` is used |
| `reload` | Unsupported parity | Current behavior is restart, not PM2 zero-downtime reload |
| Ecosystem JSON | Partial | Strict documented field subset only |
| Agent and hub | PM2go extension | Not part of PM2 compatibility |

## Supported Ecosystem Fields

The JSON `apps` array currently accepts:

- `id`, `name`, `script`, `interpreter`, `cwd`
- `args` as a string or string array
- `env` and selected `env_<name>` objects
- `out_file`, `error_file`, `log_file`
- `autorestart`, `restart_delay`

Unknown fields are rejected. JavaScript, CommonJS, YAML, and deploy ecosystem
files are unsupported.

## Lifecycle Failures

- A failed `start` returns nonzero and retains a unit already written, so the
  operator can inspect it and retry with `restart`, or remove it with `delete`.
- `delete` does not unlink a unit whose stop operation failed. Successfully
  deleted units are not restored if another member or daemon reload fails.
- `save` updates systemd links directly. A filesystem or reload failure returns
  nonzero and may leave a partially updated saved set; retry `save` after fixing
  the reported cause.
- `resurrect` attempts valid existing members and reports missing or invalid
  members as failures. It never reconstructs definitions from old backups.

Multi-application mutations do not promise transactional rollback. Systemd unit
files and links remain the inspectable state after a failure.

After a reload failure, fix the reported cause and run `systemctl --user
daemon-reload` for user scope, or `systemctl daemon-reload` for system scope,
before retrying runtime operations.

## Machine Output

`jlist` prints one JSON array to stdout. Core fields include top-level `pid`,
`name`, and `pm_id`, plus `pm2_env` and `monit`. Required field types and
semantics will be stabilized through a schema and upstream differential tests.

Diagnostics should go to stderr. Successful scalar or JSON output belongs on
stdout. Existing commands that violate this rule are migration work.

## Explicitly Unsupported

- Cluster mode, `instances`, scaling, and built-in load balancing.
- PM2 zero-downtime reload semantics.
- Watch mode and file-change restarts.
- `merge_logs`, `combine_logs`, and `max_restarts` until their PM2 semantics
  can be represented and tested without approximation.
- Cron restart, memory restart, exponential backoff, `wait_ready`, and process
  message protocols.
- PM2 modules, Plus integration, deploy commands, and the Node.js programmatic
  API.
- PM2 daemon sockets, dump format, PID files, and internal directory layout.
- Cross-platform operation without Linux and systemd.

Unsupported features must not be silently accepted.

## Conformance Direction

Conformance tests should execute a table-driven contract against both pinned
PM2 and PM2go, normalizing only volatile values such as PIDs, timestamps,
paths, usernames, colors, and metrics. Tests must compare exit codes,
stdout/stderr roles, JSON field types, and lifecycle side effects rather than
entire human tables.

See `docs/roadmap.md` for the implementation sequence.
