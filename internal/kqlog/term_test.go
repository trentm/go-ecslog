package kqlog

import (
	"testing"
)

type termTestCase struct {
	name   string
	input  string
	trm    term
	quoted bool
}

var termTestCases = []termTestCase{
	{
		"empty",
		"",
		term{Val: ""},
		false,
	},

	{
		"wildcard 1",
		"ba*",
		term{Val: "^ba.*$", Wildcard: true},
		false,
	},
	{
		"wildcard 2",
		"*",
		term{Val: "^.*$", Wildcard: true},
		false,
	},
	{
		"wildcard 3",
		"*foo*",
		term{Val: "^.*foo.*$", Wildcard: true},
		false,
	},
	{
		"wildcard 4",
		"**",
		term{Val: "^.*.*$", Wildcard: true},
		false,
	},
	{
		"wildcard and escaped asterisk",
		"bar*\\*",
		term{Val: "^bar.*\\*$", Wildcard: true},
		false,
	},

	// From regexp.go, here are the regexp metacharacters: \.+*?()|[]{}^$
	{
		"wildcard with regexp quoting",
		`foo \ . + \* ? ( ) | [ ] { } ^ $ bar*`,
		term{Val: `^foo \\ \. \+ \* \? \( \) \| \[ \] \{ \} \^ \$ bar.*$`, Wildcard: true},
		false,
	},

	// Escapes
	{"escape whitespace t", "foo\\t", term{Val: "foo\t"}, false},
	{"escape whitespace r", "foo\\r", term{Val: "foo\r"}, false},
	{"escape whitespace n", "foo\\n", term{Val: "foo\n"}, false},
	{"escape special *", "foo\\*", term{Val: "foo*"}, false},
	{"escape special \\", "foo\\\\", term{Val: "foo\\"}, false},
	{"escape special (", "foo\\(", term{Val: "foo("}, false},
	{"escape special )", "foo\\)", term{Val: "foo)"}, false},
	{"escape special :", "foo\\:", term{Val: "foo:"}, false},
	{"escape special <", "foo\\<", term{Val: "foo<"}, false},
	{"escape special >", "foo\\>", term{Val: "foo>"}, false},
	{"escape special \"", "foo\\\"", term{Val: "foo\""}, false},
	{"escape special {", "foo\\{", term{Val: "foo{"}, false},
	{"escape special }", "foo\\}", term{Val: "foo}"}, false},
	{"non-escape e", "foo\\e", term{Val: "foo\\e"}, false},
}

func TestTerm(t *testing.T) {
	for _, tc := range termTestCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("-- term test case %q\n", tc.name)
			t.Logf("  input: %q\n", tc.input)
			trm := newTerm(tc.input)
			t.Logf("  term: %v\n", trm)
			if !(trm.Val == tc.trm.Val && trm.Wildcard == tc.trm.Wildcard) {
				t.Errorf(
					"%s:\n"+
						"input:\n"+
						"\t%q\n"+
						"got term:\n"+
						"\t%v\n"+
						"want term:\n"+
						"\t%v\n",
					tc.name, tc.input, trm, tc.trm)
			}
		})
	}
}
