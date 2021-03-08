# top

- crash bug: Using this in apm-nodejs-http-client (using pino):
        this._log.warn('uncork (from _corkTimer): state=%j', this._writableState)
  generated a HUGE line that resulted in:
    ecslog: error: bufio.Scanner: token too long
  The log line had a huge "message" field... so need a guard on each field size.
  https://stackoverflow.com/questions/21124327 suggests "bufio.Reader.ReadLine"
- kqlog
  - XXXs and TODOs in rpn.go
  - type handling for exec
  - quoted literals
- `-x, --elide-fields' or something to remote from rendering
  Matching to *include* only given fields? Is this only about "extra" fields?
- get examples from the other ecs-loggers, esp. zap has some differences
- bug: crash on gargantuan string in a single record (details on other computer I think)
- config via github.com/BurntSushi/toml ?

# mvp

- [x] stream ndjson stdin, render to stdout
- [x] input args cases: stdin, one file, multiple files
- [x] validate and render ECS-format lines
- [x] pass other lines unchanged
- [x] colorized output
- [x] `-l, --level` filtering support
- [x] format/renderer support, minimal set of formats
- [ ] basic config file support (TOML? JSON?) ... at least to select personally
  preferred format. Or just envvars?
- [x] don't choke on crazy long lines, i.e. input line handler needs to have maxlen
- [ ] NOTICE.md (some BSD from go in lex.go, some MIT from fatih/color)
- [ ] less-like pager?
- [ ] basic intro docs in README
- [ ] tests
  - be resilient with type-errors and dotted-name collisions in other fields
    (i.e. don't want to spend time for full schema validation)
  - examples from all the ecs-logging libs
- [ ] more robust dotted lookup
- [ ] bug reporting facility on crash? Not sure we can with golang. Could just
  be a `--bug` CLI and github issue template with commands to gather.
- [ ] handle all `lg.Printf("Q: ...")` and `XXX` and `TODO` in the code

# docs

- specify that multifile behaviour may change later to merge on @timestamp

# later

- "http" output format
- -x,--exclude-fields option to remove the given fields from the rendering
  of any line, or -i, --include-fields?
- coloring for added zap and other levels (test case for this)
- --version flag
- get ECS log examples from all the ecs-logging-$lang examples to learn from
  and test with
- option to highlight a matching string? or leave that to the pager? Could
  pass it on to the pager. Could be a vi-like "+<num>" or "+/query".
- handling myriad other logging levels: upper case, syslog-y level names,
  spellings of 'warn/warning', etc. All these in a *sorted* order for level
  filtering.
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
- canned stats output? num records, num non-ECS, breakdown of service.name,
  http status report if http info, count of errors, breakdown of common errors
- godoc and examples (https://blog.golang.org/examples)


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

## kuery code

```typescript
import {
  esKuery,
  IIndexPattern,
  QuerySuggestion,
} from '../../../../../../../src/plugins/data/public';

function convertKueryToEsQuery(kuery: string, indexPattern: IIndexPattern) {
  const ast = esKuery.fromKueryExpression(kuery);
  return esKuery.toElasticsearchQuery(ast, indexPattern);
}
```

src/plugins/data/common/es_query/kuery/ast/ast.test.ts
~/el/kibana/src/plugins/data/common/es_query/kuery/ast/kuery.peg
