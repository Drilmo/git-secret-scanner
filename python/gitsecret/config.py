import json
import os
import re
from dataclasses import dataclass, asdict, field
from pathlib import Path
from typing import List, Optional


@dataclass
class Settings:
    """Settings holds scanner settings."""
    min_secret_length: int = 3
    max_secret_length: int = 500
    case_sensitive: bool = False


@dataclass
class ExtractionPattern:
    """ExtractionPattern defines a regex pattern for extracting key-value pairs."""
    name: str
    pattern: str
    value_group: int
    description: str


@dataclass
class KeywordGroup:
    """KeywordGroup represents a group of search patterns."""
    name: str
    patterns: List[str]
    description: str


@dataclass
class CompiledPattern:
    """CompiledPattern holds a compiled regex with metadata."""
    name: str
    regex: re.Pattern
    value_group: int


@dataclass
class Config:
    """Config holds the scanning configuration."""
    extraction_patterns: List[ExtractionPattern]
    keywords: List[KeywordGroup]
    ignored_values: List[str]
    ignored_files: List[str]
    exclude_binary_extensions: List[str]
    settings: Settings

    def get_all_keywords(self) -> List[str]:
        """Returns all search keywords from config."""
        keywords = []
        for group in self.keywords:
            keywords.extend(group.patterns)
        return keywords

    def should_ignore_file(self, file_path: str) -> bool:
        """Checks if a file should be ignored based on patterns."""
        for pattern in self.ignored_files:
            if _match_pattern(pattern, file_path):
                return True
        return False

    def should_ignore_value(self, value: str) -> bool:
        """Checks if a value should be ignored."""
        if len(value) < self.settings.min_secret_length or len(value) > self.settings.max_secret_length:
            return True

        # Ignore values that look like code (function calls, array access, etc.)
        if _looks_like_code(value):
            return True

        # Ignore URLs (values starting with protocol)
        value_lower = value.lower()
        url_prefixes = ["http://", "https://", "ftp://", "ssh://", "file://", "mailto:"]
        for prefix in url_prefixes:
            if value_lower.startswith(prefix):
                return True

        # Ignore if value equals a common keyword (exact match, case insensitive)
        common_keywords = ["password", "secret", "token", "key", "credential", "auth", "pass", "pwd"]
        for kw in common_keywords:
            if value_lower == kw:
                return True

        for ignored in self.ignored_values:
            ignored_lower = ignored
            if not self.settings.case_sensitive:
                ignored_lower = ignored.lower()
            if ignored_lower in value_lower:
                return True

        return False

    def get_compiled_patterns(self) -> List[CompiledPattern]:
        """Compiles all extraction patterns and returns them."""
        patterns = []
        for ep in self.extraction_patterns:
            try:
                regex = re.compile(ep.pattern)
                patterns.append(CompiledPattern(
                    name=ep.name,
                    regex=regex,
                    value_group=ep.value_group,
                ))
            except re.error:
                # Skip invalid patterns
                continue
        return patterns

    def save(self, path: str) -> None:
        """Saves configuration to file."""
        data = _dataclass_to_dict(self)
        with open(path, 'w') as f:
            json.dump(data, f, indent=2)


def _dataclass_to_dict(obj):
    """Recursively convert dataclass to dict with camelCase keys."""
    if not hasattr(obj, '__dataclass_fields__'):
        return obj

    result = {}
    for key, value in asdict(obj).items():
        camel_key = _snake_to_camel(key)
        if isinstance(value, list):
            result[camel_key] = [_dataclass_to_dict(item) for item in value]
        elif hasattr(value, '__dataclass_fields__'):
            result[camel_key] = _dataclass_to_dict(value)
        else:
            result[camel_key] = value
    return result


def _snake_to_camel(snake_str: str) -> str:
    """Convert snake_case to camelCase."""
    components = snake_str.split('_')
    return components[0] + ''.join(x.title() for x in components[1:])


def _camel_to_snake(camel_str: str) -> str:
    """Convert camelCase to snake_case."""
    result = []
    for i, char in enumerate(camel_str):
        if char.isupper() and i > 0:
            result.append('_')
            result.append(char.lower())
        else:
            result.append(char)
    return ''.join(result)


