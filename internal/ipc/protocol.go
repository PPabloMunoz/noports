package ipc

import "github.com/ppablomunoz/noports/internal/registry"

// Command is a daemon control operation sent over the unix socket.
type Command string

const (
	CmdGet         Command = "get"
	CmdList        Command = "list"
	CmdAliasAdd    Command = "alias_add"
	CmdAliasRemove Command = "alias_remove"
)

// Request is a single control message from a CLI client.
type Request struct {
	Command   Command `json:"command"`
	Hostname  string  `json:"hostname,omitempty"`
	LocalPort int     `json:"local_port,omitempty"`
	PID       int     `json:"pid,omitempty"`
	// WrapperPID is the `run` wrapper process owning the route (-1 for
	// user-managed aliases, 0 when sent by older clients).
	WrapperPID int `json:"wrapper_pid,omitempty"`
	// ChildStartTime/WrapperStartTime are the wall-clock start times
	// (Unix ms) of PID/WrapperPID for PID-reuse detection; 0 = unknown.
	ChildStartTime   int64 `json:"child_start_time,omitempty"`
	WrapperStartTime int64 `json:"wrapper_start_time,omitempty"`
}

// Response is the daemon reply.
type Response struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
	Data  any    `json:"data,omitempty"`
}

// DataResponseList is the payload for CmdList.
type DataResponseList struct {
	Routes []registry.Route `json:"routes"`
}

// DataResponseGet is the payload for CmdGet.
type DataResponseGet struct {
	Route registry.Route `json:"route"`
}
