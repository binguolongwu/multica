package oss

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sort"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/multica-ai/multica/server/internal/util/secretbox"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

// Service handles OSS provider configuration and file operations.
type Service struct {
	Queries   *db.Queries
	SecretBox *secretbox.Box
	Drivers   map[string]ProviderDriver
}

// New creates a new Service. box may be nil if encryption is not configured.
func New(queries *db.Queries, box *secretbox.Box) *Service {
	return &Service{
		Queries:   queries,
		SecretBox: box,
		Drivers:   make(map[string]ProviderDriver),
	}
}

// RegisterDriver adds a provider driver.
func (s *Service) RegisterDriver(name string, driver ProviderDriver) {
	s.Drivers[name] = driver
}

// driverFor returns the driver for a provider name, or an error.
func (s *Service) driverFor(provider string) (ProviderDriver, error) {
	d, ok := s.Drivers[provider]
	if !ok {
		return nil, fmt.Errorf("OSS provider %q is not supported. Supported: %v", provider, s.supportedProviders())
	}
	return d, nil
}

func (s *Service) supportedProviders() []string {
	keys := make([]string, 0, len(s.Drivers))
	for k := range s.Drivers {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// --- Config CRUD ---

// CreateConfig creates a new OSS provider config.
func (s *Service) CreateConfig(ctx context.Context, workspaceID pgtype.UUID, params CreateConfigParams) (db.OssProviderConfig, error) {
	var encrypted []byte
	if params.SecretKey != "" && s.SecretBox != nil {
		var err error
		encrypted, err = s.SecretBox.Seal([]byte(params.SecretKey))
		if err != nil {
			return db.OssProviderConfig{}, fmt.Errorf("encrypt secret_key: %w", err)
		}
	}
	cfg, err := s.Queries.CreateOSSProviderConfig(ctx, db.CreateOSSProviderConfigParams{
		WorkspaceID:        workspaceID,
		Name:               params.Name,
		Provider:           params.Provider,
		Bucket:             params.Bucket,
		Region:             params.Region,
		Endpoint:           params.Endpoint,
		AccessKey:          params.AccessKey,
		SecretKeyEncrypted: encrypted,
		CustomDomain:       params.CustomDomain,
		FolderPrefix:       params.FolderPrefix,
		IsDefault:          params.IsDefault,
	})
	if err != nil {
		return db.OssProviderConfig{}, fmt.Errorf("create oss config: %w", err)
	}
	return cfg, nil
}

// GetConfig returns a single config by ID, scoped to workspace.
func (s *Service) GetConfig(ctx context.Context, id, workspaceID pgtype.UUID) (db.OssProviderConfig, error) {
	return s.Queries.GetOSSProviderConfig(ctx, db.GetOSSProviderConfigParams{ID: id, WorkspaceID: workspaceID})
}

// ListConfigs returns all OSS configs for a workspace.
func (s *Service) ListConfigs(ctx context.Context, workspaceID pgtype.UUID) ([]db.OssProviderConfig, error) {
	cfgs, err := s.Queries.ListOSSProviderConfigs(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list oss configs: %w", err)
	}
	sort.Slice(cfgs, func(i, j int) bool { return cfgs[i].Name < cfgs[j].Name })
	return cfgs, nil
}

// UpdateConfig updates an existing OSS provider config.
func (s *Service) UpdateConfig(ctx context.Context, id, workspaceID pgtype.UUID, params UpdateConfigParams) (db.OssProviderConfig, error) {
	var encrypted []byte
	if params.SecretKey != "" && s.SecretBox != nil {
		var err error
		encrypted, err = s.SecretBox.Seal([]byte(params.SecretKey))
		if err != nil {
			return db.OssProviderConfig{}, fmt.Errorf("encrypt secret_key: %w", err)
		}
	}
	cfg, err := s.Queries.UpdateOSSProviderConfig(ctx, db.UpdateOSSProviderConfigParams{
		ID:                 id,
		Name:               params.Name,
		Provider:           params.Provider,
		Bucket:             params.Bucket,
		Region:             params.Region,
		Endpoint:           params.Endpoint,
		AccessKey:          params.AccessKey,
		SecretKeyEncrypted: encrypted,
		CustomDomain:       params.CustomDomain,
		FolderPrefix:       params.FolderPrefix,
		IsDefault:          params.IsDefault,
		WorkspaceID:        workspaceID,
	})
	if err != nil {
		return db.OssProviderConfig{}, fmt.Errorf("update oss config: %w", err)
	}
	return cfg, nil
}

// DeleteConfig deletes an OSS provider config and cascades to its objects.
func (s *Service) DeleteConfig(ctx context.Context, id, workspaceID pgtype.UUID) error {
	return s.Queries.DeleteOSSProviderConfig(ctx, db.DeleteOSSProviderConfigParams{ID: id, WorkspaceID: workspaceID})
}

// --- File Operations ---

// UploadFile stores a file via the driver and records metadata in oss_object.
func (s *Service) UploadFile(ctx context.Context, configID pgtype.UUID, key, filename string, body io.Reader, size int64, contentType string, uploadedBy pgtype.UUID) (db.OssObject, error) {
	cfgRow, err := s.Queries.GetOSSProviderConfig(ctx, db.GetOSSProviderConfigParams{
		ID: configID, WorkspaceID: pgtype.UUID{Valid: false},
	})
	if err != nil {
		return db.OssObject{}, fmt.Errorf("get oss config: %w", err)
	}

	d, err := s.driverFor(cfgRow.Provider)
	if err != nil {
		return db.OssObject{}, err
	}

	pc := s.toProviderConfig(cfgRow)
	fullKey := resolveKey(pc.FolderPrefix, key)

	_, err = d.Upload(ctx, pc, fullKey, body, size, contentType)
	if err != nil {
		return db.OssObject{}, fmt.Errorf("oss upload: %w", err)
	}

	obj, err := s.Queries.CreateOSSObject(ctx, db.CreateOSSObjectParams{
		ConfigID:    configID,
		Key:         fullKey,
		Filename:    filename,
		SizeBytes:   size,
		ContentType: contentType,
		UploadedBy:  uploadedBy,
	})
	if err != nil {
		return db.OssObject{}, fmt.Errorf("create oss object: %w", err)
	}
	return obj, nil
}

// ListFiles returns object metadata for a config.
func (s *Service) ListFiles(ctx context.Context, configID pgtype.UUID, prefix string) ([]db.OssObject, error) {
	if prefix != "" {
		return s.Queries.ListOSSObjectsByPrefix(ctx, db.ListOSSObjectsByPrefixParams{
			ConfigID: configID,
			Key:      prefix + "%",
		})
	}
	return s.Queries.ListOSSObjects(ctx, configID)
}

// GetFile returns a single object record.
func (s *Service) GetFile(ctx context.Context, id, configID pgtype.UUID) (db.OssObject, error) {
	return s.Queries.GetOSSObject(ctx, db.GetOSSObjectParams{ID: id, ConfigID: configID})
}

// GetFileDownloadURL returns the public URL for a stored object.
func (s *Service) GetFileDownloadURL(ctx context.Context, id, configID pgtype.UUID) (string, error) {
	obj, err := s.GetFile(ctx, id, configID)
	if err != nil {
		return "", err
	}
	cfgRow, err := s.Queries.GetOSSProviderConfig(ctx, db.GetOSSProviderConfigParams{
		ID: configID, WorkspaceID: pgtype.UUID{Valid: false},
	})
	if err != nil {
		return "", err
	}
	d, err := s.driverFor(cfgRow.Provider)
	if err != nil {
		return "", err
	}
	return d.PublicURL(s.toProviderConfig(cfgRow), obj.Key), nil
}

// DeleteFile removes both the object record and the cloud object.
func (s *Service) DeleteFile(ctx context.Context, id, configID pgtype.UUID) error {
	obj, err := s.GetFile(ctx, id, configID)
	if err != nil {
		return err
	}
	cfgRow, err := s.Queries.GetOSSProviderConfig(ctx, db.GetOSSProviderConfigParams{
		ID: configID, WorkspaceID: pgtype.UUID{Valid: false},
	})
	if err != nil {
		return err
	}
	d, err := s.driverFor(cfgRow.Provider)
	if err != nil {
		return err
	}
	if err := d.Delete(ctx, s.toProviderConfig(cfgRow), obj.Key); err != nil {
		return fmt.Errorf("oss delete object: %w", err)
	}
	return s.Queries.DeleteOSSObject(ctx, db.DeleteOSSObjectParams{ID: id, ConfigID: configID})
}

// --- Internal helpers ---

func (s *Service) decryptSecretKey(encrypted []byte) string {
	if len(encrypted) == 0 || s.SecretBox == nil {
		return ""
	}
	plain, err := s.SecretBox.Open(encrypted)
	if err != nil {
		slog.Warn("oss: failed to decrypt secret_key", "error", err)
		return ""
	}
	return string(plain)
}

func (s *Service) toProviderConfig(cfg db.OssProviderConfig) ProviderConfig {
	return ProviderConfig{
		Provider:     cfg.Provider,
		Bucket:       cfg.Bucket,
		Region:       cfg.Region,
		Endpoint:     cfg.Endpoint,
		AccessKey:    cfg.AccessKey,
		SecretKey:    s.decryptSecretKey(cfg.SecretKeyEncrypted),
		CustomDomain: cfg.CustomDomain,
		FolderPrefix: cfg.FolderPrefix,
	}
}

func resolveKey(prefix, key string) string {
	if prefix != "" {
		return prefix + key
	}
	return key
}

// --- Request types ---

type CreateConfigParams struct {
	Name         string `json:"name"`
	Provider     string `json:"provider"`
	Bucket       string `json:"bucket"`
	Region       string `json:"region"`
	Endpoint     string `json:"endpoint"`
	AccessKey    string `json:"access_key"`
	SecretKey    string `json:"secret_key"`
	CustomDomain string `json:"custom_domain"`
	FolderPrefix string `json:"folder_prefix"`
	IsDefault    bool   `json:"is_default"`
}

type UpdateConfigParams struct {
	Name         string `json:"name"`
	Provider     string `json:"provider"`
	Bucket       string `json:"bucket"`
	Region       string `json:"region"`
	Endpoint     string `json:"endpoint"`
	AccessKey    string `json:"access_key"`
	SecretKey    string `json:"secret_key"`
	CustomDomain string `json:"custom_domain"`
	FolderPrefix string `json:"folder_prefix"`
	IsDefault    bool   `json:"is_default"`
}
