"""
ANSI styling module for GitSecret Scanner.

Provides color constants, styling functions, and terminal utilities
for rendering TUI components with ANSI colors and box-drawing characters.
Uses only Python standard library with true-color (24-bit) ANSI escape codes.
"""

import os
import sys
from typing import Tuple, Optional

# Color Constants (RGB values)
COLOR_PRIMARY = (124, 58, 237)      # #7C3AED - Purple
COLOR_SECONDARY = (16, 185, 129)    # #10B981 - Green
COLOR_DANGER = (239, 68, 68)        # #EF4444 - Red
COLOR_WARNING = (245, 158, 11)      # #F59E0B - Orange
COLOR_MUTED = (107, 114, 128)       # #6B7280 - Gray
COLOR_TEXT = (249, 250, 251)        # #F9FAFB - White
COLOR_RESET = "\033[0m"

# ASCII Logo
LOGO = r"""   _____ _ _     _____                     _
  / ____(_) |   / ____|                   | |
 | |  __ _| |_ | (___   ___  ___ _ __ ___ | |_
 | | |_ | | __|\___ \ / _ \/ __| '__/ _ \| __|
 | |__| | | |_ ____) |  __/ (__| | |  __/| |_
  \_____|_|\__|_____/ \___|\___|_|  \___| \__|
                                Scanner & Cleaner"""


def _rgb_to_ansi(r: int, g: int, b: int) -> str:
    """Convert RGB to ANSI true color (24-bit) foreground escape code."""
    return f"\033[38;2;{r};{g};{b}m"


def _rgb_to_bg_ansi(r: int, g: int, b: int) -> str:
    """Convert RGB to ANSI true color (24-bit) background escape code."""
    return f"\033[48;2;{r};{g};{b}m"


def supports_color() -> bool:
    """
    Check if the terminal supports color output.

    Returns True if the terminal appears to support ANSI colors,
    based on environment variables and TTY detection.
    """
    # Explicit disable via NO_COLOR standard
    if os.environ.get("NO_COLOR"):
        return False

    # Explicit enable via FORCE_COLOR
    if os.environ.get("FORCE_COLOR"):
        return True

    # Check if stdout is connected to a TTY
    if not hasattr(sys.stdout, "isatty") or not sys.stdout.isatty():
        return False

    # Dumb terminal never supports colors
    term = os.environ.get("TERM", "").lower()
    if term == "dumb":
        return False

    # Modern systems support colors by default
    return True


def _colorize(
    text: str,
    color: Tuple[int, int, int],
    bold: bool = False
) -> str:
    """
    Apply color to text using ANSI escape codes.

    Args:
        text: The text to colorize
        color: RGB tuple (r, g, b)
        bold: Whether to apply bold formatting

    Returns:
        Text with ANSI color codes, or plain text if colors unsupported
    """
    if not supports_color():
        return text

    r, g, b = color
    code = _rgb_to_ansi(r, g, b)

    if bold:
        code = f"\033[1m{code}"

    return f"{code}{text}{COLOR_RESET}"


def _colorize_bg(
    text: str,
    bg_color: Tuple[int, int, int],
    fg_color: Optional[Tuple[int, int, int]] = None
) -> str:
    """
    Apply background color to text.

    Args:
        text: The text to colorize
        bg_color: RGB tuple for background (r, g, b)
        fg_color: Optional RGB tuple for foreground

    Returns:
        Text with ANSI color codes, or plain text if colors unsupported
    """
    if not supports_color():
        return text

    r, g, b = bg_color
    code = _rgb_to_bg_ansi(r, g, b)

    if fg_color:
        fr, fg, fb = fg_color
        code = f"{code}{_rgb_to_ansi(fr, fg, fb)}"

    return f"{code}{text}{COLOR_RESET}"


# ============================================================================
# Style Functions
# ============================================================================

def title(text: str) -> str:
    """Style text as a title (bold primary purple)."""
    return _colorize(text, COLOR_PRIMARY, bold=True)


def subtitle(text: str) -> str:
    """Style text as a subtitle (primary purple)."""
    return _colorize(text, COLOR_PRIMARY)


def bold(text: str) -> str:
    """Apply bold formatting."""
    if not supports_color():
        return text
    return f"\033[1m{text}{COLOR_RESET}"


def success(text: str) -> str:
    """Style text in success color (green)."""
    return _colorize(text, COLOR_SECONDARY)


def error(text: str) -> str:
    """Style text in error color (red)."""
    return _colorize(text, COLOR_DANGER)


def warning(text: str) -> str:
    """Style text in warning color (orange)."""
    return _colorize(text, COLOR_WARNING)


def muted(text: str) -> str:
    """Style text in muted color (gray)."""
    return _colorize(text, COLOR_MUTED)


def key_style(text: str) -> str:
    """Style a configuration key (bold primary)."""
    return _colorize(text, COLOR_PRIMARY, bold=True)


def value_style(text: str) -> str:
    """Style a configuration value (secondary green)."""
    return _colorize(text, COLOR_SECONDARY)


