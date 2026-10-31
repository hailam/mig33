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
| `http.tls_listen` | `MIG33_HTTP_TLS_LISTEN` (empty disables) |
| `http.read_header_timeout` | `MIG33_HTTP_READ_HEADER_TIMEOUT` |
| `http.read_timeout` | `MIG33_HTTP_READ_TIMEOUT` |
| `http.write_timeout` | `MIG33_HTTP_WRITE_TIMEOUT` |
| `http.idle_timeout` | `MIG33_HTTP_IDLE_TIMEOUT` |
| `http.max_header_bytes` | `MIG33_HTTP_MAX_HEADER_BYTES` |
| `socket.listen` | `MIG33_SOCKET_LISTEN` |
| `socket.tls_listen` | `MIG33_SOCKET_TLS_LISTEN` (empty disables) |
| `tls.cert_file` | `MIG33_TLS_CERT_FILE` |
| `tls.key_file` | `MIG33_TLS_KEY_FILE` |
| `tls.handshake_timeout` | `MIG33_TLS_HANDSHAKE_TIMEOUT` |
| `shutdown_timeout` | `MIG33_SHUTDOWN_TIMEOUT` |
| `operators` | `MIG33_OPERATORS` (comma-separated; empty disables) |

Durations use Go syntax such as `5s` or `1m`. Relative database paths are relative to the process working directory. `public_url` retains the original client's HTTP-origin requirement. The local provider mocks and seeded test accounts retain their current behavior; this configuration does not activate real SMS, email or payments.

Secure listeners are additive and disabled by default. For example, after provisioning a certificate trusted by the Expo device:

```sh
go run ./cmd/omni -env local \
  -https 127.0.0.1:8443 -tls-tcp 127.0.0.1:9120 \
  -tls-cert data/local-tls/server.pem -tls-key data/local-tls/server-key.pem
```

`-https` is available on `http` and `omni`; `-tls-tcp` is available on `socket` and `omni`. `-tls-cert` and `-tls-key` are available on server commands. Explicit empty secure-address flags disable the corresponding listener even when an overlay or environment enabled it. The original HTTP/TCP listeners remain enabled. Every enabled transport in `omni` shares one graph, including sessions and live queues.

The HTTPS listener exposes the original binary gateway at `/` and `/{sid}`, plus `/healthz`. It does **not** expose browser/WAP, operator, JSON API or direct resource routes. Expo retrieves original pages, language resources and photos inside the existing encrypted protocol packets. Their historical HTTP URL identifiers and `public_url` remain unchanged. The complete legacy HTTP router retains its existing behavior; this HTTPS listener makes no browser parity claim.

Certificate/key paths resolve relative to the process working directory. The PEM certificate file contains the leaf followed by any intermediate certificates. Both files are loaded and checked for a matching identity before the graph or any listener opens. Keep private keys out of the repository and readable only by the server account. Renewed certificate material is loaded on process restart. TLS requires version 1.2 or later. `tls.handshake_timeout` defaults to `5s`: it bounds socket handshakes before packet decoding and caps the HTTPS initial-header timeout (Go also applies any smaller read/write timeout). Shutdown closes incomplete TLS handshakes while accepted HTTPS requests drain within the configured shutdown budget.

`resources.directory` also resolves relative to the process working directory and defaults to `data/applets/resources/v4`. Populate it with the extraction command in [local applet setup](../tools/applets/SETUP.md). These optional historical resources are local and ignored; missing files do not prevent startup, authentication or ordinary text chat. Requests needing an absent resource return an unavailable response. The filesystem is supplied to the native codec by command composition; services and bootstrap do not discover files or read environment variables.
