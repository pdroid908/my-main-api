package ai

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

const MaxChars = 300
const MaxHistory = 10

type FunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}
type ToolCall struct {
	Function FunctionCall `json:"function"`
}

type GroqMessage struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
}

type ToolFunction struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Parameters  interface{} `json:"parameters"`
}

type Tool struct {
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

type GroqRequest struct {
	Model    string        `json:"model"`
	Messages []GroqMessage `json:"messages"`
	Tools    []Tool        `json:"tools,omitempty"`
}

type ChatPayload struct {
	Prompt   string        `json:"prompt"`
	Messages []GroqMessage `json:"messages"`
}

func AskAI() gin.HandlerFunc {
	return func(c *gin.Context) {
		var input ChatPayload
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
			return
		}

		// 1. Sanitasi input user (Maks 100 Karakter)
		cleanPrompt := strings.TrimSpace(input.Prompt)
		if cleanPrompt == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Pesan kosong"})
			return
		}
		runes := []rune(cleanPrompt)
		if len(runes) > MaxChars {
			cleanPrompt = string(runes[:MaxChars])
		}

		// 2. Batasi riwayat percakapan (Maks 10 Pesan Terakhir)
		var limitedHistory []GroqMessage
		if len(input.Messages) > MaxHistory {
			limitedHistory = input.Messages[len(input.Messages)-MaxHistory:]
		} else {
			limitedHistory = input.Messages
		}

		// 3. System Prompt & Pengenalan Tool
		systemContent := "Kamu adalah asisten portofolio Putra Nur Rohman (Full-Stack & Backend Developer Go). " +
			"Jawablah dengan ramah, singkat, menggunakan emoji\n" +
			"ATURAN PROSPEK & TOOL:\n" +
			"1. Jika user berniat order/rekrut, minta kontak mereka.\n" +
			"2. Jika user memberikan kontak, jangan langsung jalankan tool tanya validasi konfirmasi ke user dulu.\n" +
			"3. Jika user SUDAH mengonfirmasi/setuju (misal: 'ya', 'oke', 'setuju'), panggil tool `send_lead_to_telegram` " +
			"4. jika user bilang 100k berarti k itu ribu, berarti user tanya uang segitu bisa dapat web/fitur apa"+
			"dengan merangkum inti dari 10 riwayat percakapan dan sertakan nomor kontaknya."

		systemPrompt := GroqMessage{Role: "system", Content: systemContent}

		var fullMessages []GroqMessage
		fullMessages = append(fullMessages, systemPrompt)
		fullMessages = append(fullMessages, limitedHistory...)
		fullMessages = append(fullMessages, GroqMessage{Role: "user", Content: cleanPrompt})

		// 4. Eksekusi 1 Panggilan AI
		reply, toolArguments, err := callQwenWithTools(fullMessages)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// 5. Jika Qwen memutuskan untuk memanggil Tool, jalankan skill.go
		if toolArguments != nil {
			SendTelegramTool(toolArguments.Summary, toolArguments.Contact)

			// Cegah string kosong agar tidak memicu fallback frontend
			if strings.TrimSpace(reply) == "" {
				reply = "Terima kasih! Informasi dan kontak Anda sudah saya sampaikan langsung ke Putra. Beliau akan segera menghubungi Anda kembali! 🚀"
			}
		}

		c.JSON(http.StatusOK, gin.H{"reply": reply})
	}
}

type LeadArgs struct {
	Summary string `json:"summary"`
	Contact string `json:"contact"`
}

func callQwenWithTools(messages []GroqMessage) (string, *LeadArgs, error) {
	apiKey := os.Getenv("GROQ_API_KEY")

	// Schema definisi Tool yang diberikan ke Qwen
	telegramTool := Tool{
		Type: "function",
		Function: ToolFunction{
			Name:        "send_lead_to_telegram",
			Description: "Mengirimkan rangkuman prospek dan nomor kontak klien ke Telegram ketika klien sudah mengonfirmasi setuju.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"summary": map[string]string{
						"type":        "string",
						"description": "Rangkuman singkat inti diskusi/kebutuhan projek dari 10 riwayat percakapan.",
					},
					"contact": map[string]string{
						"type":        "string",
						"description": "Nomor WhatsApp, Telepon, atau Email klien.",
					},
				},
				"required": []string{"summary", "contact"},
			},
		},
	}

	reqBody, _ := json.Marshal(GroqRequest{
		Model:    "qwen/qwen3.6-27b",
		Messages: messages,
		Tools:    []Tool{telegramTool},
	})

	req, _ := http.NewRequest("POST", "https://api.groq.com/openai/v1/chat/completions", bytes.NewBuffer(reqBody))
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()

	var groqResp struct {
		Choices []struct {
			Message struct {
				Content   string     `json:"content"`
				ToolCalls []ToolCall `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&groqResp); err != nil || len(groqResp.Choices) == 0 {
		return "Maaf, terjadi kesalahan pada asisten.", nil, nil
	}

	choice := groqResp.Choices[0].Message
	replyContent := choice.Content

	// Bersihkan tag <think> jika ada
	if idx := strings.Index(replyContent, "</think>"); idx != -1 {
		replyContent = strings.TrimSpace(replyContent[idx+8:])
	}

	// Cek apakah Qwen memanggil tool
	var leadData *LeadArgs
	if len(choice.ToolCalls) > 0 {
		for _, tc := range choice.ToolCalls {
			if tc.Function.Name == "send_lead_to_telegram" {
				var args LeadArgs
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err == nil {
					leadData = &args
				}
			}
		}
	}

	return replyContent, leadData, nil
}