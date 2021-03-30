package main

import (
	"bytes"
	"log"
	"os/exec"
	"regexp"
	"runtime"
	"testing"
)

var EXE string

// init builds an `ecslog` binary for testing.
func init() {
	if runtime.GOOS == "windows" {
		EXE = ".\\ecslog-for-test.exe"
	} else {
		EXE = "./ecslog-for-test"
	}
	c := exec.Command("go", "build", "-o", EXE, ".")
	err := c.Run()
	if err != nil {
		log.Fatal(err)
	}
}

type mainTestCase struct {
	name     string
	argv     []string
	exitCode int
	stdout   *regexp.Regexp
	stderr   *regexp.Regexp
}

var mainTestCases = []mainTestCase{
	{
		"ecslog --version",
		[]string{"ecslog", "--version"},
		0,
		regexp.MustCompile(`^ecslog \d+\.\d+\.\d+\nhttps://`),
		nil,
	},
	{
		"ecslog --help",
		[]string{"ecslog", "--help"},
		0,
		regexp.MustCompile(`(?s)^ecslog.*usage:.*options:.*--help`),
		nil,
	},
	{
		"ecslog --bogus",
		[]string{"ecslog", "--bogus"},
		2,
		nil,
		nil,
	},
	{
		"ecslog --strict ...",
		[]string{"ecslog", "--no-config", "--strict", "./testdata/strict.log"},
		0,
		regexp.MustCompile(`^\[2021-01-19T22:51:12.142Z\]  INFO: this is valid\n$`),
		nil,
	},
	{
		// In earlier versions ecslog was using bufio.Scanner. A line >64k long
		// would error out with 'bufio.Scanner: token too long'. Here we expect
		// ecslog to handle this, and to properly render other lines.
		// '--no-config' is needed to ensure we keep the default 16k maxLineLen.
		"handle very long line",
		[]string{"ecslog", "--no-config", "./testdata/crash-long-line.log"},
		0,
		regexp.MustCompile(`^{"log\.level":"info",.*?,"message":".*?"}\n\[.*?\]  INFO: hi\n$`),
		nil,
	},
}

func TestMain(t *testing.T) {
	for _, tc := range mainTestCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("-- `ecslog` test case %q\n", tc.name)
			t.Logf("  argv: %q\n", tc.argv)
			exe := tc.argv[0]
			if exe == "ecslog" {
				exe = EXE
			}
			cmd := exec.Command(exe, tc.argv[1:]...)
			var e bytes.Buffer
			var o bytes.Buffer
			cmd.Stderr = &e
			cmd.Stdout = &o
			err := cmd.Run()
			stderr := e.Bytes()
			if err != nil {
				if exitErr, ok := err.(*exec.ExitError); ok {
					if exitErr.ExitCode() != tc.exitCode {
						t.Errorf(
							"test case %q:\n"+
								"argv:\n"+
								"\t%q\n"+
								"want exitCode:\n"+
								"\t%v\n"+
								"got exitCode:\n"+
								"\t%v\n"+
								"with stderr:\n"+
								"\t%q\n",
							tc.name, tc.argv, tc.exitCode, exitErr.ExitCode(), stderr)
					}
				} else {
					t.Errorf(
						"test case %q:\n"+
							"argv:\n"+
							"\t%q\n"+
							"err:\n"+
							"\t%v\n",
						tc.name, tc.argv, err)
				}
			} else if tc.exitCode != 0 {
				t.Errorf(
					"test case %q:\n"+
						"argv:\n"+
						"\t%q\n"+
						"want exitCode:\n"+
						"\t%v\n"+
						"got no error\n",
					tc.name, tc.argv, tc.exitCode)
			}
			if tc.stderr != nil && !tc.stderr.Match(stderr) {
				t.Errorf(
					"test case %q:\n"+
						"argv:\n"+
						"\t%q\n"+
						"want stderr to match:\n"+
						"\t%s\n"+
						"got stderr:\n"+
						"\t%q\n",
					tc.name, tc.argv, tc.stderr, stderr)
			}
			stdout := o.Bytes()
			if tc.stdout != nil && !tc.stdout.Match(stdout) {
				t.Errorf(
					"test case %q:\n"+
						"argv:\n"+
						"\t%q\n"+
						"want stdout to match:\n"+
						"\t%q\n"+
						"got stdout:\n"+
						"\t%q\n",
					tc.name, tc.argv, tc.stdout, stdout)
			}
		})
	}
}
