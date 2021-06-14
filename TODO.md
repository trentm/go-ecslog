# top

- title line re-eval (https://github.com/trentm/go-ecslog/issues/24),
  configurability, -t option
  - Note that with no log.level (allowable with ecsLenient, e.g. kibana 8.x
    current logs) the ':' sep in the title line is awkward.

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
- [x] basic intro docs in README
- [x] tests
  - [x] be resilient with type-errors and dotted-name collisions in other fields
    (i.e. don't want to spend time for full schema validation)
- [x] more robust dotted lookup
- [x] handle `TODO`s in the code

# later

- learn about verifiable builds: https://goreleaser.com/customization/gomod/
- "http" output format -> fieldRenderers?
- highlighting hits from KQL filtering would be really nice
- get ECS log examples from all the ecs-logging-$lang examples to learn from
  and test with
- option to highlight a matching string? or leave that to the pager? Could
  pass it on to the pager. Could be a vi-like "+<num>" or "+/query".
- godoc and examples (https://blog.golang.org/examples)

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


