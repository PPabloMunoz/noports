package client

import (
	"encoding/json"
	"fmt"
	"net"

	"github.com/ppablomunoz/noports/internal/ipc"
	"github.com/ppablomunoz/noports/internal/paths"
)

func ConnectToSocket() (net.Conn, error) {
	conn, err := net.Dial("unix", paths.GetSocketPath())
	if err != nil {
		return nil, fmt.Errorf("failed to connect to socket: %w", err)
	}
	return conn, nil
}

func SendRequest(encoder *json.Encoder, req *ipc.Request) error {
	if err := encoder.Encode(req); err != nil {
		return fmt.Errorf("failed to encode request: %w", err)
	}
	return nil
}

func GetResponse(decoder *json.Decoder, res *ipc.Response) error {
	if err := decoder.Decode(res); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}
	if !res.OK {
		return fmt.Errorf("%s", res.Error)
	}
	return nil
}

func GetDataList(resData any, result *ipc.DataResponseList) error {
	dataResBytes, err := json.Marshal(resData)
	if err != nil {
		return fmt.Errorf("failed to marshal data response: %w", err)
	}

	if err := json.Unmarshal(dataResBytes, &result); err != nil {
		return fmt.Errorf("failed to unmarshal %s: %w", string(dataResBytes), err)
	}
	return nil
}

func GetDataGet(resData any, result *ipc.DataResponseGet) error {
	dataResBytes, err := json.Marshal(resData)
	if err != nil {
		return fmt.Errorf("failed to marshal data response: %w", err)
	}

	if err := json.Unmarshal(dataResBytes, &result); err != nil {
		return fmt.Errorf("failed to unmarshal %s: %w", string(dataResBytes), err)
	}
	return nil
}
