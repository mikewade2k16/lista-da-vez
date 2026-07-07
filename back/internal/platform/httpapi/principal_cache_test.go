package httpapi_test

import (
	"testing"
	"time"

	"github.com/mikewade2k16/lista-da-vez/back/internal/platform/httpapi"
)

type testPrincipal struct {
	UserID string
	Role   string
}

func TestPrincipalCache_GetSet(t *testing.T) {
	cache := httpapi.NewPrincipalCache[testPrincipal](2 * time.Minute)

	p := testPrincipal{UserID: "u1", Role: "manager"}
	cache.Set("session1", "u1", p)

	got, ok := cache.Get("session1")
	if !ok {
		t.Fatal("expected cache hit")
	}
	if got.UserID != p.UserID || got.Role != p.Role {
		t.Fatalf("unexpected value: %+v", got)
	}
}

func TestPrincipalCache_MissOnEmpty(t *testing.T) {
	cache := httpapi.NewPrincipalCache[testPrincipal](2 * time.Minute)

	_, ok := cache.Get("session-nao-existe")
	if ok {
		t.Fatal("expected cache miss")
	}
}

func TestPrincipalCache_MissOnEmptySessionID(t *testing.T) {
	cache := httpapi.NewPrincipalCache[testPrincipal](2 * time.Minute)

	cache.Set("", "u1", testPrincipal{UserID: "u1"})

	_, ok := cache.Get("")
	if ok {
		t.Fatal("Get with empty sessionID deve retornar miss")
	}
}

func TestPrincipalCache_ExpiredEntry(t *testing.T) {
	cache := httpapi.NewPrincipalCache[testPrincipal](1 * time.Millisecond)
	cache.Set("session1", "u1", testPrincipal{UserID: "u1"})

	time.Sleep(5 * time.Millisecond)

	_, ok := cache.Get("session1")
	if ok {
		t.Fatal("entrada expirada deve retornar miss")
	}
}

func TestPrincipalCache_InvalidateSession(t *testing.T) {
	cache := httpapi.NewPrincipalCache[testPrincipal](2 * time.Minute)
	cache.Set("session1", "u1", testPrincipal{UserID: "u1"})
	cache.InvalidateSession("session1")

	_, ok := cache.Get("session1")
	if ok {
		t.Fatal("sessao invalidada deve retornar miss")
	}
}

func TestPrincipalCache_InvalidateUser(t *testing.T) {
	cache := httpapi.NewPrincipalCache[testPrincipal](2 * time.Minute)
	cache.Set("session1", "u1", testPrincipal{UserID: "u1"})
	cache.Set("session2", "u1", testPrincipal{UserID: "u1"})
	cache.Set("session3", "u2", testPrincipal{UserID: "u2"})

	cache.InvalidateUser("u1")

	if _, ok := cache.Get("session1"); ok {
		t.Error("session1 de u1 deve ser invalidada")
	}
	if _, ok := cache.Get("session2"); ok {
		t.Error("session2 de u1 deve ser invalidada")
	}
	if _, ok := cache.Get("session3"); !ok {
		t.Error("session3 de u2 NAO deve ser invalidada")
	}
}

func TestPrincipalCache_InvalidateAll(t *testing.T) {
	cache := httpapi.NewPrincipalCache[testPrincipal](2 * time.Minute)
	cache.Set("session1", "u1", testPrincipal{UserID: "u1"})
	cache.Set("session2", "u2", testPrincipal{UserID: "u2"})

	cache.InvalidateAll()

	if _, ok := cache.Get("session1"); ok {
		t.Error("session1 deve ser removida")
	}
	if _, ok := cache.Get("session2"); ok {
		t.Error("session2 deve ser removida")
	}
}

func TestPrincipalCache_Stats(t *testing.T) {
	cache := httpapi.NewPrincipalCache[testPrincipal](2 * time.Minute)
	cache.Set("s1", "u1", testPrincipal{UserID: "u1"})
	cache.Get("s1")         // hit
	cache.Get("nao-existe") // miss
	hits, misses := cache.Stats()
	if hits != 1 || misses != 1 {
		t.Fatalf("esperado 1 hit / 1 miss, veio %d/%d", hits, misses)
	}
	if cache.Len() != 1 {
		t.Fatalf("esperado Len=1, veio %d", cache.Len())
	}
}

func TestPrincipalCache_CleanupPrunesExpired(t *testing.T) {
	cache := httpapi.NewPrincipalCache[testPrincipal](1 * time.Millisecond)
	cache.Set("session1", "u1", testPrincipal{UserID: "u1"})

	time.Sleep(5 * time.Millisecond)
	cache.Cleanup()

	// Entrada expirada e removida apos Cleanup; Get ainda retorna miss por expiry.
	_, ok := cache.Get("session1")
	if ok {
		t.Error("entrada expirada deve retornar miss apos Cleanup")
	}
}
