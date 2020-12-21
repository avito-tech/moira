package dto

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestValidateWebHook(t *testing.T) {
	Convey("Test Validate", t, func() {
		Convey("Test ip 10.* with port", func() {
			So(validateWebhook("http://10.2.2.2:3333/foobar"), ShouldBeNil)
		})
		Convey("Test ip 10.* without port", func() {
			So(validateWebhook("http://10.2.2.2/foobar"), ShouldBeNil)
		})
		Convey("Test ip 10.* with bad port", func() {
			So(validateWebhook("http://10.2.2.2:99999/foobar"), ShouldBeError, "incorrect port 99999")
		})
		Convey("Test ip not 10.*", func() {
			So(validateWebhook("http://11.2.2.2:9999/foobar"), ShouldBeError, "incorrect host 11.2.2.2:9999")
		})

		Convey("Test bad scheme", func() {
			So(validateWebhook("httpz://ok.avito.ru"), ShouldBeError, "incorrect scheme httpz")
		})
		Convey("Test http scheme", func() {
			So(validateWebhook("http://ok.avito.ru"), ShouldBeNil)
		})
		Convey("Test https scheme", func() {
			So(validateWebhook("https://ok.avito.ru"), ShouldBeNil)
		})
		Convey("Test no scheme", func() {
			So(validateWebhook("ok.avito.ru"), ShouldBeError, "incorrect scheme ")
		})

		Convey("Test not internal avito", func() {
			So(validateWebhook("http://foo"), ShouldBeError, "incorrect host foo")
		})

		Convey("Test internal avito", func() {
			So(validateWebhook("http://foo.avITO.rU"), ShouldBeNil)
		})

	})
}
