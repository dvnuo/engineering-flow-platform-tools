package vision

import (
	"strings"
	"testing"

	"engineering-flow-platform-tools/internal/inspectimage/imagecheck"
)

func TestBuildRequestAllowsArbitraryModelAndRejectsReasoning(t *testing.T) {
	img := imagecheck.ImageInfo{MIMEType: "image/png", Data: []byte{1}}
	req, err := BuildRequest("custom-model", "medium", "task", img)
	if err != nil {
		t.Fatal(err)
	}
	if req.Model != "custom-model" {
		t.Fatalf("model was not passed through: %#v", req)
	}
	if _, err := BuildRequest("gpt-5.4", "bad", "task", img); err == nil {
		t.Fatal("expected reasoning rejection")
	}
}

func TestBuildRequestCreatesDataURLImage(t *testing.T) {
	img := imagecheck.ImageInfo{MIMEType: "image/png", Data: []byte{1, 2, 3}}
	req, err := BuildRequest("gpt-5.4", "medium", "task", img)
	if err != nil {
		t.Fatal(err)
	}
	content := req.Input[0].Content
	if content[0].Type != "input_text" || !strings.HasPrefix(content[1].ImageURL, "data:image/png;base64,") {
		t.Fatalf("bad content: %#v", content)
	}
}
