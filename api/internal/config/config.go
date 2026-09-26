package config

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Env       string
	LogFormat string
	Server    ServerConfig
	Extractor ExtractorConfig
	DB        DBConfig
	CORS      CORSConfig
	Proxy     ProxyConfig
	Limits    LimitsConfig
	GitHub    GitHubConfig
	Sentry    SentryConfig
}

type ServerConfig struct {
	Host string
	Port string
}

type ExtractorConfig struct {
	BaseURL string
	// APIKey is the shared secret sent as X-Internal-Key on every sidecar
	// call. The sidecar fronts paid-per-use (or quota-limited free-tier)
	// LLM and embedding providers - on a free hosting tier it usually ends
	// up publicly reachable, so without this anyone who found its URL could
	// spend the project's provider quota directly.
	APIKey string
}

type DBConfig struct {
	DSN string
}

type CORSConfig struct {
	AllowedOrigins []string
}

// ProxyConfig controls how the real client IP is derived for per-IP rate
// limiting. See api.ClientIP for why this is a hop count rather than
// "trust X-Forwarded-For".
type ProxyConfig struct {
	TrustedHops    int
	ClientIPHeader string
}

// LimitsConfig holds the free-tier guardrails. Every value is a ceiling on
// work that costs provider quota (embedding tokens, LLM tokens) or memory
// on a small (512MB) free instance - see Docs/Phase-6 for how each default
// was chosen.
type LimitsConfig struct {
	IngestionWorkers int
	MaxRepoFiles     int
	MaxFileBytes     int64
	// MaxEmbedChunks is a per-ingestion-run embedding budget, not a lifetime
	// cap: files left unembedded (by this cap or an exhausted daily quota)
	// are picked up by the next sync, so a large repo converges over runs.
	MaxEmbedChunks int
	MaxRepoSizeMB  int
	// EmbedBatchItems is the chunks per sidecar /embed/batch call. Keep it
	// equal to the sidecar's OPENROUTER_EMBED_BATCH_SIZE so each call is
	// exactly one provider request - free embedding models are limited per
	// request (50/day), so request count is the quantity to minimise.
	EmbedBatchItems     int
	MaxQuestionChars    int
	MaxChatSessions     int
	ChatSessionTTL      time.Duration
	MaxRequestBodyBytes int64
}

type GitHubConfig struct {
	// Token is optional - it only raises the GitHub REST API rate limit
	// used by the pre-clone repo-size check from 60 to 5000 requests/hour.
	Token string
}

type SentryConfig struct {
	DSN string
}

func (s ServerConfig) Addr() string {
	return fmt.Sprintf("%s:%s", s.Host, s.Port)
}

func (c *Config) IsProduction() bool {
	return c.Env == "production"
}

func Load() (*Config, error) {
	// A local .env is a convenience for `go run` - real deployments inject
	// environment variables directly, and a variable already set in the
	// environment always wins over the file.
	loadDotEnv(".env")

	cfg := &Config{
		Env:       getEnv("APP_ENV", "development"),
		LogFormat: getEnv("LOG_FORMAT", "text"),
		Server: ServerConfig{
			Host: getEnv("SERVER_HOST", "0.0.0.0"),
			// Most hosting platforms (Render, Koyeb, Railway, Fly) inject
			// PORT and expect the service to bind it.
			Port: getEnv("SERVER_PORT", getEnv("PORT", "8080")),
		},
		Extractor: ExtractorConfig{
			BaseURL: strings.TrimRight(getEnv("EXTRACTOR_URL", "http://localhost:8000"), "/"),
			APIKey:  getEnv("SIDECAR_API_KEY", ""),
		},
		DB: DBConfig{
			DSN: getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/knowyourrepo?sslmode=disable"),
		},
		CORS: CORSConfig{
			AllowedOrigins: splitAndTrim(getEnv("CORS_ALLOWED_ORIGINS", "http://localhost:5173")),
		},
		Proxy: ProxyConfig{
			TrustedHops:    getEnvInt("TRUSTED_PROXY_HOPS", 0),
			ClientIPHeader: getEnv("CLIENT_IP_HEADER", "X-Forwarded-For"),
		},
		Limits: LimitsConfig{
			// One concurrent ingestion by default: parsing a large repo is the
			// most memory-hungry thing this service does, and on the embedding
			// side every job shares one provider rate limit anyway, so a second
			// worker adds memory pressure without adding throughput.
			IngestionWorkers:    getEnvInt("INGESTION_WORKERS", 1),
			MaxRepoFiles:        getEnvInt("MAX_REPO_FILES", 2000),
			MaxFileBytes:        int64(getEnvInt("MAX_FILE_BYTES", 512*1024)),
			MaxEmbedChunks:      getEnvInt("MAX_EMBED_CHUNKS", 1024),
			MaxRepoSizeMB:       getEnvInt("MAX_REPO_SIZE_MB", 150),
			EmbedBatchItems:     getEnvInt("EMBED_BATCH_ITEMS", 128),
			MaxQuestionChars:    getEnvInt("MAX_QUESTION_CHARS", 2000),
			MaxChatSessions:     getEnvInt("MAX_CHAT_SESSIONS", 1000),
			ChatSessionTTL:      time.Duration(getEnvInt("CHAT_SESSION_TTL_MINUTES", 24*60)) * time.Minute,
			MaxRequestBodyBytes: int64(getEnvInt("MAX_REQUEST_BODY_BYTES", 64*1024)),
		},
		GitHub: GitHubConfig{
			Token: getEnv("GITHUB_TOKEN", ""),
		},
		Sentry: SentryConfig{
			DSN: getEnv("SENTRY_DSN", ""),
		},
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil {
		return fallback
	}
	return n
}

func splitAndTrim(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// loadDotEnv reads simple KEY=VALUE lines from path into the process
// environment, never overriding a variable that's already set. Deliberately
// minimal (no interpolation, no multi-line values) - it's a local-dev
// convenience, not a config system, and not worth a dependency.
func loadDotEnv(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		if key == "" {
			continue
		}
		if _, exists := os.LookupEnv(key); !exists {
			os.Setenv(key, value)
		}
	}
}
