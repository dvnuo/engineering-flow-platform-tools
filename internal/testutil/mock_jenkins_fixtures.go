package testutil

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// MockJenkinsBuildCount is how many builds the mock returns for a job's
// allBuilds/builds tree before the {0,N} range bounds the list.
const MockJenkinsBuildCount = 30

// Fixture layout used by build list / build params / job search tests.
//
// Builds #1..#30 for deploy/payments-api, newest first (#30 is still
// building). Build k started 5 minutes + (30-k) hours before BuildBase.
//
//	result:  k%11==0 ABORTED, else k%7==0 UNSTABLE, else k%5==0 FAILURE, else SUCCESS
//	params:  ENV=prod for even k, stage for odd k; VERSION=1.k.0;
//	         DRY_RUN=(k%3==0) as a JSON bool; API_TOKEN=s3cret-k
//	causes:  even k started by user alice; odd k by upstream folder/app-main #k
var (
	buildsTreePattern = regexp.MustCompile(`^(allBuilds|builds)\[.*\](?:\{(\d+),(\d+)\})?$`)
	buildNumberPath   = regexp.MustCompile(`/(\d+|lastBuild)/api/json$`)
)

func MockJenkinsBuildResult(k int) any {
	switch {
	case k == MockJenkinsBuildCount:
		return nil
	case k%11 == 0:
		return "ABORTED"
	case k%7 == 0:
		return "UNSTABLE"
	case k%5 == 0:
		return "FAILURE"
	default:
		return "SUCCESS"
	}
}

func (m *MockJenkins) buildTimestamp(k int) time.Time {
	return m.BuildBase.Add(-5*time.Minute - time.Duration(MockJenkinsBuildCount-k)*time.Hour)
}

func (m *MockJenkins) buildSummary(k int) map[string]any {
	env := "stage"
	if k%2 == 0 {
		env = "prod"
	}
	var causes []map[string]any
	if k%2 == 0 {
		causes = []map[string]any{{"_class": "hudson.model.Cause$UserIdCause", "shortDescription": "Started by user Alice", "userId": "alice", "userName": "Alice"}}
	} else {
		causes = []map[string]any{{"_class": "hudson.model.Cause$UpstreamCause", "shortDescription": "Started by upstream project \"folder/app-main\" build number " + strconv.Itoa(k), "upstreamProject": "folder/app-main", "upstreamBuild": k}}
	}
	building := k == MockJenkinsBuildCount
	duration := k * 1000
	if building {
		duration = 0
	}
	return map[string]any{
		"_class":      "org.jenkinsci.plugins.workflow.job.WorkflowRun",
		"number":      k,
		"url":         m.Server.URL + "/job/deploy/job/payments-api/" + strconv.Itoa(k) + "/",
		"displayName": "#" + strconv.Itoa(k),
		"building":    building,
		"result":      MockJenkinsBuildResult(k),
		"timestamp":   m.buildTimestamp(k).UnixMilli(),
		"duration":    duration,
		"actions": []map[string]any{
			{"_class": "hudson.model.ParametersAction", "parameters": []map[string]any{
				{"_class": "hudson.model.StringParameterValue", "name": "ENV", "value": env},
				{"_class": "hudson.model.StringParameterValue", "name": "VERSION", "value": "1." + strconv.Itoa(k) + ".0"},
				{"_class": "hudson.model.BooleanParameterValue", "name": "DRY_RUN", "value": k%3 == 0},
				{"_class": "hudson.model.StringParameterValue", "name": "API_TOKEN", "value": "s3cret-" + strconv.Itoa(k)},
			}},
			{"_class": "hudson.model.CauseAction", "causes": causes},
		},
	}
}

func (m *MockJenkins) buildsListJSON(tree string) ([]byte, bool) {
	match := buildsTreePattern.FindStringSubmatch(tree)
	if match == nil {
		return nil, false
	}
	count := MockJenkinsBuildCount
	if match[2] != "" {
		from, _ := strconv.Atoi(match[2])
		to, _ := strconv.Atoi(match[3])
		if n := to - from; n < count {
			count = n
		}
	}
	builds := make([]map[string]any, 0, count)
	for k := MockJenkinsBuildCount; k >= 1 && len(builds) < count; k-- {
		builds = append(builds, m.buildSummary(k))
	}
	out, _ := json.Marshal(map[string]any{"_class": "org.jenkinsci.plugins.workflow.job.WorkflowJob", match[1]: builds})
	return out, true
}

func mockBuildNumber(urlPath string) (int, bool) {
	match := buildNumberPath.FindStringSubmatch(urlPath)
	if match == nil {
		return 0, false
	}
	if match[1] == "lastBuild" {
		return MockJenkinsBuildCount, true
	}
	n, err := strconv.Atoi(match[1])
	if err != nil || n < 1 || n > MockJenkinsBuildCount {
		return 0, false
	}
	return n, true
}

