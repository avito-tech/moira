package redis

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"

	"go.avito.ru/DO/moira/test-helpers"
)

func TestInitialization(t *testing.T) {
	Convey("Initialization methods", t, func() {
		logger := test_helpers.GetTestLogger()
		database := NewDatabase(logger, emptyConfig)
		So(database, ShouldNotBeEmpty)
		_, err := database.pool.Dial()
		So(err, ShouldNotBeNil)
	})
}
