package logtool

import (
	"strings"
	"testing"
)

func TestBuildTemplateNormalizesVariables(t *testing.T) {
	a, varsA, idA := BuildTemplate(`database connection timeout after 3000ms for user "alice" password=secret`)
	b, _, idB := BuildTemplate(`database connection timeout after 5000ms for user "bob" password=other`)
	if a != b {
		t.Fatalf("templates differ:\n%s\n%s", a, b)
	}
	if idA != idB || !strings.HasPrefix(idA, "tpl_") {
		t.Fatalf("bad template ids %s %s", idA, idB)
	}
	for _, v := range varsA {
		if strings.Contains(v, "secret") {
			t.Fatalf("secret variable leaked: %#v", varsA)
		}
	}
	signal, tags := Classify(a, "ERROR")
	if signal != "latency" || !containsString(tags, "error") {
		t.Fatalf("classification signal=%s tags=%v template=%s", signal, tags, a)
	}
}
