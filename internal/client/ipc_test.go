package client

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/ppablomunoz/noports/internal/ipc"
)

func TestSendWritesRequest(t *testing.T) {
	var buf bytes.Buffer
	want := &ipc.Request{Command: ipc.CmdAliasAdd, Hostname: "api.localhost", LocalPort: 3000, PID: 123, WrapperPID: 456}
	if err := Send(json.NewEncoder(&buf), want); err != nil {
		t.Fatalf("Send: %v", err)
	}
	var got ipc.Request
	if err := json.NewDecoder(&buf).Decode(&got); err != nil {
		t.Fatalf("decode Send output: %v", err)
	}
	if got != *want {
		t.Fatalf("Send wrote %+v, want %+v", got, *want)
	}
}

func TestReceiveOk(t *testing.T) {
	payload, _ := json.Marshal(&ipc.Response{OK: true})
	var res ipc.Response
	if err := Receive(json.NewDecoder(bytes.NewReader(payload)), &res); err != nil {
		t.Fatalf("Receive(ok response): %v", err)
	}
}

func TestReceiveError(t *testing.T) {
	payload, _ := json.Marshal(&ipc.Response{OK: false, Error: "nope"})
	var res ipc.Response
	if err := Receive(json.NewDecoder(bytes.NewReader(payload)), &res); err == nil {
		t.Fatal("Receive(failed response) = nil error, want error")
	}
}

func TestDecodeData(t *testing.T) {
	t.Run("list", func(t *testing.T) {
		resData := map[string]any{
			"routes": []any{map[string]any{"hostname": "api.localhost", "port": float64(3000), "PID": float64(-1)}},
		}
		var data ipc.DataResponseList
		if err := DecodeData(&resData, &data); err != nil {
			t.Fatalf("DecodeData: %v", err)
		}
		if len(data.Routes) != 1 || data.Routes[0].Hostname != "api.localhost" || data.Routes[0].Port != 3000 {
			t.Fatalf("DecodeData list = %+v, want one api.localhost:3000 route", data)
		}
	})
	t.Run("get", func(t *testing.T) {
		resData := map[string]any{
			"route": map[string]any{"hostname": "api.localhost", "port": float64(3000), "PID": float64(99)},
		}
		var data ipc.DataResponseGet
		if err := DecodeData(&resData, &data); err != nil {
			t.Fatalf("DecodeData: %v", err)
		}
		if data.Route.PID != 99 {
			t.Fatalf("DecodeData get = %+v, want PID 99", data)
		}
	})
	t.Run("invalid", func(t *testing.T) {
		resData := map[string]any{"routes": "not-a-list"}
		var data ipc.DataResponseList
		if err := DecodeData(&resData, &data); err == nil {
			t.Fatal("DecodeData(invalid) = nil error, want error")
		}
	})
}
