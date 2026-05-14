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

func AssertOKEnvelope(t *testing.T, b []byte) map[string]any {
	t.Helper()
	v := AssertJSONEnvelope(t, b)
	if ok, _ := v["ok"].(bool); !ok {
		t.Fatalf("expected ok=true: %s", string(b))
	}
	return v
}

func AssertErrorCode(t *testing.T, b []byte, code string) map[string]any {
	t.Helper()
	v := AssertJSONEnvelope(t, b)
	if ok, _ := v["ok"].(bool); ok {
		t.Fatalf("expected ok=false code=%s: %s", code, string(b))
	}
	errObj, _ := v["error"].(map[string]any)
	if got, _ := errObj["code"].(string); got != code {
		t.Fatalf("expected error.code=%s got %s: %s", code, got, string(b))
	}
	return v
}
