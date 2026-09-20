// Copyright 2026 Matrix Origin
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0

package querycli

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"testing"

	"github.com/gogo/protobuf/proto"
)

func TestPythonUDFStatusWireRoundTrip(t *testing.T) {
	request := &pythonUDFStatusRequest{
		RequestID:                 7,
		CmdMethod:                 pythonUDFStatusCmd,
		GetPythonUdfStatusRequest: &pythonUDFStatusRequestBody{},
	}
	data, err := proto.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	var decodedRequest pythonUDFStatusRequest
	if err := proto.Unmarshal(data, &decodedRequest); err != nil {
		t.Fatal(err)
	}
	if decodedRequest.RequestID != request.RequestID || decodedRequest.CmdMethod != request.CmdMethod ||
		decodedRequest.GetPythonUdfStatusRequest == nil {
		t.Fatalf("request round trip = %#v", decodedRequest)
	}

	response := &pythonUDFStatusResponse{
		RequestID: request.RequestID,
		CmdMethod: pythonUDFStatusCmd,
		GetPythonUdfStatus: &pythonUDFStatusResponseBody{
			CNUUID: "cn-1", Language: "python", Ready: true, LeaseEpoch: 12,
			Modes: []string{"SCALAR", "VECTOR"}, NullPolicies: []string{"CALLED_ON_NULL_INPUT"},
		},
	}
	data, err = proto.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}
	var decodedResponse pythonUDFStatusResponse
	if err := proto.Unmarshal(data, &decodedResponse); err != nil {
		t.Fatal(err)
	}
	if decodedResponse.GetPythonUdfStatus == nil || decodedResponse.GetPythonUdfStatus.CNUUID != "cn-1" ||
		!decodedResponse.GetPythonUdfStatus.Ready || decodedResponse.GetPythonUdfStatus.LeaseEpoch != 12 ||
		len(decodedResponse.GetPythonUdfStatus.Modes) != 2 {
		t.Fatalf("response round trip = %#v", decodedResponse)
	}
	if err := validatePythonUDFStatusResponse(request.RequestID, &decodedResponse); err != nil {
		t.Fatalf("valid response rejected: %v", err)
	}
	decodedResponse.RequestID++
	if err := validatePythonUDFStatusResponse(request.RequestID, &decodedResponse); err == nil {
		t.Fatal("mismatched response request ID was accepted")
	}
	decodedResponse.RequestID = request.RequestID
	decodedResponse.CmdMethod++
	if err := validatePythonUDFStatusResponse(request.RequestID, &decodedResponse); err == nil {
		t.Fatal("mismatched response command was accepted")
	}
}

func TestPythonUDFStatusRPCMessageBound(t *testing.T) {
	const maxAnnotationBytes = 64 << 10
	if maxPythonStatusRPCBytes <= maxAnnotationBytes {
		t.Fatalf("RPC bound %d must include the bounded annotation payload and protocol envelope", maxPythonStatusRPCBytes)
	}
	if maxPythonStatusRPCBytes > 1<<20 {
		t.Fatalf("RPC bound %d is too large for a control-plane status message", maxPythonStatusRPCBytes)
	}
}

// Literal producer bytes from MO's current query.Response, not a round trip
// through this consumer's own struct tags.
func TestPythonStatusMOProducerFixture(t *testing.T) {
	raw, err := os.ReadFile("testdata/python_status_wire.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct{ Class, Reason, Wire string }
	if err := json.Unmarshal(raw, &rows); err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		t.Run(row.Reason, func(t *testing.T) {
			wire, err := base64.StdEncoding.DecodeString(row.Wire)
			if err != nil {
				t.Fatal(err)
			}
			var response pythonUDFStatusResponse
			if err := response.Unmarshal(wire); err != nil {
				t.Fatal(err)
			}
			if err := validatePythonUDFStatusResponse(7, &response); err != nil {
				t.Fatal(err)
			}
			body := response.GetPythonUdfStatus
			if body == nil || body.ErrorClass != row.Class || body.Reason != row.Reason || body.Ready != (row.Class == "") {
				t.Fatalf("lost runtime contract: %#v", body)
			}
		})
	}
}
