# ecslog

Playing with a CLI for pretty formatting of ecs-logging format logs, a la
`bunyan` and `pino-pretty`.

Current status: still alpha.

```sh
go run cmd/ecslog/main.go ./demo.log
```

# Goals

- Easy to install and use.
- Fast.
- Reliably handles any ECS input and doesn't crash.
- Colors are decent on dark *and* light backgrounds. Many hacker-y tools
  messy this up.

Nice to haves:

- Configurable/pluggable output formatting would be nice.
- Filtering support: levels, other fields.
- `less` integration a la `bunyan`
- Stats collection and reporting, if there are meaningful common cases
  here. Otherwise this could get out of hand.

Non-goals:

- An ambitious CLI that handles multiple formats (e.g. also bunyan format, pino
  format, etc). Leave that for a separate science project.
- Full less-like curses-based TUI for browsing around in a log file, though
  that would be fun.


# Output formats

`ecslog` has multiple output formats for rendering ECS logs that may be selected
via the `-f, --format NAME` option. Note that some formats as *lossy*, i.e.
elide some fields, typically for compactness.

- "default": A lossless default format that renders each log record with a
  title line to convey core and common fields, followed by all remaining
  extra fields. Roughly:

  ```
  [@timestamp] LOG.LEVEL (log.logger/service.name on host.hostname): message
      extraKey1: extraValue1-as-multiline-jsonish
      extraKey2: extraValue2-as-multiline-jsonish
  ```

  where "multiline jsonish" means 4-space-indented JSON with the one special
  case that multiline string values are printed indented and with newlines.
  For example, "error.stack\_trace" in the following:

  ```
  [2021-02-11T06:24:53.251Z]  WARN (myapi on purple.local): something went wrong
      process: {
          "pid": 82240
      }
      error: {
          "type": "Error"
          "message": "boom"
          "stack_trace":
              Error: boom
                  at .../pino/examples/express-simple.js:67:15
                  ...
  ```

  The format of the title line may change in future versions.

- "ecs": The native/raw ECS format, ndjson.

- "simple": A *lossy* (i.e. elides some fields for compactness) format that
  simply renders `LOG.LEVEL: message`. If extra fields (other than the core
  "@timestamp" and "ecs.version" fields) are being elided, a ellipsis is
  appended to the line.

- "compact": A lossless format similar to "default", but attempts are made
  to make the "extraKey" info more compact by balancing multiline JSON with
  80-column output.

- "http": A lossless format similar to "default", but attempts to render
  HTTP-related ECS fields in HTTP request and response text representation.
  TODO: not yet implemented.


# Troubleshooting

`ecslog --self-debug ...` will log some limited internal debugging information.
This is on stderr and is itself in ecs-logging format, so one can append the
following to include rendered ecslog self-debugging inline:

    --self-debug 2>&1 | ecslog

For example:

    ecslog ... my-log-file.log --self-debug 2>&1 | ecslog
