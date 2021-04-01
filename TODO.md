# top

- README needs a once-over
- `-x, --elide-fields' or something to remote from rendering
  Matching to *include* only given fields? Is this only about "extra" fields?
- get examples from the other ecs-loggers, esp. zap has some differences
- title line re-eval, configurability, -t option
- check painter on black bg
- review TODOs in the code
- clear out all panic()s and probably lo?g.Fatal()s? Perhaps remove from 'lg' pkg

# mvp

- [x] stream ndjson stdin, render to stdout
- [x] input args cases: stdin, one file, multiple files
- [x] validate and render ECS-format lines
- [x] pass other lines unchanged
- [x] colorized output
- [x] `-l, --level` filtering support
- [x] format/renderer support, minimal set of formats
- [x] basic config file support (TOML)
- [x] don't choke on crazy long lines, i.e. input line handler needs to have maxlen
- [ ] NOTICE.md (some BSD from go in lex.go, some MIT from fatih/color)
- [ ] less-like pager?
- [ ] basic intro docs in README
- [ ] tests
  - be resilient with type-errors and dotted-name collisions in other fields
    (i.e. don't want to spend time for full schema validation)
  - examples from all the ecs-logging libs
- [x] more robust dotted lookup
- [ ] bug reporting facility on crash? Not sure we can with golang. Could just
  be a `--bug` CLI and github issue template with commands to gather and
  `ESLOG_DEBUG` advice.
- [ ] handle `XXX` and `TODO` in the code

# docs

- specify that multifile behaviour may change later to merge on @timestamp

# later

- "http" output format
- -x,--exclude-fields option to remove the given fields from the rendering
  of any line, or -i, --include-fields?
- coloring for added zap and other levels (test case for this)
- get ECS log examples from all the ecs-logging-$lang examples to learn from
  and test with
- option to highlight a matching string? or leave that to the pager? Could
  pass it on to the pager. Could be a vi-like "+<num>" or "+/query".
- src fields: log.origin.file.\* (note that ecs-logging zap logger emits
  `"log.origin"."file.name"`, which adds a surprise)
    - also colorizing these
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
- godoc and examples (https://blog.golang.org/examples)
