package scenario

import (
	"math/rand"
	"testing"
)

func TestExpandBuiltinsAndNestedVars(t *testing.T) {
	got, err := Expand(
		"/users/${client}/${__vu}/${__iter}/${__rand_int(1,1)}",
		map[string]string{"client": "demo-${suffix}", "suffix": "42"},
		TemplateCtx{VU: 7, Iter: 11, Rand: rand.New(rand.NewSource(1))},
	)
	if err != nil {
		t.Fatal(err)
	}
	want := "/users/demo-42/7/11/1"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestExpandUnknownVar(t *testing.T) {
	_, err := Expand("${missing}", nil, TemplateCtx{})
	if err == nil {
		t.Fatal("expected error")
	}
}
