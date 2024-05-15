// Code generated by MockGen. DO NOT EDIT.
// Source: ./producer.go
//
// Generated by this command:
//
//	mockgen -source=./producer.go -destination=./mocks/producer.mock.go -package=evtmocks -typed SyncEventProducer
//
// Package evtmocks is a generated GoMock package.
package evtmocks

import (
	context "context"
	reflect "reflect"

	event "github.com/ecodeclub/webook/internal/cases/internal/event"
	gomock "go.uber.org/mock/gomock"
)

// MockSyncEventProducer is a mock of SyncEventProducer interface.
type MockSyncEventProducer struct {
	ctrl     *gomock.Controller
	recorder *MockSyncEventProducerMockRecorder
}

// MockSyncEventProducerMockRecorder is the mock recorder for MockSyncEventProducer.
type MockSyncEventProducerMockRecorder struct {
	mock *MockSyncEventProducer
}

// NewMockSyncEventProducer creates a new mock instance.
func NewMockSyncEventProducer(ctrl *gomock.Controller) *MockSyncEventProducer {
	mock := &MockSyncEventProducer{ctrl: ctrl}
	mock.recorder = &MockSyncEventProducerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockSyncEventProducer) EXPECT() *MockSyncEventProducerMockRecorder {
	return m.recorder
}

// Produce mocks base method.
func (m *MockSyncEventProducer) Produce(ctx context.Context, evt event.CaseEvent) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Produce", ctx, evt)
	ret0, _ := ret[0].(error)
	return ret0
}

// Produce indicates an expected call of Produce.
func (mr *MockSyncEventProducerMockRecorder) Produce(ctx, evt any) *SyncEventProducerProduceCall {
	mr.mock.ctrl.T.Helper()
	call := mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Produce", reflect.TypeOf((*MockSyncEventProducer)(nil).Produce), ctx, evt)
	return &SyncEventProducerProduceCall{Call: call}
}

// SyncEventProducerProduceCall wrap *gomock.Call
type SyncEventProducerProduceCall struct {
	*gomock.Call
}

// Return rewrite *gomock.Call.Return
func (c *SyncEventProducerProduceCall) Return(arg0 error) *SyncEventProducerProduceCall {
	c.Call = c.Call.Return(arg0)
	return c
}

// Do rewrite *gomock.Call.Do
func (c *SyncEventProducerProduceCall) Do(f func(context.Context, event.CaseEvent) error) *SyncEventProducerProduceCall {
	c.Call = c.Call.Do(f)
	return c
}

// DoAndReturn rewrite *gomock.Call.DoAndReturn
func (c *SyncEventProducerProduceCall) DoAndReturn(f func(context.Context, event.CaseEvent) error) *SyncEventProducerProduceCall {
	c.Call = c.Call.DoAndReturn(f)
	return c
}
