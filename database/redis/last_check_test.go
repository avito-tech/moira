package redis

import (
	"testing"

	"github.com/satori/go.uuid"
	. "github.com/smartystreets/goconvey/convey"

	"go.avito.ru/DO/moira"
	"go.avito.ru/DO/moira/database"
	"go.avito.ru/DO/moira/test-helpers"
)

func TestLastCheck(t *testing.T) {
	logger := test_helpers.GetTestLogger()
	dataBase := NewDatabase(logger, config)
	dataBase.flush()
	defer dataBase.flush()

	var (
		err error
	)

	Convey("LastCheck manipulation", t, func() {
		Convey("Test read write delete", func() {
			trigger := &triggers[0]
			triggerID := trigger.ID

			err = dataBase.SaveTrigger(trigger.ID, trigger)
			So(err, ShouldBeNil)

			err = dataBase.SetTriggerLastCheck(triggerID, lastCheckTest)
			So(err, ShouldBeNil)

			actual, err := dataBase.GetTriggerLastCheck(triggerID)
			So(err, ShouldBeNil)
			So(actual, ShouldResemble, lastCheckTest)

			err = dataBase.RemoveTriggerLastCheck(triggerID)
			So(err, ShouldBeNil)

			actual, err = dataBase.GetTriggerLastCheck(triggerID)
			So(err, ShouldResemble, database.ErrNil)
			So(actual, ShouldBeNil)
		})

		Convey("Test no lastcheck", func() {
			triggerID := uuid.NewV4().String()
			actual, err := dataBase.GetTriggerLastCheck(triggerID)
			So(err, ShouldBeError)
			So(err, ShouldResemble, database.ErrNil)
			So(actual, ShouldBeNil)
		})

		Convey("Test set trigger check maintenance", func() {
			Convey("While no metrics", func() {
				trigger := &triggers[1]
				triggerID := trigger.ID

				err = dataBase.SaveTrigger(trigger.ID, trigger)
				So(err, ShouldBeNil)

				err = dataBase.SetTriggerLastCheck(triggerID, lastCheckWithNoMetrics)
				So(err, ShouldBeNil)

				actual, err := dataBase.GetTriggerLastCheck(triggerID)
				So(err, ShouldBeNil)
				So(actual, ShouldResemble, lastCheckWithNoMetrics)
			})

			Convey("While no metrics to change", func() {
				trigger := &triggers[1]
				triggerID := trigger.ID

				err = dataBase.SaveTrigger(trigger.ID, trigger)
				So(err, ShouldBeNil)

				err = dataBase.SetTriggerLastCheck(triggerID, lastCheckTest)
				So(err, ShouldBeNil)

				actual, err := dataBase.GetTriggerLastCheck(triggerID)
				So(err, ShouldBeNil)
				So(actual, ShouldResemble, lastCheckTest)
			})
		})

		Convey("Test get trigger check ids", func() {
			dataBase.flush()

			okTrigger := &triggers[1]
			okTriggerID := okTrigger.ID
			err = dataBase.SaveTrigger(okTriggerID, okTrigger)
			So(err, ShouldBeNil)

			badTrigger := &triggers[2]
			badTriggerID := badTrigger.ID
			err = dataBase.SaveTrigger(badTriggerID, badTrigger)
			So(err, ShouldBeNil)

			err = dataBase.SetTriggerLastCheck(okTriggerID, lastCheckWithNoMetrics)
			So(err, ShouldBeNil)
			err = dataBase.SetTriggerLastCheck(badTriggerID, lastCheckTest)
			So(err, ShouldBeNil)

			actual, err := dataBase.GetTriggerCheckIDs(make([]string, 0), true)
			So(err, ShouldBeNil)
			So(actual, ShouldResemble, []string{badTriggerID})

			actual, err = dataBase.GetTriggerCheckIDs(make([]string, 0), false)
			So(err, ShouldBeNil)
			So(actual, ShouldResemble, []string{badTriggerID, okTriggerID})
		})
	})
}

func TestLastCheckErrorConnection(t *testing.T) {
	logger := test_helpers.GetTestLogger()
	dataBase := NewDatabase(logger, emptyConfig)
	dataBase.flush()
	defer dataBase.flush()

	Convey("Should throw error when no connection", t, func() {
		actual1, err := dataBase.GetTriggerLastCheck("123")
		So(actual1, ShouldBeNil)
		So(err, ShouldNotBeNil)

		err = dataBase.SetTriggerLastCheck("123", lastCheckTest)
		So(err, ShouldNotBeNil)

		err = dataBase.RemoveTriggerLastCheck("123")
		So(err, ShouldNotBeNil)

		actual2, err := dataBase.GetTriggerCheckIDs(make([]string, 0), true)
		So(actual2, ShouldResemble, []string(nil))
		So(err, ShouldNotBeNil)
	})
}

var lastCheckTest = &moira.CheckData{
	Score:     6000,
	State:     moira.OK,
	Timestamp: 1504509981,
	Metrics: map[string]*moira.MetricState{
		"metric1": {
			EventTimestamp: 1504449789,
			State:          moira.NODATA,
			Suppressed:     false,
			Timestamp:      1504509380,
		},
		"metric2": {
			EventTimestamp: 1504449789,
			State:          moira.NODATA,
			Suppressed:     false,
			Timestamp:      1504509380,
		},
		"metric3": {
			EventTimestamp: 1504449789,
			State:          moira.NODATA,
			Suppressed:     false,
			Timestamp:      1504509380,
		},
		"metric4": {
			EventTimestamp: 1504463770,
			State:          moira.NODATA,
			Suppressed:     false,
			Timestamp:      1504509380,
		},
		"metric5": {
			EventTimestamp: 1504463770,
			State:          moira.NODATA,
			Suppressed:     false,
			Timestamp:      1504509380,
		},
		"metric6": {
			EventTimestamp: 1504463770,
			State:          moira.OK,
			Suppressed:     false,
			Timestamp:      1504509380,
		},
	},
}

var lastCheckWithNoMetrics = &moira.CheckData{
	Score:     0,
	State:     moira.OK,
	Timestamp: 1504509981,
	Metrics:   make(map[string]*moira.MetricState),
}
