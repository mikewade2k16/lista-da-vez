package tasks

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const (
	maxTaskVideoBytes           = 100 * 1024 * 1024
	maxTaskVideoMultipartMemory = 8 * 1024 * 1024
)

type DiskVideoStorage struct {
	rootDir string
}

func NewDiskVideoStorage(rootDir string) *DiskVideoStorage {
	return &DiskVideoStorage{rootDir: strings.TrimSpace(rootDir)}
}

func (storage *DiskVideoStorage) Save(_ context.Context, _, _, taskID, _, fileName, contentType string, content []byte) (*StoredTaskVideo, error) {
	if len(content) == 0 || len(content) > maxTaskVideoBytes {
		return nil, ErrInvalidVideo
	}

	normalizedContentType := detectTaskVideoContentType(content, contentType)
	extension := taskVideoExtension(normalizedContentType, fileName)
	if normalizedContentType == "" || extension == "" {
		return nil, ErrInvalidVideo
	}

	rootDir := strings.TrimSpace(storage.rootDir)
	if rootDir == "" {
		return nil, ErrInvalidVideo
	}

	videosDir := filepath.Join(rootDir, "tasks")
	if err := os.MkdirAll(videosDir, 0o750); err != nil { //nolint:gosec // root vem da config; nao do request
		return nil, err
	}

	baseName := sanitizeTaskVideoSegment(taskID)
	videoID := fmt.Sprintf("%s-%s", baseName, randomTaskVideoSuffix())
	videoFileName := videoID + extension
	videoFilePath := filepath.Join(videosDir, videoFileName)
	if err := os.WriteFile(videoFilePath, content, 0o600); err != nil { //nolint:gosec // nome usa task sanitizada + sufixo aleatorio
		return nil, err
	}

	return &StoredTaskVideo{
		ID:          videoID,
		Path:        "/uploads/tasks/" + videoFileName,
		ContentType: normalizedContentType,
		SizeBytes:   len(content),
	}, nil
}

func detectTaskVideoContentType(content []byte, fallback string) string {
	if len(content) > 0 {
		sniffLen := len(content)
		if sniffLen > 512 {
			sniffLen = 512
		}
		detected := strings.ToLower(strings.TrimSpace(http.DetectContentType(content[:sniffLen])))
		if strings.HasPrefix(detected, "video/") {
			return detected
		}
	}

	fallbackValue := strings.ToLower(strings.TrimSpace(fallback))
	if strings.HasPrefix(fallbackValue, "video/") {
		return fallbackValue
	}
	return ""
}

func taskVideoExtension(contentType string, fileName string) string {
	switch strings.ToLower(strings.TrimSpace(contentType)) {
	case "video/mp4":
		return ".mp4"
	case "video/quicktime":
		return ".mov"
	case "video/webm":
		return ".webm"
	case "video/ogg":
		return ".ogv"
	case "video/x-msvideo":
		return ".avi"
	case "video/x-m4v", "video/mp4v-es", "video/m4v":
		return ".m4v"
	}

	switch strings.ToLower(filepath.Ext(strings.TrimSpace(fileName))) {
	case ".mp4":
		return ".mp4"
	case ".mov":
		return ".mov"
	case ".webm":
		return ".webm"
	case ".ogv", ".ogg":
		return ".ogv"
	case ".avi":
		return ".avi"
	case ".m4v":
		return ".m4v"
	default:
		return ""
	}
}

func sanitizeTaskVideoSegment(value string) string {
	replacer := strings.NewReplacer("/", "-", "\\", "-", " ", "-", ":", "-")
	clean := strings.Trim(strings.ToLower(replacer.Replace(strings.TrimSpace(value))), "-")
	if clean == "" {
		return "task"
	}
	return clean
}

func randomTaskVideoSuffix() string {
	bytes := make([]byte, 6)
	if _, err := rand.Read(bytes); err != nil {
		return "video"
	}
	return hex.EncodeToString(bytes)
}
