package client

import (
	"encoding/json"

	"github.com/ppablomunoz/noports/internal/ipc"
	"github.com/ppablomunoz/noports/internal/registry"
)

// ListRoutes queries the daemon over the control socket without starting it. Callers that want the daemon up first should call EnsureProxy beforehand.
func ListRoutes() ([]registry.Route, error) {
	conn, err := ipc.Dial()
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()

	encoder := json.NewEncoder(conn)
	decoder := json.NewDecoder(conn)

	req := &ipc.Request{Command: ipc.CmdList}
	if err := Send(encoder, req); err != nil {
		return nil, err
	}

	var res ipc.Response
	if err := Receive(decoder, &res); err != nil {
		return nil, err
	}

	var data ipc.DataResponseList
	if err := DecodeListResponse(&res.Data, &data); err != nil {
		return nil, err
	}

	return data.Routes, nil
}
