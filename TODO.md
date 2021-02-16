# top

- refactor render() and support multiple formats

# mvp

* stream ndjson stdin, render to stdout
* input args cases: stdin, one file, multiple files
* validate and render ECS-format lines
* pass other lines unchanged
* colorized output
* `-l, --level` filtering support
- format/renderer support, minimal set of formats
- basic config file support (TOML? JSON?) ... at least to select personally
  preferred format. Or just envvars?
* don't choke on crazy long lines, i.e. input line handler needs to have maxlen
- NOTICE.md
- bug reporting facility on crash? Not sure we can with golang.
- tests
  - be resilient with type-errors and dotted-name collisions in other fields
    (i.e. don't want to spend time for full schema validation)
- tail -f?
- less-like pager?
- basic intro docs in README

# docs

- default format:
  - special case printing of multiline extra field values (e.g. typically error.stack_trace)
  - ...
- specify that multifile behaviour may change later to merge on @timestamp

# later

- the other output formats:
  - http
  - compact
  - simple
- -x,--exclude-fields option to remove the given fields from the rendering
  of any line
- coloring for added zap and other levels (test case for this)
- --version flag
- get ECS log examples from all the ecs-logging-$lang examples to learn from
  and test with
- Long-form online help. From --help vs -h? or general man page? What's typical or
  nice in go-land.
- decide on and doc the default format (and name it). Bunyan-y fancy, or
  pino-pretty-y reasonable default. See some discussion in README and main.go
- ditto for "http" format. Should fit with default format.
- special HTTP rendering (include .body if it is added)
- option to highlight a matching string? or leave that to the pager? Could
  pass it on to the pager. Could be a vi-like "+<num>" or "+/query".
- handling myriad other logging levels: upper case, syslog-y level names,
  spellings of 'warn/warning', etc. All these in a *sorted* order for level
  filtering.
- src fields: log.origin.file.* (note that ecs-logging zap logger emits
  `"log.origin"."file.name"`, which adds a surprise)
    - also colorizing these
- naming:
    ecslog
    ecs-pretty
    ecs-logging-pretty
  Think about it for a while. Perhaps do a survey... later.
- distribute builds? GH releases?
- filtering: is there a golang impl/parser for EQL? Would be nice to mirror
  what you'd get in Kibana logs app.
    - or KQL? https://www.elastic.co/guide/en/kibana/master/kuery-query.html
    - EQL: https://www.elastic.co/guide/en/elasticsearch/reference/current/eql.html
    - or?...
- bunyan style handling of multiple input files and chrono ordering
  of records
- perhaps use https://github.com/elastic/makelogs for testing input?
  I don't know if this is ECS-y at all. Guessing only sort of. Useful
  for fuzzing-ish?
- benchmarking to be able to test out "TODO perf" ideas
