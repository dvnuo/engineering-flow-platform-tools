package commands

import (
	"encoding/json"
	"html"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"engineering-flow-platform-tools/internal/httpclient"
	"engineering-flow-platform-tools/internal/output"
	"github.com/spf13/cobra"
)

func pageCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "page"}
	get := &cobra.Command{Use: "get", RunE: func(cmd *cobra.Command, args []string) error {
		ref, err := resolvePageRef(cmd, o)
		if err != nil {
			return printPageIDError(cmd, o, err, "exactly one of --id/--url")
		}
		q := map[string]string{}
		if expand, _ := cmd.Flags().GetString("expand"); expand != "" {
			q["expand"] = expand
		}
		return doWithCtx(o, cmd, ref.Ctx, "GET", "content/"+ref.ID, q, nil)
	}}
	get.Flags().String("id", "", "")
	get.Flags().String("url", "", "")
	get.Flags().String("expand", "", "")
	c.AddCommand(get)
	gbt := &cobra.Command{Use: "get-by-title", RunE: func(cmd *cobra.Command, args []string) error {
		sp, _ := cmd.Flags().GetString("space")
		ti, _ := cmd.Flags().GetString("title")
		if sp == "" || ti == "" {
			return print(cmd, o, output.Failure("invalid_args", "--space and --title required", "confluence page get-by-title --space ENG --title 'Runtime Profile' --json", 400))
		}
		q := map[string]string{"spaceKey": sp, "title": ti, "type": "page"}
		for _, k := range []string{"expand", "limit"} {
			if v, _ := cmd.Flags().GetString(k); v != "" {
				q[k] = v
			}
		}
		return do(o, cmd, "GET", "content", q, nil)
	}}
	gbt.Flags().String("space", "", "")
	gbt.Flags().String("title", "", "")
	gbt.Flags().String("expand", "", "")
	gbt.Flags().String("limit", "", "")
	c.AddCommand(gbt)
	cr := &cobra.Command{Use: "create", RunE: func(cmd *cobra.Command, args []string) error {
		sp, _ := cmd.Flags().GetString("space")
		ti, _ := cmd.Flags().GetString("title")
		b, err := readBody(cmd)
		if err != nil {
			return print(cmd, o, output.Failure("invalid_args", err.Error(), "", 400))
		}
		if sp == "" || ti == "" || b == "" {
			return print(cmd, o, output.Failure("invalid_args", "missing required args", "", 400))
		}
		payload := map[string]any{"type": "page", "title": ti, "space": map[string]string{"key": sp}, "body": confluenceBody(cmd, b)}
		if parentID, _ := cmd.Flags().GetString("parent-id"); parentID != "" {
			payload["ancestors"] = []map[string]string{{"id": parentID}}
		}
		return do(o, cmd, "POST", "content", nil, payload)
	}}
	cr.Flags().String("space", "", "")
	cr.Flags().String("title", "", "")
	cr.Flags().String("parent-id", "", "")
	cr.Flags().String("body", "", "")
	cr.Flags().String("body-file", "", "")
	cr.Flags().Bool("body-stdin", false, "")
	cr.Flags().String("body-format", "storage", "")
	c.AddCommand(cr)
	upd := &cobra.Command{Use: "update", RunE: func(cmd *cobra.Command, args []string) error {
		ref, err := resolvePageRef(cmd, o)
		if err != nil {
			return printPageIDError(cmd, o, err, "exactly one of --id/--url")
		}
		v, _ := cmd.Flags().GetInt("version")
		if v == 0 && !o.DryRun {
			r, e := ref.Ctx.client.Do(httpclient.Request{Method: "GET", Path: "content/" + ref.ID, Query: map[string]string{"expand": "version"}})
			if e != nil {
				return print(cmd, o, output.Failure("server_error", "version fetch failed: "+httpclient.SanitizeErrorText(e.Error()), "Retry with --version or inspect the upstream error.", 500))
			}
			defer r.Body.Close()
			var m map[string]any
			_ = json.NewDecoder(r.Body).Decode(&m)
			vm, ok := m["version"].(map[string]any)
			if !ok {
				return print(cmd, o, output.Failure("server_error", "version.number missing", "Retry with --version.", 500))
			}
			num, ok := vm["number"].(float64)
			if !ok {
				return print(cmd, o, output.Failure("server_error", "version.number missing", "Retry with --version.", 500))
			}
			v = int(num) + 1
		}
		payload := map[string]any{"version": map[string]any{"number": v}}
		if minor, _ := cmd.Flags().GetBool("minor-edit"); minor {
			payload["version"].(map[string]any)["minorEdit"] = true
		}
		if t, _ := cmd.Flags().GetString("title"); t != "" {
			payload["title"] = t
		}
		b, err := readBody(cmd)
		if err != nil {
			return print(cmd, o, output.Failure("invalid_args", err.Error(), "", 400))
		}
		if b != "" {
			payload["body"] = confluenceBody(cmd, b)
		}
		return doWithCtx(o, cmd, ref.Ctx, "PUT", "content/"+ref.ID, nil, payload)
	}}
	upd.Flags().String("id", "", "")
	upd.Flags().String("url", "", "")
	upd.Flags().Int("version", 0, "")
	upd.Flags().String("title", "", "")
	upd.Flags().Bool("minor-edit", false, "")
	upd.Flags().String("body", "", "")
	upd.Flags().String("body-file", "", "")
	upd.Flags().Bool("body-stdin", false, "")
	upd.Flags().String("body-format", "storage", "")
	c.AddCommand(upd)
	del := &cobra.Command{Use: "delete", RunE: func(cmd *cobra.Command, args []string) error {
		if !o.Yes {
			return print(cmd, o, output.Failure("invalid_args", "--yes required", "", 400))
		}
		ref, err := resolvePageRef(cmd, o)
		if err != nil {
			return printPageIDError(cmd, o, err, "exactly one of --id/--url")
		}
		return doWithCtx(o, cmd, ref.Ctx, "DELETE", "content/"+ref.ID, nil, nil)
	}}
	del.Flags().String("id", "", "")
	del.Flags().String("url", "", "")
	c.AddCommand(del)
	ex := &cobra.Command{Use: "export-markdown", RunE: func(cmd *cobra.Command, args []string) error {
		ref, err := resolvePageRef(cmd, o)
		if err != nil {
			return printPageIDError(cmd, o, err, "exactly one of --id/--url")
		}
		r, e := ref.Ctx.client.Do(httpclient.Request{Method: "GET", Path: "content/" + ref.ID, Query: map[string]string{"expand": "body.view"}})
		if e != nil {
			return print(cmd, o, output.Failure("server_error", e.Error(), "", 500))
		}
		defer r.Body.Close()
		var m map[string]any
		_ = json.NewDecoder(r.Body).Decode(&m)
		html, ok := nestedString(m, "body", "view", "value")
		if !ok {
			return print(cmd, o, output.Failure("not_found", "page view body missing", "Retry with a page that has body.view available.", 404))
		}
		md := htmlToMarkdown(html)
		out, _ := cmd.Flags().GetString("output")
		if out != "" {
			if err := os.WriteFile(out, []byte(md), 0644); err != nil {
				return print(cmd, o, output.Failure("invalid_args", "failed to write --output: "+httpclient.SanitizeErrorText(err.Error()), "Choose a writable output path.", 400))
			}
			return print(cmd, o, output.Success(ref.Ctx.inst.Name, map[string]any{"output": filepath.Clean(out)}))
		}
		return print(cmd, o, output.Success(ref.Ctx.inst.Name, map[string]any{"markdown": md}))
	}}
	ex.Flags().String("id", "", "")
	ex.Flags().String("url", "", "")
	ex.Flags().String("output", "", "")
	c.AddCommand(ex)
	exh := &cobra.Command{Use: "export-html", RunE: func(cmd *cobra.Command, args []string) error {
		ref, err := resolvePageRef(cmd, o)
		if err != nil {
			return printPageIDError(cmd, o, err, "exactly one of --id/--url")
		}
		r, e := ref.Ctx.client.Do(httpclient.Request{Method: "GET", Path: "content/" + ref.ID, Query: map[string]string{"expand": "body.export_view"}})
		if e != nil {
			return print(cmd, o, output.Failure("server_error", e.Error(), "", 500))
		}
		defer r.Body.Close()
		var m map[string]any
		_ = json.NewDecoder(r.Body).Decode(&m)
		html, ok := nestedString(m, "body", "export_view", "value")
		if !ok {
			return print(cmd, o, output.Failure("not_found", "page export body missing", "Retry with a page that has body.export_view available.", 404))
		}
		out, _ := cmd.Flags().GetString("output")
		if out != "" {
			if err := os.WriteFile(out, []byte(html), 0644); err != nil {
				return print(cmd, o, output.Failure("invalid_args", "failed to write --output: "+httpclient.SanitizeErrorText(err.Error()), "Choose a writable output path.", 400))
			}
			return print(cmd, o, output.Success(ref.Ctx.inst.Name, map[string]any{"output": filepath.Clean(out)}))
		}
		return print(cmd, o, output.Success(ref.Ctx.inst.Name, map[string]any{"html": html}))
	}}
	exh.Flags().String("id", "", "")
	exh.Flags().String("url", "", "")
	exh.Flags().String("output", "", "")
	c.AddCommand(exh)
	c.AddCommand(&cobra.Command{Use: "body-storage", RunE: func(cmd *cobra.Command, args []string) error {
		return doPageContent(o, cmd, "GET", "", map[string]string{"expand": "body.storage"}, nil)
	}})
	addPageIDURLFlags(c.Commands()[len(c.Commands())-1])
	c.AddCommand(&cobra.Command{Use: "body-view", RunE: func(cmd *cobra.Command, args []string) error {
		return doPageContent(o, cmd, "GET", "", map[string]string{"expand": "body.view"}, nil)
	}})
	addPageIDURLFlags(c.Commands()[len(c.Commands())-1])
	labelList := &cobra.Command{Use: "list", RunE: func(cmd *cobra.Command, args []string) error {
		return doPageContent(o, cmd, "GET", "/label", nil, nil)
	}}
	labelAdd := &cobra.Command{Use: "add", RunE: func(cmd *cobra.Command, args []string) error {
		labels, _ := cmd.Flags().GetStringSlice("label")
		if len(labels) == 0 {
			return print(cmd, o, output.Failure("invalid_args", "--label required", "", 400))
		}
		arr := []map[string]string{}
		for _, l := range labels {
			arr = append(arr, map[string]string{"prefix": "global", "name": l})
		}
		return doPageContent(o, cmd, "POST", "/label", nil, arr)
	}}
	labelAdd.Flags().StringSlice("label", nil, "")
	labelDelete := &cobra.Command{Use: "delete", RunE: func(cmd *cobra.Command, args []string) error {
		if !o.Yes {
			return print(cmd, o, output.Failure("invalid_args", "--yes required", "", 400))
		}
		n, _ := cmd.Flags().GetString("label")
		if n == "" {
			return print(cmd, o, output.Failure("invalid_args", "--label required", "", 400))
		}
		return doPageContent(o, cmd, "DELETE", "/label", map[string]string{"name": n}, nil)
	}}
	labelDelete.Flags().String("label", "", "")
	label := &cobra.Command{Use: "label"}
	label.AddCommand(labelList, labelAdd, labelDelete)
	for _, pc := range label.Commands() {
		pc.Flags().String("id", "", "")
		pc.Flags().String("url", "", "")
	}
	c.AddCommand(label)
	c.AddCommand(hiddenAlias("label-list", labelList))
	c.AddCommand(hiddenAlias("label-add", labelAdd))
	c.AddCommand(hiddenAlias("label-delete", labelDelete))

	commentList := &cobra.Command{Use: "list", RunE: func(cmd *cobra.Command, args []string) error {
		return doPageContent(o, cmd, "GET", "/child/comment", nil, nil)
	}}
	commentAdd := &cobra.Command{Use: "add", RunE: func(cmd *cobra.Command, args []string) error {
		b, err := readBody(cmd)
		if err != nil {
			return print(cmd, o, output.Failure("invalid_args", err.Error(), "", 400))
		}
		if b == "" {
			return print(cmd, o, output.Failure("invalid_args", "--body, --body-file, or --body-stdin required", "", 400))
		}
		return doPageContent(o, cmd, "POST", "/child/comment", nil, map[string]any{"type": "comment", "body": confluenceBody(cmd, b)})
	}}
	commentAdd.Flags().String("body", "", "")
	commentAdd.Flags().String("body-file", "", "")
	commentAdd.Flags().Bool("body-stdin", false, "")
	commentAdd.Flags().String("body-format", "storage", "")
	comment := &cobra.Command{Use: "comment"}
	comment.AddCommand(commentList, commentAdd)
	for _, pc := range comment.Commands() {
		pc.Flags().String("id", "", "")
		pc.Flags().String("url", "", "")
	}
	c.AddCommand(comment)
	c.AddCommand(hiddenAlias("comment-list", commentList))
	c.AddCommand(hiddenAlias("comment-add", commentAdd))
	prop := &cobra.Command{Use: "property"}
	prop.AddCommand(&cobra.Command{Use: "list", RunE: func(cmd *cobra.Command, args []string) error {
		return doPageContent(o, cmd, "GET", "/property", nil, nil)
	}})
	prop.AddCommand(&cobra.Command{Use: "get", RunE: func(cmd *cobra.Command, args []string) error {
		key := mustS(cmd, "key")
		if key == "" {
			return print(cmd, o, output.Failure("invalid_args", "--key required", "", 400))
		}
		return doPageContent(o, cmd, "GET", "/property/"+key, nil, nil)
	}})
	prop.Commands()[1].Flags().String("key", "", "")
	prop.AddCommand(&cobra.Command{Use: "set", RunE: func(cmd *cobra.Command, args []string) error {
		key := mustS(cmd, "key")
		b, err := readBody(cmd)
		if err != nil {
			return print(cmd, o, output.Failure("invalid_args", err.Error(), "", 400))
		}
		if key == "" || b == "" {
			return print(cmd, o, output.Failure("invalid_args", "--key and body source required", "", 400))
		}
		return doPageContent(o, cmd, "PUT", "/property/"+key, nil, map[string]any{"key": key, "value": b})
	}})
	prop.Commands()[2].Flags().String("key", "", "")
	prop.Commands()[2].Flags().String("body", "", "")
	prop.Commands()[2].Flags().String("body-file", "", "")
	prop.Commands()[2].Flags().Bool("body-stdin", false, "")
	prop.AddCommand(&cobra.Command{Use: "delete", RunE: func(cmd *cobra.Command, args []string) error {
		key := mustS(cmd, "key")
		if !o.Yes {
			return print(cmd, o, output.Failure("invalid_args", "--yes required", "", 400))
		}
		if key == "" {
			return print(cmd, o, output.Failure("invalid_args", "--key required", "", 400))
		}
		return doPageContent(o, cmd, "DELETE", "/property/"+key, nil, nil)
	}})
	prop.Commands()[3].Flags().String("key", "", "")
	for _, pc := range prop.Commands() {
		pc.Flags().String("id", "", "")
		pc.Flags().String("url", "", "")
	}
	c.AddCommand(prop)
	c.AddCommand(&cobra.Command{Use: "children", RunE: func(cmd *cobra.Command, args []string) error {
		return doPageContent(o, cmd, "GET", "/child", nil, nil)
	}})
	addPageIDURLFlags(c.Commands()[len(c.Commands())-1])
	c.AddCommand(&cobra.Command{Use: "descendants", RunE: func(cmd *cobra.Command, args []string) error {
		return doPageContent(o, cmd, "GET", "/descendant", nil, nil)
	}})
	addPageIDURLFlags(c.Commands()[len(c.Commands())-1])
	c.AddCommand(&cobra.Command{Use: "ancestors", RunE: func(cmd *cobra.Command, args []string) error {
		return doPageContent(o, cmd, "GET", "", map[string]string{"expand": "ancestors"}, nil)
	}})
	addPageIDURLFlags(c.Commands()[len(c.Commands())-1])
	c.AddCommand(&cobra.Command{Use: "body", RunE: func(cmd *cobra.Command, args []string) error {
		f, _ := cmd.Flags().GetString("format")
		if f == "" {
			f = "storage"
		}
		return doPageContent(o, cmd, "GET", "", map[string]string{"expand": "body." + f}, nil)
	}})
	c.Commands()[len(c.Commands())-1].Flags().String("format", "storage", "")
	addPageIDURLFlags(c.Commands()[len(c.Commands())-1])
	c.AddCommand(&cobra.Command{Use: "history", RunE: func(cmd *cobra.Command, args []string) error {
		return doPageContent(o, cmd, "GET", "/version", nil, nil)
	}})
	addPageIDURLFlags(c.Commands()[len(c.Commands())-1])
	c.AddCommand(&cobra.Command{Use: "version", RunE: func(cmd *cobra.Command, args []string) error {
		return doPageContent(o, cmd, "GET", "/version/latest", nil, nil)
	}})
	addPageIDURLFlags(c.Commands()[len(c.Commands())-1])
	c.AddCommand(&cobra.Command{Use: "restore", RunE: func(cmd *cobra.Command, args []string) error {
		v, _ := cmd.Flags().GetInt("version")
		if v <= 0 {
			return print(cmd, o, output.Failure("invalid_args", "--version must be greater than 0", "", 400))
		}
		return doPageContent(o, cmd, "POST", "/version", nil, map[string]any{"operationKey": "restore", "params": map[string]any{"versionNumber": v}})
	}})
	c.Commands()[len(c.Commands())-1].Flags().Int("version", 0, "")
	addPageIDURLFlags(c.Commands()[len(c.Commands())-1])
	c.AddCommand(&cobra.Command{Use: "move", RunE: func(cmd *cobra.Command, args []string) error {
		pid, _ := cmd.Flags().GetString("parent-id")
		if pid == "" {
			return print(cmd, o, output.Failure("invalid_args", "--parent-id required", "", 400))
		}
		pos, _ := cmd.Flags().GetString("position")
		return doPagePath(o, cmd, "PUT", func(id string) string { return "content/" + id + "/move/" + pos + "/" + pid }, nil, nil)
	}})
	c.Commands()[len(c.Commands())-1].Flags().String("parent-id", "", "")
	c.Commands()[len(c.Commands())-1].Flags().String("position", "append", "")
	addPageIDURLFlags(c.Commands()[len(c.Commands())-1])
	c.AddCommand(&cobra.Command{Use: "watch", RunE: func(cmd *cobra.Command, args []string) error {
		return doPagePath(o, cmd, "POST", func(id string) string { return "user/watch/content/" + id }, nil, nil)
	}})
	addPageIDURLFlags(c.Commands()[len(c.Commands())-1])
	c.AddCommand(&cobra.Command{Use: "unwatch", RunE: func(cmd *cobra.Command, args []string) error {
		return doPagePath(o, cmd, "DELETE", func(id string) string { return "user/watch/content/" + id }, nil, nil)
	}})
	addPageIDURLFlags(c.Commands()[len(c.Commands())-1])
	restriction := &cobra.Command{Use: "restriction"}
	restriction.AddCommand(&cobra.Command{Use: "list", RunE: func(cmd *cobra.Command, args []string) error {
		return doPageContent(o, cmd, "GET", "/restriction/byOperation", nil, nil)
	}})
	restriction.AddCommand(&cobra.Command{Use: "add", RunE: func(cmd *cobra.Command, args []string) error {
		op := mustS(cmd, "operation")
		if !validRestrictionOperation(op) {
			return print(cmd, o, output.Failure("invalid_args", "--operation must be read or update", "", 400))
		}
		users, _ := cmd.Flags().GetStringSlice("user")
		groups, _ := cmd.Flags().GetStringSlice("group")
		if len(users) == 0 && len(groups) == 0 {
			return print(cmd, o, output.Failure("invalid_args", "--user or --group required", "", 400))
		}
		return doPageContent(o, cmd, "POST", "/restriction/byOperation", nil, map[string]any{"operation": op, "restrictions": map[string]any{"user": users, "group": groups}})
	}})
	restriction.Commands()[1].Flags().String("operation", "read", "")
	restriction.Commands()[1].Flags().StringSlice("user", nil, "")
	restriction.Commands()[1].Flags().StringSlice("group", nil, "")
	restriction.AddCommand(&cobra.Command{Use: "delete", RunE: func(cmd *cobra.Command, args []string) error {
		if !o.Yes {
			return print(cmd, o, output.Failure("invalid_args", "--yes required", "", 400))
		}
		op := mustS(cmd, "operation")
		if !validRestrictionOperation(op) {
			return print(cmd, o, output.Failure("invalid_args", "--operation must be read or update", "", 400))
		}
		return doPageContent(o, cmd, "DELETE", "/restriction/byOperation/"+op, nil, nil)
	}})
	restriction.Commands()[2].Flags().String("operation", "read", "")
	for _, pc := range restriction.Commands() {
		pc.Flags().String("id", "", "")
		pc.Flags().String("url", "", "")
	}
	c.AddCommand(restriction)
	watcher := &cobra.Command{Use: "watcher"}
	watcher.AddCommand(&cobra.Command{Use: "list", RunE: func(cmd *cobra.Command, args []string) error {
		return doPagePath(o, cmd, "GET", func(id string) string { return "user/watch/content/" + id }, nil, nil)
	}})
	watcher.Commands()[0].Flags().String("id", "", "")
	watcher.Commands()[0].Flags().String("url", "", "")
	c.AddCommand(watcher)
	c.AddCommand(pageAttachmentCmd(o))
	for _, pc := range []*cobra.Command{c.Commands()[len(c.Commands())-11], c.Commands()[len(c.Commands())-10], c.Commands()[len(c.Commands())-9], c.Commands()[len(c.Commands())-8], c.Commands()[len(c.Commands())-7], c.Commands()[len(c.Commands())-6], c.Commands()[len(c.Commands())-5], c.Commands()[len(c.Commands())-4], c.Commands()[len(c.Commands())-3], c.Commands()[len(c.Commands())-2], c.Commands()[len(c.Commands())-1]} {
		if pc.Flags().Lookup("id") == nil {
			pc.Flags().String("id", "", "")
		}
		if pc.Flags().Lookup("url") == nil {
			pc.Flags().String("url", "", "")
		}
	}
	return c
}

