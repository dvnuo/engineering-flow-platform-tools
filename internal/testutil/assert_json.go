package testutil

import (
	"encoding/json"
	"testing"
)

func AssertJSONEnvelope(t *testing.T, b []byte) map[string]any {
	t.Helper()
	var v map[string]any
	if err := json.Unmarshal(b, &v); err != nil {
		t.Fatalf("not json: %v", err)
	}
	ok, has := v["ok"].(bool)
	if !has {
		t.Fatalf("missing ok")
	}
	if ok {
		if _, e := v["data"]; !e {
			t.Fatalf("ok=true missing data")
		}
	} else {
		e, eo := v["error"].(map[string]any)
		if !eo {
			t.Fatalf("ok=false missing error")
		}
		if _, co := e["code"]; !co {
			t.Fatalf("missing error.code")
		}
		if _, mo := e["message"]; !mo {
			t.Fatalf("missing error.message")
		}
	}
	return v
}
