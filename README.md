# ecslog

Playing with a CLI for pretty formatting of ecs-logging format logs, a la
`bunyan` and `pino-pretty`.

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


# Design Notes

- Passes through unrecognized format through unchanged. Where "unrecognized"
  is "doesn't satisfy 'required' fields in spec.json".
- `--strict` option to elide unrecognized lines.
- formats:
    - ecs: the native format that is ndjson
    - <default> (TODO: name) for safe and future-proof default format
      to be defined, but leaning towards pino-pretty-like (TODO: design).
      *Perhaps* has built-in layout for ecs-logging/spec/spec.json-defined
      fields (like `error.*` and `log.*`)
    - <???>: a format that tries a bit harder to be pretty for some things
      like http req/res beyond the <default>. Perhaps just those? If so
      then could call this "http" format.
    - "short" or something like that for oneline or reduced output
      Perhaps always include an ellipsis if info is being elided?

