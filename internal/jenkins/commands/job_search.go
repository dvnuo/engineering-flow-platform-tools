package commands

import (
	"fmt"
	"net/http"
	"path"
	"strings"

	"engineering-flow-platform-tools/internal/jenkins"
	"engineering-flow-platform-tools/internal/output"
	"github.com/spf13/cobra"
)

const (
	jobSearchDefaultDepth = 3
	jobSearchMaxDepth     = 6
	jobSearchDefaultLimit = 100
	jobSearchMaxLimit     = 1000
	jobSearchFields       = "name,url,_class,buildable"
)

// jobSearchCmd finds jobs by name across nested folders and multibranch
// projects in one request, so an agent that only knows "the payments deploy
// job" does not have to walk the folder tree with repeated job list calls.
func jobSearchCmd(o *Opts) *cobra.Command {
	var pattern string
	var maxDepth, limit int
	c := &cobra.Command{Use: "search", RunE: func(cmd *cobra.Command, args []string) error {
		return jobSearch(o, cmd, pattern, maxDepth, limit)
	}}
	c.Flags().StringVar(&pattern, "pattern", "", "Case-insensitive glob (*, ?, [...]) matched against the full slash path (folder/sub/job) and, when it contains no slash, against the job name; * also spans folder separators.")
	c.Flags().IntVar(&maxDepth, "max-depth", jobSearchDefaultDepth, "Folder nesting levels to expand (1-6); folders at the limit are listed but their contents are not.")
	c.Flags().IntVar(&limit, "limit", jobSearchDefaultLimit, "Maximum matching jobs to return (1-1000).")
	return c
}

func jobSearch(o *Opts, cmd *cobra.Command, pattern string, maxDepth, limit int) error {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return print(cmd, o, output.Failure("invalid_args", "--pattern is required", "Pass --pattern with a glob such as \"*deploy*\" or \"deploy/*\".", 400))
	}
	glob, err := newJobGlob(pattern)
	if err != nil {
		return print(cmd, o, output.Failure("invalid_args", fmt.Sprintf("invalid --pattern %q: %v", pattern, err), "Use *, ?, and [...] glob syntax; escape a literal special character with a backslash.", 400))
	}
	if maxDepth <= 0 {
		maxDepth = jobSearchDefaultDepth
	}
	if maxDepth > jobSearchMaxDepth {
		maxDepth = jobSearchMaxDepth
	}
	if limit <= 0 {
		limit = jobSearchDefaultLimit
	}
	if limit > jobSearchMaxLimit {
		limit = jobSearchMaxLimit
	}
	cx, err := loadCtx(o, "")
	if err != nil {
		return print(cmd, o, envelopeError(err, "config_error"))
	}
	resp, err := cx.client.Do(jenkins.Request{Method: http.MethodGet, Path: "/api/json", Query: map[string]string{"tree": jenkinsJobsTree(maxDepth)}})
	if err != nil {
		return print(cmd, o, envelopeError(err, "server_error"))
	}
	defer resp.Body.Close()
	data := jenkins.JSONMap(resp.Body)
	jobs := make([]map[string]any, 0)
	scanned, matched, unexpanded := 0, 0, 0
	var walk func(items []any, parent string, depth int)
	walk = func(items []any, parent string, depth int) {
		for _, raw := range items {
			job, _ := raw.(map[string]any)
			if job == nil {
				continue
			}
			name, _ := job["name"].(string)
			if name == "" {
				continue
			}
			fullPath := name
			if parent != "" {
				fullPath = parent + "/" + name
			}
			scanned++
			class, _ := job["_class"].(string)
			children, hasChildren := job["jobs"].([]any)
			folder := hasChildren || isJenkinsFolderClass(class)
			if glob.match(fullPath, name) {
				matched++
				if len(jobs) < limit {
					buildable, _ := job["buildable"].(bool)
					jobs = append(jobs, map[string]any{"path": fullPath, "url": job["url"], "class": class, "buildable": buildable, "folder": folder})
				}
			}
			if !folder {
				continue
			}
			if hasChildren {
				walk(children, fullPath, depth+1)
			} else if depth >= maxDepth {
				unexpanded++
			}
		}
	}
	walk(listAny(data["jobs"]), "", 1)
	return print(cmd, o, output.Success(cx.inst.Name, map[string]any{
		"pattern":            pattern,
		"jobs":               jobs,
		"count_returned":     len(jobs),
		"matched":            matched,
		"scanned":            scanned,
		"truncated":          matched > len(jobs),
		"max_depth":          maxDepth,
		"unexpanded_folders": unexpanded,
	}))
}

// jenkinsJobsTree nests the jobs[...] selector depth levels deep so one
// request returns the whole folder tree down to that depth.
func jenkinsJobsTree(depth int) string {
	tree := "jobs[" + jobSearchFields + "]"
	for i := 1; i < depth; i++ {
		tree = "jobs[" + jobSearchFields + "," + tree + "]"
	}
	return tree
}

func isJenkinsFolderClass(class string) bool {
	for _, marker := range []string{"Folder", "WorkflowMultiBranchProject", "OrganizationFolder"} {
		if strings.Contains(class, marker) {
			return true
		}
	}
	return false
}

// jobGlob matches path.Match-style globs case-insensitively. The slash in
// job paths is swapped for a stand-in byte before matching so that * spans
// folder boundaries ("*deploy*" finds deploy/payments-api); a pattern with
// no slash is additionally tried against the bare job name.
type jobGlob struct {
	pattern   string
	flat      string
	leafMatch bool
}

const jobPathSeparatorStandIn = "\x1f"

func newJobGlob(pattern string) (*jobGlob, error) {
	lower := strings.ToLower(pattern)
	flat := flattenJobPath(lower)
	if _, err := path.Match(flat, ""); err != nil {
		return nil, err
	}
	return &jobGlob{pattern: lower, flat: flat, leafMatch: !strings.Contains(pattern, "/")}, nil
}

func (g *jobGlob) match(fullPath, leaf string) bool {
	if ok, _ := path.Match(g.flat, flattenJobPath(strings.ToLower(fullPath))); ok {
		return true
	}
	if !g.leafMatch {
		return false
	}
	ok, _ := path.Match(g.pattern, strings.ToLower(leaf))
	return ok
}

func flattenJobPath(p string) string {
	return strings.ReplaceAll(p, "/", jobPathSeparatorStandIn)
}
