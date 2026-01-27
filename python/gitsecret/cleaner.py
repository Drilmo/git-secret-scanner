import dataclasses
import json
import os
import re
import subprocess
import tempfile
from pathlib import Path
from typing import Callable, Dict, List, Optional


@dataclasses.dataclass
class CleanOptions:
    """Options for cleaning secrets from git history."""
    tool: str = "auto"
    source: str = "both"
    file_paths: Optional[Dict[str, bool]] = None
    dry_run: bool = False
    force: bool = False
    no_backup: bool = False
    on_progress: Optional[Callable[[str], None]] = None


@dataclasses.dataclass
class CleanResult:
    """Result of a cleaning operation."""
    tool: str = ""
    source: str = ""
    secrets_removed: int = 0
    patterns_used: int = 0
    files_modified: int = 0
    success: bool = False
    message: str = ""
    backup_branch: str = ""
    dry_run: bool = False
    preview_secrets: Optional[List[str]] = None


@dataclasses.dataclass
class LoadSecretsResult:
    """Result of loading secrets from a file."""
    secrets: List[str]
    file_paths: List[str]
    file_map: Dict[str, int]
    source: str


def has_filter_repo() -> bool:
    """Check if git-filter-repo is available."""
    try:
        subprocess.run(
            ["git", "filter-repo", "--version"],
            capture_output=True,
            timeout=5
        )
        return True
    except (subprocess.SubprocessError, FileNotFoundError):
        return False


def has_bfg() -> bool:
    """Check if BFG is available."""
    try:
        subprocess.run(
            ["bfg", "--version"],
            capture_output=True,
            timeout=5
        )
        return True
    except (subprocess.SubprocessError, FileNotFoundError):
        pass

    try:
        subprocess.run(
            ["java", "-jar", "bfg.jar", "--version"],
            capture_output=True,
            timeout=5
        )
        return True
    except (subprocess.SubprocessError, FileNotFoundError):
        return False


def get_available_tools() -> Dict[str, bool]:
    """Get dict of available tools."""
    return {
        "filter-repo": has_filter_repo(),
        "bfg": has_bfg(),
        "filter-branch": True,
    }


def mask_secret(value: str) -> str:
    """Mask a secret value: show first 2 and last 2 chars, replace middle with asterisks."""
    if len(value) <= 4:
        return "****"
    mask_len = min(len(value) - 4, 16)
    return value[:2] + ("*" * mask_len) + value[-2:]


