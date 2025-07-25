// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/KirkDiggler/rpg-toolkit/mechanics/features (interfaces: Feature)
//
// Generated by this command:
//
//	mockgen -destination=mock/mock_feature.go -package=mock github.com/KirkDiggler/rpg-toolkit/mechanics/features Feature
//

// Package mock is a generated GoMock package.
package mock

import (
	reflect "reflect"

	gomock "go.uber.org/mock/gomock"

	core "github.com/KirkDiggler/rpg-toolkit/core"
	events "github.com/KirkDiggler/rpg-toolkit/events"
	features "github.com/KirkDiggler/rpg-toolkit/mechanics/features"
	resources "github.com/KirkDiggler/rpg-toolkit/mechanics/resources"
)

// MockFeature is a mock of Feature interface.
type MockFeature struct {
	ctrl     *gomock.Controller
	recorder *MockFeatureMockRecorder
	isgomock struct{}
}

// MockFeatureMockRecorder is the mock recorder for MockFeature.
type MockFeatureMockRecorder struct {
	mock *MockFeature
}

// NewMockFeature creates a new mock instance.
func NewMockFeature(ctrl *gomock.Controller) *MockFeature {
	mock := &MockFeature{ctrl: ctrl}
	mock.recorder = &MockFeatureMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockFeature) EXPECT() *MockFeatureMockRecorder {
	return m.recorder
}

// Activate mocks base method.
func (m *MockFeature) Activate(entity core.Entity, bus events.EventBus) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Activate", entity, bus)
	ret0, _ := ret[0].(error)
	return ret0
}

// Activate indicates an expected call of Activate.
func (mr *MockFeatureMockRecorder) Activate(entity, bus any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Activate", reflect.TypeOf((*MockFeature)(nil).Activate), entity, bus)
}

// CanTrigger mocks base method.
func (m *MockFeature) CanTrigger(event events.Event) bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CanTrigger", event)
	ret0, _ := ret[0].(bool)
	return ret0
}

// CanTrigger indicates an expected call of CanTrigger.
func (mr *MockFeatureMockRecorder) CanTrigger(event any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CanTrigger", reflect.TypeOf((*MockFeature)(nil).CanTrigger), event)
}

// Deactivate mocks base method.
func (m *MockFeature) Deactivate(entity core.Entity, bus events.EventBus) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Deactivate", entity, bus)
	ret0, _ := ret[0].(error)
	return ret0
}

// Deactivate indicates an expected call of Deactivate.
func (mr *MockFeatureMockRecorder) Deactivate(entity, bus any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Deactivate", reflect.TypeOf((*MockFeature)(nil).Deactivate), entity, bus)
}

// Description mocks base method.
func (m *MockFeature) Description() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Description")
	ret0, _ := ret[0].(string)
	return ret0
}

// Description indicates an expected call of Description.
func (mr *MockFeatureMockRecorder) Description() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Description", reflect.TypeOf((*MockFeature)(nil).Description))
}

// GetEventListeners mocks base method.
func (m *MockFeature) GetEventListeners() []features.EventListener {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetEventListeners")
	ret0, _ := ret[0].([]features.EventListener)
	return ret0
}

// GetEventListeners indicates an expected call of GetEventListeners.
func (mr *MockFeatureMockRecorder) GetEventListeners() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetEventListeners", reflect.TypeOf((*MockFeature)(nil).GetEventListeners))
}

// GetModifiers mocks base method.
func (m *MockFeature) GetModifiers() []events.Modifier {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetModifiers")
	ret0, _ := ret[0].([]events.Modifier)
	return ret0
}

// GetModifiers indicates an expected call of GetModifiers.
func (mr *MockFeatureMockRecorder) GetModifiers() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetModifiers", reflect.TypeOf((*MockFeature)(nil).GetModifiers))
}

// GetPrerequisites mocks base method.
func (m *MockFeature) GetPrerequisites() []string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetPrerequisites")
	ret0, _ := ret[0].([]string)
	return ret0
}

// GetPrerequisites indicates an expected call of GetPrerequisites.
func (mr *MockFeatureMockRecorder) GetPrerequisites() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetPrerequisites", reflect.TypeOf((*MockFeature)(nil).GetPrerequisites))
}

// GetProficiencies mocks base method.
func (m *MockFeature) GetProficiencies() []string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetProficiencies")
	ret0, _ := ret[0].([]string)
	return ret0
}

// GetProficiencies indicates an expected call of GetProficiencies.
func (mr *MockFeatureMockRecorder) GetProficiencies() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetProficiencies", reflect.TypeOf((*MockFeature)(nil).GetProficiencies))
}

