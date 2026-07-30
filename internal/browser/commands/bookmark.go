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
		Long:  "Manage configured bookmark sources and their bookmarks so agents can resolve a requested website before opening it. Remote sources are read-only; configured local file sources support bookmark CRUD.",
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
	var sourceNames []string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "Merge bookmarks from configured sources",
		Long:  "Load configured HTTP/HTTPS or local file sources live, validate strict bookmark manifests, and merge name, aliases, description, URL, and source fields for agent routing. Repeat --source to load only selected source names. No cache is read or written.",
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
			sources, err := bookmarks.ValidateSources(bookmarkSources(cfg))
			if err != nil {
				return printBookmarkError(cmd, o, err)
			}
			sources, err = filterBookmarkSources(sources, sourceNames)
			if err != nil {
				return printBookmarkError(cmd, o, err)
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
			defer cancel()
			result, listErr := bookmarks.NewLister().List(ctx, sources)
			if listErr == nil {
				return print(cmd, o, output.Success("", result))
			}
			var bookmarkErr *bookmarks.Error
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
	cmd.Flags().StringArrayVar(&sourceNames, "source", nil, "Configured source name to load; repeat for multiple sources.")
	return cmd
}

func bookmarkAddCmd(o *Opts) *cobra.Command {
	var source, name, description, targetURL string
	var aliases []string
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Add a bookmark to a configured local source",
		Long:  "Add one bookmark to the configured local file source selected by --source. The manifest and parent directory are created on first write. Remote sources are read-only.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := bookmarkStoreForSource(o, source)
			if err != nil {
				return printBookmarkError(cmd, o, err)
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
	cmd.Flags().StringVar(&source, "source", "", "Required configured local source name to modify.")
	cmd.Flags().StringVar(&name, "name", "", "Unique bookmark name within the selected source.")
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
		Short: "Update a bookmark in a configured local source",
		Long:  "Update selected fields of a bookmark in the configured local file source selected by --source. Remote sources are read-only.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
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
			store, err := bookmarkStoreForSource(o, source)
			if err != nil {
				return printBookmarkError(cmd, o, err)
			}
			item, err := store.Update(args[0], patch)
			if err != nil {
				return printBookmarkError(cmd, o, err)
			}
			return print(cmd, o, output.Success("", map[string]any{"bookmark": item, "file": store.Path}))
		},
	}
	cmd.Flags().StringVar(&source, "source", "", "Required configured local source name to modify.")
	cmd.Flags().StringVar(&newName, "name", "", "Replacement unique bookmark name within the selected source.")
	cmd.Flags().StringArrayVar(&aliases, "alias", nil, "Replacement bookmark alias; repeat this flag for multiple aliases.")
	cmd.Flags().BoolVar(&clearAliases, "clear-aliases", false, "Remove every alias from the bookmark.")
	cmd.Flags().StringVar(&description, "description", "", "Replacement description used by agents for semantic website routing.")
	cmd.Flags().StringVar(&targetURL, "url", "", "Replacement absolute HTTP or HTTPS website URL.")
	return cmd
}

func bookmarkRemoveCmd(o *Opts) *cobra.Command {
	var source string
	var yes bool
	cmd := &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove a bookmark from a configured local source",
		Long:  "Remove one bookmark from the configured local file source selected by --source after explicit confirmation. Remote sources are read-only.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !yes {
				return print(cmd, o, output.Failure("invalid_args", "--yes required", "Pass --yes after confirming the bookmark removal.", 400))
			}
			store, err := bookmarkStoreForSource(o, source)
			if err != nil {
				return printBookmarkError(cmd, o, err)
			}
			item, err := store.Remove(args[0])
			if err != nil {
				return printBookmarkError(cmd, o, err)
			}
			return print(cmd, o, output.Success("", map[string]any{"removed": item, "file": store.Path}))
		},
	}
	cmd.Flags().StringVar(&source, "source", "", "Required configured local source name to modify.")
	cmd.Flags().BoolVar(&yes, "yes", false, "Confirm removal of the bookmark.")
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
		sources = append(sources, bookmarks.Source{Name: source.Name, Description: source.Description, URL: source.URL})
	}
	return sources
}

