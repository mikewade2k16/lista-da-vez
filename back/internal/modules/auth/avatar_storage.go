package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	maxAvatarBytes = 2 * 1024 * 1024
)

type DiskAvatarStorage struct {
	rootDir string
}

func NewDiskAvatarStorage(rootDir string) *DiskAvatarStorage {
	return &DiskAvatarStorage{
		rootDir: strings.TrimSpace(rootDir),
	}
}

func (storage *DiskAvatarStorage) Save(_ context.Context, userID string, fileName string, contentType string, content []byte, previousPath string) (string, error) {
	if len(content) == 0 || len(content) > maxAvatarBytes {
		return "", ErrInvalidAvatar
	}

	extension := avatarExtension(contentType, fileName)
	if extension == "" {
		return "", ErrInvalidAvatar
	}

	rootDir := strings.TrimSpace(storage.rootDir)
	if rootDir == "" {
		return "", ErrInvalidAvatar
	}

	avatarsDir := filepath.Join(rootDir, "avatars")
	if err := os.MkdirAll(avatarsDir, 0o755); err != nil {
		return "", err
	}

	avatarFileName := fmt.Sprintf("%s-%s%s", strings.TrimSpace(userID), randomSuffix(), extension)
	avatarFilePath := filepath.Join(avatarsDir, avatarFileName)

	if err := os.WriteFile(avatarFilePath, content, 0o644); err != nil {
		return "", err
	}

	storage.deletePrevious(previousPath)
	return "/uploads/avatars/" + avatarFileName, nil
}

func (storage *DiskAvatarStorage) deletePrevious(previousPath string) {
	normalizedPath := strings.TrimSpace(previousPath)
	if !strings.HasPrefix(normalizedPath, "/uploads/") {
		return
	}

	relativePath := strings.TrimPrefix(normalizedPath, "/uploads/")
	if relativePath == "" {
		return
	}

	absolutePath := filepath.Join(storage.rootDir, filepath.FromSlash(relativePath))
	_ = os.Remove(absolutePath)
}

func avatarExtension(contentType string, fileName string) string {
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

func randomSuffix() string {
	bytes := make([]byte, 6)
	if _, err := rand.Read(bytes); err != nil {
		return "avatar"
	}

	return hex.EncodeToString(bytes)
}
