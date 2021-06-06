# ecslog

`ecslog` is a CLI for pretty-printing (and filtering) of log files in
[ecs-logging](https://www.elastic.co/guide/en/ecs-logging/overview/master/intro.html)
format.


# Install

For homebrew users:

    brew install trentm/tap/ecslog
    # OR 'brew upgrade trentm/tap/ecslog' if you have it already

Or download a pre-built binary package from [the releases page](https://github.com/trentm/go-ecslog/releases)
and copy to somewhere on your PATH.

Or you can build from source via:

    git clone git@github.com:trentm/go-ecslog.git
    cd go-ecslog
    make  # produces "./ecslog", a single binary you can put on your PATH
    ./ecslog --version

Then, try it on a demo log file:

    curl -s https://raw.githubusercontent.com/trentm/go-ecslog/main/demo.log \
        | ecslog

# Features

TODO: fill this out

## `@timestamp` diff highlighting

By default, `ecslog` will highlight the change in a log record's `@timestamp`
from the previous log record. With the "default" formatter, the changed part
of the timestamp is underlined. For example:

![screenshot of @timestamp diff highlighting](./docs/img/timestamp-diff-highlighting.png)

This can be turned off with the `timestampShowDiff=false` config var.


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


# Config file

Any of the following `ecslog` options can be set in a `~/.ecslog.toml` file.
See https://toml.io/ for TOML syntax information.  The `--no-config` option can
be used to ignore `~/.ecslog.toml`, if there is one.

An example config:

```
format="compact"
maxLineLen=32768
ecsLenient=true
```

### config: format

Set the output format name (a string, equivalent of `-f, --format` option).
Valid values are: "default" (the default), "compact", "ecs", "simple"

```
format="default"
```

### config: color

A color mode string for whether output should be colorized.  Valid values are:
"auto" (the default), "yes", "no". "auto" will colorize if the output stream
is a TTY.

```
color="auto"
```

### config: maxLineLen

Set the maximum number of bytes long for a single line that will be considered
for processing. Longer lines will be treated as if they are not ecs-logging
records.  Valid values are: -1 (to use the default 16384), or a value between 1
and 1048576 (inclusive).

```
maxLineLen=16384
```

### config: ecsLenient

Some JSON logs are "ECS compatible" in that they attempt to follow [ECS general
guidelines](https://www.elastic.co/guide/en/ecs/current/ecs-guidelines.html) --
have a `@timestamp`, use the specified data types for ECS fields, set
`ecs.version` -- but are not "ecs-logging compliant" because they are missing
one or more of `@timestamp`, `ecs.version`, or `log.level`.

By default `ecslog` will skip rendering for any log line that does not have
those three fields. Set `ecsLenient` to true to tell `ecslog` to attempt to
rendering any log record that has **at least one** of these fields.

```
ecsLenient=false
```

### config: timestampShowDiff

If coloring the output (see [config: color](#config-color) above), by default
`ecslog` will style the change in the timestamp from the preceding log record.
Set this config var to `false` to turn off this styling.

```
timestampShowDiff=true
```


# Troubleshooting

The `ECSLOG_DEBUG` environment variable can be set to get some internal
debugging information on stderr. For example:

    ECSLOG_DEBUG=1 ecslog ...

Internal debug logging is disabled if `ECSLOG_DEBUG` is unset, or is set
to one of: the empty string, "0", or "false".