func filterBookmarkSources(sources []bookmarks.Source, requested []string) ([]bookmarks.Source, error) {
	if len(requested) == 0 {
		return sources, nil
	}
	wanted := make(map[string]struct{}, len(requested))
	for _, name := range requested {
		name = strings.TrimSpace(name)
		if name == "" {
			return nil, &bookmarks.Error{
				Code:    "bookmark_source_not_found",
				Message: "The requested bookmark source name is empty.",
				Hint:    "Run browser bookmark source list --json and pass an existing source name to --source.",
				Status:  404,
			}
		}
		wanted[strings.ToLower(name)] = struct{}{}
	}
	filtered := make([]bookmarks.Source, 0, len(wanted))
	for _, source := range sources {
		key := strings.ToLower(source.Name)
		if _, ok := wanted[key]; ok {
			filtered = append(filtered, source)
			delete(wanted, key)
		}
	}
	if len(wanted) > 0 {
		var missing string
		for _, name := range requested {
			if _, ok := wanted[strings.ToLower(strings.TrimSpace(name))]; ok {
				missing = strings.TrimSpace(name)
				break
			}
		}
		return nil, &bookmarks.Error{
			Code:    "bookmark_source_not_found",
			Message: "The bookmark source was not found: " + missing,
			Hint:    "Run browser bookmark source list --json and choose an existing source name.",
			Status:  404,
		}
	}
	return filtered, nil
}

func bookmarkStoreForSource(o *Opts, sourceName string) (bookmarks.Store, error) {
	sourceName = strings.TrimSpace(sourceName)
	if sourceName == "" {
		return bookmarks.Store{}, &bookmarks.Error{
			Code:    "bookmark_source_required",
			Message: "A bookmark source is required.",
			Hint:    "Pass --source <name> for a configured local file source. List sources with browser bookmark source list --json.",
			Status:  400,
		}
	}
	cfg, err := loadBookmarkConfig(o, false)
	if err != nil {
		return bookmarks.Store{}, &bookmarks.Error{
			Code:    "config_error",
			Message: "The EFP config file could not be loaded.",
			Hint:    "Check --config, EFP_CONFIG, or ~/.efp/config.yaml.",
			Status:  400,
		}
	}
	sources, err := bookmarks.ValidateSources(bookmarkSources(cfg))
	if err != nil {
		return bookmarks.Store{}, err
	}
	for _, source := range sources {
		if !strings.EqualFold(source.Name, sourceName) {
			continue
		}
		path, local, err := bookmarks.LocalSourcePath(source)
		if err != nil {
			return bookmarks.Store{}, err
		}
		if !local {
			return bookmarks.Store{}, &bookmarks.Error{
				Code:    "bookmark_source_read_only",
				Message: "Remote bookmark sources are read-only: " + source.Name,
				Hint:    "Choose a configured local file source, or modify the remote manifest at its origin.",
				Status:  409,
			}
		}
		return bookmarks.Store{Path: path, Source: source.Name}, nil
	}
	return bookmarks.Store{}, &bookmarks.Error{
		Code:    "bookmark_source_not_found",
		Message: "The bookmark source was not found: " + sourceName,
		Hint:    "Run browser bookmark source list --json and choose an existing source name.",
		Status:  404,
	}
}

func printBookmarkError(cmd *cobra.Command, o *Opts, err error) error {
	var bookmarkErr *bookmarks.Error
	if errors.As(err, &bookmarkErr) {
		return print(cmd, o, output.Failure(bookmarkErr.Code, bookmarkErr.Message, bookmarkErr.Hint, bookmarkErr.Status))
	}
	return print(cmd, o, output.Failure("bookmark_error", "The bookmark operation failed.", "Inspect the configured bookmark source and retry.", 500))
}
