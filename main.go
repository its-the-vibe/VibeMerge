package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/redis/go-redis/v9"
	"github.com/slack-go/slack"
)

// Config holds the application configuration
type Config struct {
	SlackBotToken   string
	RedisAddr       string
	RedisPassword   string
	RedisDB         int
	WorkDir         string
	TargetEmoji     string
	TargetBranch    string
	PoppitQueue     string
	TimeBombChannel string
	TimeBombTTL     int
	LogLevel        string
}

// ReactionEvent represents the message from slack-relay-reaction-added channel
type ReactionEvent struct {
	Token               string      `json:"token"`
	TeamID              string      `json:"team_id"`
	ContextTeamID       string      `json:"context_team_id"`
	ContextEnterpriseID interface{} `json:"context_enterprise_id"`
	APIAppID            string      `json:"api_app_id"`
	Event               struct {
		Type     string `json:"type"`
		User     string `json:"user"`
		Reaction string `json:"reaction"`
		Item     struct {
			Type    string `json:"type"`
			Channel string `json:"channel"`
			Ts      string `json:"ts"`
		} `json:"item"`
		ItemUser string `json:"item_user"`
		EventTs  string `json:"event_ts"`
	} `json:"event"`
	Type           string `json:"type"`
	EventID        string `json:"event_id"`
	EventTime      int64  `json:"event_time"`
	Authorizations []struct {
		EnterpriseID        interface{} `json:"enterprise_id"`
		TeamID              string      `json:"team_id"`
		UserID              string      `json:"user_id"`
		IsBot               bool        `json:"is_bot"`
		IsEnterpriseInstall bool        `json:"is_enterprise_install"`
	} `json:"authorizations"`
	IsExtSharedChannel bool   `json:"is_ext_shared_channel"`
	EventContext       string `json:"event_context"`
}

// PRMetadata represents the metadata embedded in Slack messages
type PRMetadata struct {
	PRNumber    int    `json:"pr_number"`
	Repository  string `json:"repository"`
	PRURL       string `json:"pr_url"`
	Author      string `json:"author"`
	Branch      string `json:"branch"`
	EventAction string `json:"event_action"`
}

// PoppitPayload represents the command payload to send to Poppit
type PoppitPayload struct {
	Repo     string   `json:"repo"`
	Branch   string   `json:"branch"`
	Type     string   `json:"type"`
	Dir      string   `json:"dir"`
	Commands []string `json:"commands"`
}

// TimeBombMessage represents the TTL message to send to TimeBomb
type TimeBombMessage struct {
	Channel string `json:"channel"`
	Ts      string `json:"ts"`
	TTL     int    `json:"ttl"`
}

// LogLevel represents the logging level
type LogLevel int

const (
	LogLevelDebug LogLevel = iota
	LogLevelInfo
	LogLevelWarning
	LogLevelError
)

var currentLogLevel LogLevel

func parseLogLevel(level string) LogLevel {
	switch level {
	case "DEBUG":
		return LogLevelDebug
	case "INFO":
		return LogLevelInfo
	case "WARNING", "WARN":
		return LogLevelWarning
	case "ERROR":
		return LogLevelError
	default:
		return LogLevelInfo
	}
}

func logDebug(format string, v ...interface{}) {
	if currentLogLevel <= LogLevelDebug {
		log.Printf("[DEBUG] "+format, v...)
	}
}

func logInfo(format string, v ...interface{}) {
	if currentLogLevel <= LogLevelInfo {
		log.Printf("[INFO] "+format, v...)
	}
}

func logWarning(format string, v ...interface{}) {
	if currentLogLevel <= LogLevelWarning {
		log.Printf("[WARNING] "+format, v...)
	}
}

func logError(format string, v ...interface{}) {
	if currentLogLevel <= LogLevelError {
		log.Printf("[ERROR] "+format, v...)
	}
}

func main() {
	config := loadConfig()
	
	// Set the log level
	currentLogLevel = parseLogLevel(config.LogLevel)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Initialize Redis client
	redisClient := redis.NewClient(&redis.Options{
		Addr:     config.RedisAddr,
		Password: config.RedisPassword,
		DB:       config.RedisDB,
	})
	defer redisClient.Close()

	// Test Redis connection
	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	logInfo("Connected to Redis successfully")

	// Initialize Slack client
	slackClient := slack.New(config.SlackBotToken)

	// Start processing
	go processReactions(ctx, redisClient, slackClient, config)

	// Wait for shutdown signal
	<-sigChan
	logInfo("Shutdown signal received, exiting...")
	cancel()
}

func loadConfig() *Config {
	config := &Config{
		SlackBotToken:   getEnv("SLACK_BOT_TOKEN", ""),
		RedisAddr:       getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword:   getEnv("REDIS_PASSWORD", ""),
		RedisDB:         0,
		WorkDir:         getEnv("WORK_DIR", "/tmp/vibemerge"),
		TargetEmoji:     getEnv("TARGET_EMOJI", "heart_eyes_cat"),
		TargetBranch:    getEnv("TARGET_BRANCH", "refs/heads/main"),
		PoppitQueue:     getEnv("POPPIT_QUEUE", "poppit-commands"),
		TimeBombChannel: getEnv("TIMEBOMB_CHANNEL", "timebomb-messages"),
		TimeBombTTL:     getEnvInt("TIMEBOMB_TTL", 86400), // 24 hours in seconds
		LogLevel:        getEnv("LOG_LEVEL", "INFO"),
	}

	if config.SlackBotToken == "" {
		log.Fatal("SLACK_BOT_TOKEN environment variable is required")
	}

	return config
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
		// Use standard log here since logging system may not be initialized yet
		log.Printf("[WARNING] invalid integer value for %s: %s, using default: %d", key, value, defaultValue)
	}
	return defaultValue
}

