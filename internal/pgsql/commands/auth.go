package commands

import (
	"strings"

	"engineering-flow-platform-tools/internal/output"
	"engineering-flow-platform-tools/internal/pgsql"
	"github.com/spf13/cobra"
)

func authCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "auth"}
	login := &cobra.Command{Use: "login", RunE: func(cmd *cobra.Command, args []string) error {
		if !mustB(cmd, "password-stdin") {
			return print(cmd, o, output.Failure("invalid_args", "--password-stdin required", "Pipe the password on stdin: printf '%s\\n' \"$PGPASSWORD\" | pgsql auth login --password-stdin --json.", 400))
		}
		cfg, err := loadCfg(o)
		if err != nil {
			return print(cmd, o, envelopeError(err))
		}
		inst, err := resolveInstance(cfg.Pgsql, o.Instance)
		if err != nil {
			return print(cmd, o, envelopeError(err))
		}
		idx := findInstance(cfg.Pgsql, inst.Name)
		secret, err := readSecretStdin(cmd)
		if err != nil {
			return print(cmd, o, output.Failure("invalid_args", err.Error(), "", 400))
		}
		if u := strings.TrimSpace(mustS(cmd, "username")); u != "" {
			cfg.Pgsql.Instances[idx].Username = u
		}
		cfg.Pgsql.Instances[idx].Password = secret
		if err := saveCfg(o, cfg); err != nil {
			return print(cmd, o, envelopeError(err))
		}
		return print(cmd, o, output.Success(inst.Name, map[string]any{"logged_in": true, "username": cfg.Pgsql.Instances[idx].Username}))
	}}
	login.Flags().String("username", "", "Database role to store alongside the password (keeps the configured role when omitted).")
	login.Flags().Bool("password-stdin", false, "Read the role password from stdin.")
	c.AddCommand(login)
	c.AddCommand(&cobra.Command{Use: "logout", RunE: func(cmd *cobra.Command, args []string) error {
		if !o.Yes {
			return print(cmd, o, output.Failure("invalid_args", "--yes required", "Pass --yes after confirming that the stored password should be cleared.", 400))
		}
		cfg, err := loadCfg(o)
		if err != nil {
			return print(cmd, o, envelopeError(err))
		}
		inst, err := resolveInstance(cfg.Pgsql, o.Instance)
		if err != nil {
			return print(cmd, o, envelopeError(err))
		}
		idx := findInstance(cfg.Pgsql, inst.Name)
		cfg.Pgsql.Instances[idx].Password = ""
		if err := saveCfg(o, cfg); err != nil {
			return print(cmd, o, envelopeError(err))
		}
		return print(cmd, o, output.Success(inst.Name, map[string]any{"logged_out": true}))
	}})
	c.AddCommand(&cobra.Command{Use: "test", RunE: func(cmd *cobra.Command, args []string) error {
		s, err := loadSession(o)
		if err != nil {
			return print(cmd, o, envelopeError(err))
		}
		if o.DryRun {
			return print(cmd, o, output.Success(s.inst.Name, dryRunData(s, dryStatement{SQL: pgsql.PresetAuthTest})))
		}
		out, err := s.query(pgsql.PresetAuthTest, nil, 2)
		if err != nil {
			return print(cmd, o, envelopeError(err))
		}
		if out.RowCount == 0 {
			return print(cmd, o, output.Failure("server_error", "auth test returned no row", "", 500))
		}
		row := out.Rows[0]
		readOnly, _ := row["read_only"].(string)
		data := map[string]any{
			"authenticated":  true,
			"user":           row["user"],
			"database":       row["database"],
			"server_version": row["server_version"],
			"read_only":      strings.EqualFold(readOnly, "on"),
			"in_recovery":    row["in_recovery"],
			"elapsed_ms":     out.ElapsedMs,
		}
		if o.Verbose {
			data["connection"] = s.target()
		}
		return print(cmd, o, output.Success(s.inst.Name, data))
	}})
	return c
}
