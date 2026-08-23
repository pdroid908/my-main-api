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

		// Batasi maksimal karakter input agar efisien
		runes := []rune(cleanPrompt)
		if len(runes) > MaxChars {
			cleanPrompt = string(runes[:MaxChars])
		}

		var limitedHistory []GroqMessage
		if len(input.Messages) > MaxHistory {
			limitedHistory = input.Messages[len(input.Messages)-MaxHistory:]
		} else {
			limitedHistory = input.Messages
		}

		parsedResp, err := callGroqCompoundJSON(cleanPrompt, limitedHistory)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		if parsedResp.SendTelegram {
			SendTelegramTool(parsedResp.Summary, parsedResp.Contact)
		}

		c.JSON(http.StatusOK, gin.H{"reply": parsedResp.Reply})
	}
}

func callGroqCompoundJSON(prompt string, history []GroqMessage) (*CompoundAIResponse, error) {
	apiKey := os.Getenv("GROQ_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("GROQ_API_KEY belum diset di .env")
	}

	systemPrompt := "Kamu adalah asisten portofolio Putra Nur Rohman (Full-Stack & Backend Developer Go).\n" +
		"Jawablah dengan ramah, singkat, dan gunakan emoji.\n\n" +
		"ATURAN BANTUAN & PROSPEK:\n" +
		"1. Jika user berniat order/rekrut, minta kontak (Nomor WA/Email) mereka.\n" +
		"2. Jika user memberikan kontak, jangan langsung panggil tool `send_lead_to_telegram`. Tanya konfirmasi/validasi dulu.\n" +
		"3. Jika user SUDAH mengonfirmasi/setuju (misal: 'ya', 'oke', 'setuju'), panggil tool `send_lead_to_telegram`.\n" +
		"4. Jika user bertanya nominal seperti '100k', pahami bahwa 'k' berarti ribu."

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
						"description": "Rangkuman lengkap mengenai jenis projek, fitur utama, dan budget dari riwayat percakapan.",
					},
					"contact": map[string]string{
						"type":        "string",
						"description": "Nama, Nomor WA, Telepon, atau Email klien.",
					},
				},
				"required": []string{"summary", "contact"},
			},
		},
	}

	var messages []GroqMessage
	messages = append(messages, GroqMessage{Role: "system", Content: systemPrompt})
	messages = append(messages, history...)
	messages = append(messages, GroqMessage{Role: "user", Content: prompt})

	reqBody, _ := json.Marshal(GroqRequest{
		Model:    "groq/compound",
		Messages: messages,
		Tools:    []Tool{telegramTool},
	})

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
				Content   string     `json:"content"`
				ToolCalls []ToolCall `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&groqResp); err != nil || len(groqResp.Choices) == 0 {
		return nil, fmt.Errorf("Gagal parsing balasan Groq API")
	}

	choice := groqResp.Choices[0].Message
	replyContent := choice.Content

	if idx := strings.Index(replyContent, "</think>"); idx != -1 {
		replyContent = strings.TrimSpace(replyContent[idx+8:])
	}

	res := &CompoundAIResponse{
		Reply:        replyContent,
		SendTelegram: false,
	}

	if len(choice.ToolCalls) > 0 {
		for _, tc := range choice.ToolCalls {
			if tc.Function.Name == "send_lead_to_telegram" {
				var args LeadArgs
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err == nil {
					res.SendTelegram = true
					res.Contact = args.Contact
					res.Summary = args.Summary
				}
			}
		}

		// Cegah balasan kosong jika AI fokus memanggil tool tanpa mengisi teks balasan
		if strings.TrimSpace(res.Reply) == "" {
			res.Reply = "Terima kasih! Informasi dan kontak Anda sudah saya sampaikan langsung ke Putra. Beliau akan segera menghubungi Anda kembali! 🚀"
		}
	}

	return res, nil
}