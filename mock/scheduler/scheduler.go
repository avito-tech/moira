// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/moira-alert/moira/notifier (interfaces: Scheduler)

// Package mock_moira_alert is a generated GoMock package.
package mock_scheduler

import (
	gomock "github.com/golang/mock/gomock"
	moira "go.avito.ru/DO/moira"
	reflect "reflect"
	time "time"
)

// MockScheduler is a mock of Scheduler interface
type MockScheduler struct {
	ctrl     *gomock.Controller
	recorder *MockSchedulerMockRecorder
}

// MockSchedulerMockRecorder is the mock recorder for MockScheduler
type MockSchedulerMockRecorder struct {
	mock *MockScheduler
}

// NewMockScheduler creates a new mock instance
func NewMockScheduler(ctrl *gomock.Controller) *MockScheduler {
	mock := &MockScheduler{ctrl: ctrl}
	mock.recorder = &MockSchedulerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use
func (m *MockScheduler) EXPECT() *MockSchedulerMockRecorder {
	return m.recorder
}

// CalculateBackoff mocks base method
func (m *MockScheduler) CalculateBackoff(arg0 int) time.Duration {
	ret := m.ctrl.Call(m, "CalculateBackoff", arg0)
	ret0, _ := ret[0].(time.Duration)
	return ret0
}

// CalculateBackoff indicates an expected call of CalculateBackoff
func (mr *MockSchedulerMockRecorder) CalculateBackoff(arg0 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CalculateBackoff", reflect.TypeOf((*MockScheduler)(nil).CalculateBackoff), arg0)
}

// GetDeliveryInfo mocks base method
func (m *MockScheduler) GetDeliveryInfo(arg0 time.Time, arg1 moira.NotificationEvent, arg2 bool, arg3 int) (time.Time, bool) {
	ret := m.ctrl.Call(m, "GetDeliveryInfo", arg0, arg1, arg2, arg3)
	ret0, _ := ret[0].(time.Time)
	ret1, _ := ret[1].(bool)
	return ret0, ret1
}

// GetDeliveryInfo indicates an expected call of GetDeliveryInfo
func (mr *MockSchedulerMockRecorder) GetDeliveryInfo(arg0, arg1, arg2, arg3 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetDeliveryInfo", reflect.TypeOf((*MockScheduler)(nil).GetDeliveryInfo), arg0, arg1, arg2, arg3)
}

// ScheduleNotification mocks base method
func (m *MockScheduler) ScheduleNotification(arg0 time.Time, arg1 bool, arg2 moira.NotificationEvent, arg3 moira.TriggerData, arg4 moira.ContactData, arg5 int, arg6 bool) *moira.ScheduledNotification {
	ret := m.ctrl.Call(m, "ScheduleNotification", arg0, arg1, arg2, arg3, arg4, arg5, arg6)
	ret0, _ := ret[0].(*moira.ScheduledNotification)
	return ret0
}

// ScheduleNotification indicates an expected call of ScheduleNotification
func (mr *MockSchedulerMockRecorder) ScheduleNotification(arg0, arg1, arg2, arg3, arg4, arg5, arg6 interface{}) *gomock.Call {
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ScheduleNotification", reflect.TypeOf((*MockScheduler)(nil).ScheduleNotification), arg0, arg1, arg2, arg3, arg4, arg5, arg6)
}