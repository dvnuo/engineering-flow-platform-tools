package commands

import (
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"engineering-flow-platform-tools/internal/config"
	"engineering-flow-platform-tools/internal/output"
	"github.com/spf13/cobra"
)

var sslModes = []string{"disable", "allow", "prefer", "require", "verify-ca", "verify-full"}

// instanceView renders an instance for list/get output; the password is
// blanked by the output redaction layer anyway, so only its presence is shown.
func instanceView(in config.PgsqlInstanceConfig) map[string]any {
	return map[string]any{
		"name":                      in.Name,
		"host":                      in.Host,
		"port":                      in.EffectivePort(),
		"database":                  in.Database,
		"username":                  in.Username,
		"auth_configured":           in.Password != "",
		"sslmode":                   in.EffectiveSSLMode(),
		"ca_cert_configured":        strings.TrimSpace(in.CACert) != "",
		"statement_timeout_seconds": in.EffectiveStatementTimeoutSeconds(),
		"max_rows":                  in.EffectiveMaxRows(),
		"enabled":                   in.IsEnabled(),
	}
}

func addInstanceFlags(cmd *cobra.Command, creating bool) {
	cmd.Flags().String("host", "", "PostgreSQL host name or address.")
	cmd.Flags().Int("port", 0, "PostgreSQL port (default 5432).")
	cmd.Flags().String("database", "", "Database name to connect to.")
	cmd.Flags().String("username", "", "Database role (should be a read-only role).")
	cmd.Flags().Bool("password-stdin", false, "Read the role password from stdin.")
	cmd.Flags().String("sslmode", "", "TLS mode: disable, allow, prefer, require (default), verify-ca, or verify-full.")
	cmd.Flags().String("ca-cert-file", "", "Path to a PEM CA bundle stored inline as ca_cert and used as sslrootcert.")
	cmd.Flags().Int("statement-timeout-seconds", 0, "Server-side statement timeout applied to every statement (default 30).")
	cmd.Flags().Int("max-rows", 0, "Upper bound for --limit on this instance (default 5000).")
	cmd.Flags().Bool("default", false, "Make this instance the default instance.")
	if !creating {
		cmd.Flags().Bool("enabled", true, "Enable or disable the instance (--enabled=false).")
	}
}

func readSecretStdin(cmd *cobra.Command) (string, error) {
	b, err := io.ReadAll(cmd.InOrStdin())
	if err != nil {
		return "", err
	}
	secret := strings.TrimRight(string(b), "\r\n")
	if secret == "" {
		return "", errors.New("stdin did not contain a password")
	}
	return secret, nil
}

// applyInstanceFlags copies changed flags onto in and validates them.
func applyInstanceFlags(cmd *cobra.Command, in *config.PgsqlInstanceConfig, creating bool) error {
	changed := func(name string) bool { return creating || cmd.Flags().Changed(name) }
	if changed("host") {
		in.Host = strings.TrimSpace(mustS(cmd, "host"))
	}
	if changed("database") {
		in.Database = strings.TrimSpace(mustS(cmd, "database"))
	}
	if changed("username") {
		in.Username = strings.TrimSpace(mustS(cmd, "username"))
	}
	if cmd.Flags().Changed("port") {
		port := mustI(cmd, "port")
		if port < 1 || port > 65535 {
			return errors.New("--port must be between 1 and 65535")
		}
		in.Port = port
	}
	if cmd.Flags().Changed("sslmode") {
		mode := strings.ToLower(strings.TrimSpace(mustS(cmd, "sslmode")))
		ok := false
		for _, m := range sslModes {
			if m == mode {
				ok = true
			}
		}
		if !ok {
			return errors.New("--sslmode must be one of " + strings.Join(sslModes, ", "))
		}
		in.SSLMode = mode
	}
	if cmd.Flags().Changed("ca-cert-file") {
		path := strings.TrimSpace(mustS(cmd, "ca-cert-file"))
		if path == "" {
			in.CACert = ""
		} else {
			b, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("cannot read --ca-cert-file: %v", err)
			}
			if block, _ := pem.Decode(b); block == nil || block.Type != "CERTIFICATE" {
				return errors.New("--ca-cert-file must contain a PEM CERTIFICATE block")
			}
			in.CACert = string(b)
		}
	}
	if cmd.Flags().Changed("statement-timeout-seconds") {
		secs := mustI(cmd, "statement-timeout-seconds")
		if secs < 1 || secs > 3600 {
			return errors.New("--statement-timeout-seconds must be between 1 and 3600")
		}
		in.StatementTimeoutSeconds = secs
	}
	if cmd.Flags().Changed("max-rows") {
		rows := mustI(cmd, "max-rows")
		if rows < 1 || rows > 1000000 {
			return errors.New("--max-rows must be between 1 and 1000000")
		}
		in.MaxRows = rows
	}
	if !creating && cmd.Flags().Changed("enabled") {
		v := mustB(cmd, "enabled")
		in.Enabled = &v
	}
	if mustB(cmd, "password-stdin") {
		secret, err := readSecretStdin(cmd)
		if err != nil {
			return err
		}
		in.Password = secret
	}
	if creating {
		var missing []string
		if in.Host == "" {
			missing = append(missing, "--host")
		}
		if in.Database == "" {
			missing = append(missing, "--database")
		}
		if in.Username == "" {
			missing = append(missing, "--username")
		}
		if len(missing) > 0 {
			return errors.New(strings.Join(missing, ", ") + " required")
		}
	}
	return nil
}

func findInstance(cfg config.PgsqlConfig, name string) int {
	for i := range cfg.Instances {
		if cfg.Instances[i].Name == name {
			return i
		}
	}
	return -1
}