def masked_value(original: str = "") -> str:
    """
    Render a masked secret value.

    Shows "***REMOVED***" in red (danger color), optionally with original length.

    Args:
        original: Optional original value to show length of

    Returns:
        Masked value with ANSI red color
    """
    masked = "***REMOVED***"
    if original:
        return error(f"{masked} (was {len(original)} chars)")
    return error(masked)


# ============================================================================
# Box Drawing Functions
# ============================================================================

def box(content: str, color: str = "purple", width: Optional[int] = None) -> str:
    """
    Draw a box around content using Unicode box-drawing characters.

    Uses ╭╮╰╯│─ characters to create a bordered box with ANSI colors.

    Args:
        content: The content to put in the box (can be multiline)
        color: Color name ("purple", "green", "red", "orange", "gray")
        width: Optional fixed width; if None, content width is used

    Returns:
        Boxed content with ANSI colors applied
    """
    color_map = {
        "purple": COLOR_PRIMARY,
        "green": COLOR_SECONDARY,
        "red": COLOR_DANGER,
        "orange": COLOR_WARNING,
        "gray": COLOR_MUTED,
    }

    rgb = color_map.get(color, COLOR_PRIMARY)

    # Split content into lines and strip ANSI codes for width calculation
    lines = content.split("\n")

    # Calculate max width (accounting for ANSI codes in existing text)
    max_width = 0
    for line in lines:
        # Remove ANSI escape codes for width calculation
        clean_line = _strip_ansi(line)
        max_width = max(max_width, len(clean_line))

    if width:
        max_width = max(max_width, width)

    # Build box with Unicode characters
    h_line = _colorize("─" * (max_width + 2), rgb)
    top = _colorize("╭", rgb) + h_line + _colorize("╮", rgb)
    bottom = _colorize("╰", rgb) + h_line + _colorize("╯", rgb)

    box_lines = [top]
    for line in lines:
        clean_line = _strip_ansi(line)
        padding = max_width - len(clean_line)
        padded = f"{line}{' ' * padding}"
        box_line = _colorize("│", rgb) + padded + _colorize("│", rgb)
        box_lines.append(box_line)
    box_lines.append(bottom)

    return "\n".join(box_lines)


def success_box(content: str) -> str:
    """Draw a green box around content."""
    return box(content, color="green")


def error_box(content: str) -> str:
    """Draw a red box around content."""
    return box(content, color="red")


# ============================================================================
# Menu Helpers
# ============================================================================

def menu_item(text: str, selected: bool = False) -> str:
    """
    Render a menu item with optional selection indicator.

    Selected items show "  ▸ text" in purple.
    Unselected items show "    text" in normal color.

    Args:
        text: The menu item text
        selected: Whether the item is selected

    Returns:
        Formatted menu item string
    """
    if selected:
        indicator = _colorize("▸", COLOR_PRIMARY)
        return f"  {indicator} {text}"
    else:
        return f"    {text}"


# ============================================================================
# Progress Helpers
# ============================================================================

def progress_bar(current: int, total: int, width: int = 40) -> str:
    """
    Render a text-based progress bar.

    Shows a filled bar in primary purple with percentage.
    Uses █ for filled portion and ░ for empty portion.

    Args:
        current: Current progress value
        total: Total progress value
        width: Width of the progress bar in characters (default 40)

    Returns:
        ASCII progress bar with percentage and ANSI colors
    """
    if total <= 0:
        percentage = 0
        filled = 0
    else:
        percentage = int((current / total) * 100)
        filled = int((current / total) * width)

    bar = "█" * filled + "░" * (width - filled)
    colored_bar = _colorize(bar, COLOR_PRIMARY)

    return f"{colored_bar} {percentage}%"


# ============================================================================
# Logo Rendering
# ============================================================================

def render_logo() -> str:
    """
    Render the GitSecret Scanner ASCII logo with ANSI colors.

    The first 5 lines are colored in primary purple (first line bold),
    and the subtitle line is colored in secondary green.

    Returns:
        The ASCII logo with ANSI color codes applied
    """
    lines = LOGO.split("\n")
    colored_lines = []

    for i, line in enumerate(lines):
        if i < 5:  # First 5 lines in primary purple
            colored_lines.append(_colorize(line, COLOR_PRIMARY, bold=(i == 0)))
        else:  # Last line (subtitle) in secondary green
            colored_lines.append(_colorize(line, COLOR_SECONDARY))

    return "\n".join(colored_lines)


# ============================================================================
# Terminal Utilities
# ============================================================================

def clear_screen() -> str:
    """Return ANSI escape code to clear the screen."""
    return "\033[2J\033[H"


def hide_cursor() -> str:
    """Return ANSI escape code to hide the cursor."""
    return "\033[?25l"


def show_cursor() -> str:
    """Return ANSI escape code to show the cursor."""
    return "\033[?25h"


# ============================================================================
# Helper Functions
# ============================================================================

def _strip_ansi(text: str) -> str:
    """
    Remove ANSI escape codes from text.

    Args:
        text: Text potentially containing ANSI escape codes

    Returns:
        Text with all ANSI codes removed
    """
    import re
    # Remove all ANSI escape sequences
    ansi_escape = re.compile(r'\033\[[0-9;]*m')
    return ansi_escape.sub('', text)
