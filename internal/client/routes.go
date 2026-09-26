package client

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"text/tabwriter"

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

// PrintRoutesTable prints routes as a NAME/HOST/PORT/PID table. It prints a hint when empty instead of an empty table.
func PrintRoutesTable(routes []registry.Route) {
	if len(routes) == 0 {
		Info("No routes registered. Run `noports run --name <name> -- <command>` to add one.\n")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)

	_, _ = fmt.Fprintf(w, "NAME\tHOST\tPORT\tPID\n")
	for _, r := range routes {
		name := registry.BareName(r.Hostname)
		_, _ = fmt.Fprintf(w, "%s\t%s\t%d\t%d\n", name, r.Hostname, r.Port, r.PID)
	}

	_ = w.Flush()
}

// RemoveRunRoute unregisters the run route for name when the child exits. It dials the daemon and sends an alias-remove request.
func RemoveRunRoute(command *exec.Cmd, name string, res *ipc.Response) error {
	conn, err := ipc.Dial()
	if err != nil {
		_ = command.Process.Signal(os.Interrupt)
		return err
	}
	defer func() { _ = conn.Close() }()

	encoder := json.NewEncoder(conn)
	decoder := json.NewDecoder(conn)

	req := &ipc.Request{Command: ipc.CmdAliasRemove, Hostname: name}
	if err := Send(encoder, req); err != nil {
		return err
	}
	if err := Receive(decoder, res); err != nil {
		return err
	}
	return nil
}