class Cleaner:
    """Cleans secrets from git repositories."""

    def clean(
        self,
        repo_path: str,
        secrets: List[str],
        opts: CleanOptions,
    ) -> CleanResult:
        """Clean secrets from git history and/or current files."""
        result = CleanResult()

        if not secrets:
            result.success = True
            result.message = "No secrets to clean"
            return result

        if not opts.source:
            opts.source = "both"

        result.source = opts.source
        result.dry_run = opts.dry_run

        if opts.tool == "auto":
            tool = self._select_best_tool()
        else:
            tool = opts.tool

        result.tool = tool

        patterns = self._group_secrets_into_patterns(secrets)
        result.patterns_used = len(patterns)

        if opts.dry_run:
            preview = [mask_secret(s) for s in secrets[:10]]
            result.preview_secrets = preview
            result.success = True
            result.message = f"Dry run: would remove {len(secrets)} secrets using {tool}"
            return result

        if not opts.no_backup:
            pid = os.getpid()
            backup_name = f"backup-before-clean-{pid}"
            try:
                subprocess.run(
                    ["git", "branch", backup_name],
                    cwd=repo_path,
                    capture_output=True,
                    check=True,
                    timeout=30,
                )
                result.backup_branch = backup_name
            except subprocess.SubprocessError as e:
                result.success = False
                result.message = f"Failed to create backup branch: {e}"
                return result

        if opts.source in ("current", "both"):
            allowed_files = opts.file_paths if opts.file_paths else {}
            modified = self._clean_current_files(repo_path, secrets, allowed_files)
            result.files_modified = modified

        if opts.source in ("history", "both"):
            try:
                if tool == "filter-repo":
                    self._clean_with_filter_repo(repo_path, patterns, opts)
                elif tool == "bfg":
                    self._clean_with_bfg(repo_path, secrets, opts)
                else:
                    self._clean_with_filter_branch(repo_path, patterns, opts)
            except subprocess.SubprocessError as e:
                result.success = False
                result.message = f"Failed to clean history with {tool}: {e}"
                return result

        try:
            subprocess.run(
                ["git", "reflog", "expire", "--expire=now", "--all"],
                cwd=repo_path,
                capture_output=True,
                timeout=60,
            )
            subprocess.run(
                ["git", "gc", "--prune=now", "--aggressive"],
                cwd=repo_path,
                capture_output=True,
                timeout=120,
            )
        except subprocess.SubprocessError as e:
            result.success = False
            result.message = f"Failed to clean up git: {e}"
            return result

        result.success = True
        result.secrets_removed = len(secrets)
        result.message = f"Successfully cleaned {len(secrets)} secrets from {opts.source}"

        return result

    def _clean_current_files(
        self,
        repo_path: str,
        secrets: List[str],
        allowed_files: Dict[str, bool],
    ) -> int:
        """Clean secrets from current files."""
        modified_count = 0

        if not allowed_files:
            try:
                result = subprocess.run(
                    ["git", "ls-files"],
                    cwd=repo_path,
                    capture_output=True,
                    text=True,
                    timeout=30,
                )
                tracked_files = result.stdout.strip().split("\n")
                allowed_files = {f: True for f in tracked_files if f}
            except subprocess.SubprocessError:
                return 0

        for file_path in allowed_files:
            full_path = os.path.join(repo_path, file_path)

            if not os.path.isfile(full_path):
                continue

            try:
                with open(full_path, "r", encoding="utf-8", errors="ignore") as f:
                    content = f.read()

                original_content = content

                for secret in secrets:
                    content = content.replace(secret, "***REMOVED***")

                if content != original_content:
                    with open(full_path, "w", encoding="utf-8") as f:
                        f.write(content)
                    modified_count += 1
            except (OSError, IOError):
                continue

        return modified_count

    def _clean_with_filter_repo(
        self,
        repo_path: str,
        patterns: List[str],
        opts: CleanOptions,
    ) -> None:
        """Clean using git-filter-repo."""
        with tempfile.NamedTemporaryFile(
            mode="w",
            suffix=".txt",
            delete=False,
            dir=repo_path,
        ) as f:
            for pattern in patterns:
                f.write(f"regex:{pattern}===>***REMOVED***\n")
            temp_file = f.name

        try:
            cmd = ["git", "filter-repo", "--replace-text", temp_file]
            if opts.force:
                cmd.append("--force")

            subprocess.run(
                cmd,
                cwd=repo_path,
                check=True,
                timeout=600,
            )
        finally:
            try:
                os.unlink(temp_file)
            except OSError:
                pass

    def _clean_with_bfg(
        self,
        repo_path: str,
        secrets: List[str],
        opts: CleanOptions,
    ) -> None:
        """Clean using BFG."""
        with tempfile.NamedTemporaryFile(
            mode="w",
            suffix=".txt",
            delete=False,
            dir=repo_path,
        ) as f:
            for secret in secrets:
                f.write(secret + "\n")
            temp_file = f.name

        try:
            cmd = ["bfg", "--replace-text", temp_file, repo_path]

            subprocess.run(
                cmd,
                cwd=repo_path,
                check=True,
                timeout=600,
            )
        except FileNotFoundError:
            cmd = ["java", "-jar", "bfg.jar", "--replace-text", temp_file, repo_path]
            subprocess.run(
                cmd,
                cwd=repo_path,
                check=True,
                timeout=600,
            )
        finally:
            try:
                os.unlink(temp_file)
            except OSError:
                pass

    def _clean_with_filter_branch(
        self,
        repo_path: str,
        patterns: List[str],
        opts: CleanOptions,
    ) -> None:
        """Clean using git-filter-branch."""
        sed_exprs = []
        for pattern in patterns:
            escaped_pattern = pattern.replace("~", "\\~")
            sed_exprs.append(f"s~{escaped_pattern}~***REMOVED***~g")

        sed_script = ";".join(sed_exprs)

        with tempfile.NamedTemporaryFile(
            mode="w",
            suffix=".sed",
            delete=False,
            dir=repo_path,
        ) as f:
            f.write(sed_script)
            sed_file = f.name

        try:
            tree_filter = f"find . -type f ! -path './.git*' -exec sed -i -f {sed_file} {{}} + 2>/dev/null || true"

            cmd = [
                "git",
                "filter-branch",
                "-f",
                "--tree-filter",
                tree_filter,
                "--",
                "--all",
            ]

            subprocess.run(
                cmd,
                cwd=repo_path,
                check=True,
                timeout=600,
            )
        finally:
            try:
                os.unlink(sed_file)
            except OSError:
                pass

    @staticmethod
    def load_secrets_from_jsonl(path: str) -> LoadSecretsResult:
        """Load secrets from JSONL file."""
        secrets = []
        file_paths = []
        file_map: Dict[str, int] = {}
        source_set = set()

        with open(path, "r") as f:
            for line in f:
                if not line.strip():
                    continue

                item = json.loads(line)

                if "value" in item:
                    secrets.append(item["value"])

                if "file" in item:
                    fp = item["file"]
                    if fp not in file_paths:
                        file_paths.append(fp)
                    file_map[fp] = file_map.get(fp, 0) + 1

                if "commit" in item and item["commit"]:
                    source_set.add("history")
                else:
                    source_set.add("current")

        if "history" in source_set and "current" in source_set:
            source_str = "both"
        elif "history" in source_set:
            source_str = "history"
        else:
            source_str = "current"

        secrets = list(set(secrets))

        return LoadSecretsResult(
            secrets=secrets,
            file_paths=file_paths,
            file_map=file_map,
            source=source_str,
        )

    @staticmethod
    def load_secrets_from_json(path: str) -> LoadSecretsResult:
        """Load secrets from JSON file."""
        with open(path, "r") as f:
            data = json.load(f)

        secrets = []
        file_paths = []
        file_map: Dict[str, int] = {}
        source_set = set()

        if "results" in data:
            for result in data["results"]:
                if "value" in result:
                    secrets.append(result["value"])

                if "file" in result:
                    fp = result["file"]
                    if fp not in file_paths:
                        file_paths.append(fp)
                    file_map[fp] = file_map.get(fp, 0) + 1

                if "commit" in result and result["commit"]:
                    source_set.add("history")
                else:
                    source_set.add("current")

        if "history" in source_set and "current" in source_set:
            source_str = "both"
        elif "history" in source_set:
            source_str = "history"
        else:
            source_str = "current"

        secrets = list(set(secrets))
        secrets.sort(key=len, reverse=True)

        return LoadSecretsResult(
            secrets=secrets,
            file_paths=file_paths,
            file_map=file_map,
            source=source_str,
        )

    def _group_secrets_into_patterns(self, secrets: List[str]) -> List[str]:
        """Group secrets into regex patterns (batches of 100)."""
        patterns = []
        batch = []

        for secret in secrets:
            batch.append(re.escape(secret))

            if len(batch) >= 100:
                patterns.append("|".join(batch))
                batch = []

        if batch:
            patterns.append("|".join(batch))

        return patterns

    def _select_best_tool(self) -> str:
        """Select the best available tool."""
        if has_filter_repo():
            return "filter-repo"
        elif has_bfg():
            return "bfg"
        else:
            return "filter-branch"


# Module-level convenience functions (delegates to Cleaner static methods)
def load_secrets_from_jsonl(path: str) -> LoadSecretsResult:
    """Load secrets from a JSONL file and detect source."""
    return Cleaner.load_secrets_from_jsonl(path)


def load_secrets_from_json(path: str) -> LoadSecretsResult:
    """Load secrets from a JSON scan result file and detect source."""
    return Cleaner.load_secrets_from_json(path)
