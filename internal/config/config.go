package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Config holds the scanning configuration
type Config struct {
	ExtractionPatterns      []ExtractionPattern `json:"extractionPatterns"`
	Keywords                []KeywordGroup      `json:"keywords"`
	IgnoredValues           []string            `json:"ignoredValues"`
	IgnoredFiles            []string            `json:"ignoredFiles"`
	ExcludeBinaryExtensions []string            `json:"excludeBinaryExtensions"`
	Settings                Settings            `json:"settings"`
}

// KeywordGroup represents a group of search patterns
type KeywordGroup struct {
	Name        string   `json:"name"`
	Patterns    []string `json:"patterns"`
	Description string   `json:"description"`
}

// Settings holds scanner settings
type Settings struct {
	MinSecretLength int  `json:"minSecretLength"`
	MaxSecretLength int  `json:"maxSecretLength"`
	CaseSensitive   bool `json:"caseSensitive"`
}

// ExtractionPattern defines a regex pattern for extracting key-value pairs
type ExtractionPattern struct {
	Name        string `json:"name"`
	Pattern     string `json:"pattern"`
	ValueGroup  int    `json:"valueGroup"`
	Description string `json:"description"`
}

// CompiledPattern holds a compiled regex with metadata
type CompiledPattern struct {
	Name       string
	Regex      *regexp.Regexp
	ValueGroup int
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		ExtractionPatterns: []ExtractionPattern{
			{
				Name:        "key_equals_value",
				Pattern:     `^\s*([a-zA-Z_][\w.$/-]*)\s*=\s*(.+)$`,
				ValueGroup:  2,
				Description: "Standard key=value format",
			},
			{
				Name:        "yaml_colon",
				Pattern:     `^\s*([a-zA-Z_][\w._-]*)\s*:\s+['"]?([^'"\n=]+)['"]?\s*$`,
				ValueGroup:  2,
				Description: "YAML key: value format",
			},
			{
				Name:        "json_quoted",
				Pattern:     `"([a-zA-Z_][\w._]*)"\s*:\s*"([^"]+)"`,
				ValueGroup:  2,
				Description: "JSON \"key\": \"value\" format",
			},
			{
				Name:        "export_env",
				Pattern:     `^\s*export\s+([A-Z_][A-Z0-9_]*)\s*=\s*['"]?([^'"\n]+)['"]?`,
				ValueGroup:  2,
				Description: "Shell export KEY=value format",
			},
		},
		Keywords: []KeywordGroup{
			{
				Name:        "password",
				Patterns:    []string{"password", "passwd", "pwd", "pass", "mot_de_passe"},
				Description: "Mots de passe",
			},
			{
				Name:        "secret",
				Patterns:    []string{"secret", "client_secret", "app_secret", "api_secret"},
				Description: "Secrets applicatifs",
			},
			{
				Name:        "api_key",
				Patterns:    []string{"api_key", "apikey", "api-key"},
				Description: "Clés API",
			},
			{
				Name:        "token",
				Patterns:    []string{"token", "access_token", "auth_token", "bearer"},
				Description: "Tokens d'authentification",
			},
			{
				Name:        "credentials",
				Patterns:    []string{"credential", "credentials", "auth"},
				Description: "Identifiants",
			},
			{
				Name:        "private_key",
				Patterns:    []string{"private_key", "privatekey", "private-key", "rsa_private"},
				Description: "Clés privées",
			},
			{
				Name:        "connection_string",
				Patterns:    []string{"connection_string", "connectionstring", "conn_str", "database_url", "db_url"},
				Description: "Chaînes de connexion",
			},
			{
				Name:        "oauth",
				Patterns:    []string{"oauth", "client_id", "client_secret", "refresh_token"},
				Description: "OAuth",
			},
			{
				Name:        "aws",
				Patterns:    []string{"aws_access_key", "aws_secret", "aws_key"},
				Description: "AWS credentials",
			},
			{
				Name:        "encryption",
				Patterns:    []string{"encryption_key", "encrypt_key", "aes_key", "cipher"},
				Description: "Clés de chiffrement",
			},
		},
		IgnoredValues: []string{
			// Empty/null values
			"<empty>",
			"<none>",
			"<null>",
			"null",
			"nil",
			"undefined",
			"none",
			"N/A",
			// Template placeholders (prefix match via contains)
			"${",
			"{{",
			"%s",
			"<value>",
			"<your_",
			"[your_",
			// Common placeholders (prefix match via contains)
			"PLACEHOLDER",
			"your_",
			"YOUR_",
			"example",
			"EXAMPLE",
			"sample",
			"xxx",
			"XXX",
			"***",
			"----",
			"____",
			// Removed/changed markers
			"REMOVED",
			"REDACTED",
			"HIDDEN",
			"MASKED",
			"changeme",
			"CHANGEME",
			"change_me",
			"TODO",
			"FIXME",
			// Default values
			"default",
			"DEFAULT",
			// Note: "password", "secret", "token", etc. are NOT here
			// because they are checked via exact match in ShouldIgnoreValue
			// We don't want to ignore "demopassword" just because it contains "password"
		},
		IgnoredFiles: []string{
			// Documentation
			"*.md",
			"*.txt",
			"*.rst",
			// Lock files
			"*.lock",
			// Source code files (to avoid false positives from variable names)
			"*.go",
			"*.js",
			"*.ts",
			"*.jsx",
			"*.tsx",
			"*.py",
			"*.java",
			"*.rb",
			"*.php",
			"*.c",
			"*.cpp",
			"*.h",
			"*.cs",
			"*.swift",
			"*.kt",
			"*.rs",
			"*.scala",
			// Config/output files of this tool
			"*.json",
			"*.jsonl",
			// Directories
			"node_modules/**",
			"vendor/**",
			".git/**",
			// Minified files
			"*.min.js",
			"*.min.css",
		},
		ExcludeBinaryExtensions: []string{
			".jar", ".war", ".zip", ".tar", ".gz", ".rar",
			".png", ".jpg", ".jpeg", ".gif", ".ico", ".svg",
			".pdf", ".doc", ".docx", ".xls", ".xlsx",
			".exe", ".dll", ".so", ".dylib",
			".class", ".pyc", ".o", ".a",
		},
		Settings: Settings{
			MinSecretLength: 3,
			MaxSecretLength: 500,
			CaseSensitive:   false,
		},
	}
}

