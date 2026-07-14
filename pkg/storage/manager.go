package storage

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"path"
	"sort"
	"strings"
)

type Driver interface {
	Name() string
	Put(ctx context.Context, objectPath string, content []byte) error
	PutStream(ctx context.Context, objectPath string, reader io.Reader, size int64, contentType string) error
	Delete(ctx context.Context, objectPaths ...string) error
	Exists(ctx context.Context, objectPath string) (bool, error)
	URL(objectPath string) string
}

type STSProvider interface {
	IssueToken(ctx context.Context, userID string) (*STSToken, error)
}

type STSToken struct {
	AccessKeyID     string
	SecretAccessKey string
	SessionToken    string
	Expiration      int64
	Prefixes        []string
	Actions         []string
}

type DiskConfig struct {
	Name      string
	Driver    string
	BaseURL   string
	Endpoint  string
	Region    string
	Bucket    string
	Prefix    string
	IsDefault bool
}

type Disk struct {
	Driver    Driver
	Config    DiskConfig
	stsIssuer STSProvider
}

type Manager struct {
	defaultDisk         string
	disks               map[string]*Disk
	defaultUploadPolicy string
	uploadPolicies      map[string]UploadPolicy
}

type ClientConfig struct {
	Disk       string           `json:"disk"`
	Driver     string           `json:"driver"`
	IsDefault  bool             `json:"is_default"`
	BaseURL    string           `json:"base_url"`
	Endpoint   string           `json:"endpoint"`
	Region     string           `json:"region"`
	Bucket     string           `json:"bucket"`
	Prefix     string           `json:"prefix"`
	UploadMode string           `json:"upload_mode"`
	STS        *STSClientConfig `json:"sts,omitempty"`
}

type STSClientConfig struct {
	AccessKeyID     string   `json:"access_key_id"`
	SecretAccessKey string   `json:"secret_access_key"`
	SessionToken    string   `json:"session_token"`
	Expiration      int64    `json:"expiration"`
	Prefixes        []string `json:"prefixes"`
	Actions         []string `json:"actions"`
}

type StoredFile struct {
	Disk        string `json:"disk"`
	Driver      string `json:"driver"`
	Purpose     string `json:"purpose"`
	Path        string `json:"path"`
	URL         string `json:"url"`
	Filename    string `json:"filename"`
	Size        int64  `json:"size"`
	ContentType string `json:"content_type"`
}

func NewManager(defaultDisk string) *Manager {
	manager := &Manager{
		defaultDisk:         strings.TrimSpace(defaultDisk),
		disks:               map[string]*Disk{},
		defaultUploadPolicy: "document",
		uploadPolicies:      map[string]UploadPolicy{},
	}
	for _, cfg := range DefaultUploadPolicyConfigs() {
		policy, err := NewUploadPolicy(cfg)
		if err == nil {
			manager.AddUploadPolicy(policy)
		}
	}
	return manager
}

func (m *Manager) AddUploadPolicy(policy UploadPolicy) {
	if m == nil || strings.TrimSpace(policy.Name()) == "" {
		return
	}
	m.uploadPolicies[policy.Name()] = policy
}

func (m *Manager) SetDefaultUploadPolicy(name string) {
	if m == nil {
		return
	}
	name = strings.ToLower(strings.TrimSpace(name))
	if name != "" {
		m.defaultUploadPolicy = name
	}
}

func (m *Manager) AddDisk(name string, driver Driver, cfg DiskConfig, stsProvider STSProvider) {
	diskName := normalizeDiskName(name)
	if diskName == "" || driver == nil {
		return
	}
	cfg.Name = diskName
	if cfg.Driver == "" {
		cfg.Driver = driver.Name()
	}
	m.disks[diskName] = &Disk{
		Driver:    driver,
		Config:    cfg,
		stsIssuer: stsProvider,
	}
	if m.defaultDisk == "" || cfg.IsDefault {
		m.defaultDisk = diskName
	}
}

func (m *Manager) DefaultDisk() string {
	return m.defaultDisk
}

