package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/datdt/k8sselfhost/internal/pkg/errors"
)

// BotClient abstracts Telegram Bot API communications.
type BotClient interface {
	SendMessage(ctx context.Context, chatID int64, text string, replyMarkup interface{}) error
	AnswerCallbackQuery(ctx context.Context, callbackQueryID string, text string) error
}

// HTTPBotClient implements BotClient using Telegram's standard HTTP API.
type HTTPBotClient struct {
	botToken   string
	httpClient *http.Client
}

// NewHTTPBotClient creates a new HTTPBotClient.
func NewHTTPBotClient(botToken string) *HTTPBotClient {
	return &HTTPBotClient{
		botToken:   botToken,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// SendMessage sends a message to Telegram. Automatically sets parse_mode to HTML if <pre> tags are present,
// otherwise defaults to Markdown.
func (c *HTTPBotClient) SendMessage(ctx context.Context, chatID int64, text string, replyMarkup interface{}) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", c.botToken)
	parseMode := "Markdown"
	if strings.Contains(text, "<pre>") {
		parseMode = "HTML"
	}
	payload := map[string]interface{}{
		"chat_id":    chatID,
		"text":       text,
		"parse_mode": parseMode,
	}
	if replyMarkup != nil {
		payload["reply_markup"] = replyMarkup
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return errors.Wrap(err, "marshaling telegram message payload")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return errors.Wrap(err, "creating telegram request")
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return errors.Wrap(err, "sending telegram message")
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return errors.NewInternal(fmt.Sprintf("telegram api returned status %d", resp.StatusCode), errors.ErrInternal)
	}
	return nil
}

// AnswerCallbackQuery answers an inline button callback query.
func (c *HTTPBotClient) AnswerCallbackQuery(ctx context.Context, callbackQueryID string, text string) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/answerCallbackQuery", c.botToken)
	payload := map[string]interface{}{
		"callback_query_id": callbackQueryID,
		"text":              text,
		"show_alert":        true,
	}

	body, _ := json.Marshal(payload)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return errors.Wrap(err, "answering telegram callback query")
	}
	defer resp.Body.Close()
	return nil
}
