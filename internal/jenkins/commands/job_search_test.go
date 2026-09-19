package commands

import (
	"strings"
	"testing"
)

func jobPaths(t *testing.T, data map[string]any) []string {
	t.Helper()
	raw, _ := data["jobs"].([]any)
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		job, _ := item.(map[string]any)
		p, ok := job["path"].(string)
		if !ok {
			t.Fatalf("job without path: %#v", job)
		}
		out = append(out, p)
	}
	return out
}

func requireJobPaths(t *testing.T, data map[string]any, want ...string) {
	t.Helper()
	got := jobPaths(t, data)
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("paths=%v want %v", got, want)
	}
	if int(data["count_returned"].(float64)) != len(want) {
		t.Fatalf("count_returned=%v want %d", data["count_returned"], len(want))
	}
}

func TestJenkinsJobSearchFlattensNestedFoldersToDefaultDepth(t *testing.T) {
	mock, cfg := jenkinsTestSetup(t)
	data := requireJenkinsOK(t, runJenkins(t, cfg, "job", "search", "--pattern", "*"))
	if mock.LastPath != "/api/json" {
		t.Fatalf("path=%q", mock.LastPath)
	}
	if got := lastTree(t, mock); got != "jobs[name,url,_class,buildable,jobs[name,url,_class,buildable,jobs[name,url,_class,buildable]]]" {
		t.Fatalf("tree=%q", got)
	}
	requireJobPaths(t, data,
		"app-main",
		"deploy", "deploy/payments-api", "deploy/payments-api/main", "deploy/payments-api/PR-7", "deploy/Orders-Deploy", "deploy/archive", "deploy/archive/old-deploy", "deploy/archive/nested",
		"tools", "tools/legacy-freestyle",
	)
	if data["scanned"].(float64) != 11 || data["matched"].(float64) != 11 || data["truncated"] != false || data["max_depth"].(float64) != 3 || data["pattern"] != "*" {
		t.Fatalf("meta=%#v", data)
	}
	if data["unexpanded_folders"].(float64) != 1 {
		t.Fatalf("unexpanded_folders=%#v (deploy/archive/nested sits at max depth)", data["unexpanded_folders"])
	}
	byPath := map[string]map[string]any{}
	for _, item := range data["jobs"].([]any) {
		job := item.(map[string]any)
		byPath[job["path"].(string)] = job
	}
	if job := byPath["app-main"]; job["folder"] != false || job["buildable"] != true || job["class"] != "org.jenkinsci.plugins.workflow.job.WorkflowJob" || !strings.HasSuffix(job["url"].(string), "/job/app-main/") {
		t.Fatalf("app-main=%#v", job)
	}
	if job := byPath["deploy"]; job["folder"] != true || job["buildable"] != false || job["class"] != "com.cloudbees.hudson.plugins.folder.Folder" {
		t.Fatalf("deploy=%#v", job)
	}
	if job := byPath["deploy/payments-api"]; job["folder"] != true || !strings.HasSuffix(job["url"].(string), "/job/deploy/job/payments-api/") {
		t.Fatalf("deploy/payments-api=%#v", job)
	}
	if job := byPath["deploy/archive/nested"]; job["folder"] != true {
		t.Fatalf("a folder at max depth is still recognized by class: %#v", job)
	}
	if job := byPath["deploy/archive/old-deploy"]; job["folder"] != false || job["buildable"] != false {
		t.Fatalf("disabled job=%#v", job)
	}
}