func (m *Manager) Names() []string {
	names := make([]string, 0, len(m.disks))
	for name := range m.disks {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (m *Manager) Get(name string) (*Disk, error) {
	if m == nil {
		return nil, fmt.Errorf("storage manager is nil")
	}
	diskName := normalizeDiskName(name)
	if diskName == "" {
		diskName = normalizeDiskName(m.defaultDisk)
	}
	disk, ok := m.disks[diskName]
	if !ok {
		return nil, fmt.Errorf("storage disk %q is not configured", diskName)
	}
	return disk, nil
}

func (m *Manager) Describe(name string) (*ClientConfig, error) {
	disk, err := m.Get(name)
	if err != nil {
		return nil, err
	}
	return describeDisk(disk), nil
}

func (m *Manager) DescribeAll() []ClientConfig {
	names := m.Names()
	items := make([]ClientConfig, 0, len(names))
	for _, name := range names {
		items = append(items, *describeDisk(m.disks[name]))
	}
	return items
}

func (m *Manager) IssueClientConfig(ctx context.Context, name, userID string) (*ClientConfig, error) {
	disk, err := m.Get(name)
	if err != nil {
		return nil, err
	}

	cfg := describeDisk(disk)
	if disk.stsIssuer == nil {
		return cfg, nil
	}

	token, err := disk.stsIssuer.IssueToken(ctx, userID)
	if err != nil {
		return nil, err
	}
	cfg.UploadMode = "sts"
	cfg.STS = &STSClientConfig{
		AccessKeyID:     token.AccessKeyID,
		SecretAccessKey: token.SecretAccessKey,
		SessionToken:    token.SessionToken,
		Expiration:      token.Expiration,
		Prefixes:        token.Prefixes,
		Actions:         token.Actions,
	}
	return cfg, nil
}

func (m *Manager) SaveUploadedFile(ctx context.Context, diskName, purpose string, file *multipart.FileHeader) (*StoredFile, error) {
	disk, err := m.Get(diskName)
	if err != nil {
		return nil, err
	}
	if file == nil {
		return nil, fmt.Errorf("upload file is required")
	}
	policyName := strings.ToLower(strings.TrimSpace(purpose))
	if policyName == "" {
		policyName = m.defaultUploadPolicy
	}
	policy, ok := m.uploadPolicies[policyName]
	if !ok {
		return nil, fmt.Errorf("upload policy %q is not configured", policyName)
	}
	src, err := file.Open()
	if err != nil {
		return nil, fmt.Errorf("open upload file: %w", err)
	}
	defer src.Close()
	validated, err := policy.Inspect(file.Filename, file.Size, src)
	if err != nil {
		return nil, err
	}
	objectDir := buildObjectDir(disk.Config.Prefix, validated.Directory)
	objectPath := buildUploadedObjectPath(objectDir, validated.Filename)
	if err := disk.Driver.PutStream(ctx, objectPath, validated.Reader, validated.Size, validated.ContentType); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUploadStore, err)
	}

	return &StoredFile{
		Disk:        disk.Config.Name,
		Driver:      disk.Config.Driver,
		Purpose:     policyName,
		Path:        objectPath,
		URL:         disk.Driver.URL(objectPath),
		Filename:    file.Filename,
		Size:        file.Size,
		ContentType: validated.ContentType,
	}, nil
}

func describeDisk(disk *Disk) *ClientConfig {
	return &ClientConfig{
		Disk:       disk.Config.Name,
		Driver:     disk.Config.Driver,
		IsDefault:  disk.Config.IsDefault,
		BaseURL:    disk.Config.BaseURL,
		Endpoint:   disk.Config.Endpoint,
		Region:     disk.Config.Region,
		Bucket:     disk.Config.Bucket,
		Prefix:     disk.Config.Prefix,
		UploadMode: "server",
	}
}

func normalizeDiskName(name string) string {
	return strings.TrimSpace(strings.ToLower(name))
}

func buildObjectDir(prefix, directory string) string {
	joined := strings.TrimSpace(path.Clean("/" + strings.TrimSpace(directory)))
	joined = strings.TrimPrefix(joined, "/")
	if joined == "." {
		joined = ""
	}
	basePrefix := strings.Trim(strings.TrimSpace(prefix), "/")
	if basePrefix == "" {
		return joined
	}
	if joined == "" {
		return basePrefix
	}
	return basePrefix + "/" + joined
}
