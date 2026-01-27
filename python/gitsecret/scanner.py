import json
import os
import re
import subprocess
import threading
from dataclasses import dataclass, field
from datetime import datetime
from pathlib import Path
from typing import Dict, List, Optional, Set, Tuple, Callable
from concurrent.futures import ThreadPoolExecutor

from .config import Config


# Data Classes

@dataclass
class SecretValue:
    """Represents a single secret value found in repository."""
    value: str
    masked_value: str
    commits: List[str] = field(default_factory=list)
    authors: List[str] = field(default_factory=list)
    first_seen: str = ""
    last_seen: str = ""

    def to_dict(self):
        """Convert to JSON-serializable dict with camelCase keys."""
        return {
            "value": self.value,
            "maskedValue": self.masked_value,
            "commits": self.commits,
            "authors": list(set(self.authors)),
            "firstSeen": self.first_seen,
            "lastSeen": self.last_seen,
        }


@dataclass
class Secret:
    """Represents a secret found across repository."""
    file: str
    key: str
    type: str
    change_count: int = 0
    total_occurrences: int = 0
    authors: List[str] = field(default_factory=list)
    history: List[SecretValue] = field(default_factory=list)

    def to_dict(self):
        """Convert to JSON-serializable dict with camelCase keys."""
        return {
            "file": self.file,
            "key": self.key,
            "type": self.type,
            "changeCount": self.change_count,
            "totalOccurrences": self.total_occurrences,
            "authors": list(set(self.authors)),
            "history": [v.to_dict() for v in self.history],
        }


@dataclass
class ScanResult:
    """Result of a repository scan."""
    repository: str
    branch: str
    secrets_found: int
    total_values: int
    secrets: List[Secret] = field(default_factory=list)
    scan_date: str = ""

    def to_dict(self):
        """Convert to JSON-serializable dict with camelCase keys."""
        return {
            "repository": self.repository,
            "branch": self.branch,
            "secretsFound": self.secrets_found,
            "totalValues": self.total_values,
            "secrets": [s.to_dict() for s in self.secrets],
            "scanDate": self.scan_date,
        }


@dataclass
class StreamEntry:
    """Single entry in JSONL stream output."""
    file: str
    key: str
    value: str
    masked_value: str
    type: str
    commit: str
    author: str
    date: str

    def to_dict(self):
        """Convert to JSON-serializable dict with camelCase keys."""
        return {
            "file": self.file,
            "key": self.key,
            "value": self.value,
            "maskedValue": self.masked_value,
            "type": self.type,
            "commit": self.commit,
            "author": self.author,
            "date": self.date,
        }


@dataclass
class ScanOptions:
    """Options for scan operations."""
    branch: str = "--all"
    config_path: str = ""
    max_concurrent: int = 4
    on_progress: Optional[Callable[[str], None]] = None


# Helper Functions

def mask_secret(value: str) -> str:
    """Mask a secret value: show first 2 and last 2 chars, replace middle with asterisks."""
    if len(value) <= 4:
        return "****"

    first_two = value[:2]
    last_two = value[-2:]
    num_asterisks = min(len(value) - 4, 16)

    return first_two + ("*" * num_asterisks) + last_two


# Scanner Class