func instanceCmd(o *Opts) *cobra.Command {
	c := &cobra.Command{Use: "instance"}
	c.AddCommand(&cobra.Command{Use: "list", RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadCfg(o)
		if err != nil {
			return print(cmd, o, envelopeError(err))
		}
		views := make([]map[string]any, 0, len(cfg.Pgsql.Instances))
		for _, in := range cfg.Pgsql.Instances {
			views = append(views, instanceView(in))
		}
		return print(cmd, o, output.Success("", map[string]any{"instances": views, "default_instance": cfg.Pgsql.DefaultInstance}))
	}})
	c.AddCommand(&cobra.Command{Use: "get <name>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadCfg(o)
		if err != nil {
			return print(cmd, o, envelopeError(err))
		}
		idx := findInstance(cfg.Pgsql, args[0])
		if idx < 0 {
			return print(cmd, o, output.Failure("not_found", "instance not found", "Run pgsql instance list --json.", 404))
		}
		view := instanceView(cfg.Pgsql.Instances[idx])
		view["default"] = cfg.Pgsql.DefaultInstance == args[0]
		return print(cmd, o, output.Success(args[0], view))
	}})
	add := &cobra.Command{Use: "add <name>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		name := strings.TrimSpace(args[0])
		if name == "" {
			return print(cmd, o, output.Failure("invalid_args", "instance name is empty", "", 400))
		}
		cfg, err := loadCfg(o)
		if err != nil && !os.IsNotExist(err) {
			return print(cmd, o, envelopeError(err))
		}
		if findInstance(cfg.Pgsql, name) >= 0 {
			return print(cmd, o, output.Failure("invalid_args", "instance "+name+" already exists", "Use pgsql instance update "+name+" to change it.", 400))
		}
		in := config.PgsqlInstanceConfig{Name: name}
		if err := applyInstanceFlags(cmd, &in, true); err != nil {
			return print(cmd, o, output.Failure("invalid_args", err.Error(), "Pass --host, --database, and --username; use --password-stdin for the password.", 400))
		}
		cfg.Pgsql.Instances = append(cfg.Pgsql.Instances, in)
		if mustB(cmd, "default") {
			cfg.Pgsql.DefaultInstance = name
		}
		if err := saveCfg(o, cfg); err != nil {
			return print(cmd, o, envelopeError(err))
		}
		view := instanceView(in)
		view["added"] = true
		view["default"] = cfg.Pgsql.DefaultInstance == name
		return print(cmd, o, output.Success(name, view))
	}}
	addInstanceFlags(add, true)
	c.AddCommand(add)
	update := &cobra.Command{Use: "update <name>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadCfg(o)
		if err != nil {
			return print(cmd, o, envelopeError(err))
		}
		idx := findInstance(cfg.Pgsql, args[0])
		if idx < 0 {
			return print(cmd, o, output.Failure("not_found", "instance not found", "Run pgsql instance list --json.", 404))
		}
		in := cfg.Pgsql.Instances[idx]
		if err := applyInstanceFlags(cmd, &in, false); err != nil {
			return print(cmd, o, output.Failure("invalid_args", err.Error(), "", 400))
		}
		cfg.Pgsql.Instances[idx] = in
		if mustB(cmd, "default") {
			cfg.Pgsql.DefaultInstance = in.Name
		}
		if err := saveCfg(o, cfg); err != nil {
			return print(cmd, o, envelopeError(err))
		}
		view := instanceView(in)
		view["updated"] = true
		view["default"] = cfg.Pgsql.DefaultInstance == in.Name
		return print(cmd, o, output.Success(in.Name, view))
	}}
	addInstanceFlags(update, false)
	c.AddCommand(update)
	c.AddCommand(&cobra.Command{Use: "remove <name>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if !o.Yes {
			return print(cmd, o, output.Failure("invalid_args", "--yes required", "Pass --yes after confirming the instance removal.", 400))
		}
		cfg, err := loadCfg(o)
		if err != nil {
			return print(cmd, o, envelopeError(err))
		}
		if findInstance(cfg.Pgsql, args[0]) < 0 {
			return print(cmd, o, output.Failure("not_found", "instance not found", "Run pgsql instance list --json.", 404))
		}
		kept := []config.PgsqlInstanceConfig{}
		for _, in := range cfg.Pgsql.Instances {
			if in.Name != args[0] {
				kept = append(kept, in)
			}
		}
		cfg.Pgsql.Instances = kept
		if cfg.Pgsql.DefaultInstance == args[0] {
			cfg.Pgsql.DefaultInstance = ""
		}
		if err := saveCfg(o, cfg); err != nil {
			return print(cmd, o, envelopeError(err))
		}
		return print(cmd, o, output.Success(args[0], map[string]any{"removed": true}))
	}})
	c.AddCommand(&cobra.Command{Use: "default [name]", Args: cobra.MaximumNArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadCfg(o)
		if err != nil {
			return print(cmd, o, envelopeError(err))
		}
		if len(args) == 0 {
			return print(cmd, o, output.Success("", map[string]any{"default_instance": cfg.Pgsql.DefaultInstance}))
		}
		if findInstance(cfg.Pgsql, args[0]) < 0 {
			return print(cmd, o, output.Failure("not_found", "instance not found", "Run pgsql instance list --json.", 404))
		}
		cfg.Pgsql.DefaultInstance = args[0]
		if err := saveCfg(o, cfg); err != nil {
			return print(cmd, o, envelopeError(err))
		}
		return print(cmd, o, output.Success(args[0], map[string]any{"default_instance": args[0]}))
	}})
	return c
}
