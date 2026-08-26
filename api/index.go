package api

import (
	"My-Api-go/internal/app"
	"net/http"
)


var engine = app.New()

func Handler(w http.ResponseWriter, r *http.Request){
	engine.ServeHTTP(w,r)
}