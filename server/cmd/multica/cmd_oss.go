package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/multica-ai/multica/server/internal/cli"
)

var ossCmd = &cobra.Command{
	Use:     "oss",
	Short:   "Object Storage operations",
	Long:    "Upload, download, list, and delete files in the workspace Object Storage Service.\nAll commands use the workspace's default OSS provider.",
	GroupID: groupCore,
}

var ossUploadCmd = &cobra.Command{
	Use:   "upload",
	Short: "Upload a file to OSS",
	Long: `Upload a local file to the default OSS provider for this workspace.
Returns the file ID (UUID) — save this to use with 'oss download' or 'oss delete'.`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		client, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		cfgID, err := resolveDefaultOSSConfig(cmd.Context(), client)
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

		// Upload via the OSS config API so the file is stored through the
		// provider driver and creates an oss_object record — consistent with
		// list / download / delete.
		u := fmt.Sprintf("/api/oss/configs/%s/files/upload", cfgID)
		body, contentType, err := multipartFileBody("file", filePath, map[string]string{"key": ossKey})
		if err != nil {
			return fmt.Errorf("oss upload: %w", err)
		}
		req, err := http.NewRequestWithContext(cmd.Context(), http.MethodPost, client.BaseURL+u, body)
		if err != nil {
			return fmt.Errorf("oss upload: %w", err)
		}
		req.Header.Set("Content-Type", contentType)
		req.Header.Set("Authorization", "Bearer "+client.Token)
		req.Header.Set("X-Workspace-ID", client.WorkspaceID)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return fmt.Errorf("oss upload: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 400 {
			b, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
			return fmt.Errorf("oss upload: HTTP %d: %s", resp.StatusCode, string(b))
		}
		var result map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return fmt.Errorf("oss upload: decode response: %w", err)
		}
		if id, ok := result["id"].(string); ok {
			fmt.Println(id)
		} else {
			cli.PrintJSON(os.Stdout, result)
		}
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
		cfgID, err := resolveDefaultOSSConfig(cmd.Context(), client)
		if err != nil {
			return err
		}
		prefix, _ := cmd.Flags().GetString("prefix")
		u := fmt.Sprintf("/api/oss/configs/%s/files", cfgID)
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
		cfgID, err := resolveDefaultOSSConfig(cmd.Context(), client)
		if err != nil {
			return err
		}
		fileID, _ := cmd.Flags().GetString("id")
		if fileID == "" {
			return fmt.Errorf("--id is required (the file UUID returned by 'oss upload')")
		}
		outputPath, _ := cmd.Flags().GetString("output")
		if outputPath == "" {
			return fmt.Errorf("--output is required")
		}

		// Get the download URL
		var obj struct {
			URL string `json:"url"`
			Key string `json:"key"`
		}
		u := fmt.Sprintf("/api/oss/configs/%s/files/%s", cfgID, fileID)
		if err := client.GetJSON(cmd.Context(), u, &obj); err != nil {
			return fmt.Errorf("oss download: %w", err)
		}
		if obj.URL == "" {
			return fmt.Errorf("oss download: no url in response for %s", fileID)
		}

		// Download the actual file bytes
		resp, err := http.Get(obj.URL)
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

var ossDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete a file from OSS",
	Long: `Delete a file from OSS by its file ID (returned by 'oss upload').`,
	RunE: func(cmd *cobra.Command, _ []string) error {
		client, err := newAPIClient(cmd)
		if err != nil {
			return err
		}
		cfgID, err := resolveDefaultOSSConfig(cmd.Context(), client)
		if err != nil {
			return err
		}
		fileID, _ := cmd.Flags().GetString("id")
		if fileID == "" {
			return fmt.Errorf("--id is required (the file UUID returned by 'oss upload')")
		}
		u := fmt.Sprintf("/api/oss/configs/%s/files/%s", cfgID, fileID)
		if err := client.DeleteJSON(cmd.Context(), u); err != nil {
			return fmt.Errorf("oss delete: %w", err)
		}
		fmt.Println("Deleted")
		return nil
	},
}

// resolveDefaultOSSConfig returns the UUID of the workspace's default OSS config.
func resolveDefaultOSSConfig(ctx context.Context, client *cli.APIClient) (string, error) {
	var configs []struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		IsDefault bool   `json:"is_default"`
	}
	if err := client.GetJSON(ctx, "/api/oss/configs", &configs); err != nil {
		return "", fmt.Errorf("list OSS configs: %w", err)
	}
	for _, c := range configs {
		if c.IsDefault {
			return c.ID, nil
		}
	}
	return "", fmt.Errorf("no default OSS config found — set one via the web UI or API")
}

// multipartFileBody builds a multipart/form-data body with a file field and optional extra fields.
func multipartFileBody(fieldName, filePath string, extraFields map[string]string) (io.Reader, string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, "", err
	}
	defer f.Close()

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile(fieldName, filepath.Base(filePath))
	if err != nil {
		return nil, "", err
	}
	if _, err := io.Copy(part, f); err != nil {
		return nil, "", err
	}
	for k, v := range extraFields {
		_ = w.WriteField(k, v)
	}
	w.Close()
	return &buf, w.FormDataContentType(), nil
}

func init() {
	ossUploadCmd.Flags().String("file", "", "Local file path to upload")
	ossUploadCmd.Flags().String("key", "", "OSS object key (default: filename)")
	ossCmd.AddCommand(ossUploadCmd)

	ossListCmd.Flags().String("prefix", "", "Filter by key prefix")
	ossCmd.AddCommand(ossListCmd)

	ossDownloadCmd.Flags().String("id", "", "File ID (UUID, returned by 'oss upload')")
	ossDownloadCmd.Flags().String("output", "", "Local output path")
	ossCmd.AddCommand(ossDownloadCmd)

	ossDeleteCmd.Flags().String("id", "", "File ID (UUID, returned by 'oss upload')")
	ossCmd.AddCommand(ossDeleteCmd)

	rootCmd.AddCommand(ossCmd)
}