func addPageIDURLFlags(cmd *cobra.Command) {
	if cmd.Flags().Lookup("id") == nil {
		cmd.Flags().String("id", "", "")
	}
	if cmd.Flags().Lookup("url") == nil {
		cmd.Flags().String("url", "", "")
	}
}

func doPageContent(o *Opts, cmd *cobra.Command, method, suffix string, q map[string]string, body any) error {
	ref, err := resolvePageRef(cmd, o)
	if err != nil {
		return printPageIDError(cmd, o, err, "--id/--url required")
	}
	return doWithCtx(o, cmd, ref.Ctx, method, "content/"+ref.ID+suffix, q, body)
}

func doPagePath(o *Opts, cmd *cobra.Command, method string, path func(string) string, q map[string]string, body any) error {
	ref, err := resolvePageRef(cmd, o)
	if err != nil {
		return printPageIDError(cmd, o, err, "--id/--url required")
	}
	return doWithCtx(o, cmd, ref.Ctx, method, path(ref.ID), q, body)
}

func validRestrictionOperation(op string) bool {
	return op == "read" || op == "update"
}

func printPageIDError(cmd *cobra.Command, o *Opts, err error, message string) error {
	switch err.Error() {
	case "instance_url_mismatch":
		return print(cmd, o, output.Failure("instance_url_mismatch", "off-instance url", "", 400))
	case "not_found":
		return print(cmd, o, output.Failure("not_found", "page not found", "Check the page URL or title.", 404))
	case "server_error":
		return print(cmd, o, output.Failure("server_error", "page lookup failed", "", 500))
	case "config_missing", "no_instance_configured", "instance_required", "ambiguous_instance", "auth_failed", "permission_denied", "network_error", "not_supported":
		return print(cmd, o, output.Failure(err.Error(), err.Error(), "", 400))
	default:
		return print(cmd, o, output.Failure("invalid_args", message, "", 400))
	}
}

