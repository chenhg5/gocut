# gocut

gocut is a source-level minimizer for Go plugin projects. It analyzes a given entry .go file and recursively extracts only the functions, variables, and types that are actually used, generating a minimal, buildable subset of source files.

This tool is designed to trim down Go plugin projects, remove unused code, and significantly reduce build size—especially useful for dynamic plugin systems or runtime-loaded modules.

## ✨ Features:
🚀 Starts from a single Go file and tracks all used symbols across the package

📦 Recursively includes struct fields, init expressions, and type references

🧠 Ignores unused declarations and unreachable code

📁 Outputs a minimized source directory directly compilable as a Go plugin

🔧 Compatible with goimports to autofix missing import statements
