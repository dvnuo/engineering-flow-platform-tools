package commands

import (
	"encoding/json"
	"os"
	"path/filepath"

	"engineering-flow-platform-tools/internal/httpclient"
	"engineering-flow-platform-tools/internal/output"
	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
	"github.com/spf13/cobra"
)

func pageCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "page"}
	get := &cobra.Command{Use: "get", RunE: func(cmd *cobra.Command, args []string) error {
		id, err := pageID(cmd, o)
		if err != nil {
			if err.Error() == "instance_url_mismatch" {
				return print(cmd, o, output.Failure("instance_url_mismatch", "off-instance url", "", 400))
			}
			return print(cmd, o, output.Failure("invalid_args", "exactly one of --id/--url", "", 400))
		}
		q := map[string]string{}
		if expand, _ := cmd.Flags().GetString("expand"); expand != "" {
			q["expand"] = expand
		}
		return do(o, cmd, "GET", "content/"+id, q, nil)
	}}
	get.Flags().String("id", "", "")
	get.Flags().String("url", "", "")
	get.Flags().String("expand", "", "")
	c.AddCommand(get)
	gbt := &cobra.Command{Use: "get-by-title", RunE: func(cmd *cobra.Command, args []string) error {
		sp, _ := cmd.Flags().GetString("space")
		ti, _ := cmd.Flags().GetString("title")
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
		b := readBody(cmd)
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
		id, err := pageID(cmd, o)
		if err != nil {
			return print(cmd, o, output.Failure("invalid_args", "exactly one of --id/--url", "", 400))
		}
		v, _ := cmd.Flags().GetInt("version")
		cx, _ := loadCtx(o, "")
		if v == 0 && !o.DryRun {
			r, e := cx.client.Do(httpclient.Request{Method: "GET", Path: "content/" + id})
			if e != nil {
				return print(cmd, o, output.Failure("server_error", "version fetch failed", "", 500))
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
		if b := readBody(cmd); b != "" {
			payload["body"] = confluenceBody(cmd, b)
		}
		return do(o, cmd, "PUT", "content/"+id, nil, payload)
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
		id, err := pageID(cmd, o)
		if err != nil {
			return print(cmd, o, output.Failure("invalid_args", "exactly one of --id/--url", "", 400))
		}
		return do(o, cmd, "DELETE", "content/"+id, nil, nil)
	}}
	del.Flags().String("id", "", "")
	del.Flags().String("url", "", "")
	c.AddCommand(del)
	ex := &cobra.Command{Use: "export-markdown", RunE: func(cmd *cobra.Command, args []string) error {
		id, err := pageID(cmd, o)
		if err != nil {
			return err
		}
		cx, _ := loadCtx(o, "")
		r, e := cx.client.Do(httpclient.Request{Method: "GET", Path: "content/" + id, Query: map[string]string{"expand": "body.view"}})
		if e != nil {
			return print(cmd, o, output.Failure("server_error", e.Error(), "", 500))
		}
		defer r.Body.Close()
		var m map[string]any
		_ = json.NewDecoder(r.Body).Decode(&m)
		html := m["body"].(map[string]any)["view"].(map[string]any)["value"].(string)
		md, _ := htmltomarkdown.ConvertString(html)
		out, _ := cmd.Flags().GetString("output")
		if out != "" {
			_ = os.WriteFile(out, []byte(md), 0644)
			return print(cmd, o, output.Success(cx.inst.Name, map[string]any{"output": filepath.Clean(out)}))
		}
		return print(cmd, o, output.Success(cx.inst.Name, map[string]any{"markdown": md}))
	}}
	ex.Flags().String("id", "", "")
	ex.Flags().String("url", "", "")
	ex.Flags().String("output", "", "")
	c.AddCommand(ex)
	exh := &cobra.Command{Use: "export-html", RunE: func(cmd *cobra.Command, args []string) error {
		id, err := pageID(cmd, o)
		if err != nil {
			return err
		}
		cx, _ := loadCtx(o, "")
		r, e := cx.client.Do(httpclient.Request{Method: "GET", Path: "content/" + id, Query: map[string]string{"expand": "body.export_view"}})
		if e != nil {
			return print(cmd, o, output.Failure("server_error", e.Error(), "", 500))
		}
		defer r.Body.Close()
		var m map[string]any
		_ = json.NewDecoder(r.Body).Decode(&m)
		html := m["body"].(map[string]any)["export_view"].(map[string]any)["value"].(string)
		out, _ := cmd.Flags().GetString("output")
		if out != "" {
			_ = os.WriteFile(out, []byte(html), 0644)
			return print(cmd, o, output.Success(cx.inst.Name, map[string]any{"output": filepath.Clean(out)}))
		}
		return print(cmd, o, output.Success(cx.inst.Name, map[string]any{"html": html}))
	}}
	exh.Flags().String("id", "", "")
	exh.Flags().String("url", "", "")
	exh.Flags().String("output", "", "")
	c.AddCommand(exh)
	c.AddCommand(&cobra.Command{Use: "body-storage", RunE: func(cmd *cobra.Command, args []string) error {
		id, e := pageID(cmd, o)
		if e != nil {
			return print(cmd, o, output.Failure("invalid_args", "--id/--url required", "", 400))
		}
		return do(o, cmd, "GET", "content/"+id, map[string]string{"expand": "body.storage"}, nil)
	}})
	c.AddCommand(&cobra.Command{Use: "body-view", RunE: func(cmd *cobra.Command, args []string) error {
		id, e := pageID(cmd, o)
		if e != nil {
			return print(cmd, o, output.Failure("invalid_args", "--id/--url required", "", 400))
		}
		return do(o, cmd, "GET", "content/"+id, map[string]string{"expand": "body.view"}, nil)
	}})
	labelList := &cobra.Command{Use: "list", RunE: func(cmd *cobra.Command, args []string) error {
		id, e := pageID(cmd, o)
		if e != nil {
			return print(cmd, o, output.Failure("invalid_args", "--id/--url required", "", 400))
		}
		return do(o, cmd, "GET", "content/"+id+"/label", nil, nil)
	}}
	labelAdd := &cobra.Command{Use: "add", RunE: func(cmd *cobra.Command, args []string) error {
		id, e := pageID(cmd, o)
		if e != nil {
			return print(cmd, o, output.Failure("invalid_args", "--id/--url required", "", 400))
		}
		labels, _ := cmd.Flags().GetStringSlice("label")
		arr := []map[string]string{}
		for _, l := range labels {
			arr = append(arr, map[string]string{"prefix": "global", "name": l})
		}
		return do(o, cmd, "POST", "content/"+id+"/label", nil, arr)
	}}
	labelAdd.Flags().StringSlice("label", nil, "")
	labelDelete := &cobra.Command{Use: "delete", RunE: func(cmd *cobra.Command, args []string) error {
		if !o.Yes {
			return print(cmd, o, output.Failure("invalid_args", "--yes required", "", 400))
		}
		id, e := pageID(cmd, o)
		if e != nil {
			return print(cmd, o, output.Failure("invalid_args", "--id/--url required", "", 400))
		}
		n, _ := cmd.Flags().GetString("label")
		return do(o, cmd, "DELETE", "content/"+id+"/label", map[string]string{"name": n}, nil)
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
		id, e := pageID(cmd, o)
		if e != nil {
			return print(cmd, o, output.Failure("invalid_args", "--id/--url required", "", 400))
		}
		return do(o, cmd, "GET", "content/"+id+"/child/comment", nil, nil)
	}}
	commentAdd := &cobra.Command{Use: "add", RunE: func(cmd *cobra.Command, args []string) error {
		id, e := pageID(cmd, o)
		if e != nil {
			return print(cmd, o, output.Failure("invalid_args", "--id/--url required", "", 400))
		}
		b := readBody(cmd)
		return do(o, cmd, "POST", "content/"+id+"/child/comment", nil, map[string]any{"type": "comment", "body": confluenceBody(cmd, b)})
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
		id, err := pageID(cmd, o)
		if err != nil {
			return print(cmd, o, output.Failure("invalid_args", "--id/--url required", "", 400))
		}
		return do(o, cmd, "GET", "content/"+id+"/property", nil, nil)
	}})
	prop.AddCommand(&cobra.Command{Use: "get", RunE: func(cmd *cobra.Command, args []string) error {
		key := mustS(cmd, "key")
		id, err := pageID(cmd, o)
		if err != nil {
			return print(cmd, o, output.Failure("invalid_args", "--id/--url required", "", 400))
		}
		return do(o, cmd, "GET", "content/"+id+"/property/"+key, nil, nil)
	}})
	prop.Commands()[1].Flags().String("key", "", "")
	prop.AddCommand(&cobra.Command{Use: "set", RunE: func(cmd *cobra.Command, args []string) error {
		key := mustS(cmd, "key")
		id, err := pageID(cmd, o)
		if err != nil {
			return print(cmd, o, output.Failure("invalid_args", "--id/--url required", "", 400))
		}
		return do(o, cmd, "PUT", "content/"+id+"/property/"+key, nil, map[string]any{"key": key, "value": readBody(cmd)})
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
		id, err := pageID(cmd, o)
		if err != nil {
			return print(cmd, o, output.Failure("invalid_args", "--id/--url required", "", 400))
		}
		return do(o, cmd, "DELETE", "content/"+id+"/property/"+key, nil, nil)
	}})
	prop.Commands()[3].Flags().String("key", "", "")
	for _, pc := range prop.Commands() {
		pc.Flags().String("id", "", "")
		pc.Flags().String("url", "", "")
	}
	c.AddCommand(prop)
	c.AddCommand(&cobra.Command{Use: "children", RunE: func(cmd *cobra.Command, args []string) error {
		id, e := pageID(cmd, o)
		if e != nil {
			return print(cmd, o, output.Failure("invalid_args", "--id/--url required", "", 400))
		}
		return do(o, cmd, "GET", "content/"+id+"/child", nil, nil)
	}})
	c.AddCommand(&cobra.Command{Use: "descendants", RunE: func(cmd *cobra.Command, args []string) error {
		id, e := pageID(cmd, o)
		if e != nil {
			return print(cmd, o, output.Failure("invalid_args", "--id/--url required", "", 400))
		}
		return do(o, cmd, "GET", "content/"+id+"/descendant", nil, nil)
	}})
	c.AddCommand(&cobra.Command{Use: "ancestors", RunE: func(cmd *cobra.Command, args []string) error {
		id, e := pageID(cmd, o)
		if e != nil {
			return print(cmd, o, output.Failure("invalid_args", "--id/--url required", "", 400))
		}
		return do(o, cmd, "GET", "content/"+id, map[string]string{"expand": "ancestors"}, nil)
	}})
	c.AddCommand(&cobra.Command{Use: "body", RunE: func(cmd *cobra.Command, args []string) error {
		id, e := pageID(cmd, o)
		if e != nil {
			return print(cmd, o, output.Failure("invalid_args", "--id/--url required", "", 400))
		}
		f, _ := cmd.Flags().GetString("format")
		if f == "" {
			f = "storage"
		}
		return do(o, cmd, "GET", "content/"+id, map[string]string{"expand": "body." + f}, nil)
	}})
	c.Commands()[len(c.Commands())-1].Flags().String("format", "storage", "")
	c.AddCommand(&cobra.Command{Use: "history", RunE: func(cmd *cobra.Command, args []string) error {
		id, e := pageID(cmd, o)
		if e != nil {
			return print(cmd, o, output.Failure("invalid_args", "--id/--url required", "", 400))
		}
		return do(o, cmd, "GET", "content/"+id+"/version", nil, nil)
	}})
	c.AddCommand(&cobra.Command{Use: "version", RunE: func(cmd *cobra.Command, args []string) error {
		id, e := pageID(cmd, o)
		if e != nil {
			return print(cmd, o, output.Failure("invalid_args", "--id/--url required", "", 400))
		}
		return do(o, cmd, "GET", "content/"+id+"/version/latest", nil, nil)
	}})
	c.AddCommand(&cobra.Command{Use: "restore", RunE: func(cmd *cobra.Command, args []string) error {
		id, e := pageID(cmd, o)
		if e != nil {
			return print(cmd, o, output.Failure("invalid_args", "--id/--url required", "", 400))
		}
		v, _ := cmd.Flags().GetInt("version")
		return do(o, cmd, "POST", "content/"+id+"/version", nil, map[string]any{"operationKey": "restore", "params": map[string]any{"versionNumber": v}})
	}})
	c.Commands()[len(c.Commands())-1].Flags().Int("version", 0, "")
	c.AddCommand(&cobra.Command{Use: "move", RunE: func(cmd *cobra.Command, args []string) error {
		id, e := pageID(cmd, o)
		if e != nil {
			return print(cmd, o, output.Failure("invalid_args", "--id/--url required", "", 400))
		}
		pid, _ := cmd.Flags().GetString("parent-id")
		pos, _ := cmd.Flags().GetString("position")
		return do(o, cmd, "PUT", "content/"+id+"/move/"+pos+"/"+pid, nil, nil)
	}})
	c.Commands()[len(c.Commands())-1].Flags().String("parent-id", "", "")
	c.Commands()[len(c.Commands())-1].Flags().String("position", "append", "")
	c.AddCommand(&cobra.Command{Use: "watch", RunE: func(cmd *cobra.Command, args []string) error {
		id, e := pageID(cmd, o)
		if e != nil {
			return print(cmd, o, output.Failure("invalid_args", "--id/--url required", "", 400))
		}
		return do(o, cmd, "POST", "user/watch/content/"+id, nil, nil)
	}})
	c.AddCommand(&cobra.Command{Use: "unwatch", RunE: func(cmd *cobra.Command, args []string) error {
		id, e := pageID(cmd, o)
		if e != nil {
			return print(cmd, o, output.Failure("invalid_args", "--id/--url required", "", 400))
		}
		return do(o, cmd, "DELETE", "user/watch/content/"+id, nil, nil)
	}})
	restriction := &cobra.Command{Use: "restriction"}
	restriction.AddCommand(&cobra.Command{Use: "list", RunE: func(cmd *cobra.Command, args []string) error {
		id, e := pageID(cmd, o)
		if e != nil {
			return print(cmd, o, output.Failure("invalid_args", "--id/--url required", "", 400))
		}
		return do(o, cmd, "GET", "content/"+id+"/restriction/byOperation", nil, nil)
	}})
	restriction.AddCommand(&cobra.Command{Use: "add", RunE: func(cmd *cobra.Command, args []string) error {
		id, e := pageID(cmd, o)
		if e != nil {
			return print(cmd, o, output.Failure("invalid_args", "--id/--url required", "", 400))
		}
		return do(o, cmd, "POST", "content/"+id+"/restriction/byOperation", nil, map[string]any{"operation": mustS(cmd, "operation")})
	}})
	restriction.Commands()[1].Flags().String("operation", "read", "")
	restriction.AddCommand(&cobra.Command{Use: "delete", RunE: func(cmd *cobra.Command, args []string) error {
		if !o.Yes {
			return print(cmd, o, output.Failure("invalid_args", "--yes required", "", 400))
		}
		id, e := pageID(cmd, o)
		if e != nil {
			return print(cmd, o, output.Failure("invalid_args", "--id/--url required", "", 400))
		}
		return do(o, cmd, "DELETE", "content/"+id+"/restriction/byOperation/"+mustS(cmd, "operation"), nil, nil)
	}})
	restriction.Commands()[2].Flags().String("operation", "read", "")
	for _, pc := range restriction.Commands() {
		pc.Flags().String("id", "", "")
		pc.Flags().String("url", "", "")
	}
	c.AddCommand(restriction)
	watcher := &cobra.Command{Use: "watcher"}
	watcher.AddCommand(&cobra.Command{Use: "list", RunE: func(cmd *cobra.Command, args []string) error {
		id, e := pageID(cmd, o)
		if e != nil {
			return print(cmd, o, output.Failure("invalid_args", "--id/--url required", "", 400))
		}
		return do(o, cmd, "GET", "user/watch/content/"+id, nil, nil)
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
