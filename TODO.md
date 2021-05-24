# top

- timestamp diff highlighting: see musing section below
- get examples from the other ecs-loggers, esp. zap has some differences
- title line re-eval, configurability, -t option
  - Note that with no log.level (allowable with ecsLenient, e.g. kibana 8.x
    current logs) the ':' sep in the title line is awkward.
- README needs a once-over
- review TODOs in the code
- clear out all panic()s and probably lo?g.Fatal()s? Perhaps remove from 'lg' pkg
- releases: because unsigned dev thing on Mac, it would be nice to get into
  brew. Look at goreleaser again

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
- [x] NOTICE.md (some BSD from go in lex.go, some MIT from fatih/color)
- [ ] less-like pager?
- [ ] basic intro docs in README
- [x] tests
  - [x] be resilient with type-errors and dotted-name collisions in other fields
    (i.e. don't want to spend time for full schema validation)
- [x] more robust dotted lookup
- [ ] bug reporting facility on crash? Not sure we can with golang. Could just
  be a `--bug` CLI and github issue template with commands to gather and
  `ESLOG_DEBUG` advice.
- [ ] handle `XXX` and `TODO` in the code

# docs

- specify that multifile behaviour may change later to merge on @timestamp

# later

- learn about verifiable builds: https://goreleaser.com/customization/gomod/
- Is there a way to do releases for macOS and not have users hit the
  "Developer cannot be verified" error?
  https://stackoverflow.com/questions/59890359/developer-cannot-be-verified-macos-error-exception-a-move-to-trash-b-cancel
  Tarball? Zip? Installer? Verifying with mac somehow (ew)? Brew tap?
- Clickable file+linenum. If the ECS log record includes file name and line
  fields (https://www.elastic.co/guide/en/ecs/current/ecs-log.html#field-log-origin-file-line)
  the it would be nice if the default rendering of that (perhaps in the title line?)
  made it feasible to have that a clickable link in terminals.
    - With iTerm2 one could use "Semantic History" (e.g. see
      https://coderwall.com/p/5hp1yg/iterm2-cmd-click-to-open-file-in-vim-in-terminal)
      to configure this to open in a local editor or perhaps a GH link to that
      line number. What about ecslog (or another project) providing a small
      command to do that mapping (of log.origin.file.{name,line}) to local
      paths and/or GH links? Then provide docs on setting that up.
- consider https://github.com/muesli/termenv for ANSI color handling.
  Supports truecolor and degradation. Supports templates which might be
  useful for config-based custom styling.
- "http" output format -> fieldRenderers?
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

# musing on @timestamp highlighting

A feature idea is to somehow highlight the *change* in @timestamp in consecutive
log lines.

  make
  elog tmp/timestamp-highlighting.log -x process,http,url,user_agent
  elog examples/apm-server.log -x log.origin

- perhaps tests -> one in ecslog_test.go
- add "Features" section to README and a screenshot of this


# musing on custom formats/profiles

~/.ecslog.toml
    profile="trent"

    [profiles]
    [profiles.trent]
    format=compact
    excludeFields=["pid", "hostname"]
    titleFields=["reqId"]

    ...

    ecslog -p trent

Undecided on re-using "format", or calling it seomthing diff like "profile"
to not overload... because a profile includes more than what is called
a format now. *Or* could still be "format", but the default built-in formats
(at least now) have empty other attributes.

    format="trent"

    [formats]
    [formats.trent]
    format=compact
    excludeFields=["pid", "hostname"]
    titleFields=["reqId"]

    ...

    ecslog -f trent

If doing this (a format includes the other attributes), then need to *not*
allow top-level attributes in config, i.e. NOT this

    # NOT this
    format=compact
    excludeFields=...
    titleFields=...

The problem is "format=compact" in our format. Need to break those down to
constituent pieces. E.g.

    ${title} [${titleFields}]
        [${fields}]

So `title` has a format or template: titleTemplate.
`fields` has a format (`fieldsFormat`): jsony, compact, none (to exclude like "simple")
`titleFieldsFormat`? Or just have the default `(key:val, key2:"val with metachars")"? Yes.
This format allows cut 'n paste into KQL.

    format="trent"

    [formats]
    [formats.trent]
    titleTemplate=...
        OR
        titleFormat=... a name for these formats ...
    fieldsFormat=compact
    excludeFields=["pid", "hostname"]
    titleFields=["reqId"]

Could also support "inherits":

    format="trent"

    [formats]
    [formats.trent]
    inherit=compact
    excludeFields=["pid", "hostname"]
    titleFields=["reqId"]

then that would be typical and don't need to *name* the titleTemplates.

Next step is to implement a template renderer for the the title format.
If that is hard work, then this is all overkill. So a first pass would be
*just* the "inherit" format... which implies the title format.


