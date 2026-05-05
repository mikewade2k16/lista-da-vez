package feedback

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	maxFeedbackImageBytes         = 1 * 1024 * 1024
	maxFeedbackMultipartMemory    = maxFeedbackImageBytes + 256*1024
	feedbackImageRetention        = 7 * 24 * time.Hour
	feedbackAttachmentCleanupBatch = 100
)

type StoredImage struct {
	Path        string
	ContentType string
	SizeBytes   int
}

type ImageStorage interface {
	Save(ctx context.Context, ownerID string, fileName string, contentType string, content []byte) (*StoredImage, error)
	Delete(path string) error
}

type DiskImageStorage struct {
	rootDir string
}

func NewDiskImageStorage(rootDir string) *DiskImageStorage {
	return &DiskImageStorage{rootDir: strings.TrimSpace(rootDir)}
}

func (storage *DiskImageStorage) Save(_ context.Context, ownerID string, fileName string, contentType string, content []byte) (*StoredImage, error) {
	if len(content) == 0 || len(content) > maxFeedbackImageBytes {
		return nil, ErrInvalidImage
	}

	normalizedContentType := detectFeedbackImageContentType(content, contentType)
	extension := feedbackImageExtension(normalizedContentType, fileName)
	if extension == "" {
		return nil, ErrInvalidImage
	}

	rootDir := strings.TrimSpace(storage.rootDir)
	if rootDir == "" {
		return nil, ErrInvalidImage
	}

	feedbackDir := filepath.Join(rootDir, "feedback")
	if err := os.MkdirAll(feedbackDir, 0o755); err != nil {
		return nil, err
	}

	baseName := strings.TrimSpace(ownerID)
	if baseName == "" {
		baseName = "feedback"
	}

	imageFileName := fmt.Sprintf("%s-%s%s", sanitizeFileSegment(baseName), randomImageSuffix(), extension)
	imageFilePath := filepath.Join(feedbackDir, imageFileName)
	if err := os.WriteFile(imageFilePath, content, 0o644); err != nil {
		return nil, err
	}

	return &StoredImage{
		Path:        "/uploads/feedback/" + imageFileName,
		ContentType: normalizedContentType,
		SizeBytes:   len(content),
	}, nil
}

func (storage *DiskImageStorage) Delete(path string) error {
	normalizedPath := strings.TrimSpace(path)
	if !strings.HasPrefix(normalizedPath, "/uploads/") {
		return nil
	}

	relativePath := strings.TrimPrefix(normalizedPath, "/uploads/")
	if relativePath == "" {
		return nil
	}

	absolutePath := filepath.Join(strings.TrimSpace(storage.rootDir), filepath.FromSlash(relativePath))
	err := os.Remove(absolutePath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	return nil
}

func detectFeedbackImageContentType(content []byte, fallback string) string {
	if len(content) > 0 {
		sniffLen := len(content)
		if sniffLen > 512 {
			sniffLen = 512
		}
		detected := strings.ToLower(strings.TrimSpace(http.DetectContentType(content[:sniffLen])))
		switch detected {
		case "image/jpeg", "image/png", "image/webp":
			return detected
		}
	}

	switch strings.ToLower(strings.TrimSpace(fallback)) {
	case "image/jpeg", "image/jpg":
		return "image/jpeg"
	case "image/png":
		return "image/png"
	case "image/webp":
		return "image/webp"
	default:
		return ""
	}
}

func feedbackImageExtension(contentType string, fileName string) string {
	switch strings.ToLower(strings.TrimSpace(contentType)) {
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	}

	switch strings.ToLower(filepath.Ext(strings.TrimSpace(fileName))) {
	case ".jpg", ".jpeg":
		return ".jpg"
	case ".png":
		return ".png"
	case ".webp":
		return ".webp"
	default:
		return ""
	}
}

func sanitizeFileSegment(value string) string {
	replacer := strings.NewReplacer("/", "-", "\\", "-", " ", "-", ":", "-")
	clean := strings.Trim(strings.ToLower(replacer.Replace(strings.TrimSpace(value))), "-")
	if clean == "" {
		return "feedback"
	}
	return clean
}

func randomImageSuffix() string {
	bytes := make([]byte, 6)
	if _, err := rand.Read(bytes); err != nil {
		return "image"
	}

	return hex.EncodeToString(bytes)
}