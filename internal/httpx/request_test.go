package httpx

import "testing"

func TestResolveURLKeepsQuery(t *testing.T) {
	got, err := ResolveURL("http://example.test/api", "/users?q=one")
	if err != nil {
		t.Fatal(err)
	}
	if got != "http://example.test/api/users?q=one" {
		t.Fatalf("got %q", got)
	}
}
