# Application configuration

Each entrypoint chooses the capabilities to run and uses the shared configuration loader. Commands own configuration loading, flags and dependency construction; bootstrap receives only lifecycle hooks, a shutdown budget and a logger. It never receives application configuration or entrypoint modes. Configuration has the same loading order for every command:

1. Embedded `configs/app.yaml` defaults, available even outside the repository.
2. `<config-dir>/app.yaml`, when present. An explicit configuration directory must contain this file.
3. `<config-dir>/<environment>.yaml`, selected by `-env` or `MIG33_ENV`.
4. Additional named `-overlay` files, in the order supplied.
5. `MIG33_*` environment overrides.
6. Explicit command-line overrides such as `-db` and `-http`.

The directory defaults to `configs`; override it with `-config-dir` or `MIG33_CONFIG_DIR`. Explicit selection flags take precedence over those two environment variables. No environment overlay is selected by default. Selected overlay files must exist, and names cannot contain paths or extensions. Nested mappings merge; lists replace their inherited value. Unknown keys, nulls, duplicate keys, multiple YAML documents and invalid values fail startup.

```sh
go run ./cmd/omni -env local
go run ./cmd/omni -env dev
MIG33_HTTP_LISTEN=127.0.0.1:28080 go run ./cmd/http -env dev
go run ./cmd/socket -env dev
go run ./cmd/cli -env local -reports
```

`omni` runs both transports in one application instance. `http` and `socket` serve their respective transports; running them as separate processes does not share their live sessions or delivery queues. The socket server advertises `public_url` for HTTP resources, so that origin needs an HTTP service when those client features are used. Report review and moderator changes run through `cmd/cli`; these action flags are unavailable on server commands.

| YAML setting | Environment override |
| --- | --- |
| `public_url` | `MIG33_PUBLIC_URL` |
| `database.path` | `MIG33_DATABASE_PATH` |
| `resources.directory` | `MIG33_RESOURCES_DIRECTORY` (empty disables) |
| `http.listen` | `MIG33_HTTP_LISTEN` |
| `http.read_header_timeout` | `MIG33_HTTP_READ_HEADER_TIMEOUT` |
| `http.read_timeout` | `MIG33_HTTP_READ_TIMEOUT` |
| `http.write_timeout` | `MIG33_HTTP_WRITE_TIMEOUT` |
| `http.idle_timeout` | `MIG33_HTTP_IDLE_TIMEOUT` |
| `http.max_header_bytes` | `MIG33_HTTP_MAX_HEADER_BYTES` |
| `socket.listen` | `MIG33_SOCKET_LISTEN` |
| `shutdown_timeout` | `MIG33_SHUTDOWN_TIMEOUT` |
| `operators` | `MIG33_OPERATORS` (comma-separated; empty disables) |

Durations use Go syntax such as `5s` or `1m`. Relative database paths are relative to the process working directory. `public_url` retains the original client's HTTP-origin requirement. The local provider mocks and seeded test accounts retain their current behavior; this configuration does not activate real SMS, email or payments.

`resources.directory` also resolves relative to the process working directory and defaults to `data/applets/resources/v4`. Populate it with the extraction command in [local applet setup](../tools/applets/SETUP.md). These optional historical resources are local and ignored; missing files do not prevent startup, authentication or ordinary text chat. Requests needing an absent resource return an unavailable response. The filesystem is supplied to the native codec by command composition; services and bootstrap do not discover files or read environment variables.
