# Contributing to TokenTracer Proxy

Thanks for your interest in contributing! Here's how to get started.

## Development Setup

1. **Prerequisites:** Go 1.25+, PostgreSQL, and a running database instance.

2. **Clone and configure:**
   ```bash
   git clone https://github.com/andyantrim/tokentracer-proxy.git
   cd tokentracer-proxy
   cp .env.example .env  # Edit with your local values
   ```

3. **Set up the database:**
   ```bash
   psql -U postgres -c "CREATE DATABASE tokentracer;"
   psql -U postgres -d tokentracer -f db/schema.sql
   ```

4. **Run the server:**
   ```bash
   source .env
   go run .
   ```

5. **Run tests:**
   ```bash
   go test ./...
   ```

## Making Changes

1. Fork the repo and create a feature branch from `main`.
2. Write tests for new functionality.
3. Ensure all tests pass (`go test ./...`).
4. Run `go vet ./...` before submitting.
5. Open a pull request with a clear description of the change.

## Code Style

- Follow standard Go conventions (`gofmt`, `go vet`).
- Keep error messages lowercase without trailing punctuation.
- Use the existing patterns in the codebase (repository pattern, provider interface, etc.).

## Reporting Issues

Open an issue on GitHub with:
- A clear description of the problem or feature request.
- Steps to reproduce (for bugs).
- Your Go version and OS.

## License

By contributing, you agree that your contributions will be licensed under the MIT License.
