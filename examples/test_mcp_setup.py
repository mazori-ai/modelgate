#!/usr/bin/env python3
"""
Test MCP Setup

Quick validation script to check if MCP examples are properly set up.

Usage:
    python test_mcp_setup.py
"""

import sys
from pathlib import Path


def check_dependencies():
    """Check if required dependencies are installed."""
    print("🔍 Checking dependencies...")

    missing = []

    try:
        import requests
        print("  ✓ requests")
    except ImportError:
        print("  ❌ requests")
        missing.append("requests")

    try:
        import mcp
        print("  ✓ mcp")
    except ImportError:
        print("  ❌ mcp")
        missing.append("mcp")

    if missing:
        print(f"\n❌ Missing dependencies: {', '.join(missing)}")
        print("Install with: pip install -r requirements.txt")
        return False

    print("✓ All dependencies installed\n")
    return True


def check_files():
    """Check if example files exist."""
    print("🔍 Checking example files...")

    required_files = [
        "mcp_server.py",
        "mcp_client.py",
        "mcp_demo.py",
        "requirements.txt",
        "README.md",
        "MCP_QUICKSTART.md"
    ]

    missing = []
    for filename in required_files:
        path = Path(filename)
        if path.exists():
            print(f"  ✓ {filename}")
        else:
            print(f"  ❌ {filename}")
            missing.append(filename)

    if missing:
        print(f"\n❌ Missing files: {', '.join(missing)}")
        return False

    print("✓ All example files found\n")
    return True


def check_modelgate():
    """Check if ModelGate is running."""
    print("🔍 Checking ModelGate connection...")

    try:
        import requests
        response = requests.get("http://localhost:8080/health", timeout=3)
        if response.status_code == 200:
            print("  ✓ ModelGate is running on http://localhost:8080")
            print("✓ ModelGate connection successful\n")
            return True
        else:
            print(f"  ⚠️  ModelGate responded with status {response.status_code}")
            print("✓ ModelGate is running but may have issues\n")
            return True
    except requests.exceptions.ConnectionError:
        print("  ❌ Cannot connect to ModelGate on http://localhost:8080")
        print("     Make sure ModelGate is running: ./bin/modelgate -config config.toml")
        print("⚠️  ModelGate not running (optional for testing setup)\n")
        return False
    except requests.exceptions.Timeout:
        print("  ❌ ModelGate connection timed out")
        return False
    except Exception as e:
        print(f"  ❌ Error checking ModelGate: {e}")
        return False


def check_python_version():
    """Check Python version."""
    print("🔍 Checking Python version...")

    version = sys.version_info
    print(f"  Python {version.major}.{version.minor}.{version.micro}")

    if version.major < 3 or (version.major == 3 and version.minor < 9):
        print("  ❌ Python 3.9+ required")
        return False

    print("  ✓ Python version compatible\n")
    return True


def show_next_steps(all_ok):
    """Show next steps based on validation results."""
    print("\n" + "="*60)

    if all_ok:
        print("✅ Setup Complete!")
        print("="*60)
        print("\n🚀 Next Steps:\n")
        print("1. Make sure ModelGate is running:")
        print("   cd /path/to/ModelGate")
        print("   ./bin/modelgate -config config.toml\n")
        print("2. Try the interactive client:")
        print("   python mcp_client.py\n")
        print("3. Or run the demo:")
        print("   python mcp_demo.py\n")
        print("4. Read the quick start guide:")
        print("   cat MCP_QUICKSTART.md\n")
    else:
        print("⚠️  Setup Incomplete")
        print("="*60)
        print("\n📝 Action Items:\n")
        print("1. Install missing dependencies:")
        print("   pip install -r requirements.txt\n")
        print("2. Make sure you're in the examples directory\n")
        print("3. Ensure ModelGate is running (optional for testing)\n")


def main():
    """Run all checks."""
    print("\n" + "="*60)
    print("🔬 ModelGate MCP Setup Validator")
    print("="*60 + "\n")

    results = []

    results.append(check_python_version())
    results.append(check_files())
    results.append(check_dependencies())

    # ModelGate check is optional
    modelgate_ok = check_modelgate()

    all_ok = all(results)

    show_next_steps(all_ok)

    if all_ok and not modelgate_ok:
        print("⚠️  Note: ModelGate not running. Start it to test the examples.\n")

    sys.exit(0 if all_ok else 1)


if __name__ == "__main__":
    main()
