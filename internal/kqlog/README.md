# kqlog

This is a Go package to parse a subset (1) of the Kibana Query Language
([KQL](https://www.elastic.co/guide/en/kibana/current/kuery-query.html),
sometimes called "kuery" in code), with some tweaks for use in the
`ecslog` CLI for filtering log records (2).

1. This currently handles (and may forever) only a *subset* of KQL
   ([authoritative code](https://github.com/elastic/kibana/tree/master/src/plugins/data/common/es_query/kuery/),
   [PEG grammar](https://github.com/elastic/kibana/blob/master/src/plugins/data/common/es_query/kuery/ast/kuery.peg)).
   Known limitations are:
   - No [nested field queries](https://www.elastic.co/guide/en/kibana/current/kuery-query.html#_nested_field_queries)
   - No "geoBoundingBox" or "geoPolygon" function handling. (I see these
     in the KQL code, but not the docs.)
   - No Lucene query syntax handling.
   - No attempt has currently been made to rigourously compare against the
     authoritative source code and tests, so there are likely edge case
     differences between this and what you'll experience in Kibana's
     query form.
   - No support for quoted literals for the *field* part of a query
     (`field:value`, e.g. `"foo bar":baz`). This may change later. Quoted
     literals for the *value* part, e.g. `foo:"bar baz"`, *is* suppported.
     (Dev Note: To implement this I would add a `type fieldTerm` to term.go
     and share the parsing in `newTerm` and `newQuotedTerm`.)
   - No support for wildcards in the *field* part of a query, e.g. `foo*: bar`
   - Some edge cases with parenthesized values are not current supported, e.g.:
     no terms `foo:()` and superfluous parentheses `foo:((a and (b)))`.

2. Special case tweaks to the syntax for the benefit of log record filtering
   on the command line are:
   - Range queries are supported for any string values. I'm not sure if Kibana
     supports this the same way.
   - Range queries are special cased on the string "log.level" field, e.g.
     `log.level >= error`. This uses the best-effort log level ordering based
     on common log level names from various libraries. (See
     `ecslog.LogLevelLess`).
   - Default fields queries, e.g. `foo`, currently match against the "message"
     field. kqlog does not have an Elasticsearch template to know what the
     "default fields" may be. Note: This may change in later versions to be
     more compatible with search in the Kibana Logs app.

Open questions:

- `response:200 404` and `response:(200 or 404)`
  How do these differ? Why support the second syntax?
- Kibana KQL docs mention a "phrase match" when using double-quotes, e.g.
  `a.field.name:"this is my phrase"`. What does that "phrase match" imply?
  Currentyl kqlog is treating this as a quoted term where the only difference
  is in the escaping rules.
- The KQL user guide doc section on wildcards gives this example for a wildcard
  in the field name:
    > To match multiple fields:
    >
    > machine.os\*:windows 10
    >
    > This syntax is handy when you have text and keyword versions of a field.
    > The query checks machine.os and machine.os.keyword for the term `windows
    > 10`.
  Is the intent to match *any* keys with the prefix "machine.os", e.g.
  "machine.oscon", "machine.os.foo.bar.baz", etc.? The ".keyword" handling
  isn't relevant for the log record matching case.
- "A terms query of multiple values in a list type" e.g.: `a.field.name:(val1 and val2)`
  Is it the "and" that distinquishes from the "(200 or 404)" example above?
  Can there by single entry with parens, e.g. `foo:(val1)`?
  Is that meant to be distinct from `foo:val1`? Or do both of those mean a
  match for an array where one of the entries is "val1"?
- For data range queries, kqlog is currently using simple string comparison.
  Does KQL in Kibana/Elasticsearch handling timezones if the timestamp
  specifies a TZ offset (e.g. as ecszap does)? If so, should kqlog special
  case `@timestamp`?
- I'm curious if KQL handles the case of a term with both a unescaped and an
  escaped asterisk: `foo*bar\*`.
