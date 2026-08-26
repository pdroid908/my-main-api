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

type CodeRequest struct {
	FileName    string                   `json:"file_name"`
	FileContent string                   `json:"file_content"`
	Prompt      string                   `json:"prompt"`
	Memory      []map[string]string      `json:"memory"` 
}

func CompoundCodingAssistant() gin.HandlerFunc {
	return func(c *gin.Context) {
		var input CodeRequest
		if err := c.ShouldBindJSON(&input); err != nil || (strings.TrimSpace(input.Prompt) == "" && strings.TrimSpace(input.FileContent) == "") {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Prompt atau file content tidak boleh kosong"})
			return
		}

		fmt.Println("🤖 Mengirim konteks file & prompt ke Groq Compound...")

		reply, updatedMemory, err := callGroqCompoundWithMemory(input.FileName, input.FileContent, input.Prompt, input.Memory)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal dari Groq Compound: " + err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"status": "sukses",
			"reply":  reply,
			"memory": updatedMemory, // Mengembalikan memory terbaru ke frontend
		})
	}
}

func callGroqCompoundWithMemory(fileName string, fileContent string, prompt string, memory []map[string]string) (string, []map[string]string, error) {
	apiKey := os.Getenv("GROQ_API_KEY")
	if apiKey == "" {
		return "", nil, fmt.Errorf("GROQ_API_KEY belum diset di .env")
	}

	endpoint := "https://api.groq.com/openai/v1/chat/completions"

	// System prompt pintar untuk menyaring bagian penting file
	systemContent := "Kamu adalah Senior Software Engineer dan Arsitek Kode Go. cukup beri kode kelas production tanpa penjelassan " +
		"Tugasmu adalah mengingat struktur file (seperti main.go, redis.go, dll) yang dikirim user. " +
		"Fokus pada: package/import, inisruktur data/struct, koneksi database/redis, dan logika utama. Abaikan kode boilerplate yang tidak penting."

	// Susun pesan user baru berdasarkan file dan instruksinya
	var userMessageContent string
	if strings.TrimSpace(fileContent) != "" {
		userMessageContent = fmt.Sprintf("File: %s\nIsi Kode Penting:\n```go\n%s\n```\nInstruksi: %s", fileName, fileContent, prompt)
	} else {
		userMessageContent = prompt
	}

	// Gabungkan system prompt, history lama, dan pesan baru
	var messages []map[string]string
	messages = append(messages, map[string]string{"role": "system", "content": systemContent})
	
	// Masukkan history sebelumnya agar AI tetap ingat file-file yang sudah di-input
	messages = append(messages, memory...)
	
	// Masukkan pesan terbaru
	messages = append(messages, map[string]string{"role": "user", "content": userMessageContent})

	payload := map[string]interface{}{
		"model":       "groq/compound",
		"messages":    messages,
		"temperature": 0.2,
	}

	reqBody, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(reqBody))
	if err != nil {
		return "", nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", nil, fmt.Errorf("groq API error %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var groqResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&groqResp); err != nil {
		return "", nil, err
	}

	if len(groqResp.Choices) == 0 {
		return "", nil, fmt.Errorf("respons kosong dari model Compound")
	}

	replyContent := groqResp.Choices[0].Message.Content
	if idx := strings.Index(replyContent, "</think>"); idx != -1 {
		replyContent = strings.TrimSpace(replyContent[idx+8:])
	}

	// Update memori lokal untuk dikembalikan
	updatedMemory := append(memory, 
		map[string]string{"role": "user", "content": userMessageContent},
		map[string]string{"role": "assistant", "content": replyContent},
	)

	return replyContent, updatedMemory, nil
}