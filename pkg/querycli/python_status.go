// Copyright 2026 Matrix Origin
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package querycli

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/go-errors/errors"
	"github.com/gogo/protobuf/proto"
	"github.com/matrixorigin/matrixone-operator/pkg/metric"
)

// pythonUDFStatusCmd is deliberately kept in sync with query.proto in the MO
// repository. The Operator currently depends on a released MO module whose
// generated query bindings predate this control-plane method, so this small
// wire adapter lets old and new components exchange the canonical protobuf
// fields without inventing a second status protocol. It can be removed when
// the next MO module release includes the generated binding.
const (
	pythonUDFStatusCmd      int32 = 41
	maxPythonStatusRPCBytes       = 128 << 10
)

type pythonUDFStatusRequest struct {
	RequestID                 uint64                      `protobuf:"varint,1,opt,name=RequestID,proto3" json:"RequestID,omitempty"`
	CmdMethod                 int32                       `protobuf:"varint,2,opt,name=CmdMethod,proto3" json:"CmdMethod,omitempty"`
	GetPythonUdfStatusRequest *pythonUDFStatusRequestBody `protobuf:"bytes,43,opt,name=GetPythonUdfStatusRequest,proto3" json:"GetPythonUdfStatusRequest,omitempty"`
}

type pythonUDFStatusRequestBody struct{}

type pythonUDFStatusResponse struct {
	RequestID          uint64                       `protobuf:"varint,1,opt,name=RequestID,proto3" json:"RequestID,omitempty"`
	CmdMethod          int32                        `protobuf:"varint,2,opt,name=CmdMethod,proto3" json:"CmdMethod,omitempty"`
	Error              []byte                       `protobuf:"bytes,3,opt,name=Error,proto3" json:"Error,omitempty"`
	GetPythonUdfStatus *pythonUDFStatusResponseBody `protobuf:"bytes,44,opt,name=GetPythonUdfStatus,proto3" json:"GetPythonUdfStatus,omitempty"`
}

type pythonUDFStatusResponseBody struct {
	CNUUID                     string   `protobuf:"bytes,1,opt,name=CNUUID,proto3" json:"CNUUID,omitempty"`
	Language                   string   `protobuf:"bytes,2,opt,name=Language,proto3" json:"Language,omitempty"`
	Enabled                    bool     `protobuf:"varint,3,opt,name=Enabled,proto3" json:"Enabled,omitempty"`
	AllowUnisolated            bool     `protobuf:"varint,4,opt,name=AllowUnisolated,proto3" json:"AllowUnisolated,omitempty"`
	Ready                      bool     `protobuf:"varint,5,opt,name=Ready,proto3" json:"Ready,omitempty"`
	ErrorClass                 string   `protobuf:"bytes,6,opt,name=ErrorClass,proto3" json:"ErrorClass,omitempty"`
	Reason                     string   `protobuf:"bytes,7,opt,name=Reason,proto3" json:"Reason,omitempty"`
	ProtocolVersion            int32    `protobuf:"varint,8,opt,name=ProtocolVersion,proto3" json:"ProtocolVersion,omitempty"`
	ABIContract                string   `protobuf:"bytes,9,opt,name=ABIContract,proto3" json:"ABIContract,omitempty"`
	AdapterVersion             string   `protobuf:"bytes,10,opt,name=AdapterVersion,proto3" json:"AdapterVersion,omitempty"`
	SDKVersion                 string   `protobuf:"bytes,11,opt,name=SDKVersion,proto3" json:"SDKVersion,omitempty"`
	DefinitionSchemaVersion    int32    `protobuf:"varint,12,opt,name=DefinitionSchemaVersion,proto3" json:"DefinitionSchemaVersion,omitempty"`
	PlanContractVersion        int32    `protobuf:"varint,13,opt,name=PlanContractVersion,proto3" json:"PlanContractVersion,omitempty"`
	TypeDescriptorContract     string   `protobuf:"bytes,14,opt,name=TypeDescriptorContract,proto3" json:"TypeDescriptorContract,omitempty"`
	TimezoneDatabaseVersion    string   `protobuf:"bytes,15,opt,name=TimezoneDatabaseVersion,proto3" json:"TimezoneDatabaseVersion,omitempty"`
	WindowBatches              int32    `protobuf:"varint,16,opt,name=WindowBatches,proto3" json:"WindowBatches,omitempty"`
	CumulativeAck              bool     `protobuf:"varint,17,opt,name=CumulativeAck,proto3" json:"CumulativeAck,omitempty"`
	MaxExecutionFrameBytes     int64    `protobuf:"varint,18,opt,name=MaxExecutionFrameBytes,proto3" json:"MaxExecutionFrameBytes,omitempty"`
	MaxHandlerProcesses        int32    `protobuf:"varint,19,opt,name=MaxHandlerProcesses,proto3" json:"MaxHandlerProcesses,omitempty"`
	MaxAccountHandlerProcesses int32    `protobuf:"varint,20,opt,name=MaxAccountHandlerProcesses,proto3" json:"MaxAccountHandlerProcesses,omitempty"`
	MaxOwnerHandlerProcesses   int32    `protobuf:"varint,21,opt,name=MaxOwnerHandlerProcesses,proto3" json:"MaxOwnerHandlerProcesses,omitempty"`
	LeaseEpoch                 uint64   `protobuf:"varint,22,opt,name=LeaseEpoch,proto3" json:"LeaseEpoch,omitempty"`
	Modes                      []string `protobuf:"bytes,23,rep,name=Modes,proto3" json:"Modes,omitempty"`
	NullPolicies               []string `protobuf:"bytes,24,rep,name=NullPolicies,proto3" json:"NullPolicies,omitempty"`
}

