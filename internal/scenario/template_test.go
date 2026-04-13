package scenario

import "testing"

func TestExpandBuiltins(t *testing.T) {
	got, err := Expand("/users?vu=${__vu}&i=${__iter}", nil, TemplateCtx{VU: 3, Iter: 7})
	if err != nil {
		t.Fatal(err)
	}
	if got != "/users?vu=3&i=7" {
		t.Fatalf("got %q", got)
	}
}

func TestExpandRejectsMissingBrace(t *testing.T) {
	if _, err := Expand("/users/${id", nil, TemplateCtx{}); err == nil {
		t.Fatal("expected error")
	}
}
