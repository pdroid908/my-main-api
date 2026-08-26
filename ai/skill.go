package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

func SendTelegramTool(summary string, contact string) error {
	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	chatID := os.Getenv("TELEGRAM_CHAT_ID")

	if botToken == "" || chatID == "" {
		return fmt.Errorf("telegram configuration tidak tersedia")
	}

	textMessage := fmt.Sprintf(
		"🚨 *PROSPEK KLIEN BARU*\n\n"+
			"📱 *Kontak*: %s\n"+
			"📝 *Rangkuman Projek*:\n%s",
		contact,
		summary,
	)

	url := fmt.Sprintf(
		"https://api.telegram.org/bot%s/sendMessage",
		botToken,
	)

	payload := map[string]string{
		"chat_id":    chatID,
		"text":       textMessage,
		"parse_mode": "Markdown",
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal telegram payload: %w", err)
	}

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	resp, err := client.Post(
		url,
		"application/json",
		bytes.NewBuffer(jsonPayload),
	)
	if err != nil {
		return fmt.Errorf("telegram request gagal: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("telegram mengembalikan status %d", resp.StatusCode)
	}

	return nil
}