package transcriptions

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

const defaultAudioDirectory = "data/media/attendance-audio"

type DiskAudioStorage struct {
	rootDir string
}

func NewDiskAudioStorage(rootDir string) *DiskAudioStorage {
	root := strings.TrimSpace(rootDir)
	if root == "" {
		root = defaultAudioDirectory
	}
	return &DiskAudioStorage{rootDir: root}
}

func AudioDirectoryFromEnv() string {
	if value := strings.TrimSpace(os.Getenv("ATTENDANCE_AUDIO_DIR")); value != "" {
		return value
	}
	return defaultAudioDirectory
}

func normalizeAudioMime(value string) string {
	mimeType := strings.ToLower(strings.TrimSpace(value))
	if separator := strings.IndexByte(mimeType, ';'); separator >= 0 {
		mimeType = strings.TrimSpace(mimeType[:separator])
	}
	return mimeType
}

func audioExtension(mimeType string) (string, bool) {
	switch normalizeAudioMime(mimeType) {
	case "audio/webm":
		return ".webm", true
	case "audio/mp4":
		return ".m4a", true
	case "audio/ogg":
		return ".ogg", true
	case "audio/mpeg":
		return ".mp3", true
	case "audio/wav", "audio/x-wav":
		return ".wav", true
	default:
		return "", false
	}
}

func sanitizePathSegment(value string) string {
	var builder strings.Builder
	for _, character := range strings.TrimSpace(value) {
		switch {
		case character >= 'a' && character <= 'z':
			builder.WriteRune(character)
		case character >= 'A' && character <= 'Z':
			builder.WriteRune(character)
		case character >= '0' && character <= '9':
			builder.WriteRune(character)
		case character == '-':
			builder.WriteRune(character)
		}
	}
	return builder.String()
}

func (storage *DiskAudioStorage) recordingDirectory(accountID, recordingID string) (string, error) {
	account := sanitizePathSegment(accountID)
	recording := sanitizePathSegment(recordingID)
	if account == "" || recording == "" {
		return "", ErrValidation
	}
	return filepath.Join(storage.rootDir, account, recording), nil
}

func copyAudioCapped(destination io.Writer, source io.Reader, maxBytes int64) (int64, error) {
	written, err := io.Copy(destination, io.LimitReader(source, maxBytes+1))
	if err != nil {
		return written, err
	}
	if written > maxBytes {
		return written, ErrTooLarge
	}
	if written == 0 {
		return 0, ErrValidation
	}
	return written, nil
}

func publishTemporaryFile(file *os.File, temporaryPath, finalPath string) error {
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	// temporaryPath vem de os.CreateTemp e finalPath e montado somente com
	// segmentos sanitizados dentro do diretorio privado da gravacao.
	if err := os.Rename(temporaryPath, finalPath); err == nil { //nolint:gosec
		return nil
	}

	if removeErr := os.Remove(finalPath); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) { //nolint:gosec
		return removeErr
	}
	return os.Rename(temporaryPath, finalPath) //nolint:gosec
}

func (storage *DiskAudioStorage) SaveChunk(accountID, recordingID string, sequence int, mimeType string, source io.Reader, maxBytes int64) (StoredChunk, error) {
	if sequence < 0 || maxBytes <= 0 {
		return StoredChunk{}, ErrValidation
	}
	if _, ok := audioExtension(mimeType); !ok {
		return StoredChunk{}, ErrUnsupported
	}

	recordingDir, err := storage.recordingDirectory(accountID, recordingID)
	if err != nil {
		return StoredChunk{}, err
	}
	chunksDir := filepath.Join(recordingDir, "chunks")
	if err := os.MkdirAll(chunksDir, 0o750); err != nil {
		return StoredChunk{}, err
	}

	file, err := os.CreateTemp(chunksDir, ".chunk-*")
	if err != nil {
		return StoredChunk{}, err
	}
	temporaryPath := file.Name()
	removeTemporary := true
	defer func() {
		if removeTemporary {
			_ = os.Remove(temporaryPath)
		}
	}()
	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		return StoredChunk{}, err
	}

	digest := sha256.New()
	sizeBytes, err := copyAudioCapped(io.MultiWriter(file, digest), source, maxBytes)
	if err != nil {
		_ = file.Close()
		return StoredChunk{}, err
	}

	sha := hex.EncodeToString(digest.Sum(nil))
	fileName := fmt.Sprintf("%08d-%s.part", sequence, sha)
	finalPath := filepath.Join(chunksDir, fileName)
	if err := publishTemporaryFile(file, temporaryPath, finalPath); err != nil {
		return StoredChunk{}, err
	}
	removeTemporary = false

	account := sanitizePathSegment(accountID)
	recording := sanitizePathSegment(recordingID)
	return StoredChunk{
		StorageKey: filepath.ToSlash(filepath.Join(account, recording, "chunks", fileName)),
		MimeType:   normalizeAudioMime(mimeType),
		SizeBytes:  sizeBytes,
		SHA256:     sha,
	}, nil
}