// The aliases intentionally do not implement Unmarshal. gogo/protobuf then
// uses its reflective decoder for these locally defined wire structs, while
// the morpc-facing types below can still expose the required Unmarshal method
// without recursing through proto.Unmarshal.
type pythonUDFStatusRequestWire pythonUDFStatusRequest
type pythonUDFStatusResponseWire pythonUDFStatusResponse

func (m *pythonUDFStatusRequestWire) Reset()         { *m = pythonUDFStatusRequestWire{} }
func (m *pythonUDFStatusRequestWire) String() string { return proto.CompactTextString(m) }
func (*pythonUDFStatusRequestWire) ProtoMessage()    {}

func (m *pythonUDFStatusResponseWire) Reset()         { *m = pythonUDFStatusResponseWire{} }
func (m *pythonUDFStatusResponseWire) String() string { return proto.CompactTextString(m) }
func (*pythonUDFStatusResponseWire) ProtoMessage()    {}

// PythonUDFStatus is the Operator-facing bounded status view returned by one
// CN. The fields mirror the MO query contract and remain free of user data.
type PythonUDFStatus struct {
	CNUUID                     string
	Language                   string
	Enabled                    bool
	AllowUnisolated            bool
	Ready                      bool
	ErrorClass                 string
	Reason                     string
	ProtocolVersion            int32
	ABIContract                string
	AdapterVersion             string
	SDKVersion                 string
	DefinitionSchemaVersion    int32
	PlanContractVersion        int32
	TypeDescriptorContract     string
	TimezoneDatabaseVersion    string
	WindowBatches              int32
	CumulativeAck              bool
	MaxExecutionFrameBytes     int64
	MaxHandlerProcesses        int32
	MaxAccountHandlerProcesses int32
	MaxOwnerHandlerProcesses   int32
	LeaseEpoch                 uint64
	Modes                      []string
	NullPolicies               []string
}

func (m *pythonUDFStatusRequestBody) Reset()         { *m = pythonUDFStatusRequestBody{} }
func (m *pythonUDFStatusRequestBody) String() string { return proto.CompactTextString(m) }
func (*pythonUDFStatusRequestBody) ProtoMessage()    {}

func (m *pythonUDFStatusRequest) Reset()          { *m = pythonUDFStatusRequest{} }
func (m *pythonUDFStatusRequest) String() string  { return proto.CompactTextString(m) }
func (*pythonUDFStatusRequest) ProtoMessage()     {}
func (m *pythonUDFStatusRequest) SetID(id uint64) { m.RequestID = id }
func (m *pythonUDFStatusRequest) GetID() uint64   { return m.RequestID }
func (m *pythonUDFStatusRequest) DebugString() string {
	return fmt.Sprintf("%d: GetPythonUdfStatus/", m.RequestID)
}

func (m *pythonUDFStatusResponseBody) Reset()         { *m = pythonUDFStatusResponseBody{} }
func (m *pythonUDFStatusResponseBody) String() string { return proto.CompactTextString(m) }
func (*pythonUDFStatusResponseBody) ProtoMessage()    {}

func (m *pythonUDFStatusResponse) Reset()          { *m = pythonUDFStatusResponse{} }
func (m *pythonUDFStatusResponse) String() string  { return proto.CompactTextString(m) }
func (*pythonUDFStatusResponse) ProtoMessage()     {}
func (m *pythonUDFStatusResponse) SetID(id uint64) { m.RequestID = id }
func (m *pythonUDFStatusResponse) GetID() uint64   { return m.RequestID }
func (m *pythonUDFStatusResponse) DebugString() string {
	return fmt.Sprintf("%d: GetPythonUdfStatus/", m.RequestID)
}

