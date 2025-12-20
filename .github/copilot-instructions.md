# VibeMerge - Copilot Instructions

## Project Overview

VibeMerge is a Go service that listens for Slack emoji reactions and automatically merges GitHub pull requests. When a user reacts with the `:heart_eyes_cat:` emoji on a Slack message containing PR metadata, VibeMerge queues merge commands for execution by [Poppit](https://github.com/its-the-vibe/Poppit).

## Technology Stack

- **Language**: Go 1.25.5
- **Dependencies**:
  - `github.com/redis/go-redis/v9` - Redis client for pub/sub
  - `github.com/slack-go/slack` - Slack API client
- **Runtime**: Docker (scratch-based minimal image)

## Build and Test Instructions

### Building the Project

```bash
go build -o vibemerge .
```

### Running Locally

1. Set required environment variables:
   ```bash
   export SLACK_BOT_TOKEN="xoxb-your-token-here"
   export REDIS_ADDR="localhost:6379"
   ```

2. Run the application:
   ```bash
   ./vibemerge
   ```

### Docker Build

```bash
docker build -t vibemerge:latest .
```

### Testing

Currently, this repository does not have automated tests. When adding tests:
- Use Go's standard `testing` package
- Place test files alongside source files with `_test.go` suffix
- Run tests with `go test ./...`
- Generate coverage with `go test -cover ./...`

## Architecture and Design

### Core Components

1. **Configuration** (`Config` struct):
   - Loads settings from environment variables
   - Required: `SLACK_BOT_TOKEN`
   - Optional: `REDIS_ADDR`, `REDIS_PASSWORD`, `WORK_DIR`, `TARGET_EMOJI`, `TARGET_BRANCH`

2. **Reaction Processing**:
   - Subscribes to `slack-relay-reaction-added` Redis pub/sub channel
   - Filters for configured emoji reactions (default: `heart_eyes_cat`)
   - Retrieves message metadata from Slack API
   - Validates PR metadata presence

3. **Command Queue**:
   - Generates Poppit payload with merge commands
   - Pushes to `poppit-commands` Redis list

### Data Structures

- **ReactionEvent**: Incoming Slack reaction event from Redis
- **PRMetadata**: PR information embedded in Slack messages
- **PoppitPayload**: Command payload for Poppit execution

### Message Flow

1. Slack reaction added → Published to `slack-relay-reaction-added` channel
2. VibeMerge receives and filters reaction events
3. Fetches Slack message metadata
4. Validates PR metadata exists
5. Creates merge command payload
6. Pushes to `poppit-commands` Redis list

## Coding Conventions

### Go Style

- Follow standard Go conventions and formatting (use `gofmt`)
- Use descriptive struct and variable names
- Prefer explicit error handling over panics
- Use structured logging with `log` package

### Error Handling

- Always check and handle errors explicitly
- Return errors up the call stack rather than logging and continuing when appropriate
- Use `fmt.Errorf` with `%w` verb for error wrapping
- Log errors with context (channel, timestamp, PR info)

### Configuration

- All configuration comes from environment variables
- Provide sensible defaults where appropriate
- Required values must be validated on startup
- Use `getEnv()` helper for reading environment variables with defaults

### Docker

- Use multi-stage builds (builder + scratch runtime)
- Keep final image minimal (scratch base)
- Include CA certificates for HTTPS requests
- Use CGO_ENABLED=0 for static binaries

## Dependencies Management

- Use Go modules (`go.mod`, `go.sum`)
- Pin to specific versions for reproducible builds
- Update dependencies carefully and test thoroughly
- Run `go mod tidy` after adding/removing dependencies

## Environment Variables

All configuration is done via environment variables:

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `SLACK_BOT_TOKEN` | Yes | - | Slack Bot User OAuth Token |
| `REDIS_ADDR` | No | `localhost:6379` | Redis server address |
| `REDIS_PASSWORD` | No | - | Redis password |
| `WORK_DIR` | No | `/tmp/vibemerge` | Working directory for Poppit commands |
| `TARGET_EMOJI` | No | `heart_eyes_cat` | Emoji reaction to listen for |
| `TARGET_BRANCH` | No | `refs/heads/main` | Target branch for merge operations |

## Important Notes

- This service is designed to work in conjunction with Poppit for executing merge commands
- It does not directly merge PRs; it queues commands for Poppit
- Slack message metadata must contain valid PR information (repository, PR number)
- The service runs continuously until receiving SIGINT or SIGTERM

## Common Tasks

### Adding New Configuration Options

1. Add field to `Config` struct
2. Update `loadConfig()` function with `getEnv()` call
3. Update environment variables table in README.md
4. Update this file's Environment Variables section

### Modifying Merge Behavior

- Merge commands are generated in `handleReactionMessage()`
- Commands use GitHub CLI (`gh pr`) format
- Default: mark PR ready + squash merge

### Changing Target Emoji

Set the `TARGET_EMOJI` environment variable to the emoji name (without colons).

## File Structure

```
.
├── main.go                 # All application logic
├── go.mod                  # Go module definition
├── go.sum                  # Go module checksums
├── Dockerfile              # Multi-stage Docker build
├── docker-compose.yml      # Docker Compose configuration
├── .env.example            # Example environment variables
├── .gitignore              # Git ignore rules
└── README.md               # Project documentation
```

## When Making Changes

1. **Always** maintain backward compatibility with existing environment variables
2. **Always** update README.md if you change configuration or behavior
3. **Always** test Docker build after changes: `docker build -t vibemerge:test .`
4. **Consider** adding tests for new functionality
5. **Verify** that error handling is comprehensive and informative
6. **Ensure** changes work with the minimal scratch-based Docker image