// GetResources mocks base method.
func (m *MockFeature) GetResources() []resources.Resource {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetResources")
	ret0, _ := ret[0].([]resources.Resource)
	return ret0
}

// GetResources indicates an expected call of GetResources.
func (mr *MockFeatureMockRecorder) GetResources() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetResources", reflect.TypeOf((*MockFeature)(nil).GetResources))
}

// GetTiming mocks base method.
func (m *MockFeature) GetTiming() features.FeatureTiming {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetTiming")
	ret0, _ := ret[0].(features.FeatureTiming)
	return ret0
}

// GetTiming indicates an expected call of GetTiming.
func (mr *MockFeatureMockRecorder) GetTiming() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetTiming", reflect.TypeOf((*MockFeature)(nil).GetTiming))
}

// HasPrerequisites mocks base method.
func (m *MockFeature) HasPrerequisites() bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "HasPrerequisites")
	ret0, _ := ret[0].(bool)
	return ret0
}

// HasPrerequisites indicates an expected call of HasPrerequisites.
func (mr *MockFeatureMockRecorder) HasPrerequisites() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "HasPrerequisites", reflect.TypeOf((*MockFeature)(nil).HasPrerequisites))
}

// IsActive mocks base method.
func (m *MockFeature) IsActive(entity core.Entity) bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "IsActive", entity)
	ret0, _ := ret[0].(bool)
	return ret0
}

// IsActive indicates an expected call of IsActive.
func (mr *MockFeatureMockRecorder) IsActive(entity any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IsActive", reflect.TypeOf((*MockFeature)(nil).IsActive), entity)
}

// IsPassive mocks base method.
func (m *MockFeature) IsPassive() bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "IsPassive")
	ret0, _ := ret[0].(bool)
	return ret0
}

// IsPassive indicates an expected call of IsPassive.
func (mr *MockFeatureMockRecorder) IsPassive() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IsPassive", reflect.TypeOf((*MockFeature)(nil).IsPassive))
}

// Key mocks base method.
func (m *MockFeature) Key() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Key")
	ret0, _ := ret[0].(string)
	return ret0
}

// Key indicates an expected call of Key.
func (mr *MockFeatureMockRecorder) Key() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Key", reflect.TypeOf((*MockFeature)(nil).Key))
}

// Level mocks base method.
func (m *MockFeature) Level() int {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Level")
	ret0, _ := ret[0].(int)
	return ret0
}

// Level indicates an expected call of Level.
func (mr *MockFeatureMockRecorder) Level() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Level", reflect.TypeOf((*MockFeature)(nil).Level))
}

// MeetsPrerequisites mocks base method.
func (m *MockFeature) MeetsPrerequisites(entity core.Entity) bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "MeetsPrerequisites", entity)
	ret0, _ := ret[0].(bool)
	return ret0
}

// MeetsPrerequisites indicates an expected call of MeetsPrerequisites.
func (mr *MockFeatureMockRecorder) MeetsPrerequisites(entity any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "MeetsPrerequisites", reflect.TypeOf((*MockFeature)(nil).MeetsPrerequisites), entity)
}

// Name mocks base method.
func (m *MockFeature) Name() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Name")
	ret0, _ := ret[0].(string)
	return ret0
}

// Name indicates an expected call of Name.
func (mr *MockFeatureMockRecorder) Name() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Name", reflect.TypeOf((*MockFeature)(nil).Name))
}

// Source mocks base method.
func (m *MockFeature) Source() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Source")
	ret0, _ := ret[0].(string)
	return ret0
}

// Source indicates an expected call of Source.
func (mr *MockFeatureMockRecorder) Source() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Source", reflect.TypeOf((*MockFeature)(nil).Source))
}

// TriggerFeature mocks base method.
func (m *MockFeature) TriggerFeature(entity core.Entity, event events.Event) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "TriggerFeature", entity, event)
	ret0, _ := ret[0].(error)
	return ret0
}

// TriggerFeature indicates an expected call of TriggerFeature.
func (mr *MockFeatureMockRecorder) TriggerFeature(entity, event any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "TriggerFeature", reflect.TypeOf((*MockFeature)(nil).TriggerFeature), entity, event)
}

// Type mocks base method.
func (m *MockFeature) Type() features.FeatureType {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Type")
	ret0, _ := ret[0].(features.FeatureType)
	return ret0
}

// Type indicates an expected call of Type.
func (mr *MockFeatureMockRecorder) Type() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Type", reflect.TypeOf((*MockFeature)(nil).Type))
}
