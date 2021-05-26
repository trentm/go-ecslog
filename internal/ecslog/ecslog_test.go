package ecslog_test

import (
	"bytes"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/trentm/go-ecslog/internal/ecslog"
)

type renderFileTestCase struct {
	name           string
	shouldColorize string
	colorScheme    string
	formatName     string
	levelFilter    string
	kqlFilter      string
	includeFields  []string
	input          string
	output         string
}

var renderFileTestCases = []renderFileTestCase{
	// Non-ecs-logging lines
	{
		"empty object",
		"no", "", "default", "", "", []string{},
		"{}",
		"{}\n",
	},

	// Basics
	{
		"basic",
		"no", "", "default", "", "", []string{},
		`{"log.level":"info","@timestamp":"2021-01-19T22:51:12.142Z","ecs":{"version":"1.5.0"},"message":"hi"}`,
		"[2021-01-19T22:51:12.142Z]  INFO: hi\n",
	},
	{
		"basic, extra var",
		"no", "", "default", "", "", []string{},
		`{"log.level":"info","@timestamp":"2021-01-19T22:51:12.142Z","ecs":{"version":"1.5.0"},"message":"hi","foo":"bar"}`,
		"[2021-01-19T22:51:12.142Z]  INFO: hi\n    foo: \"bar\"\n",
	},
	{
		"no message is allowed",
		"no", "", "default", "", "", []string{},
		`{"log.level":"info","@timestamp":"2021-01-19T22:51:12.142Z","ecs":{"version":"1.5.0"},"foo":"bar"}`,
		"[2021-01-19T22:51:12.142Z]  INFO:\n    foo: \"bar\"\n",
	},

	// Coloring
	{
		"coloring 1",
		"yes", "default", "default", "", "", []string{},
		`{"log.level":"info","@timestamp":"2021-01-19T22:51:12.142Z","ecs":{"version":"1.5.0"},"message":"hi"}`,
		"[2021-01-19T22:51:12.142Z] \x1b[32m INFO\x1b[0m: \x1b[36mhi\x1b[0m\n",
	},

	// KQL filtering
	{
		"kql filtering, yep",
		"no", "", "default", "", "foo:bar", []string{},
		`{"log.level":"info","@timestamp":"2021-01-19T22:51:12.142Z","ecs":{"version":"1.5.0"},"message":"hi","foo":"bar"}`,
		"[2021-01-19T22:51:12.142Z]  INFO: hi\n    foo: \"bar\"\n",
	},
	{
		"kql filtering, nope",
		"no", "", "default", "", "foo:baz", []string{},
		`{"log.level":"info","@timestamp":"2021-01-19T22:51:12.142Z","ecs":{"version":"1.5.0"},"message":"hi","foo":"bar"}`,
		"",
	},
	{
		"kql filtering, log.level range query, yep",
		"no", "", "default", "", "log.level > debug", []string{},
		`{"log.level":"info","@timestamp":"2021-01-19T22:51:12.142Z","ecs":{"version":"1.5.0"},"message":"hi"}`,
		"[2021-01-19T22:51:12.142Z]  INFO: hi\n",
	},
	{
		"kql filtering, log.level range query, nope",
		"no", "", "default", "", "log.level > warn", []string{},
		`{"log.level":"info","@timestamp":"2021-01-19T22:51:12.142Z","ecs":{"version":"1.5.0"},"message":"hi"}`,
		"",
	},
	{
		"only log",
		"no", "", "default", "", "", []string{"log"},
		`{"log.level":"info","@timestamp":"2021-01-19T22:51:12.142Z","ecs":{"version":"1.5.0"},"message":"hi","log.origin":{"foo":"bar",file.name":"main.go","file.line":"42"}}`,
		"{\"log.level\":\"info\",\"@timestamp\":\"2021-01-19T22:51:12.142Z\",\"ecs\":{\"version\":\"1.5.0\"},\"message\":\"hi\",\"log.origin\":{\"foo\":\"bar\",file.name\":\"main.go\",\"file.line\":\"42\"}}\n",
	},
	{
		"only log.origin.file",
		"no", "", "default", "", "", []string{"log.origin.file"},
		`{"log.level":"info","@timestamp":"2021-01-19T22:51:12.142Z","ecs":{"version":"1.5.0"},"message":"hi","log.origin":{"foo":"bar","file.name":"main.go","file.line":"42"}}`,
		"[2021-01-19T22:51:12.142Z]  INFO: hi\n    log.origin: {\n        \"file.name\": \"main.go\",\n        \"file.line\": \"42\"\n    }\n",
	},
	{
		"only log.origin.file.name",
		"no", "", "default", "", "", []string{"log.origin.file.name"},
		`{"log.level":"info","@timestamp":"2021-01-19T22:51:12.142Z","ecs":{"version":"1.5.0"},"message":"hi","log.origin":{"foo":"bar","file.name":"main.go","file.line":"42"}}`,
		"[2021-01-19T22:51:12.142Z]  INFO: hi\n    log.origin: {\n        \"file.name\": \"main.go\"\n    }\n",
	},
	{
		"only foo",
		"no", "", "default", "", "", []string{"foo"},
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
