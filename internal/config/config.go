package config

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

const (
	defaultHTTPHost           = ""
	defaultHTTPPort           = "8080"
	defaultAccessTokenTTL     = time.Hour
	defaultRefreshTokenTTL    = 7 * 24 * time.Hour
	defaultUserCacheTTL       = time.Hour
	defaultPermissionCacheTTL = 30 * time.Minute
	defaultSTSTTL             = 30 * time.Minute
)

type Config struct {
	App       AppConfig
	HTTP      HTTPConfig
	DB        DBConfig
	Redis     RedisConfig
	JWT       JWTConfig
	AI        AIConfig
	Storage   StorageConfig
	Bootstrap BootstrapConfig
}

type AppConfig struct {
	Name string
	Env  string
}

type HTTPConfig struct {
	Host string
	Port string
}

type DBConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Name     string
	SSLMode  string
}

type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

type JWTConfig struct {
	Issuer             string
	Secret             string
	AccessTokenTTL     time.Duration
	RefreshTokenTTL    time.Duration
	UserCacheTTL       time.Duration
	PermissionCacheTTL time.Duration
}

type AIConfig struct {
	BaseURL           string
	AnalysisPath      string
	APIToken          string
	RequestTimeout    time.Duration
	PollInterval      time.Duration
	JobNotFoundGrace  time.Duration
	PollBatchSize     int
	CreateConcurrency int
	PollConcurrency   int
}

type StorageConfig struct {
	Provider        string
	PublicBaseURL   string
	Bucket          string
	Region          string
	Endpoint        string
	IAMEndpoint     string
	AccessKeyID     string
	AccessKeySecret string
	STSTTL          time.Duration
}

type BootstrapConfig struct {
	Enabled           bool
	InitAdminUsername string
	InitAdminPassword string
	InitAdminRealName string
}

func Load() (Config, error) {
	// Prefer local .env for development, but do not fail if it is absent.
	_ = godotenv.Load()

	cfg := Config{
		App: AppConfig{
			Name: getEnv("APP_NAME", "skilljudge-backend"),
			Env:  getEnv("APP_ENV", "development"),
		},
		HTTP: HTTPConfig{
			Host: getEnv("HTTP_HOST", defaultHTTPHost),
			Port: getEnv("HTTP_PORT", defaultHTTPPort),
		},
		DB: DBConfig{
			Host:     getEnv("DB_HOST", "127.0.0.1"),
			Port:     getEnv("DB_PORT", "5432"),
			User:     getEnv("DB_USER", "skilljudge"),
			Password: getEnv("DB_PASSWORD", ""),
			Name:     getEnv("DB_NAME", "skilljudge_db"),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
		},
		Redis: RedisConfig{
			Addr:     getEnv("REDIS_ADDR", "127.0.0.1:6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       getEnvInt("REDIS_DB", 0),
		},
		JWT: JWTConfig{
			Issuer:             getEnv("JWT_ISSUER", "skilljudge"),
			Secret:             os.Getenv("JWT_SECRET"),
			AccessTokenTTL:     getEnvDuration("JWT_ACCESS_TOKEN_TTL", defaultAccessTokenTTL),
			RefreshTokenTTL:    getEnvDuration("JWT_REFRESH_TOKEN_TTL", defaultRefreshTokenTTL),
			UserCacheTTL:       getEnvDuration("USER_CACHE_TTL", defaultUserCacheTTL),
			PermissionCacheTTL: getEnvDuration("PERMISSION_CACHE_TTL", defaultPermissionCacheTTL),
		},
		AI: AIConfig{
			BaseURL:           getEnv("AI_BASE_URL", ""),
			AnalysisPath:      getEnv("AI_ANALYSIS_PATH", "/api/v1/analysis-jobs"),
			APIToken:          getEnv("AI_API_TOKEN", ""),
			RequestTimeout:    getEnvDuration("AI_REQUEST_TIMEOUT", 15*time.Second),
			PollInterval:      getEnvDuration("AI_POLL_INTERVAL", 2*time.Minute),
			JobNotFoundGrace:  getEnvDuration("AI_JOB_NOT_FOUND_GRACE_PERIOD", 10*time.Minute),
			PollBatchSize:     getEnvInt("AI_POLL_BATCH_SIZE", 20),
			CreateConcurrency: getEnvInt("AI_CREATE_CONCURRENCY", 5),
			PollConcurrency:   getEnvInt("AI_POLL_CONCURRENCY", 5),
		},
		Storage: StorageConfig{
			Provider:        getEnv("STORAGE_PROVIDER", "mock"),
			PublicBaseURL:   getEnv("STORAGE_PUBLIC_BASE_URL", "http://127.0.0.1:9000"),
			Bucket:          getEnv("STORAGE_BUCKET", "skilljudge-videos"),
			Region:          getEnv("STORAGE_REGION", "cn-north-4"),
			Endpoint:        getEnv("STORAGE_ENDPOINT", ""),
			IAMEndpoint:     getEnv("STORAGE_IAM_ENDPOINT", "https://iam.myhuaweicloud.com"),
			AccessKeyID:     getEnv("STORAGE_ACCESS_KEY_ID", ""),
			AccessKeySecret: getEnv("STORAGE_ACCESS_KEY_SECRET", ""),
			STSTTL:          getEnvDuration("STORAGE_STS_TTL", defaultSTSTTL),
		},
		Bootstrap: BootstrapConfig{
			Enabled:           getEnvBool("BOOTSTRAP_ENABLED", false),
			InitAdminUsername: getEnv("INIT_ADMIN_USERNAME", "admin"),
			InitAdminPassword: getEnv("INIT_ADMIN_PASSWORD", "Admin123456"),
			InitAdminRealName: getEnv("INIT_ADMIN_REAL_NAME", "系统管理员"),
		},
	}

	if cfg.JWT.Secret == "" {
		return Config{}, fmt.Errorf("JWT_SECRET is required")
	}

	return cfg, nil
}

func (c DBConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		c.Host,
		c.Port,
		c.User,
		c.Password,
		c.Name,
		c.SSLMode,
	)
}

func NewHTTPClient(timeout time.Duration) *http.Client {
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12},
	}
	return &http.Client{
		Transport: transport,
		Timeout:   timeout,
	}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return fallback
}

func getEnvInt(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}

	return parsed
}

func getEnvBool(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	switch value {
	case "1", "true", "TRUE", "True", "yes", "YES", "Yes", "on", "ON", "On":
		return true
	case "0", "false", "FALSE", "False", "no", "NO", "No", "off", "OFF", "Off":
		return false
	default:
		return fallback
	}
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}

	return parsed
}
