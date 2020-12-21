package silencer

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"

	"go.avito.ru/DO/moira/test-helpers"
)

func TestPatternMatch(t *testing.T) {
	test_helpers.InitTestLogging()
	silencer := NewSilencer(nil, nil)

	Convey("different; no dots", t, func() {
		So(silencer.isPatternMatched("foo", "bar"), ShouldBeFalse)
	})

	Convey("same; no dots", t, func() {
		So(silencer.isPatternMatched("foo", "foo"), ShouldBeTrue)
	})

	Convey("subpattern match", t, func() {
		So(silencer.isPatternMatched("nice.foo.nar", "foo"), ShouldBeTrue)
	})

	Convey("subpattern with dots match", t, func() {
		So(silencer.isPatternMatched("nice.foo.nar.bar", "foo.nar"), ShouldBeTrue)
	})

	Convey("subpattern with dots no match", t, func() {
		So(silencer.isPatternMatched("nice.foo.nar.bar", "foo.bar"), ShouldBeFalse)
	})

	Convey("subpattern with dots lookback match", t, func() {
		So(silencer.isPatternMatched("foo.foo.nar.bar", "foo.nar"), ShouldBeTrue)
	})

	Convey("subpattern with dots no reverse match", t, func() {
		So(silencer.isPatternMatched("foo.foo.nar.bar", "nar.foo"), ShouldBeFalse)
	})

	Convey("subpattern with empty", t, func() {
		So(silencer.isPatternMatched("", "bar"), ShouldBeFalse)
	})

	Convey("empty subpattern", t, func() {
		So(silencer.isPatternMatched("metric.", ""), ShouldBeTrue)
	})

	Convey("subpattern with dots lookback match(2)", t, func() {
		So(silencer.isPatternMatched("foo.nar.qaz.foo.nar.bar.end", "foo.nar.bar"), ShouldBeTrue)
	})

	Convey("subpattern with dots long lookback match(2)", t, func() {
		So(silencer.isPatternMatched("nar.nar.nar.nar.bar", "nar.nar.nar.bar"), ShouldBeTrue)
	})

	Convey("blacklist", t, func() {
		So(silencer.isPatternMatched("servers", "servers"), ShouldBeFalse)
		So(silencer.isPatternMatched("xservers", "xservers"), ShouldBeTrue)
	})
}
