package notification

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Notifier interface {
	Notify(contactID int64, message string) error
}

type TelegramNotifier struct {
	botToken string
	client   *http.Client
}

func NewTelegramNotifier(token string) *TelegramNotifier {
	return &TelegramNotifier{
		botToken: token,
		client:   &http.Client{Timeout: 10 * time.Second},
	}
}

type telegramRequest struct {
	ChatID    int64  `json:"chat_id"`
	Text      string `json:"text"`
	ParseMode string `json:"parse_mode"`
}

func (t *TelegramNotifier) Notify(contactID int64, message string) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", t.botToken)

	reqBody := &telegramRequest{
		ChatID:    contactID,
		Text:      message,
		ParseMode: "MarkdownV2",
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal telegram request: %w", err)
	}

	resp, err := t.client.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("failed to send request to telegram: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var result map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&result)
		return fmt.Errorf("telegram API returned non-200 status: %s, description: %v", resp.Status, result["description"])
	}

	return nil
}