func (storage *DiskAudioStorage) openContained(storageKey string) (*os.File, os.FileInfo, error) {
	cleanKey := filepath.Clean(filepath.FromSlash(strings.TrimSpace(storageKey)))
	if cleanKey == "" || cleanKey == "." || filepath.IsAbs(cleanKey) || strings.HasPrefix(cleanKey, "..") {
		return nil, nil, ErrNotFound
	}

	rootPath, err := filepath.Abs(storage.rootDir)
	if err != nil {
		return nil, nil, err
	}
	fullPath, err := filepath.Abs(filepath.Join(storage.rootDir, cleanKey))
	if err != nil {
		return nil, nil, err
	}
	if fullPath != rootPath && !strings.HasPrefix(fullPath, rootPath+string(filepath.Separator)) {
		return nil, nil, ErrNotFound
	}

	file, err := os.Open(fullPath) //nolint:gosec // containment validado acima
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil, ErrNotFound
	}
	if err != nil {
		return nil, nil, err
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, nil, err
	}
	return file, info, nil
}

func (storage *DiskAudioStorage) Consolidate(accountID, recordingID, mimeType string, chunks []Chunk, maxBytes int64) (ConsolidatedAudio, error) {
	extension, ok := audioExtension(mimeType)
	if !ok {
		return ConsolidatedAudio{}, ErrUnsupported
	}
	if len(chunks) == 0 || maxBytes <= 0 {
		return ConsolidatedAudio{}, ErrNotReady
	}
	for index, chunk := range chunks {
		if chunk.Sequence != index {
			return ConsolidatedAudio{}, ErrNotReady
		}
	}

	recordingDir, err := storage.recordingDirectory(accountID, recordingID)
	if err != nil {
		return ConsolidatedAudio{}, err
	}
	if err := os.MkdirAll(recordingDir, 0o750); err != nil {
		return ConsolidatedAudio{}, err
	}

	output, err := os.CreateTemp(recordingDir, ".audio-*")
	if err != nil {
		return ConsolidatedAudio{}, err
	}
	temporaryPath := output.Name()
	removeTemporary := true
	defer func() {
		if removeTemporary {
			_ = os.Remove(temporaryPath)
		}
	}()
	if err := output.Chmod(0o600); err != nil {
		_ = output.Close()
		return ConsolidatedAudio{}, err
	}

	digest := sha256.New()
	var sizeBytes int64
	for _, chunk := range chunks {
		input, _, err := storage.openContained(chunk.StorageKey)
		if err != nil {
			_ = output.Close()
			return ConsolidatedAudio{}, err
		}
		remaining := maxBytes - sizeBytes
		written, copyErr := copyAudioCapped(io.MultiWriter(output, digest), input, remaining)
		closeErr := input.Close()
		sizeBytes += written
		if copyErr != nil {
			_ = output.Close()
			return ConsolidatedAudio{}, copyErr
		}
		if closeErr != nil {
			_ = output.Close()
			return ConsolidatedAudio{}, closeErr
		}
	}

	finalName := "audio" + extension
	finalPath := filepath.Join(recordingDir, finalName)
	if err := publishTemporaryFile(output, temporaryPath, finalPath); err != nil {
		return ConsolidatedAudio{}, err
	}
	removeTemporary = false

	account := sanitizePathSegment(accountID)
	recording := sanitizePathSegment(recordingID)
	return ConsolidatedAudio{
		StorageKey: filepath.ToSlash(filepath.Join(account, recording, finalName)),
		SizeBytes:  sizeBytes,
		SHA256:     hex.EncodeToString(digest.Sum(nil)),
	}, nil
}

