"""
Interactive Terminal UI for GitSecret Scanner.

Provides a complete interactive CLI experience using Python standard library only.
Supports scan, analyze, clean, and tool checking workflows.
"""

import json
import os
import sys
from pathlib import Path
from typing import Optional, List, Dict, Any

from .config import Config, load, load_auto, default_config
from .scanner import Scanner, ScanOptions
from .analyzer import Analyzer, AnalyzeOptions
from .cleaner import (
    Cleaner,
    CleanOptions,
    has_filter_repo,
    has_bfg,
    get_available_tools,
)
from .styles import (
    render_logo,
    title,
    subtitle,
    bold,
    success,
    error,
    warning,
    muted,
    key_style,
    value_style,
    success_box,
    error_box,
)


def prompt(text: str, default: str = "") -> str:
    """Prompt user for input with optional default value."""
    if default:
        prompt_text = f"{text} [{default}]: "
    else:
        prompt_text = f"{text}: "

    result = input(prompt_text).strip()
    return result if result else default


def prompt_choice(text: str, options: List[str], default: int = 0) -> int:
    """Prompt user to choose from numbered options."""
    print()
    print(title(text))
    for i, option in enumerate(options, 1):
        print(f"{i}. {option}")

    while True:
        try:
            choice = input(f"Choose [1-{len(options)}] (default {default + 1}): ").strip()
            if not choice:
                return default
            idx = int(choice) - 1
            if 0 <= idx < len(options):
                return idx
            else:
                print(error(f"Invalid choice. Please enter a number between 1 and {len(options)}."))
        except ValueError:
            print(error("Invalid input. Please enter a number."))


def prompt_confirm(text: str, default: bool = True) -> bool:
    """Prompt user for yes/no confirmation."""
    default_str = "Y/n" if default else "y/N"
    result = input(f"{text} [{default_str}]: ").strip().lower()

    if not result:
        return default
    return result in ("y", "yes")


def _clear_screen() -> None:
    """Clear the terminal screen."""
    os.system("clear" if os.name == "posix" else "cls")


def _show_menu() -> int:
    """Show main menu and return user's choice."""
    _clear_screen()
    print(render_logo())
    print()

    options = [
        "Scan Repository",
        "Analyze Results",
        "Clean History",
        "Check Tools",
        "Quit",
    ]

    return prompt_choice("Main Menu", options, default=0)


def _scan_workflow() -> None:
    """Interactive scan workflow."""
    _clear_screen()
    print(title("Scan Repository"))
    print()

    try:
        # Get scan parameters
        repo_path = prompt("Repository path", default=".")

        # Validate repo path
        if not os.path.isdir(repo_path):
            print(error(f"Repository path '{repo_path}' does not exist."))
            input("Press Enter to continue...")
            return

        print()
        scan_mode_idx = prompt_choice(
            "Select scan mode",
            [
                "Full (slower, complete results)",
                "Stream (faster, large repos)",
                "Fast (fastest, regex only)",
            ],
            default=0,
        )
        scan_mode = ["full", "stream", "fast"][scan_mode_idx]

        print()
        source_idx = prompt_choice(
            "Select source to scan",
            [
                "Both (commits and working dir)",
                "Current (working dir only)",
                "History (commits only)",
            ],
            default=0,
        )
        source = ["both", "current", "history"][source_idx]

        branch = prompt("Git branch or ref", default="--all")

        # Determine default output file based on scan mode
        default_output = "secrets.jsonl" if scan_mode == "stream" else "secrets.json"
        output_file = prompt("Output file", default=default_output)

        config_path = prompt("Config file path (or empty for defaults)", default="")

        print()
        print(muted("Starting scan..."))
        print()

        # Load config
        if config_path:
            try:
                config = load(config_path)
            except Exception as e:
                print(error(f"Failed to load config: {e}"))
                config = default_config()
        else:
            config = load_auto()

        # Run scan
        scanner = Scanner(config)

        def on_progress(msg: str) -> None:
            print(muted(f"  {msg}"))

        options = ScanOptions(
            branch=branch,
            config_path=config_path,
            on_progress=on_progress,
        )

        # Perform the appropriate scan
        result = None
        if source == "both":
            if scan_mode == "stream":
                scanner.scan_both_stream(repo_path, output_file, options)
            else:
                result = scanner.scan_both(repo_path, options)
        elif source == "current":
            if scan_mode == "stream":
                scanner.scan_current_stream(repo_path, output_file)
            else:
                result = scanner.scan_current(repo_path)
        else:  # history
            if scan_mode == "stream":
                scanner.scan_stream(repo_path, output_file, options)
            else:
                result = scanner.scan(repo_path, options)

        # For non-stream modes, we have a result object
        if result:
            print()
            print(success_box("Scan Complete!"))
            print()
            print(f"Secrets found:    {result.secrets_found}")
            print(f"Total values:     {result.total_values}")
            print(f"Repository:       {result.repository}")
            print(f"Branch:           {result.branch}")
            print(f"Output file:      {output_file}")

            # Show top 5 secrets by frequency
            if result.secrets:
                print()
                print(subtitle("Top 5 Secrets"))
                secret_counts = {}
                for secret in result.secrets:
                    secret_counts[secret.key] = secret_counts.get(secret.key, 0) + 1

                top_secrets = sorted(
                    secret_counts.items(), key=lambda x: x[1], reverse=True
                )[:5]
                for i, (secret, count) in enumerate(top_secrets, 1):
                    print(f"  {i}. {key_style(secret)} ({count} occurrences)")
        else:
            # Stream mode - show summary
            print()
            print(success_box("Stream Scan Complete!"))
            print()
            print(f"Results written to: {output_file}")
            print(
                muted(
                    "Note: Stream mode writes JSONL format for large repositories."
                )
            )

        print()
        input("Press Enter to continue...")

    except KeyboardInterrupt:
        print()
        print(warning("Scan cancelled."))
        input("Press Enter to continue...")
    except Exception as e:
        print(error(f"Scan failed: {e}"))
        input("Press Enter to continue...")


