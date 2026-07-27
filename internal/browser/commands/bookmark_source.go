package commands

import (
	"errors"
	"strings"

	"engineering-flow-platform-tools/internal/browser/bookmarks"
	"engineering-flow-platform-tools/internal/config"
	"engineering-flow-platform-tools/internal/output"
	"github.com/spf13/cobra"
)

func bookmarkSourceCmd(o *Opts) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "source",
		Short: "Manage external read-only bookmark source registrations",
		Long:  "List, add, update, or remove external HTTP and HTTPS bookmark source registrations stored in the shared EFP config.",
	}
	cmd.AddCommand(
		bookmarkSourceListCmd(o),
		bookmarkSourceAddCmd(o),
		bookmarkSourceUpdateCmd(o),
		bookmarkSourceRemoveCmd(o),
	)
	return cmd
}

func bookmarkSourceListCmd(o *Opts) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List configured external bookmark sources",
		Long:  "List external read-only bookmark source names and URLs from browser.bookmarks.sources without fetching their manifests.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadBookmarkConfig(o, false)
			if err != nil {
				return print(cmd, o, output.Failure("config_error", "The EFP config file could not be loaded.", "Check --config, EFP_CONFIG, or ~/.efp/config.yaml.", 400))
			}
			sources, err := bookmarks.ValidateSources(bookmarkSources(cfg))
			if err != nil {
				return printBookmarkError(cmd, o, err)
			}
			return print(cmd, o, output.Success("", map[string]any{"sources": sources}))
		},
	}
}

func bookmarkSourceAddCmd(o *Opts) *cobra.Command {
	var name, sourceURL string
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Register an external bookmark source",
		Long:  "Add one external read-only HTTP or HTTPS bookmark source to browser.bookmarks.sources in the shared EFP config.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadBookmarkConfig(o, true)
			if err != nil {
				return print(cmd, o, output.Failure("config_error", "The EFP config file could not be loaded.", "Check --config, EFP_CONFIG, or ~/.efp/config.yaml.", 400))
			}
			for _, source := range cfg.Browser.Bookmarks.Sources {
				if strings.EqualFold(strings.TrimSpace(source.Name), strings.TrimSpace(name)) {
					return print(cmd, o, output.Failure("bookmark_source_exists", "A bookmark source with this name already exists.", "Use browser bookmark source update <name> to change it.", 409))
				}
			}
			cfg.Browser.Bookmarks.Sources = append(cfg.Browser.Bookmarks.Sources, config.BrowserBookmarkSource{Name: name, URL: sourceURL})
			normalized, err := bookmarks.ValidateSources(bookmarkSources(cfg))
			if err != nil {
				return printBookmarkError(cmd, o, err)
			}
			setBookmarkSources(&cfg, normalized)
			if err := config.SaveShared(o.Config, cfg); err != nil {
				return printBookmarkConfigWriteError(cmd, o, err)
			}
			return print(cmd, o, output.Success("", map[string]any{"source": normalized[len(normalized)-1]}))
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Unique external bookmark source name; local is reserved.")
	cmd.Flags().StringVar(&sourceURL, "url", "", "Absolute HTTP or HTTPS URL serving a version 1 bookmark manifest.")
	return cmd
}

