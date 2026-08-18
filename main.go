package main

import (
	"database/sql"
	_ "filetransfer/docs"
	"filetransfer/iternal/fileservice"
	"filetransfer/iternal/httpapi"
	"filetransfer/iternal/middleware"
	"filetransfer/iternal/storage"
	"log"
	"net/http"
	"time"

	httpSwagger "github.com/swaggo/http-swagger"
)

// @title FileShare API
// @version 1.0
// @description Простой файлообменник с ограниченным сроком жизни файлов
// @host localhost:8080
// @BasePath /
func main() {
	db, err := sql.Open("sqlite", "fileshare.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	store, err := storage.New(db)
	if err != nil {
		log.Fatal(err)
	}

	batchStore, err := storage.NewBatchStore(db)
	if err != nil {
		log.Fatal(err)
	}
	service := fileservice.New(store, batchStore, "temp-images")
	api := httpapi.New(service)

	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()

		for range ticker.C {
			service.CleanupExpired()
		}
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("POST /upload", api.UploadFile)
	mux.HandleFunc("GET /files/{id}", api.DownloadFile)
	mux.HandleFunc("GET /batch/{id}", api.BatchPage)
	mux.HandleFunc("GET /batch/{id}/zip", api.DownloadBatchZip)
	mux.HandleFunc("GET /options", api.OptionsList)
	mux.Handle("/swagger/", httpSwagger.WrapHandler)

	log.Fatal(http.ListenAndServe(":8080", middleware.CorsMiddleware(mux)))
}
