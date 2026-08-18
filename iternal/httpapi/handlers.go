package httpapi

import (
	"encoding/json"
	"errors"
	common_errors "filetransfer/iternal/errors"
	"filetransfer/iternal/fileservice"
	"fmt"
	"log"
	"net/http"
	"time"
)

type API struct {
	files *fileservice.Service
}

type UploadedFile struct {
	Link string `json:"link"`
}

const (
	Twenty    = 20 * time.Second
	OneDay    = 24 * time.Hour
	ThreeDays = 72 * time.Hour
	Week      = 7 * 24 * time.Hour
)

type ExpiryOptions struct {
	Key      string        `json:"key"`
	Label    string        `json:"label"`
	Duration time.Duration `json:"-"`
}

var expiryOptions = []ExpiryOptions{
	{Key: "20s", Label: "20 секунд", Duration: Twenty},
	{Key: "1d", Label: "1 день", Duration: OneDay},
	{Key: "3d", Label: "3 дня", Duration: ThreeDays},
	{Key: "7d", Label: "7 дней", Duration: Week},
}

func parseExpiry(r *http.Request) (time.Duration, error) {
	value := r.FormValue("expires")
	if value == "" {
		return OneDay, nil
	}

	for _, opt := range expiryOptions {
		if opt.Key == value {
			return opt.Duration, nil
		}
	}

	return 0, fmt.Errorf("invalid expires value: %q", value)
}

func New(files *fileservice.Service) *API {
	return &API{files: files}
}

// OptionsList godoc
// @Summary Получить доступные варианты срока хранения
// @Tags options
// @Produce json
// @Success 200 {array} ExpiryOptions
// @Router /options [get]
func (a *API) OptionsList(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(expiryOptions)
}

// UploadFile godoc
// @Summary Загрузить файлы
// @Description Загружает один или несколько файлов и возвращает ссылку на батч
// @Tags files
// @Accept multipart/form-data
// @Produce json
// @Param myFile formData file true "Файлы для загрузки"
// @Param expires formData string false "Срок хранения (20s, 1d, 3d, 7d)"
// @Success 200 {object} map[string]string
// @Failure 400 {string} string "bad request"
// @Router /upload [post]
func (a *API) UploadFile(w http.ResponseWriter, r *http.Request) {
	r.ParseMultipartForm(10 << 20)

	expiry, err := parseExpiry(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	fileHeaders := r.MultipartForm.File["myFile"]
	if len(fileHeaders) == 0 {
		http.Error(w, "no files uploaded", http.StatusBadRequest)
		return
	}

	var fileIDs []string
	for _, fh := range fileHeaders {
		file, err := fh.Open()
		if err != nil {
			http.Error(w, "opening file", http.StatusInternalServerError)
			return
		}

		id, err := a.files.SaveFile(fh, expiry)
		file.Close()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		fileIDs = append(fileIDs, id)
	}

	batchID := a.files.SaveBatch(fileIDs, expiry)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"link": "http://localhost:8080/batch/" + batchID,
	})
}

type DownloadLink struct {
	Name string `json:"name"`
	Link string `json:"link"`
}

// BatchPage godoc
// @Summary Получить список файлов батча
// @Tags batch
// @Produce json
// @Param id path string true "ID батча"
// @Success 200 {array} DownloadLink
// @Failure 404 {string} string "not found"
// @Router /batch/{id} [get]
func (a *API) BatchPage(w http.ResponseWriter, r *http.Request) {
	batchID := r.PathValue("id")

	files, err := a.files.GetBatchFiles(batchID)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	var links []DownloadLink
	for _, f := range files {
		links = append(links, DownloadLink{
			Name: f.OriginalName,
			Link: "http://localhost:8080/files/" + f.StoreName,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(links)
}

// DownloadFile godoc
// @Summary Скачать файл
// @Tags files
// @Produce octet-stream
// @Param id path string true "ID файла"
// @Success 200 {file} file
// @Failure 404 {string} string "not found"
// @Failure 410 {string} string "expired"
// @Router /files/{id} [get]
func (a *API) DownloadFile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	meta, path, err := a.files.GetFile(id)
	if err != nil {
		if errors.Is(err, common_errors.ErrNotFound) {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error(w, "gone", http.StatusGone)
		return
	}

	w.Header().Set("Content-Disposition", "attachment; filename=\""+meta.OriginalName+"\"")
	http.ServeFile(w, r, path)
}

// DownloadBatchZip godoc
// @Summary Скачать все файлы батча архивом
// @Description Архивирует все ещё не истёкшие файлы батча в zip и стримит его в ответ
// @Tags batch
// @Produce application/zip
// @Param id path string true "ID батча"
// @Success 200 {file} file
// @Failure 404 {string} string "batch not found"
// @Router /batch/{id}/zip [get]
func (a *API) DownloadBatchZip(w http.ResponseWriter, r *http.Request) {
	batchID := r.PathValue("id")

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", "attachment; filename=\"batch-"+batchID+".zip\"")

	if err := a.files.StreamBatchZip(w, batchID); err != nil {
		log.Printf("error streaming zip for batch %s: %v", batchID, err)
	}
}
