// Code generated by counterfeiter. DO NOT EDIT.
package routingfakes

import (
	"context"
	"sync"

	"github.com/voicekit/voicekit-server/pkg/routing"
	"github.com/voicekit/protocol/voicekit"
	"github.com/voicekit/psrpc"
)

type FakeRoomManagerClient struct {
	CloseStub        func()
	closeMutex       sync.RWMutex
	closeArgsForCall []struct {
	}
	CreateRoomStub        func(context.Context, voicekit.NodeID, *voicekit.CreateRoomRequest, ...psrpc.RequestOption) (*voicekit.Room, error)
	createRoomMutex       sync.RWMutex
	createRoomArgsForCall []struct {
		arg1 context.Context
		arg2 voicekit.NodeID
		arg3 *voicekit.CreateRoomRequest
		arg4 []psrpc.RequestOption
	}
	createRoomReturns struct {
		result1 *voicekit.Room
		result2 error
	}
	createRoomReturnsOnCall map[int]struct {
		result1 *voicekit.Room
		result2 error
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *FakeRoomManagerClient) Close() {
	fake.closeMutex.Lock()
	fake.closeArgsForCall = append(fake.closeArgsForCall, struct {
	}{})
	stub := fake.CloseStub
	fake.recordInvocation("Close", []interface{}{})
	fake.closeMutex.Unlock()
	if stub != nil {
		fake.CloseStub()
	}
}

func (fake *FakeRoomManagerClient) CloseCallCount() int {
	fake.closeMutex.RLock()
	defer fake.closeMutex.RUnlock()
	return len(fake.closeArgsForCall)
}

func (fake *FakeRoomManagerClient) CloseCalls(stub func()) {
	fake.closeMutex.Lock()
	defer fake.closeMutex.Unlock()
	fake.CloseStub = stub
}

func (fake *FakeRoomManagerClient) CreateRoom(arg1 context.Context, arg2 voicekit.NodeID, arg3 *voicekit.CreateRoomRequest, arg4 ...psrpc.RequestOption) (*voicekit.Room, error) {
	fake.createRoomMutex.Lock()
	ret, specificReturn := fake.createRoomReturnsOnCall[len(fake.createRoomArgsForCall)]
	fake.createRoomArgsForCall = append(fake.createRoomArgsForCall, struct {
		arg1 context.Context
		arg2 voicekit.NodeID
		arg3 *voicekit.CreateRoomRequest
		arg4 []psrpc.RequestOption
	}{arg1, arg2, arg3, arg4})
	stub := fake.CreateRoomStub
	fakeReturns := fake.createRoomReturns
	fake.recordInvocation("CreateRoom", []interface{}{arg1, arg2, arg3, arg4})
	fake.createRoomMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2, arg3, arg4...)
	}
	if specificReturn {
		return ret.result1, ret.result2
	}
	return fakeReturns.result1, fakeReturns.result2
}

func (fake *FakeRoomManagerClient) CreateRoomCallCount() int {
	fake.createRoomMutex.RLock()
	defer fake.createRoomMutex.RUnlock()
	return len(fake.createRoomArgsForCall)
}

func (fake *FakeRoomManagerClient) CreateRoomCalls(stub func(context.Context, voicekit.NodeID, *voicekit.CreateRoomRequest, ...psrpc.RequestOption) (*voicekit.Room, error)) {
	fake.createRoomMutex.Lock()
	defer fake.createRoomMutex.Unlock()
	fake.CreateRoomStub = stub
}

func (fake *FakeRoomManagerClient) CreateRoomArgsForCall(i int) (context.Context, voicekit.NodeID, *voicekit.CreateRoomRequest, []psrpc.RequestOption) {
	fake.createRoomMutex.RLock()
	defer fake.createRoomMutex.RUnlock()
	argsForCall := fake.createRoomArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2, argsForCall.arg3, argsForCall.arg4
}

func (fake *FakeRoomManagerClient) CreateRoomReturns(result1 *voicekit.Room, result2 error) {
	fake.createRoomMutex.Lock()
	defer fake.createRoomMutex.Unlock()
	fake.CreateRoomStub = nil
	fake.createRoomReturns = struct {
		result1 *voicekit.Room
		result2 error
	}{result1, result2}
}

func (fake *FakeRoomManagerClient) CreateRoomReturnsOnCall(i int, result1 *voicekit.Room, result2 error) {
	fake.createRoomMutex.Lock()
	defer fake.createRoomMutex.Unlock()
	fake.CreateRoomStub = nil
	if fake.createRoomReturnsOnCall == nil {
		fake.createRoomReturnsOnCall = make(map[int]struct {
			result1 *voicekit.Room
			result2 error
		})
	}
	fake.createRoomReturnsOnCall[i] = struct {
		result1 *voicekit.Room
		result2 error
	}{result1, result2}
}

func (fake *FakeRoomManagerClient) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	fake.closeMutex.RLock()
	defer fake.closeMutex.RUnlock()
	fake.createRoomMutex.RLock()
	defer fake.createRoomMutex.RUnlock()
	copiedInvocations := map[string][][]interface{}{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *FakeRoomManagerClient) recordInvocation(key string, args []interface{}) {
	fake.invocationsMutex.Lock()
	defer fake.invocationsMutex.Unlock()
	if fake.invocations == nil {
		fake.invocations = map[string][][]interface{}{}
	}
	if fake.invocations[key] == nil {
		fake.invocations[key] = [][]interface{}{}
	}
	fake.invocations[key] = append(fake.invocations[key], args)
}

var _ routing.RoomManagerClient = new(FakeRoomManagerClient)
