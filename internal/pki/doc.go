// Package pki manages the local certificate authority and per-hostname leaf certificates. It generates a long-lived CA, issues short-lived leafs, renews them before expiry, and integrates with the OS trust store.
package pki
