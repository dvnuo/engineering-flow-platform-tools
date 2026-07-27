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
		Short: "Discover and manage websites for semantic routing",
		Long:  "Manage authoritative local bookmarks and external read-only bookmark sources so agents can resolve a requested website before opening it.",
	}
	cmd.AddCommand(
		bookmarkListCmd(o),
		bookmarkAddCmd(o),
		bookmarkUpdateCmd(o),
		bookmarkRemoveCmd(o),
		bookmarkSourceCmd(o),
	)
	return cmd
}

func bookmarkListCmd(o *Opts) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "Merge local bookmarks and external bookmark sources",
		Long:  "Read the managed local bookmark file, fetch every configured external source live, validate strict bookmark manifests, and merge name, aliases, description, URL, and source fields for agent routing. No cache is read or written.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadBookmarkConfig(o, false)
			if err != nil {
				return print(cmd, o, output.Failure(
					"config_error",
					"The EFP config file could not be loaded.",
					"Check --config, EFP_CONFIG, or ~/.efp/config.yaml.",
					400,
				))
			}
			store, err := bookmarks.DefaultStore()
			if err != nil {
				return print(cmd, o, output.Failure("bookmark_store_error", "The local bookmark path could not be resolved.", "Check the current user's home directory.", 500))
			}
			localItems, localExists, err := store.Load()
			if err != nil {
				return printBookmarkError(cmd, o, err)
			}
			sources := bookmarkSources(cfg)
			ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
			defer cancel()
			result, listErr := bookmarks.NewLister().List(ctx, sources)
			if localExists {
				result.Bookmarks = append(localItems, result.Bookmarks...)
				result.Sources = append([]bookmarks.SourceStatus{{
					Name: bookmarks.LocalSourceName, OK: true, Count: len(localItems),
				}}, result.Sources...)
			}
			if listErr == nil {
				return print(cmd, o, output.Success("", result))
			}
			var bookmarkErr *bookmarks.Error
			if errors.As(listErr, &bookmarkErr) && bookmarkErr.Code == "bookmark_sources_unavailable" && localExists {
				return print(cmd, o, output.Success("", result))
			}
			if errors.As(listErr, &bookmarkErr) {
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

func bookmarkAddCmd(o *Opts) *cobra.Command {
	var source, name, description, targetURL string
	var aliases []string
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a bookmark to the managed local source",
		Long:  "Add one bookmark to ~/.efp/bookmarks.yaml. External HTTP and HTTPS sources are read-only and cannot be modified by this command.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireLocalBookmarkSource(source); err != nil {
				return printBookmarkError(cmd, o, err)
			}
			store, err := bookmarks.DefaultStore()
			if err != nil {
				return print(cmd, o, output.Failure("bookmark_store_error", "The local bookmark path could not be resolved.", "Check the current user's home directory.", 500))
			}
			item, err := store.Add(bookmarks.Bookmark{
				Name: name, Aliases: aliases, Description: description, URL: targetURL,
			})
			if err != nil {
				return printBookmarkError(cmd, o, err)
			}
			return print(cmd, o, output.Success("", map[string]any{"bookmark": item, "file": store.Path}))
		},
	}
	cmd.Flags().StringVar(&source, "source", bookmarks.LocalSourceName, "Bookmark source to modify; only local is writable.")
	cmd.Flags().StringVar(&name, "name", "", "Unique local bookmark name.")
	cmd.Flags().StringArrayVar(&aliases, "alias", nil, "Optional bookmark alias; repeat this flag for multiple aliases.")
	cmd.Flags().StringVar(&description, "description", "", "Required description used by agents for semantic website routing.")
	cmd.Flags().StringVar(&targetURL, "url", "", "Absolute HTTP or HTTPS website URL.")
	return cmd
}

