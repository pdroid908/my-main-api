package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

// Tool/Skill murni: Menerima data rangkuman & kontak, lalu kirim ke Telegram
func SendTelegramTool(summary string, contact string) {
	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	chatID := os.Getenv("TELEGRAM_CHAT_ID")

	if botToken == "" || chatID == "" {
		return
	}

	textMessage := fmt.Sprintf(
		"🚨 *PROSPEK KLIEN BARU*\n\n"+
			"📱 *Kontak*: %s\n"+
			"📝 *Rangkuman Projek*:\n%s",
		contact, summary,
	)

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)
	payload := map[string]string{
		"chat_id":    chatID,
		"text":       textMessage,
		"parse_mode": "Markdown",
	}

	jsonPayload, _ := json.Marshal(payload)

	// Kirim ke Telegram via Goroutine (Asynchronous)
	go func() {
		_, _ = http.Post(url, "application/json", bytes.NewBuffer(jsonPayload))
	}()
}