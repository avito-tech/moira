package redis

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"

	"go.avito.ru/DO/moira/test-helpers"
)

func TestSelfCheck(t *testing.T) {
	logger := test_helpers.GetTestLogger()

	dataBase := NewDatabase(logger, config)
	dataBase.flush()
	defer dataBase.flush()

	Convey("Self state triggers manipulation", t, func() {
		Convey("Empty config", func() {
			count, err := dataBase.GetMetricsUpdatesCount()
			So(count, ShouldEqual, 0)
			So(err, ShouldBeNil)

			count, err = dataBase.GetChecksUpdatesCount()
			So(count, ShouldEqual, 0)
			So(err, ShouldBeNil)
		})

		Convey("Update metrics heartbeat test", func() {
			err := dataBase.UpdateMetricsHeartbeat()
			So(err, ShouldBeNil)

			count, err := dataBase.GetMetricsUpdatesCount()
			So(count, ShouldEqual, 1)
			So(err, ShouldBeNil)
		})

		Convey("Update metrics checks updates count", func() {
			trigger := &triggers[0]
			triggerID := trigger.ID

			err := dataBase.SaveTrigger(triggerID, trigger)
			So(err, ShouldBeNil)

			err = dataBase.SetTriggerLastCheck(triggerID, lastCheckTest)
			So(err, ShouldBeNil)

			count, err := dataBase.GetChecksUpdatesCount()
			So(count, ShouldEqual, 1)
			So(err, ShouldBeNil)
		})
	})
}

func TestSelfCheckErrorConnection(t *testing.T) {
	logger := test_helpers.GetTestLogger()

	dataBase := NewDatabase(logger, emptyConfig)
	dataBase.flush()
	defer dataBase.flush()

	Convey("Should throw error when no connection", t, func() {
		count, err := dataBase.GetMetricsUpdatesCount()
		So(count, ShouldEqual, 0)
		So(err, ShouldNotBeNil)

		count, err = dataBase.GetChecksUpdatesCount()
		So(count, ShouldEqual, 0)
		So(err, ShouldNotBeNil)

		err = dataBase.UpdateMetricsHeartbeat()
		So(err, ShouldNotBeNil)
	})
}
