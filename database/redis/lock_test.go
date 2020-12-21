package redis

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"

	"go.avito.ru/DO/moira/test-helpers"
)

func TestLock(t *testing.T) {
	logger := test_helpers.GetTestLogger()
	dataBase := NewDatabase(logger, config)
	dataBase.flush()
	defer dataBase.flush()

	Convey("Test lock manipulation", t, func() {
		triggerID1 := "id"

		isSet, err := dataBase.SetTriggerCheckLock(triggerID1)
		So(err, ShouldBeNil)
		So(isSet, ShouldBeTrue)

		isSet, err = dataBase.SetTriggerCheckLock(triggerID1)
		So(err, ShouldBeNil)
		So(isSet, ShouldBeFalse)

		err = dataBase.AcquireTriggerCheckLock(triggerID1)
		So(err, ShouldNotBeNil)

		err = dataBase.DeleteTriggerCheckLock(triggerID1)
		So(err, ShouldBeNil)

		err = dataBase.AcquireTriggerCheckLock(triggerID1)
		So(err, ShouldBeNil)

		isSet, err = dataBase.SetTriggerCheckLock(triggerID1)
		So(err, ShouldBeNil)
		So(isSet, ShouldBeFalse)

		err = dataBase.DeleteTriggerCheckLock(triggerID1)
		So(err, ShouldBeNil)
	})
}

func TestLockErrorConnection(t *testing.T) {
	logger := test_helpers.GetTestLogger()
	dataBase := NewDatabase(logger, emptyConfig)
	dataBase.flush()
	defer dataBase.flush()
	Convey("Should throw error when no connection", t, func() {
		err := dataBase.AcquireTriggerCheckLock("tr1")
		So(err, ShouldNotBeNil)

		actual, err := dataBase.SetTriggerCheckLock("tr1")
		So(err, ShouldNotBeNil)
		So(actual, ShouldBeFalse)

		err = dataBase.DeleteTriggerCheckLock("tr1")
		So(err, ShouldNotBeNil)
	})
}
