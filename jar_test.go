package tspdnative

import "testing"

func TestJarOrderAndHeader(t *testing.T) {
	jar := newCookieJar()
	jar.Set("a", "1")
	jar.Set("b", "2")
	jar.Set("c", "3")

	if got := jar.Header(); got != "a=1; b=2; c=3" {
		t.Fatalf("Header() = %q", got)
	}
	if got := jar.Len(); got != 3 {
		t.Fatalf("Len() = %d, want 3", got)
	}
}

func TestJarUpdatePreservesPosition(t *testing.T) {
	jar := newCookieJar()
	jar.Set("a", "1")
	jar.Set("b", "2")
	jar.Set("a", "10")

	if got := jar.Header(); got != "a=10; b=2" {
		t.Fatalf("Header() = %q, want a=10; b=2", got)
	}
}

func TestJarRemoveUsesSwapRemove(t *testing.T) {
	jar := newCookieJar()
	jar.Set("a", "1")
	jar.Set("b", "2")
	jar.Set("c", "3")
	jar.Remove("b")

	if got := jar.Header(); got != "a=1; c=3" {
		t.Fatalf("Header() = %q, want a=1; c=3", got)
	}
	if _, ok := jar.Get("b"); ok {
		t.Fatal("removed cookie is still present")
	}
	if value, ok := jar.Get("c"); !ok || value != "3" {
		t.Fatalf("Get(c) = %q, %v", value, ok)
	}
}

func TestJarSetCookieHeaders(t *testing.T) {
	jar := newCookieJar()
	jar.applySetCookies([]string{
		"session=abc; Path=/; HttpOnly",
		"other=1; Path=/",
		"session=def; Path=/",
	})

	if got := jar.Header(); got != "session=def; other=1" {
		t.Fatalf("Header() = %q", got)
	}

	jar.applySetCookie("other=ignored; Expires=Thu, 01 Jan 1970 00:00:00 GMT")
	if got := jar.Header(); got != "session=def" {
		t.Fatalf("after expiry Header() = %q", got)
	}
}

func TestJarSnapshotIsIndependent(t *testing.T) {
	jar := newCookieJar()
	jar.Set("a", "1")

	snapshot := jar.Snapshot()
	snapshot[0].Value = "changed"

	if value, _ := jar.Get("a"); value != "1" {
		t.Fatalf("jar value changed through snapshot: %q", value)
	}
}