// The morpc Message interface uses the generated protobuf size/marshal
// contract. Reflection is used only by this compatibility adapter; all data
// fields and field numbers are still the canonical generated-proto contract.
func marshalPythonStatus(data []byte, message proto.Message) (int, error) {
	b, err := proto.Marshal(message)
	if err != nil {
		return 0, err
	}
	if len(data) < len(b) {
		return 0, io.ErrShortBuffer
	}
	copy(data, b)
	return len(b), nil
}

func (m *pythonUDFStatusRequest) ProtoSize() int { return proto.Size(m) }
func (m *pythonUDFStatusRequest) MarshalTo(data []byte) (int, error) {
	return marshalPythonStatus(data, m)
}
func (m *pythonUDFStatusRequest) Unmarshal(data []byte) error {
	return proto.Unmarshal(data, (*pythonUDFStatusRequestWire)(m))
}

func (m *pythonUDFStatusResponse) ProtoSize() int { return proto.Size(m) }
func (m *pythonUDFStatusResponse) MarshalTo(data []byte) (int, error) {
	return marshalPythonStatus(data, m)
}
func (m *pythonUDFStatusResponse) Unmarshal(data []byte) error {
	return proto.Unmarshal(data, (*pythonUDFStatusResponseWire)(m))
}

func validatePythonUDFStatusResponse(requestID uint64, response *pythonUDFStatusResponse) error {
	if response == nil {
		return errors.New("CN returned an empty Python UDF status response")
	}
	if response.RequestID != requestID {
		return errors.Errorf("Python UDF status response request ID %d does not match request %d", response.RequestID, requestID)
	}
	if response.CmdMethod != pythonUDFStatusCmd {
		return errors.Errorf("Python UDF status response command %d does not match command %d", response.CmdMethod, pythonUDFStatusCmd)
	}
	return nil
}

func (c *Client) GetPythonUdfStatus(ctx context.Context, address string) (*PythonUDFStatus, error) {
	if c == nil || c.status == nil {
		return nil, errors.New("python UDF status client is not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	queryCtx := ctx
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		queryCtx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	start := time.Now()
	var err error
	defer func() {
		result := "ok"
		if err != nil {
			result = "error"
		}
		metric.CnRPCDuration.WithLabelValues("GetPythonUdfStatus", address, result).Observe(time.Since(start).Seconds())
	}()

	request := &pythonUDFStatusRequest{
		CmdMethod:                 pythonUDFStatusCmd,
		GetPythonUdfStatusRequest: &pythonUDFStatusRequestBody{},
	}
	future, sendErr := c.status.Send(queryCtx, address, request)
	if sendErr != nil {
		err = errors.WrapPrefix(sendErr, "error send Python UDF status request", 0)
		return nil, err
	}
	defer future.Close()
	message, getErr := future.Get()
	if getErr != nil {
		err = errors.WrapPrefix(getErr, "error get Python UDF status response", 0)
		return nil, err
	}
	if message == nil {
		err = errors.New("CN returned an empty Python UDF status response")
		return nil, err
	}
	response, ok := message.(*pythonUDFStatusResponse)
	if !ok {
		err = errors.Errorf("message is not a Python UDF status response: %s", message.DebugString())
		return nil, err
	}
	if err = validatePythonUDFStatusResponse(request.RequestID, response); err != nil {
		return nil, err
	}
	if len(response.Error) != 0 {
		err = errors.New("CN rejected Python UDF status request")
		return nil, err
	}
	if response.GetPythonUdfStatus == nil {
		err = errors.New("CN returned an empty Python UDF status")
		return nil, err
	}
	body := response.GetPythonUdfStatus
	return &PythonUDFStatus{
		CNUUID: body.CNUUID, Language: body.Language, Enabled: body.Enabled,
		AllowUnisolated: body.AllowUnisolated, Ready: body.Ready,
		ErrorClass: body.ErrorClass, Reason: body.Reason,
		ProtocolVersion: body.ProtocolVersion, ABIContract: body.ABIContract,
		AdapterVersion: body.AdapterVersion, SDKVersion: body.SDKVersion,
		DefinitionSchemaVersion: body.DefinitionSchemaVersion,
		PlanContractVersion:     body.PlanContractVersion,
		TypeDescriptorContract:  body.TypeDescriptorContract,
		TimezoneDatabaseVersion: body.TimezoneDatabaseVersion,
		WindowBatches:           body.WindowBatches, CumulativeAck: body.CumulativeAck,
		MaxExecutionFrameBytes:     body.MaxExecutionFrameBytes,
		MaxHandlerProcesses:        body.MaxHandlerProcesses,
		MaxAccountHandlerProcesses: body.MaxAccountHandlerProcesses,
		MaxOwnerHandlerProcesses:   body.MaxOwnerHandlerProcesses,
		LeaseEpoch:                 body.LeaseEpoch,
		Modes:                      append([]string(nil), body.Modes...),
		NullPolicies:               append([]string(nil), body.NullPolicies...),
	}, nil
}
