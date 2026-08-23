package main

import (
	"log"
	"net/http"

	"My-Api-go/ai" // Sesuaikan dengan nama module di go.mod kamu

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

var app *gin.Engine

func init() {
	// Memuat .env saat lokal (abaikan error karena Vercel menggunakan Environment Variables di dashboard)
	_ = godotenv.Load()

	// Ubah mode Gin ke Release untuk Vercel
	gin.SetMode(gin.ReleaseMode)

	r := gin.New()
	r.Use(gin.Recovery())

	// --- SETUP CORS DINAMIS ---
	r.Use(func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")

		if origin == "https://putra-nur-rohman-high-tech-portfoli.vercel.app" || origin == "http://localhost:5173" {
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
		} else {
			c.Writer.Header().Set("Access-Control-Allow-Origin", "https://putra-nur-rohman-high-tech-portfoli.vercel.app")
		}

		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

	// Endpoint utama untuk menerima chat dari frontend React
	r.POST("/api/chat/asisten", ai.AskGroqCompoundWithSkill())
	r.POST("/api/chat/asisten/g", ai.AskGemini())
	r.POST("/api/chat/asisten/compound-skill", ai.AskGroqCompoundWithSkill())

	app = r
}

// Handler adalah Entry Point utama yang dipanggil oleh Vercel
func Handler(w http.ResponseWriter, r *http.Request) {
	app.ServeHTTP(w, r)
}

// main dipanggil saat kamu jalankan `go run main.go` atau `air` secara lokal
func main() {
	log.Println("Server lokal berjalan di port http://localhost:8080")
	if err := app.Run(":8080"); err != nil {
		log.Fatalf("Gagal menjalankan server: %v", err)
	}
}