func bookmarkSourceUpdateCmd(o *Opts) *cobra.Command {
	var newName, sourceURL string
	cmd := &cobra.Command{
		Use:   "update <name>",
		Short: "Update an external bookmark source registration",
		Long:  "Update the name or URL of an external read-only bookmark source in the shared EFP config.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmd.Flags().Changed("name") && !cmd.Flags().Changed("url") {
				return print(cmd, o, output.Failure("invalid_args", "No bookmark source fields were selected for update.", "Pass --name or --url.", 400))
			}
			cfg, err := loadBookmarkConfig(o, false)
			if err != nil {
				return print(cmd, o, output.Failure("config_error", "The EFP config file could not be loaded.", "Check --config, EFP_CONFIG, or ~/.efp/config.yaml.", 400))
			}
			index := findBookmarkSource(cfg.Browser.Bookmarks.Sources, args[0])
			if index < 0 {
				return print(cmd, o, output.Failure("bookmark_source_not_found", "The bookmark source was not found.", "Run browser bookmark source list --json and choose an existing source name.", 404))
			}
			if cmd.Flags().Changed("name") {
				cfg.Browser.Bookmarks.Sources[index].Name = newName
			}
			if cmd.Flags().Changed("url") {
				cfg.Browser.Bookmarks.Sources[index].URL = sourceURL
			}
			normalized, err := bookmarks.ValidateSources(bookmarkSources(cfg))
			if err != nil {
				return printBookmarkError(cmd, o, err)
			}
			setBookmarkSources(&cfg, normalized)
			if err := config.SaveShared(o.Config, cfg); err != nil {
				return printBookmarkConfigWriteError(cmd, o, err)
			}
			return print(cmd, o, output.Success("", map[string]any{"source": normalized[index]}))
		},
	}
	cmd.Flags().StringVar(&newName, "name", "", "Replacement unique external bookmark source name.")
	cmd.Flags().StringVar(&sourceURL, "url", "", "Replacement absolute HTTP or HTTPS manifest URL.")
	return cmd
}

func bookmarkSourceRemoveCmd(o *Opts) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove an external bookmark source registration",
		Long:  "Remove one external bookmark source registration from the shared EFP config after explicit confirmation. The remote manifest is not changed.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !yes {
				return print(cmd, o, output.Failure("invalid_args", "--yes required", "Pass --yes after confirming the source registration removal.", 400))
			}
			cfg, err := loadBookmarkConfig(o, false)
			if err != nil {
				return print(cmd, o, output.Failure("config_error", "The EFP config file could not be loaded.", "Check --config, EFP_CONFIG, or ~/.efp/config.yaml.", 400))
			}
			index := findBookmarkSource(cfg.Browser.Bookmarks.Sources, args[0])
			if index < 0 {
				return print(cmd, o, output.Failure("bookmark_source_not_found", "The bookmark source was not found.", "Run browser bookmark source list --json and choose an existing source name.", 404))
			}
			removed := cfg.Browser.Bookmarks.Sources[index]
			cfg.Browser.Bookmarks.Sources = append(cfg.Browser.Bookmarks.Sources[:index], cfg.Browser.Bookmarks.Sources[index+1:]...)
			normalized, err := bookmarks.ValidateSources(bookmarkSources(cfg))
			if err != nil {
				return printBookmarkError(cmd, o, err)
			}
			setBookmarkSources(&cfg, normalized)
			if err := config.SaveShared(o.Config, cfg); err != nil {
				return printBookmarkConfigWriteError(cmd, o, err)
			}
			return print(cmd, o, output.Success("", map[string]any{"removed": removed}))
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "Confirm removal of the external bookmark source registration.")
	return cmd
}

func findBookmarkSource(sources []config.BrowserBookmarkSource, name string) int {
	name = strings.TrimSpace(name)
	for i, source := range sources {
		if strings.EqualFold(strings.TrimSpace(source.Name), name) {
			return i
		}
	}
	return -1
}

func setBookmarkSources(cfg *config.RootConfig, sources []bookmarks.Source) {
	cfg.Browser.Bookmarks.Sources = make([]config.BrowserBookmarkSource, 0, len(sources))
	for _, source := range sources {
		cfg.Browser.Bookmarks.Sources = append(cfg.Browser.Bookmarks.Sources, config.BrowserBookmarkSource{Name: source.Name, URL: source.URL})
	}
}

func printBookmarkConfigWriteError(cmd *cobra.Command, o *Opts, err error) error {
	if errors.Is(err, config.ErrEnvManaged) {
		return print(cmd, o, output.Failure(
			"config_env_managed",
			"The EFP config is managed by environment variables and cannot be changed through the default file.",
			"Pass --config <path> to explicitly manage a bookmark source file.",
			409,
		))
	}
	return print(cmd, o, output.Failure("config_error", "The EFP config file could not be saved.", "Check the config path permissions and free disk space.", 500))
}