def _analyze_workflow() -> None:
    """Interactive analyze workflow."""
    _clear_screen()
    print(title("Analyze Results"))
    print()

    try:
        # Get analysis parameters
        input_file = prompt("Input file", default="secrets.json")

        # Validate input file
        if not os.path.isfile(input_file):
            print(error(f"Input file '{input_file}' does not exist."))
            input("Press Enter to continue...")
            return

        csv_output = prompt("CSV output file", default="secrets_analysis.csv")

        print()
        print(muted("Running analysis..."))
        print()

        # Detect format and load
        analyzer = Analyzer()
        options = AnalyzeOptions()

        if input_file.endswith(".jsonl"):
            analysis = analyzer.analyze_jsonl(input_file, options)
        else:
            analysis = analyzer.analyze_json(input_file, options)

        # Display results
        print(success_box("Analysis Complete!"))
        print()
        print(f"Total entries:      {analysis.stats.total_entries}")
        print(f"Unique secrets:     {analysis.stats.unique_secrets}")
        print(f"Unique values:      {analysis.stats.unique_values}")

        # Show top 5 authors
        if analysis.stats.top_authors:
            print()
            print(subtitle("Top 5 Authors"))
            for i, author_stat in enumerate(analysis.stats.top_authors[:5], 1):
                print(f"  {i}. {author_stat.author} ({author_stat.count} secrets)")

        # Show top 5 changed secrets
        if analysis.secrets:
            print()
            print(subtitle("Top 5 Changed Secrets"))
            for i, secret in enumerate(analysis.secrets[:5], 1):
                print(f"  {i}. {key_style(secret.key)} ({secret.change_count} changes)")

        # Export CSV
        try:
            from .analyzer import export_csv

            export_csv(analysis, csv_output)
            print()
            print(success(f"CSV exported to {csv_output}"))
        except Exception as e:
            print(error(f"Failed to export CSV: {e}"))

        print()
        input("Press Enter to continue...")

    except KeyboardInterrupt:
        print()
        print(warning("Analysis cancelled."))
        input("Press Enter to continue...")
    except Exception as e:
        print(error(f"Analysis failed: {e}"))
        input("Press Enter to continue...")