func TestJenkinsJobSearchMaxDepthControlsTreeAndExpansion(t *testing.T) {
	mock, cfg := jenkinsTestSetup(t)
	data := requireJenkinsOK(t, runJenkins(t, cfg, "job", "search", "--pattern", "*", "--max-depth", "4"))
	if got := lastTree(t, mock); got != "jobs[name,url,_class,buildable,jobs[name,url,_class,buildable,jobs[name,url,_class,buildable,jobs[name,url,_class,buildable]]]]" {
		t.Fatalf("tree=%q", got)
	}
	paths := jobPaths(t, data)
	if len(paths) != 12 || paths[9] != "deploy/archive/nested/deep-deploy" {
		t.Fatalf("paths=%v", paths)
	}
	if data["unexpanded_folders"].(float64) != 0 || data["max_depth"].(float64) != 4 {
		t.Fatalf("meta=%#v", data)
	}

	data = requireJenkinsOK(t, runJenkins(t, cfg, "job", "search", "--pattern", "*", "--max-depth", "1"))
	if got := lastTree(t, mock); got != "jobs[name,url,_class,buildable]" {
		t.Fatalf("tree=%q", got)
	}
	requireJobPaths(t, data, "app-main", "deploy", "tools")
	if data["unexpanded_folders"].(float64) != 2 {
		t.Fatalf("unexpanded_folders=%#v", data["unexpanded_folders"])
	}

	data = requireJenkinsOK(t, runJenkins(t, cfg, "job", "search", "--pattern", "*", "--max-depth", "9"))
	if data["max_depth"].(float64) != 6 || strings.Count(lastTree(t, mock), "jobs[") != 6 || len(jobPaths(t, data)) != 12 {
		t.Fatalf("max-depth should be capped at 6: %#v tree=%q", data, lastTree(t, mock))
	}

	data = requireJenkinsOK(t, runJenkins(t, cfg, "job", "search", "--pattern", "*", "--max-depth", "0"))
	if data["max_depth"].(float64) != 3 || len(jobPaths(t, data)) != 11 {
		t.Fatalf("max-depth 0 should fall back to the default: %#v", data)
	}
}

func TestJenkinsJobSearchGlobMatchesPathAndLeafCaseInsensitively(t *testing.T) {
	_, cfg := jenkinsTestSetup(t)
	cases := []struct {
		pattern string
		want    []string
	}{
		{"*deploy*", []string{"deploy", "deploy/payments-api", "deploy/payments-api/main", "deploy/payments-api/PR-7", "deploy/Orders-Deploy", "deploy/archive", "deploy/archive/old-deploy", "deploy/archive/nested"}},
		{"*orders*", []string{"deploy/Orders-Deploy"}},
		{"PAYMENTS-API", []string{"deploy/payments-api"}},
		{"deploy/*", []string{"deploy/payments-api", "deploy/payments-api/main", "deploy/payments-api/PR-7", "deploy/Orders-Deploy", "deploy/archive", "deploy/archive/old-deploy", "deploy/archive/nested"}},
		{"deploy/*-api", []string{"deploy/payments-api"}},
		{"main", []string{"deploy/payments-api/main"}},
		{"*/main", []string{"deploy/payments-api/main"}},
		{"pr-?", []string{"deploy/payments-api/PR-7"}},
		{"*-deploy", []string{"deploy/Orders-Deploy", "deploy/archive/old-deploy"}},
		{"tools/*", []string{"tools/legacy-freestyle"}},
		{"[at]*", []string{"app-main", "deploy/archive", "tools", "tools/legacy-freestyle"}},
		{"nomatch-*", nil},
	}
	for _, tc := range cases {
		data := requireJenkinsOK(t, runJenkins(t, cfg, "job", "search", "--pattern", tc.pattern))
		got := jobPaths(t, data)
		if strings.Join(got, "|") != strings.Join(tc.want, "|") {
			t.Fatalf("pattern %q: paths=%v want %v", tc.pattern, got, tc.want)
		}
		if data["truncated"] != false || int(data["matched"].(float64)) != len(tc.want) {
			t.Fatalf("pattern %q: meta=%#v", tc.pattern, data)
		}
	}
}

