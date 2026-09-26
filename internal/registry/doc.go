// Package registry owns the hostname-to-port route table. It normalizes names, tracks owning processes for liveness, persists routes to disk, and prunes orphans left by crashed run wrappers.
package registry
