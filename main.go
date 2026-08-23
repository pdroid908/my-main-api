package main

// import (
// 	"log"

// 	"My-Api-go/ai"
	
// 	"github.com/gin-gonic/gin"
// 	"github.com/joho/godotenv"
// )

// func main() {
// 	// Memuat file .env
// 	err := godotenv.Load()
// 	if err != nil {
// 		log.Fatal("env ga bisa di baca")
// 	}

// 	r := gin.Default()

// 	// --- SETUP CORS DINAMIS (Vercel & Localhost) ---
// 	r.Use(func(c *gin.Context) {
// 		origin := c.Request.Header.Get("Origin")
		
// 		// Izinkan jika request berasal dari Vercel atau Localhost Vite (port 5173)
// 		if origin == "https://putra-nur-rohman-high-tech-portfoli.vercel.app" || origin == "http://localhost:5173" {
// 			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
// 		} else {
// 			// Fallback aman (atau bisa diset "*" jika ingin bebas selama development)
// 			c.Writer.Header().Set("Access-Control-Allow-Origin", "http://localhost:5173")
// 		}

// 		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
// 		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
// 		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

// 		// Tangani preflight request OPTIONS dari browser
// 		if c.Request.Method == "METHOD_OPTIONS" || c.Request.Method == "OPTIONS" {
// 			c.AbortWithStatus(204)
// 			return
// 		}

// 		c.Next()
// 	})
// 	// ---------------------------------------------

// 	// Endpoint utama untuk menerima chat dari frontend React
// 	r.POST("/api/chat/asisten", ai.AskAI())
// 	// Jalankan server di port 8080
// 	log.Println("Server Go berjalan di port 8080...")
// 	r.Run(":8080")
// }