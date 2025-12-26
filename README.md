# VibeMerge

Listen for emoji reactions and merge PRs

## Overview

VibeMerge is a Go service that listens for Slack emoji reactions and automatically merges GitHub pull requests. When a user reacts with the `:heart_eyes_cat:` emoji on a Slack message containing PR metadata, VibeMerge will queue merge commands for execution by [Poppit](https://github.com/its-the-vibe/Poppit).

## Features

- Subscribes to Redis pub/sub channel for Slack reaction events
- Filters for specific emoji reactions (`heart_eyes_cat`)
- Retrieves message metadata from Slack API
- Publishes merge commands to Redis list for Poppit execution
- Configurable via environment variables
- Lightweight Docker deployment using scratch image

## Configuration

VibeMerge is configured using environment variables:

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `SLACK_BOT_TOKEN` | Slack Bot User OAuth Token | - | Yes |
| `REDIS_ADDR` | Redis server address | `localhost:6379` | No |
| `REDIS_PASSWORD` | Redis password | - | No |
| `WORK_DIR` | Working directory for Poppit commands | `/tmp/vibemerge` | No |
| `TARGET_EMOJI` | Emoji reaction to listen for | `heart_eyes_cat` | No |
| `TARGET_BRANCH` | Target branch for merge operations | `refs/heads/main` | No |
| `TIMEBOMB_CHANNEL` | Redis channel for TimeBomb TTL messages | `timebomb-messages` | No |

## Running Locally

### Prerequisites

- Go 1.24 or later
- Redis server running
- Slack Bot Token with appropriate permissions

### Build and Run

```bash
# Build
go build -o vibemerge

# Run
export SLACK_BOT_TOKEN="xoxb-your-token-here"
export REDIS_ADDR="localhost:6379"
./vibemerge
```

## Docker Deployment

### Using Docker Compose

1. Create a `.env` file with your configuration (see `.env.example` for a template):

```env
SLACK_BOT_TOKEN=xoxb-your-token-here
REDIS_ADDR=your-redis-host:6379
REDIS_PASSWORD=your-redis-password
WORK_DIR=/tmp/vibemerge
```

2. Run with docker-compose:

```bash
docker-compose up -d
```

### Building the Docker Image

```bash
docker build -t vibemerge:latest .
```

### Running the Docker Container

```bash
docker run -d \
  --name vibemerge \
  --network host \
  -e SLACK_BOT_TOKEN=xoxb-your-token-here \
  -e REDIS_ADDR=localhost:6379 \
  vibemerge:latest
```

## How It Works

1. **Reaction Event**: VibeMerge subscribes to the `slack-relay-reaction-added` Redis channel
2. **Filter**: Only `heart_eyes_cat` reactions are processed
3. **Metadata Retrieval**: Fetches the Slack message using the Slack API
4. **Validation**: Checks for PR metadata (repository, PR number, etc.)
5. **Command Generation**: Creates Poppit payload with merge commands
6. **Queue**: Pushes the payload to the `poppit-commands` Redis list
7. **TTL Setting**: Publishes a message to TimeBomb to delete the processed message after 24 hours

## Expected Message Format

### Slack Reaction Event

The service expects messages on the `slack-relay-reaction-added` channel in this format:

```json
{
  "event": {
    "type": "reaction_added",
    "user": "U123456",
    "reaction": "heart_eyes_cat",
    "item": {
      "type": "message",
      "channel": "C123456",
      "ts": "1766236581.981479"
    }
  }
}
```

### Slack Message Metadata

Messages must contain PR metadata:

```json
{
  "pr_number": 42,
  "repository": "its-the-vibe/VibeMerge",
  "pr_url": "https://github.com/its-the-vibe/VibeMerge/pull/42",
  "author": "username123",
  "branch": "feature/add-metadata",
  "event_action": "review_requested"
}
```

### Poppit Command Payload

VibeMerge generates commands for Poppit:

```json
{
  "repo": "its-the-vibe/VibeMerge",
  "branch": "refs/heads/main",
  "type": "git-webhook",
  "dir": "/tmp/vibemerge",
  "commands": [
    "gh pr --repo its-the-vibe/VibeMerge ready 42",
    "gh pr --repo its-the-vibe/VibeMerge merge 42 --squash"
  ]
}
```

## License

MIT