// Load loads configuration from file or returns default
// If path is empty, returns built-in defaults (no auto-detection)
func Load(path string) (*Config, error) {
	if path == "" {
		return DefaultConfig(), nil
	}
	return loadFromFile(path)
}

// LoadAuto tries to find a config file in common locations, or returns default
func LoadAuto() (*Config, error) {
	locations := []string{
		"patterns.json",
		"config/patterns.json",
		filepath.Join(os.Getenv("HOME"), ".config", "git-secret-scanner", "patterns.json"),
	}

	for _, loc := range locations {
		if _, err := os.Stat(loc); err == nil {
			return loadFromFile(loc)
		}
	}

	return DefaultConfig(), nil
}

func loadFromFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	config := DefaultConfig()
	if err := json.Unmarshal(data, config); err != nil {
		return nil, err
	}

	return config, nil
}

// GetAllKeywords returns all search keywords from config
func (c *Config) GetAllKeywords() []string {
	var keywords []string
	for _, group := range c.Keywords {
		keywords = append(keywords, group.Patterns...)
	}
	return keywords
}

// ShouldIgnoreFile checks if a file should be ignored based on patterns
func (c *Config) ShouldIgnoreFile(filePath string) bool {
	for _, pattern := range c.IgnoredFiles {
		if matchPattern(pattern, filePath) {
			return true
		}
	}
	return false
}

