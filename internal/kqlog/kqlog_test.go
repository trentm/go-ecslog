package kqlog

import (
	"testing"

	"github.com/valyala/fastjson"
)

type matchTestCase struct {
	name  string
	rec   *fastjson.Value
	kql   string
	match bool
}

var matchTestCases = []matchTestCase{
	{
		"empty KQL matches all",
		fastjson.MustParse(`{"foo": "bar"}`),
		"",
		true,
	},

	// Exists queries
	{
		"exists query",
		fastjson.MustParse(`{"foo": "bar"}`),
		"foo:*",
		true,
	},
	{
		"exists query: false",
		fastjson.MustParse(`{"foo": "bar"}`),
		"baz:*",
		false,
	},

	// Terms queries
	{
		"terms query",
		fastjson.MustParse(`{"foo": "bar"}`),
		"foo:bar",
		true,
	},
	{
		"terms query: false",
		fastjson.MustParse(`{"foo": "bar"}`),
		"foo :baz",
		false,
	},
	{
		"terms query: multiple values",
		fastjson.MustParse(`{"foo": "baz"}`),
		"foo: bar baz",
		true,
	},
	{
		"terms query: bool match 1",
		fastjson.MustParse(`{"foo": true}`),
		"foo:true",
		true,
	},
	{
		"terms query: bool match 2",
		fastjson.MustParse(`{"foo": false}`),
		"foo:false",
		true,
	},
	{
		"terms query: bool match 3",
		fastjson.MustParse(`{"foo": false}`),
		"foo:nope",
		false,
	},
	{
		"terms query: bool match 4",
		fastjson.MustParse(`{"foo.bar": true}`),
		"foo.bar:baz true",
		true,
	},

	{
		"terms query: num match 1",
		fastjson.MustParse(`{"foo": 42}`),
		"foo:42",
		true,
	},
	{
		"terms query: num match 2",
		fastjson.MustParse(`{"foo": 42.0}`),
		"foo:42",
		true,
	},
	{
		"terms query: num match 3",
		fastjson.MustParse(`{"foo": 42}`),
		"foo:42.000",
		true,
	},
	{
		"terms query: num match 4",
		fastjson.MustParse(`{"foo": 42}`),
		"foo:4.2e1",
		true,
	},
	{
		"terms query: num match 5",
		fastjson.MustParse(`{"foo": "bar"}`),
		"foo:42",
		false,
	},
	{
		"terms query: num match 6",
		fastjson.MustParse(`{"foo": 43}`),
		"foo:41 42 buzz 43",
		true,
	},
	{
		"terms query: num match 7",
		fastjson.MustParse(`{"foo": 3.1415926535}`),
		"foo:3.1415926535",
		true,
	},
	{
		"terms query: null match 1",
		fastjson.MustParse(`{"foo": null}`),
		"foo:null",
		true,
	},
	{
		"terms query: null match 2",
		fastjson.MustParse(`{"foo":"null"}`),
		"foo:null",
		true,
	},
	{
		"terms query: null match 3",
		fastjson.MustParse(`{"foo":"bar"}`),
		"foo:null",
		false,
	},
	{
		"terms query: null match 4",
		fastjson.MustParse(`{"foo": null}`),
		"foo:bar blah null",
		true,
	},

	{
		"terms query: object match",
		fastjson.MustParse(`{"foo": {"bar": "baz"}}`),
		"foo:bar",
		false,
	},
	{
		"terms query: array match",
		fastjson.MustParse(`{"foo": ["bar", 2]}`),
		"foo:bar",
		false,
	},
	{
		"terms query: no array index lookups",
		fastjson.MustParse(`{"foo": ["bar", 2]}`),
		"foo.0:bar",
		false,
	},

	// "matchAll" terms queries, e.g. `foo:(bar and baz)` which
	// https://www.elastic.co/guide/en/kibana/current/kuery-query.html
	// describes as:
	// > To match multi-value fields that contain a list of terms:
	// > tags:(success and info and security)
	{
		"matchAll terms query: yep",
		fastjson.MustParse(`{"tags": ["a", "success", "security", "b", "info"]}`),
		"tags:(success and info and security)",
		true,
	},
	{
		"matchAll terms query: nope",
		fastjson.MustParse(`{"tags": ["a", "success", "b", "info"]}`),
		"tags:(success and info and security)",
		false,
	},
	{
		"matchAll terms query: yep, mixed types",
		fastjson.MustParse(`{"foo": ["one", 2, "three", 42]}`),
		"foo:(one and 42)",
		true,
	},
	{
		"matchAll terms query: non-array",
		fastjson.MustParse(`{"foo": "bar"}`),
		"foo:(bar and baz)",
		false,
	},

	// Range queries
	{
		"range query: gt",
		fastjson.MustParse(`{"foo": 2}`),
		"foo > 1",
		true,
	},
	{
		"range query: gt, false",
		fastjson.MustParse(`{"foo": 2}`),
		"foo > 2",
		false,
	},
	{
		"range query: gt, no spaces",
		fastjson.MustParse(`{"foo": 2}`),
		"foo>1",
		true,
	},
	{
		"range query: gt, strings 1",
		fastjson.MustParse(`{"foo": "bar"}`),
		"foo > baa",
		true,
	},
	{
		"range query: gt, strings 2",
		fastjson.MustParse(`{"foo": "bar"}`),
		"foo > bar",
		false,
	},
	{
		"range query: gt, log.level special casing 1",
		// Intentionally pick a comparison where regular string comparison
		// would fail. I.e. we are relying on the expected ordering from the
		// given `LogLevelLessFn`.
		fastjson.MustParse(`{"log.level": "error"}`),
		"log.level > info",
		true,
	},
	{
		"range query: gt, log.level special casing 2",
		fastjson.MustParse(`{"log.level": "error"}`),
		"log.level > error",
		false,
	},

	{
		"range query: gte",
		fastjson.MustParse(`{"foo": 2}`),
		"foo >= 2.0",
		true,
	},
	{
		"range query: gte, false",
		fastjson.MustParse(`{"foo": 2}`),
		"foo >= 2.5",
		false,
	},
	{
		"range query: gte, no spaces",
		fastjson.MustParse(`{"foo": 2}`),
		"foo>=1",
		true,
	},
	{
		"range query: gte, strings 1",
		fastjson.MustParse(`{"foo": "bar"}`),
		"foo >= baa",
		true,
	},
	{
		"range query: gte, strings 2",
		fastjson.MustParse(`{"foo": "bar"}`),
		"foo >= bar",
		true,
	},
	{
		"range query: gte, log.level special casing 1",
		// Intentionally pick a comparison where regular string comparison
		// would fail. I.e. we are relying on the expected ordering from the
		// given `LogLevelLessFn`.
		fastjson.MustParse(`{"log.level": "error"}`),
		"log.level >= info",
		true,
	},
	{
		"range query: gte, log.level special casing 2",
		fastjson.MustParse(`{"log.level": "error"}`),
		"log.level >= error",
		true,
	},

	{
		"range query: lt",
		fastjson.MustParse(`{"foo": 2}`),
		"foo < 3",
		true,
	},
	{
		"range query: lt, false",
		fastjson.MustParse(`{"foo": 2}`),
		"foo < 2",
		false,
	},
	{
		"range query: lt, no spaces",
		fastjson.MustParse(`{"foo": 2}`),
		"foo<3",
		true,
	},
	{
		"range query: lt, strings 1",
		fastjson.MustParse(`{"foo": "bar"}`),
		"foo < baz",
		true,
	},
	{
		"range query: lt, strings 2",
		fastjson.MustParse(`{"foo": "bar"}`),
		"foo < bar",
		false,
	},
	{
		"range query: lt, log.level special casing 1",
		// Intentionally pick a comparison where regular string comparison
		// would fail. I.e. we are relying on the expected ordering from the
		// given `LogLevelLessFn`.
		fastjson.MustParse(`{"log.level": "trace"}`),
		"log.level < info",
		true,
	},
	{
		"range query: lt, log.level special casing 2",
		fastjson.MustParse(`{"log.level": "error"}`),
		"log.level < error",
		false,
	},

	{
		"range query: lte",
		fastjson.MustParse(`{"foo": 2}`),
		"foo <= 2",
		true,
	},
	{
		"range query: lte, false",
		fastjson.MustParse(`{"foo": 2}`),
		"foo <= 1",
		false,
	},
	{
		"range query: lte, no spaces",
		fastjson.MustParse(`{"foo": 2}`),
		"foo<=2.5",
		true,
	},
	{
		"range query: lte, strings 1",
		fastjson.MustParse(`{"foo": "bar"}`),
		"foo <= baz",
		true,
	},
	{
		"range query: lte, strings 2",
		fastjson.MustParse(`{"foo": "bar"}`),
		"foo <= bar",
		true,
	},
	{
		"range query: lte, log.level special casing 1",
		// Intentionally pick a comparison where regular string comparison
		// would fail. I.e. we are relying on the expected ordering from the
		// given `LogLevelLessFn`.
		fastjson.MustParse(`{"log.level": "trace"}`),
		"log.level <= info",
		true,
	},
	{
		"range query: lte, log.level special casing 2",
		fastjson.MustParse(`{"log.level": "error"}`),
		"log.level <= error",
		true,
	},

	// TODO: re-enable these tests when support quoted literals
	// // Date range queries.
	// //
	// // Q: Does Kibana specially handle time fields comparison? The KQL docs
	// // aren't very specific. E.g. are dates with timezone offsets normalized
	// // to UTC for comparison?
	// // https://www.elastic.co/guide/en/kibana/current/kuery-query.html#_date_range_queries
	// //
	// // Here we treat them just as a string comparisons, relying on time/date
	// // strings that are comparable.
	// {
	// 	"date range query 1",
	// 	fastjson.MustParse(`{"log.level":"info","@timestamp":"2021-01-19T22:51:12.142Z","ecs":{"version":"1.5.0"},"message":"hi"}`),
	// 	`@timestamp < 2021-02-14T21:55:59`,
	// 	true,
	// },
	// {
	// 	"date range query 2",
	// 	fastjson.MustParse(`{"log.level":"info","@timestamp":"2021-01-19T22:51:12.142Z","ecs":{"version":"1.5.0"},"message":"hi"}`),
	// 	`@timestamp < 2021-02`,
	// 	true,
	// },
	// {
	// 	"date range query 3",
	// 	fastjson.MustParse(`{"log.level":"info","@timestamp":"2021-01-19T22:51:12.142Z","ecs":{"version":"1.5.0"},"message":"hi"}`),
	// 	`@timestamp >= 2021`,
	// 	true,
	// },
}