func (storage *DiskAudioStorage) BuildLiveWindow(
	ctx context.Context,
	accountID string,
	recordingID string,
	mimeType string,
	segment LiveTranscriptSegment,
	chunks []Chunk,
	maxBytes int64,
) (OpenedAudio, error) {
	extension, ok := audioExtension(mimeType)
	if !ok {
		return OpenedAudio{}, ErrUnsupported
	}
	if len(chunks) == 0 || maxBytes <= 0 {
		return OpenedAudio{}, ErrNotReady
	}

	recordingDir, err := storage.recordingDirectory(accountID, recordingID)
	if err != nil {
		return OpenedAudio{}, err
	}
	if err := os.MkdirAll(recordingDir, 0o750); err != nil {
		return OpenedAudio{}, err
	}

	source, err := os.CreateTemp(recordingDir, ".live-source-*"+extension)
	if err != nil {
		return OpenedAudio{}, err
	}
	sourcePath := source.Name()
	defer func() { _ = os.Remove(sourcePath) }()
	if err := source.Chmod(0o600); err != nil {
		_ = source.Close()
		return OpenedAudio{}, err
	}

	var sizeBytes int64
	for _, chunk := range chunks {
		input, _, openErr := storage.openContained(chunk.StorageKey)
		if openErr != nil {
			_ = source.Close()
			return OpenedAudio{}, openErr
		}
		remaining := maxBytes - sizeBytes
		written, copyErr := copyAudioCapped(source, input, remaining)
		closeErr := input.Close()
		sizeBytes += written
		if copyErr != nil {
			_ = source.Close()
			return OpenedAudio{}, copyErr
		}
		if closeErr != nil {
			_ = source.Close()
			return OpenedAudio{}, closeErr
		}
	}
	if err := source.Sync(); err != nil {
		_ = source.Close()
		return OpenedAudio{}, err
	}
	if err := source.Close(); err != nil {
		return OpenedAudio{}, err
	}

	output, err := os.CreateTemp(recordingDir, ".live-audio-*.wav")
	if err != nil {
		return OpenedAudio{}, err
	}
	outputPath := output.Name()
	if err := output.Close(); err != nil {
		_ = os.Remove(outputPath)
		return OpenedAudio{}, err
	}
	removeOutput := true
	defer func() {
		if removeOutput {
			_ = os.Remove(outputPath)
		}
	}()

	audioFilter := "asetpts=N/SR/TB"
	if segment.StartSequence > 0 {
		// Os clusters posteriores preservam o timestamp absoluto da sessao
		// MediaRecorder. O primeiro bloco fornece apenas o cabecalho WebM; o
		// corte precisa usar a posicao da sequencia, nao a duracao do cabecalho.
		trimSeconds := float64(segment.StartSequence*5000+segment.TrimStartMS) / 1000
		audioFilter = fmt.Sprintf("atrim=start=%.3f,asetpts=N/SR/TB", trimSeconds)
	}
	command := exec.CommandContext( //nolint:gosec // binario e argumentos sao fixos; paths ficam no storage privado.
		ctx,
		"ffmpeg",
		"-nostdin",
		"-hide_banner",
		"-loglevel",
		"error",
		"-y",
		"-i",
		sourcePath,
		"-vn",
		"-af",
		audioFilter,
		"-ac",
		"1",
		"-ar",
		"16000",
		"-c:a",
		"pcm_s16le",
		outputPath,
	)
	if err := command.Run(); err != nil {
		return OpenedAudio{}, fmt.Errorf("transcode live attendance window: %w", err)
	}
	if err := os.Chmod(outputPath, 0o600); err != nil {
		return OpenedAudio{}, err
	}
	file, err := os.Open(outputPath) //nolint:gosec // path criado pelo processo neste diretorio privado.
	if err != nil {
		return OpenedAudio{}, err
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return OpenedAudio{}, err
	}
	removeOutput = false
	return OpenedAudio{
		File:      file,
		FileName:  fmt.Sprintf("atendimento-janela-%04d.wav", segment.SegmentIndex),
		MimeType:  "audio/wav",
		ModTime:   info.ModTime(),
		Temporary: true,
	}, nil
}

func (storage *DiskAudioStorage) Open(storageKey, mimeType, fileName string) (OpenedAudio, error) {
	if _, ok := audioExtension(mimeType); !ok {
		return OpenedAudio{}, ErrUnsupported
	}
	file, info, err := storage.openContained(storageKey)
	if err != nil {
		return OpenedAudio{}, err
	}
	name := strings.TrimSpace(filepath.Base(fileName))
	if name == "" || name == "." {
		name = "atendimento" + filepath.Ext(file.Name())
	}
	return OpenedAudio{
		File:     file,
		FileName: name,
		MimeType: normalizeAudioMime(mimeType),
		ModTime:  info.ModTime(),
	}, nil
}

func (storage *DiskAudioStorage) Remove(storageKey string) error {
	file, _, err := storage.openContained(storageKey)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	path := file.Name()
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Remove(path); errors.Is(err, os.ErrNotExist) {
		return nil
	} else {
		return err
	}
}

func audioDownloadName(recording Recording) string {
	return "atendimento-" + sanitizePathSegment(recording.ServiceID) + "-" +
		strconv.FormatInt(recording.StartedAt, 10)
}
