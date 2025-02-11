// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/ecodeclub/ginx/session (interfaces: Session)
//
// Generated by this command:
//
//	mockgen -destination=internal/test/mocks/session.mock.go -package=mocks github.com/ecodeclub/ginx/session Session
//
// Package mocks is a generated GoMock package.
package mocks

import (
	context "context"
	reflect "reflect"

	ekit "github.com/ecodeclub/ekit"
	session "github.com/ecodeclub/ginx/session"
	gomock "go.uber.org/mock/gomock"
)

// MockSession is a mock of Session interface.
type MockSession struct {
	ctrl     *gomock.Controller
	recorder *MockSessionMockRecorder
}

// MockSessionMockRecorder is the mock recorder for MockSession.
type MockSessionMockRecorder struct {
	mock *MockSession
}

// NewMockSession creates a new mock instance.
func NewMockSession(ctrl *gomock.Controller) *MockSession {
	mock := &MockSession{ctrl: ctrl}
	mock.recorder = &MockSessionMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockSession) EXPECT() *MockSessionMockRecorder {
	return m.recorder
}

// Claims mocks base method.
func (m *MockSession) Claims() session.Claims {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Claims")
	ret0, _ := ret[0].(session.Claims)
	return ret0
}

// Claims indicates an expected call of Claims.
func (mr *MockSessionMockRecorder) Claims() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Claims", reflect.TypeOf((*MockSession)(nil).Claims))
}

// Del mocks base method.
func (m *MockSession) Del(arg0 context.Context, arg1 string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Del", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// Del indicates an expected call of Del.
func (mr *MockSessionMockRecorder) Del(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Del", reflect.TypeOf((*MockSession)(nil).Del), arg0, arg1)
}

// Destroy mocks base method.
func (m *MockSession) Destroy(arg0 context.Context) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Destroy", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// Destroy indicates an expected call of Destroy.
func (mr *MockSessionMockRecorder) Destroy(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Destroy", reflect.TypeOf((*MockSession)(nil).Destroy), arg0)
}

// Get mocks base method.
func (m *MockSession) Get(arg0 context.Context, arg1 string) ekit.AnyValue {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Get", arg0, arg1)
	ret0, _ := ret[0].(ekit.AnyValue)
	return ret0
}

// Get indicates an expected call of Get.
func (mr *MockSessionMockRecorder) Get(arg0, arg1 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Get", reflect.TypeOf((*MockSession)(nil).Get), arg0, arg1)
}

// Set mocks base method.
func (m *MockSession) Set(arg0 context.Context, arg1 string, arg2 any) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Set", arg0, arg1, arg2)
	ret0, _ := ret[0].(error)
	return ret0
}

// Set indicates an expected call of Set.
func (mr *MockSessionMockRecorder) Set(arg0, arg1, arg2 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Set", reflect.TypeOf((*MockSession)(nil).Set), arg0, arg1, arg2)
}
