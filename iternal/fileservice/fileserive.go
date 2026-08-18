package fileservice

import (
	"archive/zip"
	common_errors "filetransfer/iternal/errors"
	"filetransfer/iternal/id"
	"filetransfer/iternal/storage"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Service struct {
	store      *storage.Store
	batchStore *storage.BatchStore
	baseDir    string
}

func New(store *storage.Store, batchStore *storage.BatchStore, baseDir string) *Service {
	return &Service{
		store:      store,
		batchStore: batchStore,
		baseDir:    baseDir,
	}
}

const maxFileSize = 5 << 20

var allowedTypes = map[string]bool{
	"image/jpeg":      true,
	"image/png":       true,
	"application/pdf": true,
	"application/zip": true,
	"text/plain":      true,
}

func validateFileSize(fh *multipart.FileHeader) error {
	if fh.Size > maxFileSize {
		return fmt.Errorf("file %s exceeds size limit", fh.Filename)
	}
	return nil
}

func detectContentType(file multipart.File) (string, error) {
	buf := make([]byte, 512)
	n, err := file.Read(buf)
	if err != nil && err != io.EOF {
		return "", err
	}

	contentType := http.DetectContentType(buf[:n])

	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", err
	}

	return contentType, nil
}

func validateContentType(contentType string) error {
	base := strings.TrimSpace(strings.SplitN(contentType, ";", 2)[0])
	if !allowedTypes[base] {
		return fmt.Errorf("file type %s is not allowed", base)
	}
	return nil
}

func (s *Service) SaveFile(fh *multipart.FileHeader, expiry time.Duration) (string, error) {
	if err := validateFileSize(fh); err != nil {
		return "", err
	}

	file, err := fh.Open()
	if err != nil {
		return "", fmt.Errorf("opening file %w", err)
	}
	defer file.Close()

	contentType, err := detectContentType(file)
	if err != nil {
		return "", fmt.Errorf("detecting content type %w", err)
	}

	if err := validateContentType(contentType); err != nil {
		return "", err
	}

	ext := filepath.Ext(fh.Filename)
	tempFile, err := os.CreateTemp(s.baseDir, "upload-*"+ext)
	if err != nil {
		return "", fmt.Errorf("creating temp file: %w", err)
	}
	defer tempFile.Close()

	_, err = io.Copy(tempFile, file)
	if err != nil {
		return "", fmt.Errorf("writing to temp file: %w", err)
	}

	id := filepath.Base(tempFile.Name())
	now := time.Now()

	s.store.Set(id, storage.FileMeta{
		OriginalName: fh.Filename,
		StoreName:    id,
		UploadedAt:   now,
		ExpiresAt:    now.Add(expiry),
	})

	return id, nil
}

func (s *Service) GetFile(id string) (storage.FileMeta, string, error) {
	meta, ok := s.store.Get(id)
	if !ok {
		return storage.FileMeta{}, "", common_errors.ErrNotFound
	}

	if time.Now().After(meta.ExpiresAt) {
		return storage.FileMeta{}, "", fmt.Errorf("expired")
	}

	return meta, filepath.Join(s.baseDir, id), nil
}

func (s *Service) CleanupExpired() {
	now := time.Now()

	files, err := s.store.All()
	if err != nil {
		log.Printf("cleanup: error fetching files: %v", err)
		return
	}

	for id, meta := range files {
		if now.After(meta.ExpiresAt) {
			path := filepath.Join(s.baseDir, id)
			if err := os.Remove(path); err != nil {
				log.Printf("error removing file %s: %v", path, err)
				continue
			}
			if err := s.store.Delete(id); err != nil {
				log.Printf("error deleting from store %s: %v", id, err)
			}
		}
	}
}

func (s *Service) SaveBatch(fileIds []string, expiry time.Duration) string {
	batchId := id.GenerateID()
	now := time.Now()

	s.batchStore.Set(batchId, storage.Batch{
		ID:        batchId,
		FileIDs:   fileIds,
		CreatedAt: now,
		ExpiresAt: now.Add(expiry),
	})

	return batchId
}

func (s *Service) GetBatchFiles(batchId string) ([]storage.FileMeta, error) {
	batch, ok := s.batchStore.Get(batchId)
	if !ok {
		return nil, common_errors.ErrNotFound
	}

	var files []storage.FileMeta
	for _, id := range batch.FileIDs {
		if meta, ok := s.store.Get(id); ok {
			files = append(files, meta)
		}
	}

	return files, nil
}

func (s *Service) StreamBatchZip(w io.Writer, batchID string) error {
	batch, ok := s.batchStore.Get(batchID)
	if !ok {
		return common_errors.ErrNotFound
	}

	zw := zip.NewWriter(w)
	defer zw.Close()

	for _, id := range batch.FileIDs {
		meta, ok := s.store.Get(id)
		if !ok {
			continue
		}
		if time.Now().After(meta.ExpiresAt) {
			continue
		}

		path := filepath.Join(s.baseDir, meta.StoreName)
		src, err := os.Open(path)
		if err != nil {
			continue
		}

		dst, err := zw.Create(meta.OriginalName)
		if err != nil {
			src.Close()
			return err
		}

		if _, err := io.Copy(dst, src); err != nil {
			src.Close()
			return err
		}
		src.Close()
	}

	return nil
}
