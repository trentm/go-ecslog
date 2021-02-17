# top

- implement simple and compact formats

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
- tail -f? Not sure if there is a need. Why not pipe from `tail -f ... | ecslog`?
- coloring for added zap and other levels (test case for this)
- coloring JSON values: see `rq` (true, false, number), also bolds the puncs
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
- src fields: log.origin.file.\* (note that ecs-logging zap logger emits
  `"log.origin"."file.name"`, which adds a surprise)
    - also colorizing these
- naming:
    ecslog
    ecs-pretty
    ecs-logging-pretty
  Think about it for a while. Perhaps do a survey... later.
- distribute builds? GH releases?
- filtering:
    - KQL (https://www.elastic.co/guide/en/kibana/master/kuery-query.html) or
      EQL (https://www.elastic.co/guide/en/elasticsearch/reference/current/eql.html)?
      Let's try KQL.
      Is that what you use by default in the Kibana Logs App?
- bunyan style handling of multiple input files and chrono ordering
  of records
- perhaps use https://github.com/elastic/makelogs for testing input?
  I don't know if this is ECS-y at all. Guessing only sort of. Useful
  for fuzzing-ish?
- benchmarking to be able to test out "TODO perf" ideas


# KQL notes

- exact search terms:

        dotted.field.name:value1 value2 value3
        dotted.field.name:"value with spaces"
        e.g.:
            http.response.status_code:400 401 404

  Note that this works with multi-value fields.

- Exact search terms on all "default fields in your index settings". Not
  sure if we'd support this. Perhaps config to define the default fields.
  Default to "message" and "service.name", for example? Or on the raw line?

        value1 value2

- boolean queries: `or`, `and`, `not`

        response:200 or extension:php
        response:200 and extension:php
        response:(200 or 404)
        response:200 and (extension:php or extension:css)
        response:200 and extension:php or extension:css  # and binds like langs I'm used to
        not response:200

  To match multi-value fields that contain a list of terms:

        tags:(success and info and security)

- Range queries on numbers:

        account_number >= 100 and items_sold <= 200

- Date range queries. Dates will be hard. Start with only supporting
  "@timestamp"?

        @timestamp < "2021-01-02T21:55:59"
        @timestamp < "2021-01"
        @timestamp < "2021"

  Might be nice to have separate options to mimic Kibana's time filter UI to
  support easy to type ranges like "-1y" for the last year, etc.

- "Exist" queries:

        response:*

- Wildcard queries of values:

        machine.os:win*

- Wildcard queries of multiple fields. This query checks machine.os and
  machine.os.keyword for the term `windows 10`.

        machine.os*:windows 10

  Q: It isn't clear to me if this searches `machine.osasdf`.

- Nested field queries
  (<https://www.elastic.co/guide/en/kibana/current/kuery-query.html#_nested_field_queries>)
  Skip supporting this for starters. There can't be a lot of *log* records
  with arrays of objects, can there?

