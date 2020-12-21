package moira

import (
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestIsScheduleAllows(t *testing.T) {
	allDaysExcludedSchedule := ScheduleData{
		StartOffset: 0,
		EndOffset:   1439,
		Days: []ScheduleDataDay{
			{
				Name:    "Mon",
				Enabled: false,
			},
			{
				Name:    "Tue",
				Enabled: false,
			},
			{
				Name:    "Wed",
				Enabled: false,
			},
			{
				Name:    "Thu",
				Enabled: false,
			},
			{
				Name:    "Fri",
				Enabled: false,
			},
			{
				Name:    "Sat",
				Enabled: false,
			},
			{
				Name:    "Sun",
				Enabled: false,
			},
		},
	}

	// 367980 - 01/05/1970 6:13am (UTC) Mon
	// 454380 - 01/06/1970 6:13am (UTC) Tue

	Convey("No schedule", t, func() {
		var noSchedule *ScheduleData
		So(noSchedule.IsScheduleAllows(367980), ShouldBeTrue)
	})

	Convey("Full schedule", t, func() {
		schedule := getDefaultSchedule()
		So(schedule.IsScheduleAllows(367980), ShouldBeTrue)
	})

	Convey("Exclude monday", t, func() {
		schedule := getDefaultSchedule()
		schedule.Days[0].Enabled = false
		So(schedule.IsScheduleAllows(367980), ShouldBeFalse)
		So(schedule.IsScheduleAllows(367980+86400), ShouldBeTrue)
		So(schedule.IsScheduleAllows(367980+86400*2), ShouldBeTrue)
	})

	Convey("Exclude all days", t, func() {
		schedule := allDaysExcludedSchedule
		So(schedule.IsScheduleAllows(367980), ShouldBeFalse)
		So(schedule.IsScheduleAllows(367980+86400), ShouldBeFalse)
		So(schedule.IsScheduleAllows(367980+86400*5), ShouldBeFalse)
	})

	Convey("Include only morning", t, func() {
		schedule := getDefaultSchedule()
		schedule.StartOffset = 60
		schedule.EndOffset = 540
		So(schedule.IsScheduleAllows(86400+129*60), ShouldBeTrue)  // 2/01/1970 2:09
		So(schedule.IsScheduleAllows(86400+239*60), ShouldBeTrue)  // 1/01/1970 3:59
		So(schedule.IsScheduleAllows(86400-241*60), ShouldBeFalse) // 1/01/1970 19:58
		So(schedule.IsScheduleAllows(86400+541*60), ShouldBeFalse) // 2/01/1970 9:01
		So(schedule.IsScheduleAllows(86400-255*60), ShouldBeFalse) // 1/01/1970 19:45
	})
}

func TestIsScheduleAllows_Overlapped(t *testing.T) {
	Convey("Each day is allowed, 07:00 - 01:00", t, func() {
		scheduleData := ScheduleData{
			StartOffset: 420,
			EndOffset:   60,
			Days: []ScheduleDataDay{
				{
					Name:    "Mon",
					Enabled: true,
				},
				{
					Name:    "Tue",
					Enabled: true,
				},
				{
					Name:    "Wed",
					Enabled: true,
				},
				{
					Name:    "Thu",
					Enabled: true,
				},
				{
					Name:    "Fri",
					Enabled: true,
				},
				{
					Name:    "Sat",
					Enabled: true,
				},
				{
					Name:    "Sun",
					Enabled: true,
				},
			},
		}

		So(scheduleData.IsScheduleAllows(1590429223), ShouldBeTrue)  // 1590429223 - Mon, 25 May 2020, 20:53:43 MSK
		So(scheduleData.IsScheduleAllows(1590441515), ShouldBeTrue)  // 1590441515 - Tue, 26 May 2020, 00:18:35 MSK
		So(scheduleData.IsScheduleAllows(1590377447), ShouldBeFalse) // 1590377447 - Mon, 25 May 2020, 06:30:47 MSK
		So(scheduleData.IsScheduleAllows(1590444047), ShouldBeFalse) // 1590444047 - Tue, 26 May 2020, 01:00:47 MSK
	})

	Convey("Workdays are allowed, 14:00 - 03:00", t, func() {
		scheduleData := ScheduleData{
			StartOffset: 840,
			EndOffset:   180,
			Days: []ScheduleDataDay{
				{
					Name:    "Mon",
					Enabled: true,
				},
				{
					Name:    "Tue",
					Enabled: true,
				},
				{
					Name:    "Wed",
					Enabled: true,
				},
				{
					Name:    "Thu",
					Enabled: true,
				},
				{
					Name:    "Fri",
					Enabled: true,
				},
				{
					Name:    "Sat",
					Enabled: false,
				},
				{
					Name:    "Sun",
					Enabled: false,
				},
			},
		}

		So(scheduleData.IsScheduleAllows(1590144703), ShouldBeFalse) // 1590144703 - Fri, 22 May 2020, 13:51:43 MSK
		So(scheduleData.IsScheduleAllows(1590153259), ShouldBeTrue)  // 1590153259 - Fri, 22 May 2020, 16:14:19 MSK
		So(scheduleData.IsScheduleAllows(1590176483), ShouldBeTrue)  // 1590176483 - Fri, 22 May 2020, 22:41:23 MSK
		So(scheduleData.IsScheduleAllows(1590188282), ShouldBeTrue)  // 1590188282 - Sat, 23 May 2020, 01:58:02 MSK
		So(scheduleData.IsScheduleAllows(1590231720), ShouldBeFalse) // 1590231720 - Sat, 23 May 2020, 14:02:00 MSK
		So(scheduleData.IsScheduleAllows(1590257764), ShouldBeFalse) // 1590257764 - Sat, 23 May 2020, 21:16:04 MSK
		So(scheduleData.IsScheduleAllows(1590276974), ShouldBeFalse) // 1590276974 - Sun, 24 May 2020, 02:36:14 MSK
	})
}

func TestEventsData_GetSubjectState(t *testing.T) {
	Convey("Get ERROR state", t, func() {
		message := "mes1"
		var value float64 = 1
		states := NotificationEvents{{State: OK}, {State: ERROR, Message: &message, Value: &value}}
		So(states.GetSubjectState(), ShouldResemble, ERROR)
		So(states[0].String(), ShouldResemble, "TriggerId: , Metric: \nValue: 0, OldValue: 0\nState: OK, OldState: \nMessage: \nTimestamp: 0")
		So(states[1].String(), ShouldResemble, "TriggerId: , Metric: \nValue: 1, OldValue: 0\nState: ERROR, OldState: \nMessage: mes1\nTimestamp: 0")
	})
}

func TestTriggerData_GetTags(t *testing.T) {
	Convey("Test one tag", t, func() {
		triggerData := TriggerData{
			Tags: []string{"tag1"},
		}
		So(triggerData.GetTags(), ShouldResemble, "[tag1]")
	})
	Convey("Test many tags", t, func() {
		triggerData := TriggerData{
			Tags: []string{"tag1", "tag2", "tag...orNot"},
		}
		So(triggerData.GetTags(), ShouldResemble, "[tag1][tag2][tag...orNot]")
	})
	Convey("Test no tags", t, func() {
		triggerData := TriggerData{
			Tags: make([]string, 0),
		}
		So(triggerData.GetTags(), ShouldBeEmpty)
	})
}

func TestScheduledNotification_GetKey(t *testing.T) {
	Convey("Get key", t, func() {
		notification := ScheduledNotification{
			Contact:   ContactData{Type: "email", Value: "my@mail.com"},
			Event:     NotificationEvent{Value: nil, State: NODATA, Metric: "my.metric"},
			Timestamp: 123456789,
		}
		So(notification.GetKey(), ShouldResemble, "email:my@mail.com::my.metric:NODATA:0:0.000000:0:123456789")
	})
}

func TestCheckData_GetOrCreateMetricState(t *testing.T) {
	Convey("Test no metric", t, func() {
		checkData := CheckData{
			Metrics: make(map[string]*MetricState),
		}
		So(
			checkData.GetOrCreateMetricState("my.metric", 12343),
			ShouldResemble,
			&MetricState{State: NODATA, Timestamp: 12343, IsNoData: true},
		)
	})
	Convey("Test has metric", t, func() {
		metricState := &MetricState{Timestamp: 11211, IsNoData: true}
		checkData := CheckData{
			Metrics: map[string]*MetricState{
				"my.metric": metricState,
			},
		}
		So(checkData.GetOrCreateMetricState("my.metric", 12343), ShouldResemble, metricState)
	})
}

func TestMetricState_GetCheckPoint(t *testing.T) {
	Convey("Get check point", t, func() {
		metricState := MetricState{Timestamp: 800, EventTimestamp: 700}
		So(metricState.GetCheckPoint(120), ShouldEqual, 700)

		metricState = MetricState{Timestamp: 830, EventTimestamp: 700}
		So(metricState.GetCheckPoint(120), ShouldEqual, 710)

		metricState = MetricState{Timestamp: 699, EventTimestamp: 700}
		So(metricState.GetCheckPoint(1), ShouldEqual, 700)
	})
}

func TestMetricState_GetEventTimestamp(t *testing.T) {
	Convey("Get event timestamp", t, func() {
		metricState := MetricState{Timestamp: 800, EventTimestamp: 0}
		So(metricState.GetEventTimestamp(), ShouldEqual, 800)

		metricState = MetricState{Timestamp: 830, EventTimestamp: 700}
		So(metricState.GetEventTimestamp(), ShouldEqual, 700)
	})
}

func TestTrigger_IsSimple(t *testing.T) {
	Convey("Is Simple", t, func() {
		trigger := Trigger{
			Patterns: []string{"123"},
			Targets:  []string{"123"},
		}

		So(trigger.IsSimple(), ShouldBeTrue)
	})

	Convey("Not simple", t, func() {
		triggers := []Trigger{
			{Patterns: []string{"123", "1233"}},
			{Patterns: []string{"123", "1233"}, Targets: []string{"123", "1233"}},
			{Targets: []string{"123", "1233"}},
			{Patterns: []string{"123"}, Targets: []string{"123", "1233"}},
			{Patterns: []string{"123?"}, Targets: []string{"123"}},
			{Patterns: []string{"12*3"}, Targets: []string{"123"}},
			{Patterns: []string{"1{23"}, Targets: []string{"123"}},
			{Patterns: []string{"[123"}, Targets: []string{"123"}},
			{Patterns: []string{"[12*3"}, Targets: []string{"123"}},
		}

		for _, trigger := range triggers {
			So(trigger.IsSimple(), ShouldBeFalse)
		}
	})
}

func TestCheckData_GetEventTimestamp(t *testing.T) {
	Convey("Get event timestamp", t, func() {
		checkData := CheckData{Timestamp: 800, EventTimestamp: 0}
		So(checkData.GetEventTimestamp(), ShouldEqual, 800)

		checkData = CheckData{Timestamp: 830, EventTimestamp: 700}
		So(checkData.GetEventTimestamp(), ShouldEqual, 700)
	})
}

func TestCheckData_UpdateScore(t *testing.T) {
	Convey("Update score", t, func() {
		checkData := CheckData{State: ERROR}
		So(checkData.UpdateScore(), ShouldEqual, 100)
		So(checkData.Score, ShouldEqual, 100)

		checkData = CheckData{
			State: OK,
			Metrics: map[string]*MetricState{
				"123": {State: ERROR},
				"321": {State: OK},
				"345": {State: WARN},
			},
		}
		So(checkData.UpdateScore(), ShouldEqual, 101)
		So(checkData.Score, ShouldEqual, 101)

		checkData = CheckData{
			State: ERROR,
			Metrics: map[string]*MetricState{
				"123": {State: ERROR},
				"321": {State: OK},
				"345": {State: WARN},
			},
		}
		So(checkData.UpdateScore(), ShouldEqual, 201)
		So(checkData.Score, ShouldEqual, 201)
	})
}

func getDefaultSchedule() ScheduleData {
	return ScheduleData{
		StartOffset: 0,
		EndOffset:   1439,
		Days: []ScheduleDataDay{
			{
				Name:    "Mon",
				Enabled: true,
			},
			{
				Name:    "Tue",
				Enabled: true,
			},
			{
				Name:    "Wed",
				Enabled: true,
			},
			{
				Name:    "Thu",
				Enabled: true,
			},
			{
				Name:    "Fri",
				Enabled: true,
			},
			{
				Name:    "Sat",
				Enabled: true,
			},
			{
				Name:    "Sun",
				Enabled: true,
			},
		},
	}
}
