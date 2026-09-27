package registry

import (
	"testing"
)

func testStore(routes []Route) *Store {
	s := NewStore()
	s.importRoutes(routes)
	return s
}

func TestImportRoutesSkipsInvalid(t *testing.T) {
	s := testStore([]Route{
		{Hostname: "api", Port: 3000, PID: -1, WrapperPID: -1},
		{Hostname: "bad name!", Port: 3001, PID: -1, WrapperPID: -1},
		{Hostname: "noport", Port: 0, PID: -1, WrapperPID: -1},
		{Hostname: "localhost", Port: 3002, PID: -1, WrapperPID: -1},
	})
	all := s.List()
	if len(all) != 1 {
		t.Fatalf("List() has %d routes, want 1 (only the valid route)", len(all))
	}
	if _, ok := all["api.localhost"]; !ok {
		t.Fatalf("List() missing api.localhost, got %v", all)
	}
}

func TestGetAfterImport(t *testing.T) {
	s := testStore([]Route{{Hostname: "API", Port: 8080, PID: -1, WrapperPID: -1}})
	got, err := s.Get("api.localhost")
	if err != nil {
		t.Fatalf("Get unexpected error: %v", err)
	}
	if got.Hostname != "api.localhost" || got.Port != 8080 {
		t.Fatalf("Get = %+v, want normalized api.localhost:8080", got)
	}
	if _, err := s.Get("missing"); err == nil {
		t.Fatal("Get(missing) = nil error, want error")
	}
}

func TestPruneRemovesOnlyDead(t *testing.T) {
	alive := map[int]bool{100: true, 200: true}
	s := testStore([]Route{
		{Hostname: "alive", Port: 3000, PID: 100, WrapperPID: 200},
		{Hostname: "dead-child", Port: 3001, PID: 101, WrapperPID: 200},
		{Hostname: "dead-wrapper", Port: 3002, PID: 100, WrapperPID: 201},
	})
	pruned := s.Prune(func(r Route) bool { return alive[r.PID] && alive[r.WrapperPID] })
	if len(pruned) != 2 {
		t.Fatalf("Prune removed %d routes, want 2", len(pruned))
	}
	if _, err := s.Get("alive"); err != nil {
		t.Fatalf("Prune removed live route: %v", err)
	}
	if _, err := s.Get("dead-child"); err == nil {
		t.Error("Prune kept dead-child route")
	}
	if _, err := s.Get("dead-wrapper"); err == nil {
		t.Error("Prune kept dead-wrapper route")
	}
}

func TestPruneKeepsAliases(t *testing.T) {
	s := testStore([]Route{
		{Hostname: "alias", Port: 3000, PID: -1, WrapperPID: -1},
	})
	pruned := s.Prune(func(Route) bool { return false })
	if len(pruned) != 0 {
		t.Fatalf("Prune removed %d routes, want 0 (aliases are never pruned)", len(pruned))
	}
	if _, err := s.Get("alias"); err != nil {
		t.Fatalf("Prune removed alias route: %v", err)
	}
}
