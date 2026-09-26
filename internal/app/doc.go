// Package app owns the daemon lifecycle. It loads persisted routes, serves the HTTP redirect, HTTPS proxy, and control socket, then shuts everything down on signal or context cancel.
package app