def _clean_workflow() -> None:
    """Interactive clean workflow."""
    _clear_screen()
    print(title("Clean History"))
    print()

    try:
        # Get clean parameters
        scan_file = prompt("Scan results file", default="secrets.json")

        # Validate scan file
        if not os.path.isfile(scan_file):
            print(error(f"Scan file '{scan_file}' does not exist."))
            input("Press Enter to continue...")
            return

        repo_path = prompt("Repository path", default=".")

        # Validate repo path
        if not os.path.isdir(repo_path):
            print(error(f"Repository path '{repo_path}' does not exist."))
            input("Press Enter to continue...")
            return

        print()
        available_tools = get_available_tools()
        tool_options = ["Auto (detect best available)"]

        if available_tools.get("filter-repo"):
            tool_options.append("git-filter-repo (recommended)")
        if available_tools.get("bfg"):
            tool_options.append("BFG Repo-Cleaner")
        tool_options.append("git-filter-branch (slow)")

        tool_idx = prompt_choice("Select cleaning tool", tool_options, default=0)

        if tool_idx == 0:
            tool = "auto"
        elif "filter-repo" in tool_options[tool_idx]:
            tool = "filter-repo"
        elif "BFG" in tool_options[tool_idx]:
            tool = "bfg"
        else:
            tool = "filter-branch"

        print()
        dry_run = prompt_confirm("Dry run (no actual changes)", default=True)

        if not dry_run:
            print()
            print(warning("IMPORTANT: This will rewrite git history!"))
            print(warning("All commits containing secrets will be modified."))
            print()
            confirm = input("Type 'yes' to confirm: ").strip().lower()
            if confirm != "yes":
                print(warning("Clean operation cancelled."))
                input("Press Enter to continue...")
                return

        print()
        print(muted("Starting clean operation..."))
        print()

        # Load secrets
        cleaner = Cleaner()

        if scan_file.endswith(".jsonl"):
            result = cleaner.load_secrets_from_jsonl(scan_file)
        else:
            result = cleaner.load_secrets_from_json(scan_file)

        secrets = result.secrets

        # Run clean
        options = CleanOptions(
            tool=tool,
            dry_run=dry_run,
            source=result.source,
            file_paths={fp: True for fp in result.file_paths},
        )

        clean_result = cleaner.clean(repo_path, secrets, options)

        # Display results
        print()
        if clean_result.success:
            print(success_box("Clean Complete!"))
        else:
            print(error_box("Clean Failed!"))

        print()
        print(f"Tool used:         {clean_result.tool}")
        print(f"Secrets processed: {clean_result.secrets_removed}")
        print(f"Files modified:    {clean_result.files_modified}")
        print(f"Dry run:           {'Yes' if dry_run else 'No'}")

        if clean_result.backup_branch:
            print(f"Backup branch:     {clean_result.backup_branch}")

        if clean_result.preview_secrets and dry_run:
            print()
            print(subtitle("Sample Secrets (would be removed)"))
            for i, secret in enumerate(clean_result.preview_secrets[:5], 1):
                print(f"  {i}. {secret}")

        if not dry_run and clean_result.success:
            print()
            print(subtitle("Next Steps"))
            print("1. Review the changes: git log, git diff, etc.")
            print("2. Test thoroughly in a test environment")
            print("3. Force push to remove secrets from remote (if needed):")
            print("   git push --force-with-lease")
            print("4. Notify team about the rewritten history")

        if clean_result.message:
            print()
            print(muted(f"Status: {clean_result.message}"))

        print()
        input("Press Enter to continue...")

    except KeyboardInterrupt:
        print()
        print(warning("Clean cancelled."))
        input("Press Enter to continue...")
    except Exception as e:
        print(error(f"Clean failed: {e}"))
        import traceback

        print(traceback.format_exc())
        input("Press Enter to continue...")


def _check_tools_workflow() -> None:
    """Show status of available tools."""
    _clear_screen()
    print(title("Check Tools"))
    print()

    tools = [
        ("git-filter-repo", has_filter_repo(), "Recommended tool for cleaning history"),
        (
            "BFG Repo-Cleaner",
            has_bfg(),
            "Fast alternative for large repos",
        ),
        (
            "git-filter-branch",
            True,
            "Built-in Git tool (slow, not recommended)",
        ),
    ]

    print(subtitle("Tool Status"))
    print()

    available = []
    for tool_name, is_available, description in tools:
        status = success("OK") if is_available else error("NOT FOUND")
        print(f"{status}  {bold(tool_name)}")
        print(f"      {muted(description)}")
        print()
        if is_available:
            available.append(tool_name)

    if available:
        print(success(f"Available tools: {', '.join(available)}"))
    else:
        print(warning("No cleaning tools found. Please install git-filter-repo:"))
        print(muted("  pip install git-filter-repo"))
        print(muted("  # or"))
        print(muted("  brew install bfg"))

    print()
    input("Press Enter to continue...")


def run() -> None:
    """Main entry point - run the interactive TUI."""
    try:
        while True:
            choice = _show_menu()

            if choice == 0:  # Scan Repository
                _scan_workflow()
            elif choice == 1:  # Analyze Results
                _analyze_workflow()
            elif choice == 2:  # Clean History
                _clean_workflow()
            elif choice == 3:  # Check Tools
                _check_tools_workflow()
            elif choice == 4:  # Quit
                _clear_screen()
                print(muted("Goodbye!"))
                break

    except KeyboardInterrupt:
        print()
        print(warning("Interrupted by user."))
        sys.exit(0)
    except Exception as e:
        print(error(f"Unexpected error: {e}"))
        import traceback

        print(traceback.format_exc())
        sys.exit(1)


if __name__ == "__main__":
    run()
