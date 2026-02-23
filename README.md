# TokenTracer Proxy

> **Note:** This project is untested and currently in development. Use at your own risk.

A unified proxy for LLM APIs that provides token tracking, cost optimization, and intelligent routing across OpenAI, Anthropic, and Google Gemini.
<img width="1916" height="994" alt="alias_setup" src="https://github.com/user-attachments/assets/a67aeae6-0cae-4dcd-8bad-0e5023c2fff9" />

## Features

- **Unified API** — Send OpenAI-compatible requests and route them to any supported provider
- **Model Aliasing** — Create aliases like `prod-gpt4` that map to any provider/model combination
- **Fallback Routing** — Automatically retry with a different provider if the primary fails
- **Light Model Optimization** — Route simple requests to cheaper models based on token count
- **Token & Cost Tracking** — Log every request with input/output token counts
- **Per-User Rate Limiting** — Configurable per-minute and daily limits via environment variables, with per-user overrides
- **API Key Management** — Generate long-lived API keys for programmatic access

## Quick Start

### Prerequisites

- Go 1.25+
- Docker & Docker Compose (for dependencies)

### Setup

1. **Clone the repository:**
   ```bash
   git clone https://github.com/andyantrim/tokentracer-proxy.git
   cd tokentracer-proxy
   ```

2. **Start dependencies (PostgreSQL + Redis):**
   ```bash
   make deps
   ```

3. **Apply the database schema:**
   ```bash
   make db-migrate
   ```

4. **Configure environment variables:**
   ```bash
   cp .env.example .env
   # Edit .env with your values
   source .env
   ```

5. **Run the server:**
   ```bash
   make run
   ```

   The server starts at `http://localhost:8080`.

### Docker Compose

```bash
# Start dependencies only (postgres + redis)
make deps

# Start everything (deps + app)
make up

# Stop everything
make down

# View logs
make logs
```

### Docker (standalone)

```bash
make docker-build
docker run -p 8080:8080 \
  -e DATABASE_URL="postgres://user:pass@host:5432/tokentracer" \
  -e JWT_SECRET="your-secret" \
  -e ENCRYPTION_KEY="your-encryption-key" \
  tokentracer-proxy
```

## Development

```bash
make help          # Show all available targets
make test          # Run tests with race detector
make lint          # Run golangci-lint
make check         # Run vet + lint + tests
make fmt           # Format code
```

Linting requires [golangci-lint](https://golangci-lint.run/welcome/install/).

## Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `DATABASE_URL` | Yes | PostgreSQL connection string |
| `JWT_SECRET` | Yes | Secret for signing JWT tokens |
| `ENCRYPTION_KEY` | Yes | Secret for AES-256-GCM encryption of provider API keys |
| `PORT` | No | HTTP port (default: `8080`) |
| `RATE_LIMIT_MINUTE` | No | Default per-minute rate limit (default: `0` = unlimited) |
| `RATE_LIMIT_DAILY` | No | Default daily rate limit (default: `0` = unlimited) |
| `ANTHROPIC_BASE_URL` | No | Override Anthropic API base URL |
| `GEMINI_BASE_URL` | No | Override Gemini API base URL |

## API Overview

### Auth

```
POST /auth/signup          # Create account
POST /auth/login           # Get session token
GET  /auth/me              # Get user info (authenticated)
POST /auth/key             # Generate API key (authenticated)
```

### Proxy

```
POST /v1/chat/completions  # Send chat completion (authenticated, rate-limited)
```

Uses the OpenAI request format. The `model` field should be one of your configured aliases.

### Management

```
POST   /manage/providers               # Add a provider API key
GET    /manage/providers               # List provider keys
GET    /manage/providers/{keyID}/models # List models for a provider
GET    /manage/models                  # List all cached models
POST   /manage/aliases                 # Create/update a model alias
GET    /manage/aliases                 # List aliases
PATCH  /manage/aliases/{alias}         # Update alias fields
GET    /manage/usage                   # Get usage statistics
```

### Example: Proxy a Request

```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "my-alias",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

## Rate Limits

Rate limits are configured via environment variables:

- `RATE_LIMIT_MINUTE` — requests per minute (default `0` = unlimited)
- `RATE_LIMIT_DAILY` — requests per day (default `0` = unlimited)

Per-user overrides can be set in the `users` table (`rate_limit_minute`, `rate_limit_daily` columns). A value of `0` means "use the server default".

## License

[Do what the fuck you want](LICENSE), cause I know I have.
