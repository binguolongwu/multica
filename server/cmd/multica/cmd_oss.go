package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/multica-ai/multica/server/internal/cli"
)

var ossCmd = &cobra.Command{
	Use:     "oss",
	Short:   "Object Storage operations",
	Long:    "Upload, download, and list files in the workspace Object Storage Service.",
	GroupID: groupCore,
}

var ossUploadCmd = &cobra.Command{
	Use:   "upload",
	Short: "Upload a file to OSS",
	RunE: func(cmd *cobra.Command, _ []string) error {
		client, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		filePath, _ := cmd.Flags().GetString("file")
		if filePath == "" {
			return fmt.Errorf("--file is required")
		}
		ossKey, _ := cmd.Flags().GetString("key")
		if ossKey == "" {
			ossKey = filepath.Base(filePath)
		}

		data, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("read file: %w", err)
		}

		// Use the existing UploadFile helper from the CLI client
		url, _, err := client.UploadFileWithURL(cmd.Context(), data, ossKey)
		if err != nil {
			return fmt.Errorf("oss upload: %w", err)
		}
		fmt.Println(url)
		return nil
	},
}

var ossListCmd = &cobra.Command{
	Use:   "list",
	Short: "List uploaded files",
	RunE: func(cmd *cobra.Command, _ []string) error {
		client, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		prefix, _ := cmd.Flags().GetString("prefix")
		u := "/api/oss/configs/default/files"
		if prefix != "" {
			u += "?prefix=" + prefix
		}
		var result any
		if err := client.GetJSON(cmd.Context(), u, &result); err != nil {
			return fmt.Errorf("oss list: %w", err)
		}
		return cli.PrintJSON(os.Stdout, result)
	},
}

var ossDownloadCmd = &cobra.Command{
	Use:   "download",
	Short: "Download a file from OSS",
	RunE: func(cmd *cobra.Command, _ []string) error {
		client, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		ossKey, _ := cmd.Flags().GetString("key")
		if ossKey == "" {
			return fmt.Errorf("--key is required")
		}
		outputPath, _ := cmd.Flags().GetString("output")
		if outputPath == "" {
			outputPath = filepath.Base(ossKey)
		}

		u := fmt.Sprintf("/api/oss/configs/default/files?key=%s&redirect=true", ossKey)
		var result map[string]any
		headers, err := client.GetJSONWithHeaders(cmd.Context(), u, &result)
		if err != nil {
			return fmt.Errorf("oss download: %w", err)
		}

		// Get the download URL from the response
		downloadURL, ok := result["url"].(string)
		if !ok || downloadURL == "" {
			return fmt.Errorf("oss download: no url in response")
		}
		_ = headers

		// Download the file
		resp, err := http.Get(downloadURL)
		if err != nil {
			return fmt.Errorf("download file: %w", err)
		}
		defer resp.Body.Close()

		out, err := os.Create(outputPath)
		if err != nil {
			return fmt.Errorf("create output: %w", err)
		}
		defer out.Close()

		if _, err := io.Copy(out, resp.Body); err != nil {
			return fmt.Errorf("write output: %w", err)
		}

		fmt.Fprintf(os.Stderr, "Downloaded to %s\n", outputPath)
		return nil
	},
}

func init() {
	ossUploadCmd.Flags().String("file", "", "Local file path to upload")
	ossUploadCmd.Flags().String("key", "", "OSS object key (default: filename)")
	ossCmd.AddCommand(ossUploadCmd)

	ossListCmd.Flags().String("prefix", "", "Filter by key prefix")
	ossCmd.AddCommand(ossListCmd)

	ossDownloadCmd.Flags().String("key", "", "OSS object key")
	ossDownloadCmd.Flags().String("output", "", "Local output path (default: basename of key)")
	ossCmd.AddCommand(ossDownloadCmd)

	rootCmd.AddCommand(ossCmd)
}
