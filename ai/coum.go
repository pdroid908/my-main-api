package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

// Struct Payload dan Response untuk Handler Gin & Compound JSON
type CompoundPayload struct {
	Prompt   string        `json:"prompt"`
	Messages []GroqMessage `json:"messages"`
}

type CompoundAIResponse struct {
	Reply        string `json:"reply"`
	SendTelegram bool   `json:"send_telegram"`
	Contact      string `json:"contact"`
	Summary      string `json:"summary"`
}

// Handler Utama Gin
func AskGroqCompoundWithSkill() gin.HandlerFunc {
	return func(c *gin.Context) {
		var input CompoundPayload
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
			return
		}

		cleanPrompt := strings.TrimSpace(input.Prompt)
		if cleanPrompt == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Pesan kosong"})
			return
		}

		// Batasi history percakapan
		var limitedHistory []GroqMessage
		if len(input.Messages) > MaxHistory {
			limitedHistory = input.Messages[len(input.Messages)-MaxHistory:]
		} else {
			limitedHistory = input.Messages
		}

		// Panggil AI Compound
		parsedResp, err := callGroqCompoundJSON(cleanPrompt, limitedHistory)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// Jika Compound menginstruksikan untuk kirim ke Telegram (send_telegram = true)
		if parsedResp.SendTelegram {
			// Menggunakan SendTelegramTool dari skill.go
			SendTelegramTool(parsedResp.Summary, parsedResp.Contact)
		}

		c.JSON(http.StatusOK, gin.H{"reply": parsedResp.Reply})
	}
}

// Fungsi Panggilan API ke Groq Model `groq/compound`
func callGroqCompoundJSON(prompt string, history []GroqMessage) (*CompoundAIResponse, error) {
	apiKey := os.Getenv("GROQ_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("GROQ_API_KEY belum diset di .env")
	}

	systemPrompt := "Kamu adalah asisten portofolio Putra Nur Rohman (Full-Stack & Backend Developer Go).\n" +
		"Jawablah dengan ramah, singkat, dan gunakan emoji.\n\n" +
		"TUGAS UTAMA: Jawab pertanyaan user dan tentukan apakah perlu mengirim notifikasi ke Telegram internal Putra.\n" +
		"Wajib berikan output JSON murni dengan format persis seperti ini:\n" +
		`{"reply": "pesan balasan ke user", "send_telegram": true, "contact": "nomor/email user", "summary": "rangkuman projek"}` + "\n\n" +
		"ATURAN BANTUAN & PROSPEK:\n" +
		"1. Jika user berniat order/rekrut, minta kontak (Nomor WA/Email) mereka.\n" +
		"2. Jika user memberikan kontak, jangan langsung set send_telegram ke true. Tanya konfirmasi/validasi ke user dulu.\n" +
		"3. Jika user SUDAH mengonfirmasi/setuju (misal: 'ya', 'oke', 'setuju', 'boleh', 'sampaikan', 'kirim'), set 'send_telegram': true.\n" +
		"4. Jika user bertanya nominal seperti '100k' atau '1000k', pahami bahwa 'k' berarti ribu (misal 100k = 100 ribu). Jelaskan estimasi fitur/website yang bisa didapatkan dengan anggaran tersebut secara bijak.\n" +
		"5. Pada field 'contact', ambil Nama dan Kontak (WA/Email) dari riwayat chat.\n" +
		"6. Pada field 'summary', buat rangkuman lengkap mengenai jenis projek, fitur utama, estimasi budget, serta kendala/catatan khusus user dari riwayat percakapan."
	var messages []map[string]string
	messages = append(messages, map[string]string{"role": "system", "content": systemPrompt})

	for _, msg := range history {
		role := "user"
		if msg.Role == "assistant" || msg.Role == "model" {
			role = "assistant"
		}
		messages = append(messages, map[string]string{"role": role, "content": msg.Content})
	}
	messages = append(messages, map[string]string{"role": "user", "content": prompt})

	// Paksa format JSON via response_format
	payload := map[string]interface{}{
		"model": "groq/compound",
		"messages": messages,
		"response_format": map[string]string{
			"type": "json_object",
		},
	}

	reqBody, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", "https://api.groq.com/openai/v1/chat/completions", bytes.NewBuffer(reqBody))
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Groq Error (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	var groqResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&groqResp); err != nil || len(groqResp.Choices) == 0 {
		return nil, fmt.Errorf("Gagal parsing balasan Groq API")
	}

	rawReply := groqResp.Choices[0].Message.Content

	// Print ke terminal VS Code untuk memantau JSON dari Compound
	fmt.Println("--- RAW JSON FROM COMPOUND ---")
	fmt.Println(rawReply)

	var result CompoundAIResponse
	if err := json.Unmarshal([]byte(rawReply), &result); err != nil {
		return &CompoundAIResponse{
			Reply:        rawReply,
			SendTelegram: false,
		}, nil
	}

	return &result, nil
}