func processReactions(ctx context.Context, redisClient *redis.Client, slackClient *slack.Client, config *Config) {
	pubsub := redisClient.Subscribe(ctx, "slack-relay-reaction-added")
	defer pubsub.Close()

	logInfo("Subscribed to slack-relay-reaction-added channel")

	for {
		select {
		case <-ctx.Done():
			return
		default:
			msg, err := pubsub.ReceiveMessage(ctx)
			if err != nil {
				logError("Error receiving message: %v", err)
				continue
			}

			if err := handleReactionMessage(ctx, msg.Payload, redisClient, slackClient, config); err != nil {
				logError("Error handling reaction message: %v", err)
			}
		}
	}
}

func handleReactionMessage(ctx context.Context, payload string, redisClient *redis.Client, slackClient *slack.Client, config *Config) error {
	var reactionEvent ReactionEvent
	if err := json.Unmarshal([]byte(payload), &reactionEvent); err != nil {
		return fmt.Errorf("failed to unmarshal reaction event: %w", err)
	}

	// Only process configured target emoji reactions
	if reactionEvent.Event.Reaction != config.TargetEmoji {
		logDebug("Ignoring reaction: %s", reactionEvent.Event.Reaction)
		return nil
	}

	logInfo("Processing %s reaction on message %s in channel %s",
		config.TargetEmoji, reactionEvent.Event.Item.Ts, reactionEvent.Event.Item.Channel)

	// Retrieve the message from Slack
	metadata, err := getMessageMetadata(slackClient, reactionEvent.Event.Item.Channel, reactionEvent.Event.Item.Ts)
	if err != nil {
		return fmt.Errorf("failed to get message metadata: %w", err)
	}

	if metadata == nil {
		logDebug("No PR metadata found in message, ignoring")
		return nil
	}

	logInfo("Found PR metadata: repo=%s, pr=%d", metadata.Repository, metadata.PRNumber)

	// Create Poppit payload
	poppitPayload := PoppitPayload{
		Repo:   metadata.Repository,
		Branch: config.TargetBranch,
		Type:   metadata.EventAction,
		Dir:    config.WorkDir,
		Commands: []string{
			fmt.Sprintf("gh pr --repo %s ready %d", metadata.Repository, metadata.PRNumber),
			fmt.Sprintf("gh pr --repo %s merge %d --squash", metadata.Repository, metadata.PRNumber),
		},
	}

	// Publish to Poppit queue
	payloadJSON, err := json.Marshal(poppitPayload)
	if err != nil {
		return fmt.Errorf("failed to marshal poppit payload: %w", err)
	}

	if err := redisClient.RPush(ctx, config.PoppitQueue, string(payloadJSON)).Err(); err != nil {
		return fmt.Errorf("failed to push to %s: %w", config.PoppitQueue, err)
	}

	logInfo("Successfully queued merge command for PR %d in %s", metadata.PRNumber, metadata.Repository)

	// Set TTL on the processed message by publishing to TimeBomb
	channel := reactionEvent.Event.Item.Channel
	timestamp := reactionEvent.Event.Item.Ts
	if err := publishTimeBombMessage(ctx, redisClient, config, channel, timestamp); err != nil {
		// Log the error but don't fail the entire operation
		logWarning("Failed to set TTL on message: %v", err)
	}

	return nil
}

func getMessageMetadata(slackClient *slack.Client, channel, timestamp string) (*PRMetadata, error) {
	// Retrieve the message using conversations.history
	params := &slack.GetConversationHistoryParameters{
		ChannelID:          channel,
		Latest:             timestamp,
		Limit:              1,
		Inclusive:          true,
		IncludeAllMetadata: true,
	}

	history, err := slackClient.GetConversationHistory(params)
	if err != nil {
		return nil, fmt.Errorf("failed to get conversation history: %w", err)
	}

	if len(history.Messages) == 0 {
		return nil, fmt.Errorf("no message found at timestamp %s", timestamp)
	}

	message := history.Messages[0]

	// Check if message has metadata
	if message.Metadata.EventType == "" {
		return nil, nil
	}

	// Parse metadata as PRMetadata
	var metadata PRMetadata
	metadataJSON, err := json.Marshal(message.Metadata.EventPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal metadata: %w", err)
	}

	if err := json.Unmarshal(metadataJSON, &metadata); err != nil {
		return nil, fmt.Errorf("failed to unmarshal PR metadata: %w", err)
	}

	// Validate that required fields are present
	if metadata.PRNumber == 0 || metadata.Repository == "" {
		return nil, nil
	}

	return &metadata, nil
}

func publishTimeBombMessage(ctx context.Context, redisClient *redis.Client, config *Config, channel, timestamp string) error {
	timeBombMsg := TimeBombMessage{
		Channel: channel,
		Ts:      timestamp,
		TTL:     config.TimeBombTTL,
	}

	msgJSON, err := json.Marshal(timeBombMsg)
	if err != nil {
		return fmt.Errorf("failed to marshal timebomb message: %w", err)
	}

	if err := redisClient.Publish(ctx, config.TimeBombChannel, string(msgJSON)).Err(); err != nil {
		return fmt.Errorf("failed to publish to %s: %w", config.TimeBombChannel, err)
	}

	logInfo("Successfully set TTL of %d seconds on message %s in channel %s", config.TimeBombTTL, timestamp, channel)
	return nil
}
