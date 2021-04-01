# ecslog changelog

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
