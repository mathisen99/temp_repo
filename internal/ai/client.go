package ai

import (
	"context"
	"errors"
	"os"
	"sync"
	"time"

	"github.com/sashabaranov/go-openai"
	"ircbot/internal/logger"
)

var (
	apiKey      string
	client      *openai.Client
	initialized bool
	clientMutex sync.Mutex

	ErrMissingAPIKey = errors.New("OpenAI API key not found")
)

var modelMap = map[string]string{
	"gpt-4o":  openai.GPT4o,
	"gpt-4.5": "gpt-4.5-preview",
}

func InitializeClient() error {
	clientMutex.Lock()
	defer clientMutex.Unlock()

	apiKey = os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		logger.Warnf("AI client initialized without API key. AI features will be limited.")
		initialized = true
		return ErrMissingAPIKey
	}

	client = openai.NewClient(apiKey)
	logger.Successf("OpenAI client initialized with API key")
	initialized = true
	return nil
}

func IsInitialized() bool {
	return initialized && apiKey != ""
}

func GetClient() *openai.Client {
	return client
}

func CreateContext() (context.Context, context.CancelFunc) {
	timeout := time.Duration(GetConfig().DefaultAPITimeout) * time.Second
	return context.WithTimeout(context.Background(), timeout)
}

func MapModelName(modelName string) string {
	if mapped, exists := modelMap[modelName]; exists {
		return mapped
	}
	return modelName
}