func TestJenkinsJobSearchLimitTruncates(t *testing.T) {
	_, cfg := jenkinsTestSetup(t)
	data := requireJenkinsOK(t, runJenkins(t, cfg, "job", "search", "--pattern", "*deploy*", "--limit", "2"))
	requireJobPaths(t, data, "deploy", "deploy/payments-api")
	if data["matched"].(float64) != 8 || data["truncated"] != true || data["scanned"].(float64) != 11 {
		t.Fatalf("meta=%#v", data)
	}
	data = requireJenkinsOK(t, runJenkins(t, cfg, "job", "search", "--pattern", "*deploy*", "--limit", "0"))
	if len(jobPaths(t, data)) != 8 || data["truncated"] != false {
		t.Fatalf("limit 0 should fall back to the default: %#v", data)
	}
}

func TestJenkinsJobSearchRejectsMissingOrMalformedPattern(t *testing.T) {
	_, cfg := jenkinsTestSetup(t)
	errObj := requireJenkinsError(t, runJenkins(t, cfg, "job", "search"), "invalid_args")
	if !strings.Contains(errObj["message"].(string), "--pattern") {
		t.Fatalf("message=%#v", errObj["message"])
	}
	errObj = requireJenkinsError(t, runJenkins(t, cfg, "job", "search", "--pattern", "[deploy"), "invalid_args")
	if !strings.Contains(errObj["message"].(string), "[deploy") {
		t.Fatalf("message=%#v", errObj["message"])
	}
}

func TestJenkinsJobsTree(t *testing.T) {
	if got := jenkinsJobsTree(1); got != "jobs[name,url,_class,buildable]" {
		t.Fatalf("depth 1: %q", got)
	}
	if got := jenkinsJobsTree(2); got != "jobs[name,url,_class,buildable,jobs[name,url,_class,buildable]]" {
		t.Fatalf("depth 2: %q", got)
	}
}

func TestJobGlob(t *testing.T) {
	cases := []struct {
		pattern, fullPath, leaf string
		want                    bool
	}{
		{"*deploy*", "deploy/payments-api", "payments-api", true},
		{"*Deploy*", "team/deploy-x", "deploy-x", true},
		{"payments-*", "deploy/payments-api", "payments-api", true},
		{"payments-*", "deploy/payments-api/main", "main", false},
		{"deploy/*", "deploy/payments-api/main", "main", true},
		{"deploy/*", "deploy", "deploy", false},
		{"*/main", "deploy/payments-api/main", "main", true},
		{"main", "deploy/payments-api/main", "main", true},
		{"main", "deploy/payments-api/main-old", "main-old", false},
		{"PR-?", "x/PR-7", "PR-7", true},
		{"PR-?", "x/PR-77", "PR-77", false},
		{"[dt]*", "tools", "tools", true},
		{"[dt]*", "app-main", "app-main", false},
		{`\*literal`, "a/*literal", "*literal", true},
		{`\*literal`, "a/xliteral", "xliteral", false},
	}
	for _, tc := range cases {
		g, err := newJobGlob(tc.pattern)
		if err != nil {
			t.Fatalf("newJobGlob(%q): %v", tc.pattern, err)
		}
		if got := g.match(tc.fullPath, tc.leaf); got != tc.want {
			t.Fatalf("glob %q on %q/%q = %v want %v", tc.pattern, tc.fullPath, tc.leaf, got, tc.want)
		}
	}
	for _, bad := range []string{"[", "[a-", `a\`} {
		if _, err := newJobGlob(bad); err == nil {
			t.Fatalf("newJobGlob(%q) unexpectedly succeeded", bad)
		}
	}
}

func TestIsJenkinsFolderClass(t *testing.T) {
	for class, want := range map[string]bool{
		"com.cloudbees.hudson.plugins.folder.Folder":                            true,
		"org.jenkinsci.plugins.workflow.multibranch.WorkflowMultiBranchProject": true,
		"jenkins.branch.OrganizationFolder":                                     true,
		"org.jenkinsci.plugins.workflow.job.WorkflowJob":                        false,
		"hudson.model.FreeStyleProject":                                         false,
		"":                                                                      false,
	} {
		if got := isJenkinsFolderClass(class); got != want {
			t.Fatalf("isJenkinsFolderClass(%q)=%v want %v", class, got, want)
		}
	}
}
