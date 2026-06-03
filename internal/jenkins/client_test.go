package jenkins

import "testing"

func TestJobPathFromFolderPath(t *testing.T) {
	tests := map[string]string{
		"app":                            "/job/app",
		"folder/app":                     "/job/folder/job/app",
		"/job/folder/job/app/":           "/job/folder/job/app",
		"folder/feature%2Fbranch":        "/job/folder/job/feature%2Fbranch",
		"https://ci.example/job/a/job/b": "/job/a/job/b",
	}
	for input, want := range tests {
		if got := JobPath(input); got != want {
			t.Fatalf("JobPath(%q)=%q want %q", input, got, want)
		}
	}
}

func TestQueueIDFromLocation(t *testing.T) {
	if got := QueueIDFromLocation("https://ci.example/queue/item/123/"); got != "123" {
		t.Fatalf("queue id=%q", got)
	}
}