// matchPattern checks if a file path matches a glob-like pattern
func matchPattern(pattern, filePath string) bool {
	// Handle ** (match any path)
	if strings.Contains(pattern, "**") {
		// e.g., "node_modules/**" matches "node_modules/foo/bar.js"
		prefix := strings.Split(pattern, "**")[0]
		if strings.HasPrefix(filePath, prefix) {
			return true
		}
		return false
	}

	// Handle * (match extension or filename)
	if strings.HasPrefix(pattern, "*.") {
		// e.g., "*.md" matches "README.md" and "docs/file.md"
		ext := pattern[1:] // ".md"
		return strings.HasSuffix(filePath, ext)
	}

	// Handle exact directory match
	if strings.HasSuffix(pattern, "/") {
		return strings.HasPrefix(filePath, pattern)
	}

	// Exact match
	return filePath == pattern
}

// ShouldIgnoreValue checks if a value should be ignored
func (c *Config) ShouldIgnoreValue(value string) bool {
	if len(value) < c.Settings.MinSecretLength || len(value) > c.Settings.MaxSecretLength {
		return true
	}

	// Ignore values that look like code (function calls, array access, etc.)
	if looksLikeCode(value) {
		return true
	}

	// Ignore URLs (values starting with protocol)
	valueLower := toLower(value)
	urlPrefixes := []string{"http://", "https://", "ftp://", "ssh://", "file://", "mailto:"}
	for _, prefix := range urlPrefixes {
		if strings.HasPrefix(valueLower, prefix) {
			return true
		}
	}

	// Ignore if value equals a common keyword (exact match, case insensitive)
	commonKeywords := []string{"password", "secret", "token", "key", "credential", "auth", "pass", "pwd"}
	for _, kw := range commonKeywords {
		if valueLower == kw {
			return true
		}
	}

	for _, ignored := range c.IgnoredValues {
		ignoredLower := ignored
		if !c.Settings.CaseSensitive {
			ignoredLower = toLower(ignored)
		}
		if contains(valueLower, ignoredLower) {
			return true
		}
	}

	return false
}

// looksLikeCode checks if a value appears to be code rather than a secret
func looksLikeCode(value string) bool {
	// Function calls: append(...), make(...), etc.
	if strings.Contains(value, "(") && strings.Contains(value, ")") {
		return true
	}
	// Array/slice access: foo[...]
	if strings.Contains(value, "[") && strings.Contains(value, "]") {
		return true
	}
	// Object/struct literals: {...}
	if strings.HasPrefix(value, "{") || strings.HasSuffix(value, "}") {
		return true
	}
	// Method chains or field access with multiple dots: foo.bar.baz
	if strings.Count(value, ".") > 2 {
		return true
	}
	// Struct field access pattern: identifier.Identifier (e.g., entry.Date, config.Value)
	// This detects Go-style field access where second part starts with uppercase
	if strings.Count(value, ".") == 1 {
		parts := strings.Split(value, ".")
		if len(parts) == 2 && len(parts[0]) > 0 && len(parts[1]) > 0 {
			// Check if it looks like struct.Field (camelCase.PascalCase)
			firstChar := parts[1][0]
			if firstChar >= 'A' && firstChar <= 'Z' {
				// Also check first part is a simple identifier (no special chars except underscore)
				isSimpleIdent := true
				for _, c := range parts[0] {
					if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_') {
						isSimpleIdent = false
						break
					}
				}
				if isSimpleIdent {
					return true
				}
			}
		}
	}
	// Go keywords at start
	codeKeywords := []string{"func ", "return ", "if ", "for ", "range ", "make(", "append(", "new(", "len("}
	for _, kw := range codeKeywords {
		if strings.HasPrefix(value, kw) {
			return true
		}
	}
	return false
}

func toLower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 32
		}
		result[i] = c
	}
	return string(result)
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Save saves configuration to file
func (c *Config) Save(path string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// GetCompiledPatterns compiles all extraction patterns and returns them
func (c *Config) GetCompiledPatterns() []*CompiledPattern {
	patterns := make([]*CompiledPattern, 0, len(c.ExtractionPatterns))

	for _, ep := range c.ExtractionPatterns {
		regex, err := regexp.Compile(ep.Pattern)
		if err != nil {
			// Skip invalid patterns
			continue
		}
		patterns = append(patterns, &CompiledPattern{
			Name:       ep.Name,
			Regex:      regex,
			ValueGroup: ep.ValueGroup,
		})
	}

	return patterns
}
