// nolint
package dto

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"

	"go.avito.ru/DO/moira/api"
)

var rewriteRules = []api.RewriteRule{
	{From: "env.prod.", To: ""},
	{From: "complex.agricultural.", To: "farm."},

	{From: "complex.omicron-test.", To: "something.different."},
	{From: "env.omicron.", To: "complex.omicron-test."},
	{From: "complex.omicron-test.", To: "something.different."},
}

func mustRewriteTargets(targets []string, rewriteRules []api.RewriteRule) []string {
	res, err := rewriteTargets(targets, rewriteRules)
	if err != nil {
		panic(err)
	}
	return res
}

func TestRewriteTargets(t *testing.T) {
	copyTestData := func() []string {
		return []string{
			"env.prod.foo.bar.metric",
			"env.omicron.horse",
			"someFunction(sum(complex.agricultural.goats.*.horn*.status), 373, '1min')",
			"sumSeries(env.prod.one, env.omicron.env.prod.two)",
			"not.replacing",
			"sumSeries(env.prod.one, not.really.env.prod.one)",
			"sumSeries(env.prod.one,not.really.env.prod.one)",
			"env.prod.env.prod.mouse",
		}
	}
	expected := []string{
		"foo.bar.metric",
		"complex.omicron-test.horse",
		"someFunction(sum(farm.goats.*.horn*.status), 373, '1min')",
		"sumSeries(one, complex.omicron-test.env.prod.two)",
		"not.replacing",
		"sumSeries(one, not.really.env.prod.one)",
		"sumSeries(one,not.really.env.prod.one)",
		"env.prod.mouse",
	}
	Convey("test rewriteTargets", t, func() {
		So(
			mustRewriteTargets(copyTestData(), rewriteRules),
			ShouldResemble, // not ShouldEqual because ShouldEqual cannot handle slices
			expected,
		)

		// duplicate all rewriteRules
		duplicateRules := append([]api.RewriteRule{}, rewriteRules...)
		duplicateRules = append(duplicateRules, rewriteRules...)
		So(
			mustRewriteTargets(copyTestData(), duplicateRules),
			ShouldResemble, // not ShouldEqual because ShouldEqual cannot handle slices
			expected,
		)
	})
}
