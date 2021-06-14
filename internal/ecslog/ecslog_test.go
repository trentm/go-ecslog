package ecslog_test

import (
	"bytes"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/trentm/go-ecslog/internal/ecslog"
)

type renderFileTestCase struct {
	name string

	// Renderer options
	shouldColorize    string
	colorScheme       string
	formatName        string
	ecsLenient        bool
	levelFilter       string
	kqlFilter         string
	timestampShowDiff bool
	includeFields     []string

	input  string
	output string
}

var renderFileTestCases = []renderFileTestCase{
	// Non-ecs-logging lines
	{
		"empty object",
		"no", "", "default", false, "", "", false, []string{},
		"{}",
		"{}\n",
	},

	// Basics
	{
		"basic",
		"no", "", "default", false, "", "", false, []string{},
		`{"log.level":"info","@timestamp":"2021-01-19T22:51:12.142Z","ecs":{"version":"1.5.0"},"message":"hi"}`,
		"[2021-01-19T22:51:12.142Z]  INFO: hi\n",
	},
	{
		"basic, extra var",
		"no", "", "default", false, "", "", false, []string{},
		`{"log.level":"info","@timestamp":"2021-01-19T22:51:12.142Z","ecs":{"version":"1.5.0"},"message":"hi","foo":"bar"}`,
		"[2021-01-19T22:51:12.142Z]  INFO: hi\n    foo: \"bar\"\n",
	},
	{
		"no message is allowed",
		"no", "", "default", false, "", "", false, []string{},
		`{"log.level":"info","@timestamp":"2021-01-19T22:51:12.142Z","ecs":{"version":"1.5.0"},"foo":"bar"}`,
		"[2021-01-19T22:51:12.142Z]  INFO:\n    foo: \"bar\"\n",
	},

	// Coloring
	{
		"coloring 1",
		"yes", "default", "default", false, "", "", false, []string{},
		`{"log.level":"info","@timestamp":"2021-01-19T22:51:12.142Z","ecs":{"version":"1.5.0"},"message":"hi"}`,
		"[2021-01-19T22:51:12.142Z] \x1b[32m INFO\x1b[0m: \x1b[36mhi\x1b[0m\n",
	},
	{
		"timestamp diff highlighting 1",
		"yes", "default", "default", false, "", "", true, []string{},
		`{"log.level":"info","@timestamp":"2021-01-19T22:51:12.142Z","ecs":{"version":"1.5.0"},"message":"hi"}
{"log.level":"info","@timestamp":"2021-01-19T22:51:23.456Z","ecs":{"version":"1.5.0"},"message":"hi"}`,
		"[2021-01-19T22:51:12.142Z] \x1b[32m INFO\x1b[0m: \x1b[36mhi\x1b[0m\n" +
			"[2021-01-19T22:51:\x1b[4m23.456\x1b[0mZ] \x1b[32m INFO\x1b[0m: \x1b[36mhi\x1b[0m\n",
	},

	// KQL filtering
	{
		"kql filtering, yep",
		"no", "", "default", false, "", "foo:bar", false, []string{},
		`{"log.level":"info","@timestamp":"2021-01-19T22:51:12.142Z","ecs":{"version":"1.5.0"},"message":"hi","foo":"bar"}`,
		"[2021-01-19T22:51:12.142Z]  INFO: hi\n    foo: \"bar\"\n",
	},
	{
		"kql filtering, nope",
		"no", "", "default", false, "", "foo:baz", false, []string{},
		`{"log.level":"info","@timestamp":"2021-01-19T22:51:12.142Z","ecs":{"version":"1.5.0"},"message":"hi","foo":"bar"}`,
		"",
	},
	{
		"kql filtering, log.level range query, yep",
		"no", "", "default", false, "", "log.level > debug", false, []string{},
		`{"log.level":"info","@timestamp":"2021-01-19T22:51:12.142Z","ecs":{"version":"1.5.0"},"message":"hi"}`,
		"[2021-01-19T22:51:12.142Z]  INFO: hi\n",
	},
	{
		"kql filtering, log.level range query, nope",
		"no", "", "default", false, "", "log.level > warn", false, []string{},
		`{"log.level":"info","@timestamp":"2021-01-19T22:51:12.142Z","ecs":{"version":"1.5.0"},"message":"hi"}`,
		"",
	},

	// ecsLenient
	{
		"lenient: missing log.level",
		"no", "", "default", true, "", "", false, []string{},
		`{"@timestamp":"2021-01-19T22:51:12.142Z","ecs":{"version":"1.5.0"},"message":"hi"}`,
		"[2021-01-19T22:51:12.142Z] : hi\n",
	},
	{
		"lenient: missing @timestamp",
		"no", "", "default", true, "", "", false, []string{},
		`{"log.level":"info","ecs":{"version":"1.5.0"},"message":"hi"}`,
		" INFO: hi\n",
	},
	{
		"lenient: missing ecs.version",
		"no", "", "default", true, "", "", false, []string{},
		`{"log.level":"info","@timestamp":"2021-01-19T22:51:12.142Z","message":"hi"}`,
		"[2021-01-19T22:51:12.142Z]  INFO: hi\n",
	},
	{
		"lenient: @timestamp only",
		"no", "", "default", true, "", "", false, []string{},
		`{"@timestamp":"2021-01-19T22:51:12.142Z","message":"hi"}`,
		"[2021-01-19T22:51:12.142Z] : hi\n",
	},
	// Include fields
	{
		"include fields: only log",
		"no", "", "default", true, "", "", false, []string{"log"},
		`{"log.level":"info","@timestamp":"2021-01-19T22:51:12.142Z","ecs":{"version":"1.5.0"},"message":"hi","log.origin":{"foo":"bar","file.name":"main.go","file.line":"42"}}`,
		"[2021-01-19T22:51:12.142Z]  INFO: hi\n    log.origin: {\n        \"foo\": \"bar\",\n        \"file.name\": \"main.go\",\n        \"file.line\": \"42\"\n    }\n",
	},
	{
		"include fields: only log.origin.file",
		"no", "", "default", true, "", "", false, []string{"log.origin.file"},
		`{"log.level":"info","@timestamp":"2021-01-19T22:51:12.142Z","ecs":{"version":"1.5.0"},"message":"hi","log.origin":{"foo":"bar","file.name":"main.go","file.line":"42"}}`,
		"[2021-01-19T22:51:12.142Z]  INFO: hi\n    log.origin: {\n        \"file.name\": \"main.go\",\n        \"file.line\": \"42\"\n    }\n",
	},
	{
		"include fields: only log.origin.file.name",
		"no", "", "default", true, "", "", false, []string{"log.origin.file.name"},
		`{"log.level":"info","@timestamp":"2021-01-19T22:51:12.142Z","ecs":{"version":"1.5.0"},"message":"hi","log.origin":{"foo":"bar","file.name":"main.go","file.line":"42"}}`,
		"[2021-01-19T22:51:12.142Z]  INFO: hi\n    log.origin: {\n        \"file.name\": \"main.go\"\n    }\n",
	},
	{
		"include fields: only foo",
		"no", "", "default", true, "", "", false, []string{"foo"},
		`{"log.level":"info","@timestamp":"2021-01-19T22:51:12.142Z","ecs":{"version":"1.5.0"},"message":"hi","foo":0,"bar":1}`,
		"[2021-01-19T22:51:12.142Z]  INFO: hi\n    foo: 0\n",
	},
}

func TestRenderFile(t *testing.T) {
	for _, tc := range renderFileTestCases {
		t.Run(tc.name, func(t *testing.T) {
			r, err := ecslog.NewRenderer(
				tc.shouldColorize,
				tc.colorScheme,
				tc.formatName,
				-1,
				[]string{},
				tc.includeFields,
				tc.ecsLenient,
				tc.timestampShowDiff,
			)
			if err != nil {
				t.Errorf("ecslog.NewRenderer(%q, %q, %q) error: %s",
					tc.shouldColorize, tc.colorScheme, tc.formatName, err)
				return
			}
			if tc.levelFilter != "" {
				r.SetLevelFilter(tc.levelFilter)
			}
			if tc.kqlFilter != "" {
				err = r.SetKQLFilter(tc.kqlFilter)
				if err != nil {
					t.Errorf("r.SetKQLFilter(%q) error: %s",
						tc.kqlFilter, err)
					return
				}
			}

			in := bytes.NewBufferString(tc.input)
			var out bytes.Buffer
			r.RenderFile(in, &out)
			if diff := cmp.Diff(tc.output, out.String()); diff != "" {
				t.Errorf("r.RenderFile() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
