package client

import (
	"encoding/json"

	"github.com/ppablomunoz/noports/internal/ipc"
	"github.com/ppablomunoz/noports/internal/registry"
)

// ListRoutes queries the daemon over the control socket without starting it.
// Callers that want the daemon up first should call EnsureProxy beforehand.
func ListRoutes() ([]registry.Route, error) {
	conn, err := ConnectToSocket()
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()

	encoder := json.NewEncoder(conn)
	decoder := json.NewDecoder(conn)

	req := &ipc.Request{Command: ipc.CmdList}
	if err := SendRequest(encoder, req); err != nil {
		return nil, err
	}

	var res ipc.Response
	if err := GetResponse(decoder, &res); err != nil {
		return nil, err
	}

	var data ipc.DataResponseList
	if err := GetDataList(&res.Data, &data); err != nil {
		return nil, err
	}

	return data.Routes, nil
}