class Scanner:
    """Scanner performs Git history and current filesystem scanning for secrets."""

    MAX_FILE_SIZE = 1024 * 1024  # 1MB

    def __init__(self, cfg: Config):
        """Initialize scanner with configuration."""
        self.config = cfg
        self.compiled_patterns = cfg.get_compiled_patterns()

    def extract_key_value(self, line: str) -> Tuple[Optional[str], Optional[str], bool]:
        """Try all patterns to extract key-value pair from line.

        Returns:
            Tuple of (key, value, found) where found is True if extraction succeeded.
        """
        for compiled_pattern in self.compiled_patterns:
            match = compiled_pattern.regex.search(line)
            if match:
                groups = match.groups()
                if len(groups) >= compiled_pattern.value_group:
                    # Group indices are 1-based in the compiled_pattern.value_group
                    key = groups[0]
                    value_index = compiled_pattern.value_group - 1
                    if value_index < len(groups):
                        value = groups[value_index]

                        # Skip ignored values
                        if self.config.should_ignore_value(value):
                            continue

                        return key, value, True

        return None, None, False

    def scan(self, repo_path: str, opts: ScanOptions) -> ScanResult:
        """Full scan using git log -S per keyword with parallel processing.

        Args:
            repo_path: Path to git repository
            opts: Scan options including branch and concurrency

        Returns:
            ScanResult containing found secrets
        """
        repo_path = str(Path(repo_path).resolve())

        # Collect all keywords from config
        keywords = self.config.get_all_keywords()

        # Shared index for results
        index: Dict[str, Dict[str, SecretValue]] = {}
        lock = threading.Lock()

        # Parallel processing
        with ThreadPoolExecutor(max_workers=opts.max_concurrent) as executor:
            futures = []
            for i, keyword in enumerate(keywords):
                future = executor.submit(
                    self._search_keyword,
                    repo_path,
                    keyword,
                    opts.branch,
                    index,
                    lock
                )
                futures.append(future)

                if opts.on_progress:
                    opts.on_progress(f"Searching keyword {i+1}/{len(keywords)}: {keyword}")

            # Wait for all to complete
            for future in futures:
                try:
                    future.result()
                except Exception:
                    pass

        # Build result
        secrets = self._build_secrets(index)
        total_values = sum(len(v) for v in index.values())

        result = ScanResult(
            repository=repo_path,
            branch=opts.branch,
            secrets_found=len(secrets),
            total_values=total_values,
            secrets=secrets,
            scan_date=datetime.now().isoformat()
        )

        return result

    def _search_keyword(
        self,
        repo_path: str,
        keyword: str,
        branch: str,
        index: Dict,
        lock: threading.Lock
    ) -> None:
        """Run git log for keyword and extract secrets.

        This method is called in parallel for each keyword. It:
        1. Runs git log with pickaxe (-S) to find commits containing keyword
        2. Parses output to extract file paths, commits, and content
        3. Uses regex patterns to extract key-value pairs
        4. Stores results in thread-safe shared index
        """
        try:
            # Build git command
            git_cmd = [
                "git", "log", branch, f"-S{keyword}",
                "--pretty=format:COMMIT_START|%H|%an|%aI",
                "-p"
            ]

            # Add file exclusions based on ignored extensions
            for ext in self.config.exclude_binary_extensions:
                git_cmd.extend(["--", f":!*{ext}"])

            # Run git command
            process = subprocess.Popen(
                git_cmd,
                cwd=repo_path,
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE,
                text=True
            )

            current_file = None
            current_commit = None
            current_author = None
            current_date = None

            for line in process.stdout:
                line = line.rstrip('\n')

                # Parse commit header
                if line.startswith("COMMIT_START|"):
                    parts = line.split("|", 3)  # Split on first 3 pipes only
                    if len(parts) >= 4:
                        current_commit = parts[1]
                        current_author = parts[2]
                        current_date = parts[3]

                # Parse file path
                elif line.startswith("diff --git"):
                    match = re.search(r' b/(.+)$', line)
                    if match:
                        current_file = match.group(1)

                # Parse content lines
                elif line.startswith("+") and not line.startswith("+++"):
                    if current_file and keyword in line:
                        content = line[1:]  # Remove leading +

                        # Extract key-value pair
                        key, value, found = self.extract_key_value(content)
                        if found and key and value:
                            # Check ignored files
                            if self.config.should_ignore_file(current_file):
                                continue

                            masked = mask_secret(value)

                            with lock:
                                if current_file not in index:
                                    index[current_file] = {}

                                file_index = index[current_file]

                                if key not in file_index:
                                    file_index[key] = SecretValue(
                                        value=value,
                                        masked_value=masked,
                                        commits=[],
                                        authors=[],
                                        first_seen="",
                                        last_seen=""
                                    )

                                secret_value = file_index[key]
                                if current_commit not in secret_value.commits:
                                    secret_value.commits.append(current_commit)
                                if current_author not in secret_value.authors:
                                    secret_value.authors.append(current_author)

                                if not secret_value.first_seen or current_date < secret_value.first_seen:
                                    secret_value.first_seen = current_date
                                if not secret_value.last_seen or current_date > secret_value.last_seen:
                                    secret_value.last_seen = current_date

            process.wait()

        except Exception:
            pass

    def _build_secrets(self, index: Dict[str, Dict[str, SecretValue]]) -> List[Secret]:
        """Convert index dict to sorted Secret list."""
        secrets = []

        for file_path, key_values in index.items():
            for key, secret_value in key_values.items():
                # Determine type from config
                secret_type = "unknown"
                for keyword_group in self.config.keywords:
                    if key in keyword_group.patterns:
                        secret_type = keyword_group.name
                        break

                secret = Secret(
                    file=file_path,
                    key=key,
                    type=secret_type,
                    change_count=len(secret_value.commits),
                    total_occurrences=len(secret_value.commits),
                    authors=secret_value.authors,
                    history=[secret_value]
                )
                secrets.append(secret)

        # Sort by file, then key
        secrets.sort(key=lambda s: (s.file, s.key))

        return secrets

    def scan_stream(
        self,
        repo_path: str,
        output_path: str,
        opts: ScanOptions
    ) -> None:
        """Sequential JSONL streaming per keyword.

        Args:
            repo_path: Path to git repository
            output_path: Output JSONL file path
            opts: Scan options
        """
        repo_path = str(Path(repo_path).resolve())

        # Collect all keywords
        keywords = self.config.get_all_keywords()
        seen: Set[str] = set()

        with open(output_path, 'w') as f:
            for i, keyword in enumerate(keywords):
                self._stream_keyword(repo_path, keyword, opts.branch, f, seen)

                if opts.on_progress:
                    opts.on_progress(f"Streaming keyword {i+1}/{len(keywords)}: {keyword}")

    def _stream_keyword(
        self,
        repo_path: str,
        keyword: str,
        branch: str,
        file,
        seen: Set[str]
    ) -> None:
        """Stream keyword results to JSONL file.

        Args:
            repo_path: Path to git repository
            keyword: Keyword to search for
            branch: Git branch to search
            file: Open file handle to write JSONL entries
            seen: Set to track already-written entries for deduplication
        """
        try:
            git_cmd = [
                "git", "log", branch, f"-S{keyword}",
                "--pretty=format:COMMIT_START|%H|%an|%aI",
                "-p"
            ]

            for ext in self.config.exclude_binary_extensions:
                git_cmd.extend(["--", f":!*{ext}"])

            process = subprocess.Popen(
                git_cmd,
                cwd=repo_path,
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE,
                text=True
            )

            current_file = None
            current_commit = None
            current_author = None
            current_date = None

            for line in process.stdout:
                line = line.rstrip('\n')

                if line.startswith("COMMIT_START|"):
                    parts = line.split("|", 3)
                    if len(parts) >= 4:
                        current_commit = parts[1]
                        current_author = parts[2]
                        current_date = parts[3]

                elif line.startswith("diff --git"):
                    match = re.search(r' b/(.+)$', line)
                    if match:
                        current_file = match.group(1)

                elif line.startswith("+") and not line.startswith("+++"):
                    if current_file and keyword in line:
                        content = line[1:]

                        key, value, found = self.extract_key_value(content)
                        if found and key and value:
                            if self.config.should_ignore_file(current_file):
                                continue

                            # Dedup by file|key|value
                            dedup_key = f"{current_file}|{key}|{value}"
                            if dedup_key in seen:
                                continue
                            seen.add(dedup_key)

                            # Determine type
                            secret_type = "unknown"
                            for keyword_group in self.config.keywords:
                                if key in keyword_group.patterns:
                                    secret_type = keyword_group.name
                                    break

                            masked = mask_secret(value)

                            entry = StreamEntry(
                                file=current_file,
                                key=key,
                                value=value,
                                masked_value=masked,
                                type=secret_type,
                                commit=current_commit,
                                author=current_author,
                                date=current_date
                            )

                            file.write(json.dumps(entry.to_dict()) + "\n")

            process.wait()

        except Exception:
            pass

    def scan_current(self, repo_path: str) -> ScanResult:
        """Walk filesystem and search files for keywords.

        Args:
            repo_path: Path to scan

        Returns:
            ScanResult containing secrets found in current working tree
        """
        repo_path = str(Path(repo_path).resolve())

        index: Dict[str, Dict[str, SecretValue]] = {}

        # Collect keywords
        keywords = self.config.get_all_keywords()

        # Walk filesystem
        for root, dirs, files in os.walk(repo_path):
            # Skip .git directory
            if ".git" in dirs:
                dirs.remove(".git")

            for filename in files:
                file_path = os.path.join(root, filename)
                rel_path = os.path.relpath(file_path, repo_path)

                # Skip ignored files
                if self.config.should_ignore_file(rel_path):
                    continue

                # Skip binary files
                if any(filename.endswith(ext) for ext in self.config.exclude_binary_extensions):
                    continue

                # Skip large files
                try:
                    if os.path.getsize(file_path) > self.MAX_FILE_SIZE:
                        continue
                except OSError:
                    continue

                # Read and search file
                try:
                    with open(file_path, 'r', encoding='utf-8', errors='ignore') as f:
                        for line_num, line in enumerate(f, 1):
                            for keyword in keywords:
                                if keyword in line:
                                    key, value, found = self.extract_key_value(line)
                                    if found and key and value:
                                        if rel_path not in index:
                                            index[rel_path] = {}

                                        if key not in index[rel_path]:
                                            now_iso = datetime.now().isoformat()
                                            index[rel_path][key] = SecretValue(
                                                value=value,
                                                masked_value=mask_secret(value),
                                                commits=[],
                                                authors=[],
                                                first_seen=now_iso,
                                                last_seen=now_iso
                                            )
                except Exception:
                    continue

        secrets = self._build_secrets(index)
        total_values = sum(len(v) for v in index.values())

        return ScanResult(
            repository=repo_path,
            branch="HEAD",
            secrets_found=len(secrets),
            total_values=total_values,
            secrets=secrets,
            scan_date=datetime.now().isoformat()
        )

    def scan_current_stream(self, repo_path: str, output_path: str) -> None:
        """Stream current files to JSONL.

        Args:
            repo_path: Path to scan
            output_path: Output JSONL file path
        """
        repo_path = str(Path(repo_path).resolve())

        # Collect keywords
        keywords = self.config.get_all_keywords()

        seen: Set[str] = set()

        with open(output_path, 'w') as f:
            for root, dirs, files in os.walk(repo_path):
                if ".git" in dirs:
                    dirs.remove(".git")

                for filename in files:
                    file_path = os.path.join(root, filename)
                    rel_path = os.path.relpath(file_path, repo_path)

                    if self.config.should_ignore_file(rel_path):
                        continue

                    if any(filename.endswith(ext) for ext in self.config.exclude_binary_extensions):
                        continue

                    try:
                        if os.path.getsize(file_path) > self.MAX_FILE_SIZE:
                            continue
                    except OSError:
                        continue

                    try:
                        with open(file_path, 'r', encoding='utf-8', errors='ignore') as file:
                            for line in file:
                                for keyword in keywords:
                                    if keyword in line:
                                        key, value, found = self.extract_key_value(line)
                                        if found and key and value:
                                            dedup_key = f"{rel_path}|{key}|{value}"
                                            if dedup_key in seen:
                                                continue
                                            seen.add(dedup_key)

                                            secret_type = "unknown"
                                            for keyword_group in self.config.keywords:
                                                if key in keyword_group.patterns:
                                                    secret_type = keyword_group.name
                                                    break

                                            entry = StreamEntry(
                                                file=rel_path,
                                                key=key,
                                                value=value,
                                                masked_value=mask_secret(value),
                                                type=secret_type,
                                                commit="HEAD",
                                                author="",
                                                date=datetime.now().isoformat()
                                            )

                                            f.write(json.dumps(entry.to_dict()) + "\n")
                    except Exception:
                        continue

    def scan_both(self, repo_path: str, opts: ScanOptions) -> ScanResult:
        """Combine current + history results.

        Args:
            repo_path: Path to git repository
            opts: Scan options

        Returns:
            ScanResult combining both history and current findings
        """
        # Scan history
        result = self.scan(repo_path, opts)

        # Scan current files
        current_result = self.scan_current(repo_path)

        # Merge results
        all_secrets = {}

        for secret in result.secrets:
            key = f"{secret.file}|{secret.key}"
            all_secrets[key] = secret

        for secret in current_result.secrets:
            key = f"{secret.file}|{secret.key}"
            if key not in all_secrets:
                all_secrets[key] = secret

        merged_secrets = list(all_secrets.values())
        merged_secrets.sort(key=lambda s: (s.file, s.key))

        return ScanResult(
            repository=repo_path,
            branch=opts.branch,
            secrets_found=len(merged_secrets),
            total_values=result.total_values + current_result.total_values,
            secrets=merged_secrets,
            scan_date=datetime.now().isoformat()
        )

    def scan_both_stream(self, repo_path: str, output_path: str, opts: ScanOptions) -> None:
        """Stream both history and current to JSONL.

        Args:
            repo_path: Path to git repository
            output_path: Output JSONL file path
            opts: Scan options
        """
        seen: Set[str] = set()

        with open(output_path, 'w') as f:
            # First stream history
            temp_file = output_path + ".tmp"
            self.scan_stream(repo_path, temp_file, opts)

            with open(temp_file, 'r') as tmp:
                for line in tmp:
                    entry = json.loads(line)
                    dedup = f"{entry['file']}|{entry['key']}|{entry['value']}"
                    if dedup not in seen:
                        seen.add(dedup)
                        f.write(line)

            try:
                os.remove(temp_file)
            except Exception:
                pass

            # Then stream current
            self.scan_current_stream(repo_path, temp_file)

            with open(temp_file, 'r') as tmp:
                for line in tmp:
                    entry = json.loads(line)
                    dedup = f"{entry['file']}|{entry['key']}|{entry['value']}"
                    if dedup not in seen:
                        seen.add(dedup)
                        f.write(line)

            try:
                os.remove(temp_file)
            except Exception:
                pass

    @staticmethod
    def get_all_values(scan_result: ScanResult) -> List[str]:
        """Extract unique values sorted by length descending.

        Args:
            scan_result: ScanResult to extract values from

        Returns:
            List of unique secret values sorted by length (descending)
        """
        values = set()
        for secret in scan_result.secrets:
            for secret_value in secret.history:
                values.add(secret_value.value)

        return sorted(list(values), key=lambda x: len(x), reverse=True)