func htmlToMarkdown(src string) string {
	replacements := []struct {
		re   *regexp.Regexp
		with string
	}{
		{regexp.MustCompile(`(?i)<br\s*/?>`), "\n"},
		{regexp.MustCompile(`(?i)</p\s*>`), "\n\n"},
		{regexp.MustCompile(`(?i)</h[1-6]\s*>`), "\n\n"},
		{regexp.MustCompile(`(?i)<li[^>]*>`), "- "},
		{regexp.MustCompile(`(?i)</li\s*>`), "\n"},
	}
	out := src
	for _, r := range replacements {
		out = r.re.ReplaceAllString(out, r.with)
	}
	out = regexp.MustCompile(`<[^>]+>`).ReplaceAllString(out, "")
	out = html.UnescapeString(out)
	lines := strings.Split(out, "\n")
	for i := range lines {
		lines[i] = strings.TrimSpace(lines[i])
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func nestedString(m map[string]any, keys ...string) (string, bool) {
	var cur any = m
	for i, k := range keys {
		obj, ok := cur.(map[string]any)
		if !ok {
			return "", false
		}
		next, ok := obj[k]
		if !ok {
			return "", false
		}
		if i == len(keys)-1 {
			s, ok := next.(string)
			return s, ok
		}
		cur = next
	}
	return "", false
}
