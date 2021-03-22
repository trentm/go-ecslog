# ecslog changelog

## v0.1.0

- Added KQL filtering via `ecslog -k,--kql KQL-FILTER`. For example:

        cat demo.log | ./ecslog -k error:*
        cat demo.log | ./ecslog -k 'http.response.status_code>=200'

  See [the Kibana KQL docs](https://www.elastic.co/guide/en/kibana/current/kuery-query.html)
  for an introduction to KQL, and see [the kqlog README](./internal/kqlog/README.md)
  for notes on the subset of KQL implemented.
