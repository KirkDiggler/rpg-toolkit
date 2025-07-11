// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/KirkDiggler/rpg-toolkit/mechanics/proficiency (interfaces: Proficiency)
//
// Generated by this command:
//
//	mockgen -destination=mock/mock_proficiency.go -package=mock github.com/KirkDiggler/rpg-toolkit/mechanics/proficiency Proficiency
//

// Package mock is a generated GoMock package.
package mock

import (
	reflect "reflect"

	gomock "go.uber.org/mock/gomock"

	core "github.com/KirkDiggler/rpg-toolkit/core"
	events "github.com/KirkDiggler/rpg-toolkit/events"
)

// MockProficiency is a mock of Proficiency interface.
type MockProficiency struct {
	ctrl     *gomock.Controller
	recorder *MockProficiencyMockRecorder
	isgomock struct{}
}

// MockProficiencyMockRecorder is the mock recorder for MockProficiency.
type MockProficiencyMockRecorder struct {
	mock *MockProficiency
}

// NewMockProficiency creates a new mock instance.
func NewMockProficiency(ctrl *gomock.Controller) *MockProficiency {
	mock := &MockProficiency{ctrl: ctrl}
	mock.recorder = &MockProficiencyMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockProficiency) EXPECT() *MockProficiencyMockRecorder {
	return m.recorder
}

// Apply mocks base method.
func (m *MockProficiency) Apply(bus events.EventBus) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Apply", bus)
	ret0, _ := ret[0].(error)
	return ret0
}

// Apply indicates an expected call of Apply.
func (mr *MockProficiencyMockRecorder) Apply(bus any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Apply", reflect.TypeOf((*MockProficiency)(nil).Apply), bus)
}

// GetID mocks base method.
func (m *MockProficiency) GetID() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetID")
	ret0, _ := ret[0].(string)
	return ret0
}

// GetID indicates an expected call of GetID.
func (mr *MockProficiencyMockRecorder) GetID() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetID", reflect.TypeOf((*MockProficiency)(nil).GetID))
}

// GetType mocks base method.
func (m *MockProficiency) GetType() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetType")
	ret0, _ := ret[0].(string)
	return ret0
}

// GetType indicates an expected call of GetType.
func (mr *MockProficiencyMockRecorder) GetType() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetType", reflect.TypeOf((*MockProficiency)(nil).GetType))
}

// IsActive mocks base method.
func (m *MockProficiency) IsActive() bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "IsActive")
	ret0, _ := ret[0].(bool)
	return ret0
}

// IsActive indicates an expected call of IsActive.
func (mr *MockProficiencyMockRecorder) IsActive() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IsActive", reflect.TypeOf((*MockProficiency)(nil).IsActive))
}

// Owner mocks base method.
func (m *MockProficiency) Owner() core.Entity {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Owner")
	ret0, _ := ret[0].(core.Entity)
	return ret0
}

// Owner indicates an expected call of Owner.
func (mr *MockProficiencyMockRecorder) Owner() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Owner", reflect.TypeOf((*MockProficiency)(nil).Owner))
}

// Remove mocks base method.
func (m *MockProficiency) Remove(bus events.EventBus) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Remove", bus)
	ret0, _ := ret[0].(error)
	return ret0
}

// Remove indicates an expected call of Remove.
func (mr *MockProficiencyMockRecorder) Remove(bus any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Remove", reflect.TypeOf((*MockProficiency)(nil).Remove), bus)
}

// Source mocks base method.
func (m *MockProficiency) Source() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Source")
	ret0, _ := ret[0].(string)
	return ret0
}

// Source indicates an expected call of Source.
func (mr *MockProficiencyMockRecorder) Source() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Source", reflect.TypeOf((*MockProficiency)(nil).Source))
}

// Subject mocks base method.
func (m *MockProficiency) Subject() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Subject")
	ret0, _ := ret[0].(string)
	return ret0
}

// Subject indicates an expected call of Subject.
func (mr *MockProficiencyMockRecorder) Subject() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Subject", reflect.TypeOf((*MockProficiency)(nil).Subject))
}