func indexOf(items []string, val string) int {
	for i, item := range items {
		if item == val {
			return i
		}
	}
	return -1
}

// limitedLogLevelLess is a less complete `LogLevelLessFn` than
// ecslog.LogLevelLess that is sufficient for testing.
func limitedLogLevelLess(level1, level2 string) bool {
	order := []string{
		"trace",
		"debug",
		"info",
		"warn",
		"error",
	}
	idx1 := indexOf(order, level1)
	idx2 := indexOf(order, level2)
	if idx1 == -1 || idx2 == -1 {
		return false
	}
	return idx1 < idx2
}

func TestMatch(t *testing.T) {
	for _, tc := range matchTestCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("-- match test case %q\n", tc.name)
			t.Logf("  rec: %s\n", tc.rec)
			t.Logf("  kql: %s\n", tc.kql)
			filter, err := NewFilter(tc.kql, limitedLogLevelLess)
			if err != nil {
				t.Errorf("%s: NewFilter(kql) error: %s\nkql:\n\t%s\n",
					tc.name, err, tc.kql)
				return
			}
			t.Logf("  filter: %s\n", filter)
			match := filter.Match(tc.rec)
			t.Logf("  match: %v\n", match)
			if match != tc.match {
				t.Errorf(
					"%s:\n"+
						"kql:\n"+
						"\t%s\n"+
						"rec:\n"+
						"\t%s\n"+
						"got:\n"+
						"\t%v\n"+
						"expected:\n"+
						"\t%v\n",
					tc.name, tc.kql, tc.rec, match, tc.match)
			}
		})
	}
}
