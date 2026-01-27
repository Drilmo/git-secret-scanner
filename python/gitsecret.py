#!/usr/bin/env python3
"""
Git Secret Scanner - Enterprise-compatible Python version.
Run: python gitsecret.py
"""
import sys
import os

# Add the directory containing this script to the Python path
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from gitsecret.__main__ import main

if __name__ == "__main__":
    main()
