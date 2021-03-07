package ecslog_test

import (
	"bytes"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/trentm/go-ecslog/internal/ecslog"
	"github.com/valyala/fastjson"
	"go.elastic.co/ecszap"
	"go.uber.org/zap"
)

type renderFileTestCase struct {
	name           string
	shouldColorize string
	colorScheme    string
	formatName     string
	levelFilter    string
	kqlFilter      string
	input          string
	output         string
}

var renderFileTestCases = []renderFileTestCase{
	// Non-ecs-logging lines
	{
		"empty object",
		"no", "", "default", "", "",
		"{}",
		"{}",
	},
}

func equalVal(a, b *fastjson.Value) bool {
	if a == nil {
		return b == nil
	} else if b == nil {
		return false
	} else {
		return a.String() == b.String()
	}
}

// createTestLogger creates a zap logger required for using Renderer.
// It is set to the "fatal" level to silence logging output.
func createTestLogger() *zap.Logger {
	encoderConfig := ecszap.NewDefaultEncoderConfig()
	core := ecszap.NewCore(encoderConfig, os.Stderr, zap.FatalLevel)
	lg := zap.New(core, zap.AddCaller()).Named("ecslog")
	return lg
}

func TestRenderFile(t *testing.T) {
	lg := createTestLogger()
	for _, tc := range renderFileTestCases {
		t.Run(tc.name, func(t *testing.T) {
			r, err := ecslog.NewRenderer(lg, tc.shouldColorize, tc.colorScheme, tc.formatName)
			if err != nil {
				t.Errorf("ecslog.NewRenderer(lg, %q, %q, %q) error: %s",
					tc.shouldColorize, tc.colorScheme, tc.formatName, err)
				return
			}
			in := bytes.NewBufferString(tc.input)
			var out bytes.Buffer
			r.RenderFile(in, &out)
			// Add newline here because adding the trailing newline in all test
			// cases above is a PITA.
			want := tc.output + "\n"
			if diff := cmp.Diff(want, out.String()); diff != "" {
				t.Errorf("r.RenderFile() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