def _match_pattern(pattern: str, file_path: str) -> bool:
    """Checks if a file path matches a glob-like pattern."""
    # Handle ** (match any path)
    if "**" in pattern:
        # e.g., "node_modules/**" matches "node_modules/foo/bar.js"
        prefix = pattern.split("**")[0]
        if file_path.startswith(prefix):
            return True
        return False

    # Handle * (match extension or filename)
    if pattern.startswith("*."):
        # e.g., "*.md" matches "README.md" and "docs/file.md"
        ext = pattern[1:]  # ".md"
        return file_path.endswith(ext)

    # Handle exact directory match
    if pattern.endswith("/"):
        return file_path.startswith(pattern)

    # Exact match
    return file_path == pattern


def _looks_like_code(value: str) -> bool:
    """Checks if a value appears to be code rather than a secret."""
    # Function calls: append(...), make(...), etc.
    if "(" in value and ")" in value:
        return True

    # Array/slice access: foo[...]
    if "[" in value and "]" in value:
        return True

    # Object/struct literals: {...}
    if value.startswith("{") or value.endswith("}"):
        return True

    # Method chains or field access with multiple dots: foo.bar.baz
    if value.count(".") > 2:
        return True

    # Struct field access pattern: identifier.Identifier (e.g., entry.Date, config.Value)
    # This detects Go-style field access where second part starts with uppercase
    if value.count(".") == 1:
        parts = value.split(".")
        if len(parts) == 2 and len(parts[0]) > 0 and len(parts[1]) > 0:
            # Check if it looks like struct.Field (camelCase.PascalCase)
            first_char = parts[1][0]
            if 'A' <= first_char <= 'Z':
                # Also check first part is a simple identifier (no special chars except underscore)
                is_simple_ident = True
                for c in parts[0]:
                    if not (('a' <= c <= 'z') or ('A' <= c <= 'Z') or ('0' <= c <= '9') or c == '_'):
                        is_simple_ident = False
                        break
                if is_simple_ident:
                    return True

    # Go keywords at start
    code_keywords = ["func ", "return ", "if ", "for ", "range ", "make(", "append(", "new(", "len("]
    for kw in code_keywords:
        if value.startswith(kw):
            return True

    return False


def default_config() -> Config:
    """Returns the default configuration."""
    return Config(
        extraction_patterns=[
            ExtractionPattern(
                name="key_equals_value",
                pattern=r"^\s*([a-zA-Z_][\w.$/-]*)\s*=\s*(.+)$",
                value_group=2,
                description="Standard key=value format",
            ),
            ExtractionPattern(
                name="yaml_colon",
                pattern=r"^\s*([a-zA-Z_][\w._-]*)\s*:\s+['\"]?([^'\"\n=]+)['\"]?\s*$",
                value_group=2,
                description="YAML key: value format",
            ),
            ExtractionPattern(
                name="json_quoted",
                pattern=r'"([a-zA-Z_][\w._]*)"\s*:\s*"([^"]+)"',
                value_group=2,
                description='JSON "key": "value" format',
            ),
            ExtractionPattern(
                name="export_env",
                pattern=r"^\s*export\s+([A-Z_][A-Z0-9_]*)\s*=\s*['\"]?([^'\"\n]+)['\"]?",
                value_group=2,
                description="Shell export KEY=value format",
            ),
        ],
        keywords=[
            KeywordGroup(
                name="password",
                patterns=["password", "passwd", "pwd", "pass", "mot_de_passe"],
                description="Mots de passe",
            ),
            KeywordGroup(
                name="secret",
                patterns=["secret", "client_secret", "app_secret", "api_secret"],
                description="Secrets applicatifs",
            ),
            KeywordGroup(
                name="api_key",
                patterns=["api_key", "apikey", "api-key"],
                description="Clés API",
            ),
            KeywordGroup(
                name="token",
                patterns=["token", "access_token", "auth_token", "bearer"],
                description="Tokens d'authentification",
            ),
            KeywordGroup(
                name="credentials",
                patterns=["credential", "credentials", "auth"],
                description="Identifiants",
            ),
            KeywordGroup(
                name="private_key",
                patterns=["private_key", "privatekey", "private-key", "rsa_private"],
                description="Clés privées",
            ),
            KeywordGroup(
                name="connection_string",
                patterns=["connection_string", "connectionstring", "conn_str", "database_url", "db_url"],
                description="Chaînes de connexion",
            ),
            KeywordGroup(
                name="oauth",
                patterns=["oauth", "client_id", "client_secret", "refresh_token"],
                description="OAuth",
            ),
            KeywordGroup(
                name="aws",
                patterns=["aws_access_key", "aws_secret", "aws_key"],
                description="AWS credentials",
            ),
            KeywordGroup(
                name="encryption",
                patterns=["encryption_key", "encrypt_key", "aes_key", "cipher"],
                description="Clés de chiffrement",
            ),
        ],
        ignored_values=[
            # Empty/null values
            "<empty>",
            "<none>",
            "<null>",
            "null",
            "nil",
            "undefined",
            "none",
            "N/A",
            # Template placeholders (prefix match via contains)
            "${",
            "{{",
            "%s",
            "<value>",
            "<your_",
            "[your_",
            # Common placeholders (prefix match via contains)
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
            # Removed/changed markers
            "REMOVED",
            "REDACTED",
            "HIDDEN",
            "MASKED",
            "changeme",
            "CHANGEME",
            "change_me",
            "TODO",
            "FIXME",
            # Default values
            "default",
            "DEFAULT",
        ],
        ignored_files=[
            # Documentation
            "*.md",
            "*.txt",
            "*.rst",
            # Lock files
            "*.lock",
            # Source code files (to avoid false positives from variable names)
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
            # Config/output files of this tool
            "*.json",
            "*.jsonl",
            # Directories
            "node_modules/**",
            "vendor/**",
            ".git/**",
            # Minified files
            "*.min.js",
            "*.min.css",
        ],
        exclude_binary_extensions=[
            ".jar", ".war", ".zip", ".tar", ".gz", ".rar",
            ".png", ".jpg", ".jpeg", ".gif", ".ico", ".svg",
            ".pdf", ".doc", ".docx", ".xls", ".xlsx",
            ".exe", ".dll", ".so", ".dylib",
            ".class", ".pyc", ".o", ".a",
        ],
        settings=Settings(
            min_secret_length=3,
            max_secret_length=500,
            case_sensitive=False,
        ),
    )


