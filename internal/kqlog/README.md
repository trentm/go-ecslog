# kqlog

This is a Go package to parse a subset (1) of the Kibana Query Language
([KQL](https://www.elastic.co/guide/en/kibana/current/kuery-query.html),
sometimes called "kuery" in code), with some tweaks for use in the
`ecslog` CLI for filtering log records (2).

1. This currently handles (and may forever) only a subset of KQL
   ([authoritative code](https://github.com/elastic/kibana/tree/master/src/plugins/data/common/es_query/kuery/),
   [PEG grammar](https://github.com/elastic/kibana/blob/master/src/plugins/data/common/es_query/kuery/ast/kuery.peg)).
   Known limitations are:
   - No [nested field queries](https://www.elastic.co/guide/en/kibana/current/kuery-query.html#_nested_field_queries)
   - No "geoBoundingBox" or "geoPolygon" function handling. (I see these
     in the KQL code, but not the docs.)
   - No support for leading wildcards (`*field:value`). This may change later.
   - No attempt has currently been made to rigourously compare against the
     authoritative source code and tests, so there are likely edge case
     differences between this and what you'll experience in Kibana's
     query form.
   - No Lucene query syntax handling.

2. Special case tweaks to the syntax for the benefit of log record filtering
   on the command line are:
   - Range queries are allow on the string "log.level" field, e.g.
     `log.level >= "error"`. This uses the best-effort log level ordering
     from `ecslog.ECSLevelLess`. (This special case is somewhat akin to special
     case range queries for dates.)
   - TODO: possibly something for "default" index fields for a bare `foo` query.
   - ...
  - kqlog might support range queries on any strings, and just merge date
    range queries in with that, because a log record doesn't know which
    fields are dates (other than spec'd "@timestamp")
  - kqlog might differ in "phrase matches". I'm not sure I can do that. I was
    considering just doing quoted literals the same as unquoted except: (a)
    allows spaces and special chars; and (b) implies a string type.

Open questions:

- `response:200 404` and `response:(200 or 404)`
  How do these differ? Why support the second syntax?
- Kibana KQL docs mention a "phrase match" when using double-quotes, e.g.
  `a.field.name:"this is my phrase"`. What does that "phrase match" imply.
- "A terms query of multiple values in a list type" e.g.: `a.field.name:(val1 and val2)`
  Is it the "and" that distinquishes from the "(200 or 404)" example above?
  Can there by single entry with parens, e.g. `foo:(val1)`?
  Is that meant to be distinct from `foo:val1`? Or do both of those mean a
  match for an array where one of the entries is "val1"?
- For a "date range query", e.g. `@timestamp < "2021-02"`, does this value need
  to be quoted?
- For data range queries, if I did string range comparison, will I screw up
  TZ things? E.g. if @timestamp has a TZ -- which I believe the go ecs-logging
  libs might do, e.g. ecszap.
