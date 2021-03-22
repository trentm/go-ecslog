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
		"basic",
		"foo",
		term{Val: "foo"},
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

	// Escaped keywords
	{"escape keyword and", "\\and", term{Val: "and"}, false},
	{"escape keyword or", "\\or", term{Val: "or"}, false},
	{"escape keyword not", "\\not", term{Val: "not"}, false},
	{"do NOT escape keyword and-prefix", "\\andMORE", term{Val: "\\andMORE"}, false},
	{"do NOT escape keyword or-prefix", "\\orMORE", term{Val: "\\orMORE"}, false},
	{"do NOT escape keyword not-prefix", "\\notMORE", term{Val: "\notMORE"}, false},

	{"'yo dawg' escaping test from kibana/.../ast.test.js",
		"\\\\\\(\\)\\:\\<\\>\\\"\\*",
		term{Val: "\\():<>\"*"},
		false,
	},

	// Quoted terms
	{
		"quoted empty",
		`""`,
		term{Val: ``},
		true,
	},
	{
		"quoted basic",
		`"foo"`,
		term{Val: `foo`},
		true,
	},
	{
		"quoted no wildcard",
		`"ba*"`,
		term{Val: `ba*`},
		true,
	},

	// Quoted escapes
	{"quoted escape whitespace t", "\"foo\\t\"", term{Val: "foo\t"}, true},
	{"quoted escape whitespace r", "\"foo\\r\"", term{Val: "foo\r"}, true},
	{"quoted escape whitespace n", "\"foo\\n\"", term{Val: "foo\n"}, true},
	{"quoted escape special \"", "\"foo\\\"\"", term{Val: "foo\""}, true},
	{"quoted escape special \\", "\"foo\\\\\"", term{Val: "foo\\"}, true},
	{"quoted non-escape e", "\"foo\\e\"", term{Val: "foo\\e"}, true},
	{"quoted do not escape *", "\"foo\\*\"", term{Val: "foo\\*"}, true},
	{"quoted do not escape (", "\"foo\\(\"", term{Val: "foo\\("}, true},
	{"quoted do not escape )", "\"foo\\)\"", term{Val: "foo\\)"}, true},
	{"quoted do not escape :", "\"foo\\:\"", term{Val: "foo\\:"}, true},
	{"quoted do not escape <", "\"foo\\<\"", term{Val: "foo\\<"}, true},
	{"quoted do not escape >", "\"foo\\>\"", term{Val: "foo\\>"}, true},
	{"quoted do not escape {", "\"foo\\{\"", term{Val: "foo\\{"}, true},
	{"quoted do not escape }", "\"foo\\}\"", term{Val: "foo\\}"}, true},
}

func TestTerm(t *testing.T) {
	for _, tc := range termTestCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("-- term test case %q\n", tc.name)
			t.Logf("  input: %q\n", tc.input)
			var trm term
			if tc.quoted {
				trm = newQuotedTerm(tc.input)
			} else {
				trm = newTerm(tc.input)
			}
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
