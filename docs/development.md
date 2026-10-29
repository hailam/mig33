# Go development and style

Use the Go version declared in `go.mod` and a C compiler for SQLite. Ordinary Go builds, tests and CI do not require Java, an applet JAR or recovered resources. Those inputs belong to optional original-client compatibility testing described in [applet setup](../tools/applets/SETUP.md).

## Formatting commands

Install the pinned tools once, then use the same commands locally and in CI:

```sh
sh scripts/format-go.sh --install
sh scripts/format-go.sh --write
sh scripts/format-go.sh --check
go vet ./...
go test -race ./...
```

The wrapper runs from any working directory. `--write` is the default. `--check` lists files whose bytes would change and exits unsuccessfully without writing source. It requires the tools to be installed and disables module downloads while formatting.

Versions are pinned in [format-go.py](../scripts/format-go.py): `goimports` from `golang.org/x/tools v0.52.0`, followed by `gofumpt v0.12.0`. Installation uses versioned commands independently of application dependencies and caches executable tools under the ignored `data/tools/` directory. It does not modify `go.mod` or `go.sum`. Imports are grouped with this module after external dependencies; the language version comes from `go.mod`.

Only handwritten `.go` files under `cmd/`, `configs/`, `db/` and `internal/` are candidates. Generated files with a standard `Code generated ... DO NOT EDIT.` marker are skipped, including OpenAPI, sqlc and templ output. The formatter does not follow symbolic links or traverse `research/`, `data/` or applet tooling. Change generator inputs and rerun the appropriate generator when generated code needs updating.

## Readability conventions

Use the Go project's formatting and review conventions as the baseline. The formatter handles indentation, alignment and consistent syntax; [Effective Go](https://go.dev/doc/effective_go#formatting) and [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments) explain the underlying conventions. Google's [Go style guide](https://google.github.io/styleguide/go/guide) is supplementary guidance for readability, not a separate mandatory rule set.

- Separate logical phases with a blank line when it helps a reader follow the function. Do not insert a blank line mechanically after every `if` block.
- Keep an operation and its immediate error check together. Handle the error before continuing with the result.
- Put large keyed struct literals on multiple lines, with one field per line. Short literals can remain compact when their meaning is obvious.
- Prefer guard clauses to unnecessary nesting, meaningful names and small functions with a clear purpose. Do not compress unrelated operations into a single line.
- Keep comments focused on purpose, constraints and non-obvious behavior. Preserve historical protocol explanations and generated-file notices.

These layout decisions still require review: formatters do not identify conceptual phases or automatically expand every long literal. For example:

```go
profile, err := accounts.Profile(ctx, username)
if err != nil {
	return err
}

request := Notification{
	Recipient: profile.Username,
	Subject:   "Contact request",
	Body:      message,
}
return notifications.Send(ctx, request)
```

The blank line marks the change from loading a profile to preparing a notification. It is not a general requirement after an error check. Gofumpt also keeps a simple assignment and its error check adjacent, as documented in its [formatting rules](https://github.com/mvdan/gofumpt#added-rules).

## Continuous integration

[The Go workflow](../.github/workflows/go.yml) checks formatting, runs `go vet` and runs all Go tests with the race detector on Linux. It reads the Go version from `go.mod`. GitHub actions are pinned to the verified release commits for [checkout v7.0.1](https://github.com/actions/checkout/releases/tag/v7.0.1) and [setup-go v7.0.0](https://github.com/actions/setup-go/releases/tag/v7.0.0). The workflow needs no local applet files or Java setup.

Use conventional commit messages, for example `refactor: format handwritten Go source` or `chore: add Go formatting checks`.