// pipelineBuildJSON is a WorkflowRun with two git changeSets (3 commits).
func (m *MockJenkins) pipelineBuildJSON(k int) []byte {
	build := m.buildSummary(k)
	base := m.buildTimestamp(k)
	build["changeSets"] = []map[string]any{
		{"_class": "hudson.plugins.git.GitChangeSetList", "kind": "git", "items": []map[string]any{
			{"commitId": "a1b2c3d4", "msg": "Bump payments-api to 1." + strconv.Itoa(k) + ".0", "author": map[string]any{"fullName": "Alice Example"}, "timestamp": base.Add(-30 * time.Minute).UnixMilli(), "affectedPaths": []string{"charts/values.yaml", "VERSION"}},
			{"commitId": "e5f6a7b8", "msg": "Fix rollout readiness probe", "author": map[string]any{"fullName": "Bob Example"}, "timestamp": base.Add(-20 * time.Minute).UnixMilli(), "affectedPaths": []string{"deploy/rollout.yaml"}},
		}},
		{"_class": "hudson.plugins.git.GitChangeSetList", "kind": "git", "items": []map[string]any{
			{"commitId": "c9d0e1f2", "msg": "Docs touch-up", "author": map[string]any{"fullName": "Carol Example"}, "timestamp": base.Add(-10 * time.Minute).UnixMilli(), "affectedPaths": []string{"README.md", "docs/a.md", "docs/b.md"}},
		}},
	}
	out, _ := json.Marshal(build)
	return out
}

// freestyleBuildJSON is an older FreeStyleBuild with the singular changeSet
// shape; the second commit has no timestamp, like some non-git SCMs.
func (m *MockJenkins) freestyleBuildJSON(k int) []byte {
	base := m.buildTimestamp(k)
	build := map[string]any{
		"_class":      "hudson.model.FreeStyleBuild",
		"number":      k,
		"url":         m.Server.URL + "/job/tools/job/legacy-freestyle/" + strconv.Itoa(k) + "/",
		"displayName": "#" + strconv.Itoa(k),
		"building":    false,
		"result":      "SUCCESS",
		"timestamp":   base.UnixMilli(),
		"duration":    k * 500,
		"actions": []map[string]any{
			{"_class": "hudson.model.ParametersAction", "parameters": []map[string]any{{"name": "TARGET", "value": "prod"}}},
			{"_class": "hudson.model.CauseAction", "causes": []map[string]any{{"_class": "hudson.triggers.TimerTrigger$TimerTriggerCause", "shortDescription": "Started by timer"}}},
		},
		"changeSet": map[string]any{"_class": "hudson.scm.SubversionChangeLogSet", "kind": "svn", "items": []map[string]any{
			{"commitId": "1001", "msg": "svn change one", "author": map[string]any{"fullName": "Dan Example"}, "timestamp": base.Add(-time.Hour).UnixMilli(), "affectedPaths": []string{"a.txt"}},
			{"commitId": "1002", "msg": "svn change two", "author": map[string]any{"fullName": "Eve Example"}, "affectedPaths": []string{}},
		}},
	}
	out, _ := json.Marshal(build)
	return out
}

type mockJob struct {
	name      string
	class     string
	buildable bool
	children  []mockJob
}

const (
	classWorkflowJob = "org.jenkinsci.plugins.workflow.job.WorkflowJob"
	classFolder      = "com.cloudbees.hudson.plugins.folder.Folder"
	classMultibranch = "org.jenkinsci.plugins.workflow.multibranch.WorkflowMultiBranchProject"
	classFreestyle   = "hudson.model.FreeStyleProject"
)

// Folder tree (depth in parentheses):
//
//	app-main (1)
//	deploy (1) / payments-api (2) / main, PR-7 (3)
//	deploy (1) / Orders-Deploy (2)
//	deploy (1) / archive (2) / old-deploy (3), nested (3) / deep-deploy (4)
//	tools (1) / legacy-freestyle (2)
func mockFolderTree() []mockJob {
	return []mockJob{
		{name: "app-main", class: classWorkflowJob, buildable: true},
		{name: "deploy", class: classFolder, children: []mockJob{
			{name: "payments-api", class: classMultibranch, children: []mockJob{
				{name: "main", class: classWorkflowJob, buildable: true},
				{name: "PR-7", class: classWorkflowJob, buildable: true},
			}},
			{name: "Orders-Deploy", class: classFreestyle, buildable: true},
			{name: "archive", class: classFolder, children: []mockJob{
				{name: "old-deploy", class: classFreestyle, buildable: false},
				{name: "nested", class: classFolder, children: []mockJob{
					{name: "deep-deploy", class: classFreestyle, buildable: true},
				}},
			}},
		}},
		{name: "tools", class: classFolder, children: []mockJob{
			{name: "legacy-freestyle", class: classFreestyle, buildable: true},
		}},
	}
}

// folderTreeJSON renders the fixture the way Jenkins answers a nested
// jobs[...] tree: folders below maxDepth carry a jobs array, folders at
// maxDepth do not, and only buildable items expose buildable.
func (m *MockJenkins) folderTreeJSON(maxDepth int) []byte {
	var render func(jobs []mockJob, parentURL string, depth int) []map[string]any
	render = func(jobs []mockJob, parentURL string, depth int) []map[string]any {
		out := make([]map[string]any, 0, len(jobs))
		for _, job := range jobs {
			url := parentURL + "job/" + job.name + "/"
			node := map[string]any{"_class": job.class, "name": job.name, "url": url}
			if job.children == nil {
				node["buildable"] = job.buildable
			} else if depth < maxDepth {
				node["jobs"] = render(job.children, url, depth+1)
			}
			out = append(out, node)
		}
		return out
	}
	out, _ := json.Marshal(map[string]any{"_class": "hudson.model.Hudson", "jobs": render(mockFolderTree(), m.Server.URL+"/", 1)})
	return out
}

func isJobsSearchTree(tree string) bool {
	return strings.HasPrefix(tree, "jobs[name,url,_class,buildable")
}