func bookmarkUpdateCmd(o *Opts) *cobra.Command {
	var source, newName, description, targetURL string
	var aliases []string
	var clearAliases bool
	cmd := &cobra.Command{
		Use:   "update <name>",
		Short: "Update a managed local bookmark",
		Long:  "Update selected fields of a bookmark in ~/.efp/bookmarks.yaml. External HTTP and HTTPS sources are read-only.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireLocalBookmarkSource(source); err != nil {
				return printBookmarkError(cmd, o, err)
			}
			aliasChanged := cmd.Flags().Changed("alias")
			if aliasChanged && clearAliases {
				return print(cmd, o, output.Failure("invalid_args", "--alias and --clear-aliases cannot be used together.", "Use repeated --alias flags to replace aliases, or --clear-aliases to remove all aliases.", 400))
			}
			patch := bookmarks.Update{}
			if cmd.Flags().Changed("name") {
				patch.Name = &newName
			}
			if aliasChanged {
				patch.Aliases = &aliases
			}
			if clearAliases {
				empty := []string{}
				patch.Aliases = &empty
			}
			if cmd.Flags().Changed("description") {
				patch.Description = &description
			}
			if cmd.Flags().Changed("url") {
				patch.URL = &targetURL
			}
			if patch.Name == nil && patch.Aliases == nil && patch.Description == nil && patch.URL == nil {
				return print(cmd, o, output.Failure("invalid_args", "No bookmark fields were selected for update.", "Pass --name, --alias, --clear-aliases, --description, or --url.", 400))
			}
			store, err := bookmarks.DefaultStore()
			if err != nil {
				return print(cmd, o, output.Failure("bookmark_store_error", "The local bookmark path could not be resolved.", "Check the current user's home directory.", 500))
			}
			item, err := store.Update(args[0], patch)
			if err != nil {
				return printBookmarkError(cmd, o, err)
			}
			return print(cmd, o, output.Success("", map[string]any{"bookmark": item, "file": store.Path}))
		},
	}
	cmd.Flags().StringVar(&source, "source", bookmarks.LocalSourceName, "Bookmark source to modify; only local is writable.")
	cmd.Flags().StringVar(&newName, "name", "", "Replacement unique local bookmark name.")
	cmd.Flags().StringArrayVar(&aliases, "alias", nil, "Replacement bookmark alias; repeat this flag for multiple aliases.")
	cmd.Flags().BoolVar(&clearAliases, "clear-aliases", false, "Remove every alias from the local bookmark.")
	cmd.Flags().StringVar(&description, "description", "", "Replacement description used by agents for semantic website routing.")
	cmd.Flags().StringVar(&targetURL, "url", "", "Replacement absolute HTTP or HTTPS website URL.")
	return cmd
}

func bookmarkRemoveCmd(o *Opts) *cobra.Command {
	var source string
	var yes bool
	cmd := &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a managed local bookmark",
		Long:  "Remove one bookmark from ~/.efp/bookmarks.yaml after explicit confirmation. External HTTP and HTTPS sources are read-only.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !yes {
				return print(cmd, o, output.Failure("invalid_args", "--yes required", "Pass --yes after confirming the local bookmark removal.", 400))
			}
			if err := requireLocalBookmarkSource(source); err != nil {
				return printBookmarkError(cmd, o, err)
			}
			store, err := bookmarks.DefaultStore()
			if err != nil {
				return print(cmd, o, output.Failure("bookmark_store_error", "The local bookmark path could not be resolved.", "Check the current user's home directory.", 500))
			}
			item, err := store.Remove(args[0])
			if err != nil {
				return printBookmarkError(cmd, o, err)
			}
			return print(cmd, o, output.Success("", map[string]any{"removed": item, "file": store.Path}))
		},
	}
	cmd.Flags().StringVar(&source, "source", bookmarks.LocalSourceName, "Bookmark source to modify; only local is writable.")
	cmd.Flags().BoolVar(&yes, "yes", false, "Confirm removal of the local bookmark.")
	return cmd
}

func loadBookmarkConfig(o *Opts, allowExplicitMissing bool) (config.RootConfig, error) {
	cfg, _, err := config.LoadShared(o.Config)
	if err == nil {
		return cfg, nil
	}
	if defaultConfigMissing(o.Config, err) || (allowExplicitMissing && os.IsNotExist(err)) {
		return config.RootConfig{}, nil
	}
	return config.RootConfig{}, err
}

func defaultConfigMissing(flagPath string, err error) bool {
	return os.IsNotExist(err) &&
		strings.TrimSpace(flagPath) == "" &&
		strings.TrimSpace(os.Getenv(config.EnvConfigPath)) == "" &&
		strings.TrimSpace(os.Getenv(config.EnvLegacyConfigPath)) == ""
}

func bookmarkSources(cfg config.RootConfig) []bookmarks.Source {
	sources := make([]bookmarks.Source, 0, len(cfg.Browser.Bookmarks.Sources))
	for _, source := range cfg.Browser.Bookmarks.Sources {
		sources = append(sources, bookmarks.Source{Name: source.Name, URL: source.URL})
	}
	return sources
}

func requireLocalBookmarkSource(source string) error {
	if strings.EqualFold(strings.TrimSpace(source), bookmarks.LocalSourceName) {
		return nil
	}
	return &bookmarks.Error{
		Code:    "bookmark_source_read_only",
		Message: "External bookmark sources are read-only.",
		Hint:    "Modify the source manifest in its owning system, or omit --source to write ~/.efp/bookmarks.yaml.",
		Status:  409,
	}
}

func printBookmarkError(cmd *cobra.Command, o *Opts, err error) error {
	var bookmarkErr *bookmarks.Error
	if errors.As(err, &bookmarkErr) {
		return print(cmd, o, output.Failure(bookmarkErr.Code, bookmarkErr.Message, bookmarkErr.Hint, bookmarkErr.Status))
	}
	return print(cmd, o, output.Failure("bookmark_error", "The bookmark operation failed.", "Inspect ~/.efp/bookmarks.yaml and retry.", 500))
}
