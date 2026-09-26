package ipc

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"

	"github.com/ppablomunoz/noports/internal/registry"
)

// HandleConnection serves a single unix-socket client.
// onRemove, when non-nil, is called with the normalized hostname after a
// successful remove so the caller can evict cached state (e.g. TLS certs).
func HandleConnection(conn net.Conn, store *registry.Store, onRemove func(hostname string)) {
	defer func() { _ = conn.Close() }()

	decoder := json.NewDecoder(conn)
	encoder := json.NewEncoder(conn)

	var req Request
	if err := decoder.Decode(&req); err != nil {
		if errors.Is(err, io.EOF) {
			// Probe dial (e.g. IsDaemonRunning) closed without a request.
			return
		}
		log.Printf("[ERROR] failed to decode request to socket: %v\n", err)
		return
	}

	var err error
	switch req.Command {
	case CmdList:
		err = handleList(encoder, store)
	case CmdGet:
		err = handleGet(encoder, store, &req)
	case CmdAliasAdd:
		err = handleAliasAdd(encoder, store, &req)
	case CmdAliasRemove:
		err = handleAliasRemove(encoder, store, &req, onRemove)
	default:
		err = respond(encoder, &Response{OK: false, Error: "command is not valid"})
	}
	if err != nil {
		log.Println("[ERROR]", err)
	}
}

func handleList(encoder *json.Encoder, store *registry.Store) error {
	result := []registry.Route{}
	for _, r := range store.List() {
		result = append(result, r)
	}
	res := &Response{OK: true, Data: DataResponseList{Routes: result}}
	return respond(encoder, res)
}

func handleGet(encoder *json.Encoder, store *registry.Store, req *Request) error {
	if req.Hostname == "" {
		return respond(encoder, &Response{OK: false, Error: "hostname is required"})
	}

	hostname, err := registry.NormalizeHostname(req.Hostname)
	if err != nil {
		return respond(encoder, &Response{OK: false, Error: err.Error()})
	}

	route, err := store.Get(hostname)
	if err != nil {
		return respond(encoder, &Response{OK: false, Error: err.Error()})
	}
	return respond(encoder, &Response{OK: true, Data: DataResponseGet{Route: *route}})
}

func respond(encoder *json.Encoder, res *Response) error {
	if err := encoder.Encode(res); err != nil {
		return fmt.Errorf("failed to encode daemon response: %w", err)
	}
	return nil
}

func handleAliasAdd(encoder *json.Encoder, store *registry.Store, req *Request) error {
	name := req.Hostname
	port := req.LocalPort
	pid := req.PID
	wrapperPID := req.WrapperPID

	hostname, err := registry.NormalizeHostname(name)
	if err != nil {
		return respond(encoder, &Response{OK: false, Error: err.Error()})
	}
	if port <= 0 {
		return respond(encoder, &Response{OK: false, Error: "port is required"})
	}
	if pid == -1 {
		// User-managed alias: no processes to watch. Tolerate wrapper 0
		// from older clients.
		if wrapperPID != -1 && wrapperPID != 0 {
			return respond(encoder, &Response{OK: false, Error: "wrapper pid must be -1 for aliases"})
		}
		wrapperPID = -1
	} else {
		if pid <= 0 { // pid == -1 --> Custom alias
			return respond(encoder, &Response{OK: false, Error: "pid is invalid"})
		}
		if wrapperPID <= 0 {
			return respond(encoder, &Response{OK: false, Error: "wrapper pid is invalid"})
		}
	}

	childStart := req.ChildStartTime
	wrapperStart := req.WrapperStartTime
	if pid == -1 {
		childStart, wrapperStart = 0, 0
	} else {
		// Backfill start times the client could not determine so PID
		// reuse is still detectable on later sweeps.
		if childStart == 0 {
			if st, err := registry.ProcessStartTime(pid); err == nil {
				childStart = st
			}
		}
		if wrapperStart == 0 {
			if st, err := registry.ProcessStartTime(wrapperPID); err == nil {
				wrapperStart = st
			}
		}
	}

	newRoute := &registry.Route{
		Hostname:         hostname,
		Port:             port,
		PID:              pid,
		WrapperPID:       wrapperPID,
		ChildStartTime:   childStart,
		WrapperStartTime: wrapperStart,
	}

	if err := store.Add(*newRoute); err != nil {
		return respond(encoder, &Response{OK: false, Error: fmt.Sprintf("failed to add route: %v", err)})
	}
	log.Printf("Added %v\n", newRoute)
	return respond(encoder, &Response{OK: true})
}

func handleAliasRemove(encoder *json.Encoder, store *registry.Store, req *Request, onRemove func(hostname string)) error {
	hostname, err := registry.NormalizeHostname(req.Hostname)
	if err != nil {
		return respond(encoder, &Response{OK: false, Error: err.Error()})
	}

	if err := store.Remove(hostname); err != nil {
		return respond(encoder, &Response{OK: false, Error: fmt.Sprintf("failed to remove '%s' route: %v", hostname, err)})
	}
	if onRemove != nil {
		onRemove(hostname)
	}
	log.Printf("Removed: %s\n", hostname)
	return respond(encoder, &Response{OK: true})
}
