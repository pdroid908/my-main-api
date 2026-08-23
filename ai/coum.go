package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type ImageRequest struct {
	Topic string `json:"topic"`
}

type Scene struct {
	Scene       int    `json:"scene"`
	ImagePrompt string `json:"image_prompt"`
}

type AISceneResponse struct {
	Scenes []Scene `json:"scenes"`
}

var httpClient = &http.Client{
	Timeout: 45 * time.Second,
}

func GenerateVideoPipeline() gin.HandlerFunc {
	return func(c *gin.Context) {
		var input ImageRequest
		if err := c.ShouldBindJSON(&input); err != nil || strings.TrimSpace(input.Topic) == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Topik tidak boleh kosong"})
			return
		}

		fmt.Printf("[1/2] Membuat Prompt Varian Karakter via Gemini AI: %s\n", input.Topic)
		scenes, err := generateScenesFromGemini(input.Topic)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal dari Gemini AI: " + err.Error()})
			return
		}

		outputDir := "output_images"
		os.MkdirAll(outputDir, os.ModePerm)

		var savedFiles []string

		fmt.Println("[2/2] Mengunduh Batch Gambar Kualitas Tinggi dari Pollinations AI...")
		for i, scene := range scenes {
			if i > 0 {
				time.Sleep(2 * time.Second) // Delay hindari Rate Limit
			}

			filePath := fmt.Sprintf("%s/image_%d.jpg", outputDir, i+1)
			fmt.Printf("📸 [%d/%d] Unduh: '%s'...\n", i+1, len(scenes), scene.ImagePrompt)

			err := fetchAIImageWithRetry(scene.ImagePrompt, filePath, i, 3)
			if err != nil {
				fmt.Printf("⚠️ Gagal unduh scene %d: %v\n", i+1, err)
				continue
			}

			savedFiles = append(savedFiles, filePath)
		}

		if len(savedFiles) == 0 {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Gagal mengunduh gambar."})
			return
		}

		fmt.Printf("🎉 Berhasil Mengunduh %d Gambar ke Folder '%s'!\n", len(savedFiles), outputDir)
		c.JSON(http.StatusOK, gin.H{
			"message":       "Gambar berhasil dibuat!",
			"total_images":  len(savedFiles),
			"output_folder": outputDir,
			"files":         savedFiles,
		})
	}
}

func generateScenesFromGemini(topic string) ([]Scene, error) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("GEMINI_API_KEY belum diset di .env")
	}

	endpoint := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/gemini-3.6-flash:generateContent?key=%s", apiKey)

	systemInstruction := "Kamu adalah Art Director untuk AMV TikTok/Shorts. Pecah permintaan user menjadi 8 prompt adegan/pose karakter yang SANGAT DETAIL dan VARIATIF dalam bahasa Inggris (tiap prompt max 15 kata).\n" +
		"Sertakan gaya lighting, sudut pandang, ekspresi wajah, dan latar belakang estetis.\n" +
		`Wajib output JSON murni: {"scenes": [{"scene": 1, "image_prompt": "Rem ReZero blue hair dramatic lighting close up gaze anime style"}]}`

	payload := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": []map[string]string{
					{"text": systemInstruction + "\nTopik User: " + topic},
				},
			},
		},
		"generationConfig": map[string]interface{}{
			"response_mime_type": "application/json",
		},
	}

	reqBody, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", endpoint, bytes.NewBuffer(reqBody))
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("gemini API error %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var geminiResp struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&geminiResp); err != nil {
		return nil, err
	}

	if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("respons kosong dari Gemini")
	}

	jsonText := geminiResp.Candidates[0].Content.Parts[0].Text

	var aiData AISceneResponse
	if err := json.Unmarshal([]byte(jsonText), &aiData); err != nil {
		return nil, fmt.Errorf("gagal parsing JSON dari Gemini: %v", err)
	}

	return aiData.Scenes, nil
}

func fetchAIImageWithRetry(prompt, outputPath string, seedIndex int, maxRetries int) error {
	cleanPrompt := url.QueryEscape(prompt)
	// Menggunakan model 'flux' dengan resolusi HP (480x854) untuk kualitas anime maksimal
	imageURL := fmt.Sprintf("https://image.pollinations.ai/prompt/%s?width=480&height=854&seed=%d&nologo=true&model=flux", cleanPrompt, 100+seedIndex*50)

	for attempt := 1; attempt <= maxRetries; attempt++ {
		req, _ := http.NewRequest("GET", imageURL, nil)
		req.Header.Set("User-Agent", "Mozilla/5.0")

		resp, err := httpClient.Do(req)
		if err == nil && resp.StatusCode == 200 {
			contentType := resp.Header.Get("Content-Type")
			if strings.Contains(contentType, "image") {
				out, err := os.Create(outputPath)
				if err != nil {
					resp.Body.Close()
					return err
				}
				_, err = io.Copy(out, resp.Body)
				resp.Body.Close()
				out.Close()
				return err
			}
		}

		if resp != nil {
			resp.Body.Close()
		}

		fmt.Printf("🔄 Retry (%d/%d) scene %d...\n", attempt, maxRetries, seedIndex+1)
		time.Sleep(2 * time.Second)
	}

	return fmt.Errorf("gagal setelah %d percobaan", maxRetries)
}