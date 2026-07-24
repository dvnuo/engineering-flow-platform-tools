package commands

import (
	"context"
	"errors"
	"os"
	"strings"
	"time"

	"engineering-flow-platform-tools/internal/browser/bookmarks"
	"engineering-flow-platform-tools/internal/config"
	"engineering-flow-platform-tools/internal/output"
	"github.com/spf13/cobra"
)

func bookmarkCmd(o *Opts) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bookmark",
		Short: "Discover configured websites for semantic routing",
		Long:  "Fetch and merge external bookmark manifests configured under browser.bookmarks.sources so agents can resolve a requested website before opening it.",
	}
	cmd.AddCommand(bookmarkListCmd(o))
	return cmd
}

func bookmarkListCmd(o *Opts) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "Fetch and merge external bookmark sources",
		Long:  "Fetch every configured external bookmark source live, validate its strict bookmark manifest, and merge the available name, aliases, description, URL, and source fields for agent routing. No cache is read or written.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := config.LoadShared(o.Config)
			if err != nil {
				if defaultConfigMissing(o.Config, err) {
					cfg = config.RootConfig{}
				} else {
					return print(cmd, o, output.Failure(
						"config_error",
						"The EFP config file could not be loaded.",
						"Check --config, EFP_CONFIG, or ~/.efp/config.yaml.",
						400,
					))
				}
			}
			sources := make([]bookmarks.Source, 0, len(cfg.Browser.Bookmarks.Sources))
			for _, source := range cfg.Browser.Bookmarks.Sources {
				sources = append(sources, bookmarks.Source{Name: source.Name, URL: source.URL})
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
			defer cancel()
			result, err := bookmarks.NewLister().List(ctx, sources)
			if err == nil {
				return print(cmd, o, output.Success("", result))
			}
			var bookmarkErr *bookmarks.Error
			if errors.As(err, &bookmarkErr) {
				env := output.Failure(bookmarkErr.Code, bookmarkErr.Message, bookmarkErr.Hint, bookmarkErr.Status)
				env.Data = result
				return print(cmd, o, env)
			}
			return print(cmd, o, output.Failure(
				"bookmark_list_failed",
				"Bookmark sources could not be listed.",
				"Verify the source configuration and retry.",
				500,
			))
		},
	}
}

func defaultConfigMissing(flagPath string, err error) bool {
	return os.IsNotExist(err) &&
		strings.TrimSpace(flagPath) == "" &&
		strings.TrimSpace(os.Getenv(config.EnvConfigPath)) == "" &&
		strings.TrimSpace(os.Getenv(config.EnvLegacyConfigPath)) == ""
}
