"""Entry point for running as: python -m gitsecret"""
import sys
from .tui import run

def main():
    try:
        run()
    except KeyboardInterrupt:
        print("\nExiting...")
        sys.exit(0)
    except Exception as e:
        print(f"\nError: {e}", file=sys.stderr)
        sys.exit(1)

if __name__ == "__main__":
    main()
