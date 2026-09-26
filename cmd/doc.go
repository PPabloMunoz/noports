// Package cmd wires the Cobra CLI to the daemon control plane. Each command validates flags, ensures the CA and daemon are running, and issues a single IPC request.
package cmd
