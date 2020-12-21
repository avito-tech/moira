package redis

import (
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"

	"go.avito.ru/DO/moira/test-helpers"
)

func TestThrottlingErrorConnection(t *testing.T) {
	logger := test_helpers.GetTestLogger()
	dataBase := NewDatabase(logger, emptyConfig)
	dataBase.flush()
	defer dataBase.flush()
	Convey("Should throw error when no connection", t, func() {
		t1, t2 := dataBase.GetTriggerThrottling("")
		So(t1, ShouldResemble, time.Unix(0, 0))
		So(t2, ShouldResemble, time.Unix(0, 0))

		err := dataBase.SetTriggerThrottling("", time.Now())
		So(err, ShouldNotBeNil)

		err = dataBase.DeleteTriggerThrottling("")
		So(err, ShouldNotBeNil)
	})
}
