package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/multica-ai/multica/server/internal/cli"
)

var wikiCmd = &cobra.Command{
	Use:     "wiki",
	Short:   "Wiki knowledge base operations",
	Long:    "Read, write, and search the workspace wiki knowledge base.",
	GroupID: groupCore,
}

var wikiReadCmd = &cobra.Command{
	Use:   "read-page",
	Short: "Read a wiki page by path",
	Long:  "Read a wiki page and display its content, links, and backlinks.",
	RunE: func(cmd *cobra.Command, _ []string) error {
		client, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		space, _ := cmd.Flags().GetString("space")
		if space == "" {
			space = "default"
		}
		path, _ := cmd.Flags().GetString("path")
		if path == "" {
			return fmt.Errorf("--path is required")
		}

		var result any
		if err := client.GetJSON(cmd.Context(),
			fmt.Sprintf("/api/wiki/spaces/%s/pages/%s", space, path),
			&result); err != nil {
			return fmt.Errorf("wiki read-page: %w", err)
		}
		return cli.PrintJSON(os.Stdout, result)
	},
}

var wikiWriteCmd = &cobra.Command{
	Use:   "write-page",
	Short: "Create or update a wiki page",
	RunE: func(cmd *cobra.Command, _ []string) error {
		client, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		space, _ := cmd.Flags().GetString("space")
		if space == "" {
			space = "default"
		}
		path, _ := cmd.Flags().GetString("path")
		if path == "" {
			return fmt.Errorf("--path is required")
		}
		content, ok, err := resolveTextFlag(cmd, "content")
		if err != nil {
			return err
		}
		if !ok || content == "" {
			return fmt.Errorf("--content, --content-stdin, or --content-file is required")
		}

		body := map[string]string{"content": content}
		var result any
		if err := client.PutJSON(cmd.Context(),
			fmt.Sprintf("/api/wiki/spaces/%s/pages/%s", space, path),
			body, &result); err != nil {
			return fmt.Errorf("wiki write-page: %w", err)
		}
		return cli.PrintJSON(os.Stdout, result)
	},
}

var wikiSearchCmd = &cobra.Command{
	Use:   "search",
	Short: "Full-text search wiki pages",
	RunE: func(cmd *cobra.Command, _ []string) error {
		client, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		space, _ := cmd.Flags().GetString("space")
		if space == "" {
			space = "default"
		}
		query, _ := cmd.Flags().GetString("query")
		if query == "" {
			return fmt.Errorf("--query is required")
		}

		var result any
		if err := client.GetJSON(cmd.Context(),
			fmt.Sprintf("/api/wiki/spaces/%s/pages?search=%s", space, query),
			&result); err != nil {
			return fmt.Errorf("wiki search: %w", err)
		}
		return cli.PrintJSON(os.Stdout, result)
	},
}

var wikiListCmd = &cobra.Command{
	Use:   "list-pages",
	Short: "List wiki pages",
	RunE: func(cmd *cobra.Command, _ []string) error {
		client, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		space, _ := cmd.Flags().GetString("space")
		if space == "" {
			space = "default"
		}
		search, _ := cmd.Flags().GetString("search")
		u := fmt.Sprintf("/api/wiki/spaces/%s/pages", space)
		if search != "" {
			u += "?search=" + search
		}

		var result any
		if err := client.GetJSON(cmd.Context(), u, &result); err != nil {
			return fmt.Errorf("wiki list-pages: %w", err)
		}
		return cli.PrintJSON(os.Stdout, result)
	},
}

var wikiCaptureSourceCmd = &cobra.Command{
	Use:   "capture-source",
	Short: "Capture a raw source into the wiki",
	RunE: func(cmd *cobra.Command, _ []string) error {
		client, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		space, _ := cmd.Flags().GetString("space")
		if space == "" {
			space = "default"
		}
		title, _ := cmd.Flags().GetString("title")
		if title == "" {
			return fmt.Errorf("--title is required")
		}
		content, ok, err := resolveTextFlag(cmd, "content")
		if err != nil {
			return err
		}
		if !ok || content == "" {
			return fmt.Errorf("--content, --content-stdin, or --content-file is required")
		}

		body := map[string]string{
			"source_type": "text",
			"title":       title,
			"content":     content,
		}
		var result any
		if err := client.PostJSON(cmd.Context(),
			fmt.Sprintf("/api/wiki/spaces/%s/sources", space),
			body, &result); err != nil {
			return fmt.Errorf("wiki capture-source: %w", err)
		}
		return cli.PrintJSON(os.Stdout, result)
	},
}

func init() {
	wikiReadCmd.Flags().String("path", "", "Page path (e.g. wiki/concepts/foo.md)")
	wikiReadCmd.Flags().String("space", "default", "Wiki space slug")
	wikiCmd.AddCommand(wikiReadCmd)

	wikiWriteCmd.Flags().String("path", "", "Page path")
	wikiWriteCmd.Flags().String("content", "", "Page content (markdown)")
	wikiWriteCmd.Flags().Bool("content-stdin", false, "Read content from stdin")
	wikiWriteCmd.Flags().String("content-file", "", "Read content from file")
	wikiWriteCmd.Flags().String("space", "default", "Wiki space slug")
	wikiCmd.AddCommand(wikiWriteCmd)

	wikiSearchCmd.Flags().String("query", "", "Search query")
	wikiSearchCmd.Flags().String("space", "default", "Wiki space slug")
	wikiCmd.AddCommand(wikiSearchCmd)

	wikiListCmd.Flags().String("search", "", "Optional search filter")
	wikiListCmd.Flags().String("space", "default", "Wiki space slug")
	wikiCmd.AddCommand(wikiListCmd)

	wikiCaptureSourceCmd.Flags().String("title", "", "Source title")
	wikiCaptureSourceCmd.Flags().String("content", "", "Source content")
	wikiCaptureSourceCmd.Flags().Bool("content-stdin", false, "Read content from stdin")
	wikiCaptureSourceCmd.Flags().String("content-file", "", "Read content from file")
	wikiCaptureSourceCmd.Flags().String("space", "default", "Wiki space slug")
	wikiCmd.AddCommand(wikiCaptureSourceCmd)

	rootCmd.AddCommand(wikiCmd)
}
