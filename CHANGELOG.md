# ecslog changelog

## v0.4.0

- Add [`@timestamp` diff highlighting](README.md#timestamp-diff-highlighting):
  the part of the timestamp that has changed from the preceding record is
  underlined (in the default color scheme). This highlighting can be turned
  off with the `timestampShowDiff: false` config var.
  ([#20](https://github.com/trentm/go-ecslog/pull/20))

- Add `ecsLenient: false` config option to allow rendering of lines that are
  likely ECS-compatible, but do not have all three required ecs-logging fields:
  `@timestamp`, `ecs.version`, `log.level`. Only one of those three is required
  to be rendered.

  This intentially doesn't have a command-line option for now.  Currently it is
  considered a crutch for ES 8.x and Kibana 8.x logs that, at time of writing,
  are missing one or two of the above fields. If that is long-standing,
  `ecsLenient: true` might eventually become the default.

## v0.3.0

- Use goreleaser for releases. The "Version" generally includes the leading "v"
  now. Built binaries should be reproducible from a given commit. They should
  be smaller now ("-s -w" in ldflags). Homebrew support.

- Add `-x, --exclude-fields ...` option to exclude fields from the rendering.
  For example, say you have log records that always has static "foo" and
  "bar" fields. They add two lines to the output for every record, wasting
  space.  `ecslog -x foo,bar` will remove them from the record before rendering
  it, helping with info density.

- Fix a bug where `-f FORMAT` would be ignored if there was a "format: FORMAT"
  in "~/.ecslog.toml".

## v0.2.0

- Add `--strict` option that will suppress input lines that are not valid
  ecs-logging records. Normally non-ecs-logging records are passed through
  unchanged.

- Support there not being a "message" field (allowed in ecs-logging spec
  in https://github.com/elastic/ecs-logging/pull/55).

- Fix a bug in the "simple" formatter, where the ellipsis would always be
  printed because the "@timestamp" field was not discounted.

- Refactor the read loop to handle very long lines without crashing, and without
  using unbounded memory. One side-effect -- due to the usage of
  `bufio.Reader.ReadLine` -- is that ecslog output will always finish with a
  newline, even if the input did not.

- Potentially much faster passing through unprocessed lines, moving to
  `out.Write` instead of unnecessary usage of `fmt.Fprintln`.

## v0.1.0

- Added KQL filtering via `ecslog -k,--kql KQL-FILTER`. For example:

        cat demo.log | ./ecslog -k error:*
        cat demo.log | ./ecslog -k 'http.response.status_code>=200'

  See [the Kibana KQL docs](https://www.elastic.co/guide/en/kibana/current/kuery-query.html)
  for an introduction to KQL, and see [the kqlog README](./internal/kqlog/README.md)
  for notes on the subset of KQL implemented.
