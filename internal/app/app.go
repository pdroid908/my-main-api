package app

import (
	"My-Api-go/internal/ai"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func New() *gin.Engine {
	_ = godotenv.Load()

	gin.SetMode(gin.ReleaseMode)

	r := gin.New()
	r.Use(gin.Recovery())

	SetupCORS(r)
	SetupRoutes(r)

	return r
}

func SetupCORS(r *gin.Engine) {
	r.Use(func(ctx *gin.Context) {
		origin := ctx.Request.Header.Get("Origin")

		if origin == "https://putra-nur-rohman-high-tech-portfoli.vercel.app" ||
			origin == "http://localhost:5173" {
			ctx.Writer.Header().Set("Access-Control-Allow-Origin", origin)
		}

		ctx.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		ctx.Writer.Header().Set("Access-Control-Allow-Headers",
			"Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		ctx.Writer.Header().Set("Access-Control-Allow-Methods",
			"POST, OPTIONS, GET, PUT, DELETE")

		if ctx.Request.Method == "OPTIONS"{
			ctx.AbortWithStatus(204)
			return

		}
		ctx.Next()

	})
}

func SetupRoutes(r *gin.Engine){
	//===== penting ai chat di portofolio ======//
	r.POST("/api/chat/asisten", ai.AskAI())
	

}
