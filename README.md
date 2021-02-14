# ecslog

Playing with a CLI for pretty formatting of ecs-logging format logs, a la
`bunyan` and `pino-pretty`.

Current status:

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
- `tail -f`-like support
- `less` integration a la `bunyan`
- Stats collection and reporting, if there are meaningful common cases
  here. Otherwise this could get out of hand.

Non-goals:

- An ambitious CLI that handles multiple formats (e.g. also bunyan format, pino
  format, etc). Leave that for a separate science project.
- Full less-like curses-based TUI for browsing around in a log file, though
  that would be fun.

