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

func AskGemini() gin.HandlerFunc {
	return func(c *gin.Context) {
		var input ChatPayload
		if err := c.ShouldBindJSON(&input); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON"})
			return
		}

		cleanPrompt := strings.TrimSpace(input.Prompt)
		if cleanPrompt == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Pesan kosong"})
			return
		}
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

		reply, toolArguments, err := callGeminiWithTools(cleanPrompt, limitedHistory)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		if toolArguments != nil {
			SendTelegramTool(toolArguments.Summary, toolArguments.Contact)
			if strings.TrimSpace(reply) == "" {
				reply = "Terima kasih! Informasi dan kontak Anda sudah saya sampaikan langsung ke Putra. Beliau akan segera menghubungi Anda kembali! 🚀"
			}
		}

		c.JSON(http.StatusOK, gin.H{"reply": reply})
	}
}

// Struct Khusus Gemini REST API
type GeminiPart struct {
	Text         string              `json:"text,omitempty"`
	FunctionCall *GeminiFunctionCall `json:"functionCall,omitempty"`
}

type GeminiFunctionCall struct {
	Name string                 `json:"name"`
	Args map[string]interface{} `json:"args"`
}

type GeminiContent struct {
	Role  string       `json:"role"`
	Parts []GeminiPart `json:"parts"`
}

type GeminiFunctionDeclaration struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Parameters  interface{} `json:"parameters"`
}

type GeminiTool struct {
	FunctionDeclarations []GeminiFunctionDeclaration `json:"functionDeclarations"`
}

type GeminiRequest struct {
	SystemInstruction *struct {
		Parts []GeminiPart `json:"parts"`
	} `json:"systemInstruction,omitempty"`
	Contents []GeminiContent `json:"contents"`
	Tools    []GeminiTool    `json:"tools,omitempty"`
}

func callGeminiWithTools(prompt string, history []GroqMessage) (string, *LeadArgs, error) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		return "", nil, fmt.Errorf("GEMINI_API_KEY belum diset di environment variable")
	}

	systemInstruction := "Kamu adalah asisten portofolio Putra Nur Rohman (Full-Stack & Backend Developer Go). " +
		"Jawablah dengan ramah, singkat, menggunakan emoji.\n" +
		"ATURAN PROSPEK & TOOL:\n" +
		"1. Jika user berniat order/rekrut, minta kontak mereka.\n" +
		"2. Jika user memberikan kontak, jangan langsung jalankan tool, tanya validasi/konfirmasi ke user dulu.\n" +
		"3. Jika user SUDAH mengonfirmasi/setuju (misal: 'ya', 'oke', 'setuju'), panggil tool send_lead_to_telegram."

	// Definisi Tool Versi Gemini
	telegramTool := GeminiTool{
		FunctionDeclarations: []GeminiFunctionDeclaration{
			{
				Name:        "send_lead_to_telegram",
				Description: "Mengirimkan rangkuman prospek dan nomor kontak klien ke Telegram ketika klien sudah mengonfirmasi setuju.",
				Parameters: map[string]interface{}{
					"type": "OBJECT",
					"properties": map[string]interface{}{
						"summary": map[string]string{
							"type":        "STRING",
							"description": "Rangkuman singkat inti diskusi/kebutuhan projek.",
						},
						"contact": map[string]string{
							"type":        "STRING",
							"description": "Nomor WhatsApp, Telepon, atau Email klien.",
						},
					},
					"required": []string{"summary", "contact"},
				},
			},
		},
	}

	// Konversi History ke Format Gemini
	var geminiContents []GeminiContent
	for _, msg := range history {
		role := "user"
		if msg.Role == "assistant" || msg.Role == "model" {
			role = "model"
		}
		geminiContents = append(geminiContents, GeminiContent{
			Role:  role,
			Parts: []GeminiPart{{Text: msg.Content}},
		})
	}

	// Masukkan prompt user terbaru
	geminiContents = append(geminiContents, GeminiContent{
		Role:  "user",
		Parts: []GeminiPart{{Text: prompt}},
	})

	reqBodyObj := GeminiRequest{
		SystemInstruction: &struct {
			Parts []GeminiPart `json:"parts"`
		}{
			Parts: []GeminiPart{{Text: systemInstruction}},
		},
		Contents: geminiContents,
		Tools:    []GeminiTool{telegramTool},
	}

	reqBody, _ := json.Marshal(reqBodyObj)

	// Memanggil model Gemini 2.5 Flash
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/gemini-3.6-flash:generateContent?key=%s", apiKey)
	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(reqBody))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()

	// Jika ada error HTTP (misal key salah atau bad request), kembalikan detail errornya
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", nil, fmt.Errorf("Gemini API error (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	var geminiResp struct {
		Candidates []struct {
			Content struct {
				Parts []GeminiPart `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&geminiResp); err != nil || len(geminiResp.Candidates) == 0 {
		return "Maaf, terjadi kesalahan pada pemrosesan balasan Gemini.", nil, nil
	}

	parts := geminiResp.Candidates[0].Content.Parts
	var replyContent string
	var leadData *LeadArgs

	for _, part := range parts {
		if part.Text != "" {
			replyContent += part.Text
		}
		if part.FunctionCall != nil && part.FunctionCall.Name == "send_lead_to_telegram" {
			args := part.FunctionCall.Args
			leadData = &LeadArgs{
				Summary: fmt.Sprintf("%v", args["summary"]),
				Contact: fmt.Sprintf("%v", args["contact"]),
			}
		}
	}

	return replyContent, leadData, nil
}