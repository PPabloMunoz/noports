package client

import (
	"encoding/json"
	"fmt"

	"github.com/ppablomunoz/noports/internal/ipc"
)

// Send encodes a single control request to the daemon. It wraps encode failures with context and leaves connection handling to the caller.
func Send(encoder *json.Encoder, req *ipc.Request) error {
	if err := encoder.Encode(req); err != nil {
		return fmt.Errorf("failed to encode request: %w", err)
	}
	return nil
}

// Receive decodes one daemon response and returns an error when the daemon reports failure. It wraps decode failures with context for CLI reporting.
func Receive(decoder *json.Decoder, res *ipc.Response) error {
	if err := decoder.Decode(res); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}
	if !res.OK {
		return fmt.Errorf("%s", res.Error)
	}
	return nil
}

// DecodeListResponse converts the generic response payload into a route list. It round-trips through JSON so daemon and CLI can evolve independently.
func DecodeListResponse(resData any, result *ipc.DataResponseList) error {
	dataResBytes, err := json.Marshal(resData)
	if err != nil {
		return fmt.Errorf("failed to marshal data response: %w", err)
	}

	if err := json.Unmarshal(dataResBytes, &result); err != nil {
		return fmt.Errorf("failed to unmarshal %s: %w", string(dataResBytes), err)
	}
	return nil
}

// DecodeGetResponse converts the generic response payload into a single route. It round-trips through JSON so daemon and CLI can evolve independently.
func DecodeGetResponse(resData any, result *ipc.DataResponseGet) error {
	dataResBytes, err := json.Marshal(resData)
	if err != nil {
		return fmt.Errorf("failed to marshal data response: %w", err)
	}

	if err := json.Unmarshal(dataResBytes, &result); err != nil {
		return fmt.Errorf("failed to unmarshal %s: %w", string(dataResBytes), err)
	}
	return nil
}