def load(path: str = "") -> Config:
    """Loads configuration from file or returns default.
    If path is empty, returns built-in defaults (no auto-detection).
    """
    if not path:
        return default_config()
    return _load_from_file(path)


def load_auto() -> Config:
    """Tries to find a config file in common locations, or returns default."""
    home = os.getenv("HOME", "")
    locations = [
        "patterns.json",
        "config/patterns.json",
        os.path.join(home, ".config", "git-secret-scanner", "patterns.json"),
    ]

    for loc in locations:
        if os.path.exists(loc):
            try:
                return _load_from_file(loc)
            except Exception:
                # Continue to next location if this one fails
                continue

    return default_config()


def _load_from_file(path: str) -> Config:
    """Loads configuration from a JSON file."""
    with open(path, 'r') as f:
        data = json.load(f)

    # Start with defaults
    config = default_config()

    # Parse extraction patterns
    if "extractionPatterns" in data:
        config.extraction_patterns = [
            ExtractionPattern(
                name=p["name"],
                pattern=p["pattern"],
                value_group=p.get("valueGroup", p.get("value_group", 2)),
                description=p.get("description", ""),
            )
            for p in data["extractionPatterns"]
        ]

    # Parse keywords
    if "keywords" in data:
        config.keywords = [
            KeywordGroup(
                name=k["name"],
                patterns=k["patterns"],
                description=k.get("description", ""),
            )
            for k in data["keywords"]
        ]

    # Parse ignored values
    if "ignoredValues" in data:
        config.ignored_values = data["ignoredValues"]

    # Parse ignored files
    if "ignoredFiles" in data:
        config.ignored_files = data["ignoredFiles"]

    # Parse binary extensions
    if "excludeBinaryExtensions" in data:
        config.exclude_binary_extensions = data["excludeBinaryExtensions"]

    # Parse settings
    if "settings" in data:
        s = data["settings"]
        config.settings = Settings(
            min_secret_length=s.get("minSecretLength", s.get("min_secret_length", 3)),
            max_secret_length=s.get("maxSecretLength", s.get("max_secret_length", 500)),
            case_sensitive=s.get("caseSensitive", s.get("case_sensitive", False)),
        )

    return config
