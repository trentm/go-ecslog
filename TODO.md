# top

- color: https://github.com/fatih/color
- get ECS log examples from all the ecs-logging-$lang examples to learn from
  and test with

# mvp

* stream ndjson stdin, render to stdout
- Take a file to read from. Decide on args and opts for this. Same as bunyan?
  What other comparisons? jq? json? pino-pretty?
* validate and render ECS-format lines (recognized by just required fields)
- be resilient with type-errors and dotted-name collisions in other fields
  (i.e. don't want to spend time for full schema validation)
* pass other lines unchanged
- colorized output
- `-l, --level` filtering support
- format/renderer support, minimal set of formats
- basic config file support (TOML? JSON?) ... at least to select personally
  preferred format
- don't choke on crazy long lines, i.e. input line handler needs to have maxlen
- NOTICE.md
- bug reporting facility on crash? Not sure we can with golang.
- tests
- tail -f?
- less-like pager?
- basic intro docs in README

# later

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
- decide on and doc the default format (and name it). Bunyan-y fancy, or
  pino-pretty-y reasonable default. See some discussion in README and main.go
- ditto for "http" format. Should fit with default format.
- naming:
    ecslog
    ecs-pretty
    ecs-logging-pretty
  Think about it for a while. Perhaps do a survey... later.
- distribute builds? GH releases?
- filtering: is there a golang impl/parser for EQL? Would be nice to mirror
  what you'd get in Kibana logs app.
- bunyan style handling of multiple input files and chrono ordering
  of records
- perhaps use https://github.com/elastic/makelogs for testing input?
  I don't know if this is ECS-y at all. Guessing only sort of. Useful
  for fuzzing-ish?

