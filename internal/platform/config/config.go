package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	ServiceUpload = "upload-service"
	ServiceAccess = "access-service"
	ServiceAdmin  = "admin-service"
	ServiceJob    = "job-service"
)

type Config struct {
	App     AppConfig
	HTTP    HTTPConfig
	DB      DBConfig
	Redis   RedisConfig
	Storage StorageConfig
	Metrics MetricsConfig
	OTEL    OTELConfig

	Upload UploadConfig
	Access AccessConfig
	Admin  AdminConfig
	Job    JobConfig
}

type AppConfig struct {
	Env             string
	ServiceName     string
	LogLevel        string
	ShutdownTimeout time.Duration
}

type HTTPConfig struct {
	Port         int
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

type DBConfig struct {
	DSN             string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

type StorageConfig struct {
	Endpoint       string
	Region         string
	AccessKey      string
	SecretKey      string
	PublicBucket   string
	PrivateBucket  string
	PublicBaseURL  string
	ForcePathStyle bool
}

type MetricsConfig struct {
	Enabled bool
}

type OTELConfig struct {
	Endpoint       string
	ServiceVersion string
}

type UploadConfig struct {
	MaxInlineSize   int64
	SessionTTL      time.Duration
	CompleteTimeout time.Duration
	PresignTTL      time.Duration
	AllowedModes    []string
}

type AccessConfig struct {
	TicketSigningKey    string
	TicketTTL           time.Duration
	DownloadRedirectTTL time.Duration
	PublicURLEnabled    bool
	PrivatePresignTTL   time.Duration
}

type AdminConfig struct {
	AuthJWKS             string
	AuthAllowedIssuers   []string
	DeleteRequiresReason bool
	ListDefaultLimit     int
	ListMaxLimit         int
}

type JobConfig struct {
	SchedulerEnabled              bool
	DefaultBatchSize              int
	DefaultMaxConcurrency         int
	LockBackend                   string
	LockTTL                       time.Duration
	LockRenewInterval             time.Duration
	ExpireUploadSessionsInterval  time.Duration
	RepairStuckCompletingInterval time.Duration
	FinalizeFileDeleteInterval    time.Duration
	FileDeleteRetention           time.Duration
	CleanupOrphanBlobsInterval    time.Duration
	ReconcileTenantUsageInterval  time.Duration
}

func Load(serviceName string) (Config, error) {
	cfg := Config{
		App: AppConfig{
			Env:             "dev",
			LogLevel:        "info",
			ShutdownTimeout: 15 * time.Second,
		},
		HTTP: HTTPConfig{
			Port:         8080,
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 30 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
		DB: DBConfig{
			MaxOpenConns:    50,
			MaxIdleConns:    10,
			ConnMaxLifetime: 30 * time.Minute,
		},
		Redis: RedisConfig{DB: 0},
		Storage: StorageConfig{
			ForcePathStyle: true,
		},
		Metrics: MetricsConfig{Enabled: true},
		OTEL: OTELConfig{
			ServiceVersion: "dev",
		},
		Upload: UploadConfig{
			MaxInlineSize:   10 * 1024 * 1024,
			SessionTTL:      24 * time.Hour,
			CompleteTimeout: 30 * time.Second,
			PresignTTL:      15 * time.Minute,
			AllowedModes:    []string{"INLINE", "PRESIGNED_SINGLE", "DIRECT"},
		},
		Access: AccessConfig{
			TicketTTL:           5 * time.Minute,
			DownloadRedirectTTL: 2 * time.Minute,
			PublicURLEnabled:    true,
			PrivatePresignTTL:   2 * time.Minute,
		},
		Admin: AdminConfig{
			DeleteRequiresReason: true,
			ListDefaultLimit:     50,
			ListMaxLimit:         200,
		},
		Job: JobConfig{
			SchedulerEnabled:              true,
			DefaultBatchSize:              100,
			DefaultMaxConcurrency:         4,
			LockBackend:                   "redis",
			LockTTL:                       30 * time.Second,
			LockRenewInterval:             10 * time.Second,
			ExpireUploadSessionsInterval:  5 * time.Minute,
			RepairStuckCompletingInterval: 2 * time.Minute,
			FinalizeFileDeleteInterval:    1 * time.Minute,
			FileDeleteRetention:           168 * time.Hour,
			CleanupOrphanBlobsInterval:    10 * time.Minute,
			ReconcileTenantUsageInterval:  30 * time.Minute,
		},
	}

	if serviceName == "" {
		serviceName = strings.TrimSpace(os.Getenv("APP_SERVICE_NAME"))
	}

	cfg.App.Env = env("APP_ENV", cfg.App.Env)
	cfg.App.ServiceName = serviceName
	cfg.App.LogLevel = strings.ToLower(env("APP_LOG_LEVEL", cfg.App.LogLevel))
	cfg.DB.DSN = strings.TrimSpace(os.Getenv("DB_DSN"))
	cfg.Redis.Addr = strings.TrimSpace(os.Getenv("REDIS_ADDR"))
	cfg.Redis.Password = os.Getenv("REDIS_PASSWORD")
	cfg.Storage.Endpoint = strings.TrimSpace(os.Getenv("STORAGE_ENDPOINT"))
	cfg.Storage.Region = strings.TrimSpace(os.Getenv("STORAGE_REGION"))
	cfg.Storage.AccessKey = strings.TrimSpace(os.Getenv("STORAGE_ACCESS_KEY"))
	cfg.Storage.SecretKey = strings.TrimSpace(os.Getenv("STORAGE_SECRET_KEY"))
	cfg.Storage.PublicBucket = strings.TrimSpace(os.Getenv("STORAGE_PUBLIC_BUCKET"))
	cfg.Storage.PrivateBucket = strings.TrimSpace(os.Getenv("STORAGE_PRIVATE_BUCKET"))
	cfg.Storage.PublicBaseURL = strings.TrimSpace(os.Getenv("STORAGE_PUBLIC_BASE_URL"))
	cfg.OTEL.Endpoint = strings.TrimSpace(os.Getenv("OTEL_ENDPOINT"))
	cfg.OTEL.ServiceVersion = env("OTEL_SERVICE_VERSION", cfg.OTEL.ServiceVersion)
	cfg.Access.TicketSigningKey = strings.TrimSpace(os.Getenv("ACCESS_TICKET_SIGNING_KEY"))
	cfg.Admin.AuthJWKS = strings.TrimSpace(os.Getenv("ADMIN_AUTH_JWKS"))
	cfg.Admin.AuthAllowedIssuers = splitCSV(os.Getenv("ADMIN_AUTH_ALLOWED_ISSUERS"))

	var errs []error

	if value, err := durationFromEnv("APP_SHUTDOWN_TIMEOUT", cfg.App.ShutdownTimeout); err != nil {
		errs = append(errs, err)
	} else {
		cfg.App.ShutdownTimeout = value
	}
	if value, err := intFromEnv("HTTP_PORT", cfg.HTTP.Port); err != nil {
		errs = append(errs, err)
	} else {
		cfg.HTTP.Port = value
	}
	if value, err := durationFromEnv("HTTP_READ_TIMEOUT", cfg.HTTP.ReadTimeout); err != nil {
		errs = append(errs, err)
	} else {
		cfg.HTTP.ReadTimeout = value
	}
	if value, err := durationFromEnv("HTTP_WRITE_TIMEOUT", cfg.HTTP.WriteTimeout); err != nil {
		errs = append(errs, err)
	} else {
		cfg.HTTP.WriteTimeout = value
	}
	if value, err := durationFromEnv("HTTP_IDLE_TIMEOUT", cfg.HTTP.IdleTimeout); err != nil {
		errs = append(errs, err)
	} else {
		cfg.HTTP.IdleTimeout = value
	}
	if value, err := intFromEnv("DB_MAX_OPEN_CONNS", cfg.DB.MaxOpenConns); err != nil {
		errs = append(errs, err)
	} else {
		cfg.DB.MaxOpenConns = value
	}
	if value, err := intFromEnv("DB_MAX_IDLE_CONNS", cfg.DB.MaxIdleConns); err != nil {
		errs = append(errs, err)
	} else {
		cfg.DB.MaxIdleConns = value
	}
	if value, err := durationFromEnv("DB_CONN_MAX_LIFETIME", cfg.DB.ConnMaxLifetime); err != nil {
		errs = append(errs, err)
	} else {
		cfg.DB.ConnMaxLifetime = value
	}
	if value, err := intFromEnv("REDIS_DB", cfg.Redis.DB); err != nil {
		errs = append(errs, err)
	} else {
		cfg.Redis.DB = value
	}
	if value, err := boolFromEnv("STORAGE_FORCE_PATH_STYLE", cfg.Storage.ForcePathStyle); err != nil {
		errs = append(errs, err)
	} else {
		cfg.Storage.ForcePathStyle = value
	}
	if value, err := boolFromEnv("METRICS_ENABLED", cfg.Metrics.Enabled); err != nil {
		errs = append(errs, err)
	} else {
		cfg.Metrics.Enabled = value
	}

	if value, err := int64FromEnv("UPLOAD_MAX_INLINE_SIZE", cfg.Upload.MaxInlineSize); err != nil {
		errs = append(errs, err)
	} else {
		cfg.Upload.MaxInlineSize = value
	}
	if value, err := durationFromEnv("UPLOAD_SESSION_TTL", cfg.Upload.SessionTTL); err != nil {
		errs = append(errs, err)
	} else {
		cfg.Upload.SessionTTL = value
	}
	if value, err := durationFromEnv("UPLOAD_COMPLETE_TIMEOUT", cfg.Upload.CompleteTimeout); err != nil {
		errs = append(errs, err)
	} else {
		cfg.Upload.CompleteTimeout = value
	}
	if value, err := durationFromEnv("UPLOAD_PRESIGN_TTL", cfg.Upload.PresignTTL); err != nil {
		errs = append(errs, err)
	} else {
		cfg.Upload.PresignTTL = value
	}
	if rawModes := strings.TrimSpace(os.Getenv("UPLOAD_ALLOWED_MODES")); rawModes != "" {
		cfg.Upload.AllowedModes = splitCSV(rawModes)
	}

	if value, err := durationFromEnv("ACCESS_TICKET_TTL", cfg.Access.TicketTTL); err != nil {
		errs = append(errs, err)
	} else {
		cfg.Access.TicketTTL = value
	}
	if value, err := durationFromEnv("ACCESS_DOWNLOAD_REDIRECT_TTL", cfg.Access.DownloadRedirectTTL); err != nil {
		errs = append(errs, err)
	} else {
		cfg.Access.DownloadRedirectTTL = value
	}
	if value, err := boolFromEnv("ACCESS_PUBLIC_URL_ENABLED", cfg.Access.PublicURLEnabled); err != nil {
		errs = append(errs, err)
	} else {
		cfg.Access.PublicURLEnabled = value
	}
	if value, err := durationFromEnv("ACCESS_PRIVATE_PRESIGN_TTL", cfg.Access.PrivatePresignTTL); err != nil {
		errs = append(errs, err)
	} else {
		cfg.Access.PrivatePresignTTL = value
	}
	if value, err := boolFromEnv("ADMIN_DELETE_REQUIRES_REASON", cfg.Admin.DeleteRequiresReason); err != nil {
		errs = append(errs, err)
	} else {
		cfg.Admin.DeleteRequiresReason = value
	}
	if value, err := intFromEnv("ADMIN_LIST_DEFAULT_LIMIT", cfg.Admin.ListDefaultLimit); err != nil {
		errs = append(errs, err)
	} else {
		cfg.Admin.ListDefaultLimit = value
	}
	if value, err := intFromEnv("ADMIN_LIST_MAX_LIMIT", cfg.Admin.ListMaxLimit); err != nil {
		errs = append(errs, err)
	} else {
		cfg.Admin.ListMaxLimit = value
	}

	if value, err := boolFromEnv("JOB_SCHEDULER_ENABLED", cfg.Job.SchedulerEnabled); err != nil {
		errs = append(errs, err)
	} else {
		cfg.Job.SchedulerEnabled = value
	}
	if value, err := intFromEnv("JOB_DEFAULT_BATCH_SIZE", cfg.Job.DefaultBatchSize); err != nil {
		errs = append(errs, err)
	} else {
		cfg.Job.DefaultBatchSize = value
	}
	if value, err := intFromEnv("JOB_DEFAULT_MAX_CONCURRENCY", cfg.Job.DefaultMaxConcurrency); err != nil {
		errs = append(errs, err)
	} else {
		cfg.Job.DefaultMaxConcurrency = value
	}
	cfg.Job.LockBackend = env("JOB_LOCK_BACKEND", cfg.Job.LockBackend)
	if value, err := durationFromEnv("JOB_LOCK_TTL", cfg.Job.LockTTL); err != nil {
		errs = append(errs, err)
	} else {
		cfg.Job.LockTTL = value
	}
	if value, err := durationFromEnv("JOB_LOCK_RENEW_INTERVAL", cfg.Job.LockRenewInterval); err != nil {
		errs = append(errs, err)
	} else {
		cfg.Job.LockRenewInterval = value
	}
	if value, err := durationFromEnv("JOB_EXPIRE_UPLOAD_SESSIONS_INTERVAL", cfg.Job.ExpireUploadSessionsInterval); err != nil {
		errs = append(errs, err)
	} else {
		cfg.Job.ExpireUploadSessionsInterval = value
	}
	if value, err := durationFromEnv("JOB_REPAIR_STUCK_COMPLETING_INTERVAL", cfg.Job.RepairStuckCompletingInterval); err != nil {
		errs = append(errs, err)
	} else {
		cfg.Job.RepairStuckCompletingInterval = value
	}
	if value, err := durationFromEnv("JOB_FINALIZE_FILE_DELETE_INTERVAL", cfg.Job.FinalizeFileDeleteInterval); err != nil {
		errs = append(errs, err)
	} else {
		cfg.Job.FinalizeFileDeleteInterval = value
	}
	if value, err := durationFromEnv("JOB_FILE_DELETE_RETENTION", cfg.Job.FileDeleteRetention); err != nil {
		errs = append(errs, err)
	} else {
		cfg.Job.FileDeleteRetention = value
	}
	if value, err := durationFromEnv("JOB_CLEANUP_ORPHAN_BLOBS_INTERVAL", cfg.Job.CleanupOrphanBlobsInterval); err != nil {
		errs = append(errs, err)
	} else {
		cfg.Job.CleanupOrphanBlobsInterval = value
	}
	if value, err := durationFromEnv("JOB_RECONCILE_TENANT_USAGE_INTERVAL", cfg.Job.ReconcileTenantUsageInterval); err != nil {
		errs = append(errs, err)
	} else {
		cfg.Job.ReconcileTenantUsageInterval = value
	}

	if len(errs) > 0 {
		return Config{}, errors.Join(errs...)
	}

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func LoadFromEnv(serviceName string) (Config, error) {
	return Load(serviceName)
}

func (c Config) Validate() error {
	var errs []error

	if strings.TrimSpace(c.App.Env) == "" {
		errs = append(errs, errors.New("APP_ENV is required"))
	}
	if strings.TrimSpace(c.App.ServiceName) == "" {
		errs = append(errs, errors.New("APP_SERVICE_NAME is required"))
	}
	switch c.App.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		errs = append(errs, fmt.Errorf("APP_LOG_LEVEL must be one of debug/info/warn/error: %q", c.App.LogLevel))
	}

	if c.App.ShutdownTimeout <= 0 {
		errs = append(errs, errors.New("APP_SHUTDOWN_TIMEOUT must be > 0"))
	}
	if c.HTTP.Port <= 0 || c.HTTP.Port > 65535 {
		errs = append(errs, errors.New("HTTP_PORT must be in range 1-65535"))
	}
	if c.HTTP.ReadTimeout <= 0 {
		errs = append(errs, errors.New("HTTP_READ_TIMEOUT must be > 0"))
	}
	if c.HTTP.WriteTimeout <= 0 {
		errs = append(errs, errors.New("HTTP_WRITE_TIMEOUT must be > 0"))
	}
	if c.HTTP.IdleTimeout <= 0 {
		errs = append(errs, errors.New("HTTP_IDLE_TIMEOUT must be > 0"))
	}

	if strings.TrimSpace(c.DB.DSN) == "" {
		errs = append(errs, errors.New("DB_DSN is required"))
	}
	if c.DB.MaxOpenConns <= 0 {
		errs = append(errs, errors.New("DB_MAX_OPEN_CONNS must be > 0"))
	}
	if c.DB.MaxIdleConns < 0 {
		errs = append(errs, errors.New("DB_MAX_IDLE_CONNS must be >= 0"))
	}
	if c.DB.MaxIdleConns > c.DB.MaxOpenConns {
		errs = append(errs, errors.New("DB_MAX_IDLE_CONNS must be <= DB_MAX_OPEN_CONNS"))
	}
	if c.DB.ConnMaxLifetime <= 0 {
		errs = append(errs, errors.New("DB_CONN_MAX_LIFETIME must be > 0"))
	}

	if strings.TrimSpace(c.Storage.Endpoint) == "" {
		errs = append(errs, errors.New("STORAGE_ENDPOINT is required"))
	}
	if strings.TrimSpace(c.Storage.AccessKey) == "" {
		errs = append(errs, errors.New("STORAGE_ACCESS_KEY is required"))
	}
	if strings.TrimSpace(c.Storage.SecretKey) == "" {
		errs = append(errs, errors.New("STORAGE_SECRET_KEY is required"))
	}
	if strings.TrimSpace(c.Storage.PublicBucket) == "" {
		errs = append(errs, errors.New("STORAGE_PUBLIC_BUCKET is required"))
	}
	if strings.TrimSpace(c.Storage.PrivateBucket) == "" {
		errs = append(errs, errors.New("STORAGE_PRIVATE_BUCKET is required"))
	}

	switch c.App.ServiceName {
	case ServiceUpload, ServiceJob:
		if strings.TrimSpace(c.Redis.Addr) == "" {
			errs = append(errs, errors.New("REDIS_ADDR is required for upload-service and job-service"))
		}
	case ServiceAccess:
		if strings.TrimSpace(c.Access.TicketSigningKey) == "" {
			errs = append(errs, errors.New("ACCESS_TICKET_SIGNING_KEY is required for access-service"))
		}
	case ServiceAdmin:
		if strings.TrimSpace(c.Admin.AuthJWKS) == "" {
			errs = append(errs, errors.New("ADMIN_AUTH_JWKS is required for admin-service"))
		}
	}

	if c.Upload.MaxInlineSize <= 0 {
		errs = append(errs, errors.New("UPLOAD_MAX_INLINE_SIZE must be > 0"))
	}
	if c.Upload.SessionTTL <= 0 {
		errs = append(errs, errors.New("UPLOAD_SESSION_TTL must be > 0"))
	}
	if len(c.Upload.AllowedModes) == 0 {
		errs = append(errs, errors.New("UPLOAD_ALLOWED_MODES must contain at least one value"))
	} else {
		allowedModes := map[string]struct{}{
			"INLINE":           {},
			"PRESIGNED_SINGLE": {},
			"DIRECT":           {},
		}
		for _, mode := range c.Upload.AllowedModes {
			if _, ok := allowedModes[mode]; !ok {
				errs = append(errs, fmt.Errorf("UPLOAD_ALLOWED_MODES contains unsupported value: %q", mode))
			}
		}
	}
	if c.Admin.ListDefaultLimit <= 0 {
		errs = append(errs, errors.New("ADMIN_LIST_DEFAULT_LIMIT must be > 0"))
	}
	if c.Admin.ListMaxLimit < c.Admin.ListDefaultLimit {
		errs = append(errs, errors.New("ADMIN_LIST_MAX_LIMIT must be >= ADMIN_LIST_DEFAULT_LIMIT"))
	}
	if c.Job.DefaultBatchSize <= 0 {
		errs = append(errs, errors.New("JOB_DEFAULT_BATCH_SIZE must be > 0"))
	}
	if c.Job.DefaultMaxConcurrency <= 0 {
		errs = append(errs, errors.New("JOB_DEFAULT_MAX_CONCURRENCY must be > 0"))
	}
	if c.Job.LockBackend != "redis" {
		errs = append(errs, fmt.Errorf("JOB_LOCK_BACKEND must be redis: %q", c.Job.LockBackend))
	}
	if c.Job.LockTTL <= 0 {
		errs = append(errs, errors.New("JOB_LOCK_TTL must be > 0"))
	}
	if c.Job.LockRenewInterval <= 0 {
		errs = append(errs, errors.New("JOB_LOCK_RENEW_INTERVAL must be > 0"))
	}
	if c.Job.LockRenewInterval >= c.Job.LockTTL {
		errs = append(errs, errors.New("JOB_LOCK_RENEW_INTERVAL must be < JOB_LOCK_TTL"))
	}
	if c.Job.ExpireUploadSessionsInterval <= 0 {
		errs = append(errs, errors.New("JOB_EXPIRE_UPLOAD_SESSIONS_INTERVAL must be > 0"))
	}
	if c.Job.RepairStuckCompletingInterval <= 0 {
		errs = append(errs, errors.New("JOB_REPAIR_STUCK_COMPLETING_INTERVAL must be > 0"))
	}
	if c.Job.FinalizeFileDeleteInterval <= 0 {
		errs = append(errs, errors.New("JOB_FINALIZE_FILE_DELETE_INTERVAL must be > 0"))
	}
	if c.Job.FileDeleteRetention <= 0 {
		errs = append(errs, errors.New("JOB_FILE_DELETE_RETENTION must be > 0"))
	}
	if c.Job.CleanupOrphanBlobsInterval <= 0 {
		errs = append(errs, errors.New("JOB_CLEANUP_ORPHAN_BLOBS_INTERVAL must be > 0"))
	}
	if c.Job.ReconcileTenantUsageInterval <= 0 {
		errs = append(errs, errors.New("JOB_RECONCILE_TENANT_USAGE_INTERVAL must be > 0"))
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

func (c Config) RedisRequired() bool {
	return c.App.ServiceName == ServiceUpload || c.App.ServiceName == ServiceJob
}

func durationFromEnv(key string, fallback time.Duration) (time.Duration, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("%s: invalid duration %q: %w", key, value, err)
	}
	return parsed, nil
}

func intFromEnv(key string, fallback int) (int, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s: invalid int %q: %w", key, value, err)
	}
	return parsed, nil
}

func int64FromEnv(key string, fallback int64) (int64, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s: invalid int64 %q: %w", key, value, err)
	}
	return parsed, nil
}

func boolFromEnv(key string, fallback bool) (bool, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("%s: invalid bool %q: %w", key, value, err)
	}
	return parsed, nil
}

func env(key string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func splitCSV(raw string) []string {
	parts := strings.Split(raw, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value != "" {
			result = append(result, value)
		}
	}
	return result
}
