package auth

import (
	"strings"
	"testing"

	"engineering-flow-platform-tools/internal/config"
)

func TestAuthHeaders(t *testing.T) {
	cases := []config.AuthConfig{{Type: "basic_password", Username: "u", Password: "p"}, {Type: "basic_api_key", Username: "u", APIKey: "k"}, {Type: "bearer_token", Token: "t"}}
	for _, c := range cases {
		h, err := AuthHeaders(c)
		if err != nil {
			t.Fatal(err)
		}
		if c.Type == "bearer_token" && h["Authorization"] != "Bearer t" {
			t.Fatal("bad bearer")
		}
		if strings.HasPrefix(c.Type, "basic") && !strings.HasPrefix(h["Authorization"], "Basic ") {
			t.Fatal("bad basic")
		}
	}
}
