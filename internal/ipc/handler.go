package ipc

import (
	"encoding/json"
	"fmt"
	"log"
	"net"

	"github.com/ppablomunoz/noports/internal/registry"
)

// HandleConnection serves a single unix-socket client.
func HandleConnection(conn net.Conn, store *registry.Store) {
	defer func() { _ = conn.Close() }()

	decoder := json.NewDecoder(conn)
	encoder := json.NewEncoder(conn)

	var req Request
	if err := decoder.Decode(&req); err != nil {
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
		err = handleAliasRemove(encoder, store, &req)
	default:
		err = sendResponse(encoder, &Response{OK: false, Error: "command is not valid"})
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
	return sendResponse(encoder, res)
}

func handleGet(encoder *json.Encoder, store *registry.Store, req *Request) error {
	if req.Hostname == "" {
		return sendResponse(encoder, &Response{OK: false, Error: "hostname is required"})
	}

	route, err := store.Get(req.Hostname)
	if err != nil {
		return sendResponse(encoder, &Response{OK: false, Error: err.Error()})
	}
	return sendResponse(encoder, &Response{OK: true, Data: DataResponseGet{Route: *route}})
}

func sendResponse(encoder *json.Encoder, res *Response) error {
	if err := encoder.Encode(res); err != nil {
		return fmt.Errorf("failed to encode daemon response: %w", err)
	}
	return nil
}

func handleAliasAdd(encoder *json.Encoder, store *registry.Store, req *Request) error {
	name := req.Hostname
	port := req.LocalPort
	pid := req.PID

	if name == "" {
		return sendResponse(encoder, &Response{OK: false, Error: "name is required"})
	}
	if port <= 0 {
		return sendResponse(encoder, &Response{OK: false, Error: "port is required"})
	}
	if pid == 0 { // pid == -1 --> Custom alias
		return sendResponse(encoder, &Response{OK: false, Error: "pid is invalid"})
	}

	hostname := fmt.Sprintf("%s.localhost", name)
	newRoute := &registry.Route{Hostname: hostname, Port: port, PID: pid}

	if err := store.Add(*newRoute); err != nil {
		return sendResponse(encoder, &Response{OK: false, Error: fmt.Sprintf("failed to add route: %v", err)})
	}
	log.Printf("Added %v\n", newRoute)
	return sendResponse(encoder, &Response{OK: true})
}

func handleAliasRemove(encoder *json.Encoder, store *registry.Store, req *Request) error {
	name := req.Hostname

	if name == "" {
		return sendResponse(encoder, &Response{OK: false, Error: "name is required"})
	}

	hostname := fmt.Sprintf("%s.localhost", name)

	if err := store.Remove(hostname); err != nil {
		return sendResponse(encoder, &Response{OK: false, Error: fmt.Sprintf("failed to remove '%s' route: %v", hostname, err)})
	}
	log.Printf("Removed: %s\n", hostname)
	return sendResponse(encoder, &Response{OK: true})
}
