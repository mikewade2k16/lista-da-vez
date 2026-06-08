package erp

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"sort"
	"strings"
)

type LocalSource struct {
	root       string
	filesystem fs.FS
}

func NewLocalSource(options SourceOptions) (*LocalSource, error) {
	root := strings.TrimSpace(options.LocalDir)
	if root == "" && options.LocalFS == nil {
		return nil, ErrSourceNotConfigured
	}

	filesystem := options.LocalFS
	if filesystem == nil {
		filesystem = os.DirFS(root)
	}

	return &LocalSource{
		root:       root,
		filesystem: filesystem,
	}, nil
}

func (source *LocalSource) List(ctx context.Context, storeCode string) ([]SourceFileInfo, error) {
	normalizedStoreCode := strings.TrimSpace(storeCode)
	if normalizedStoreCode == "" {
		return nil, ErrStoreRequired
	}

	files := make([]SourceFileInfo, 0, 32)
	err := fs.WalkDir(source.filesystem, ".", func(filePath string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if entry.IsDir() || !strings.EqualFold(path.Ext(entry.Name()), ".csv") {
			return nil
		}

		meta, parseErr := parseCSVFilename(entry.Name())
		if parseErr != nil || meta.StoreCode != normalizedStoreCode {
			return nil
		}

		info, err := entry.Info()
		if err != nil {
			return err
		}

		files = append(files, SourceFileInfo{
			Name:    path.Clean(filePath),
			Size:    info.Size(),
			ModTime: info.ModTime(),
		})
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(files, func(left int, right int) bool {
		return files[left].Name < files[right].Name
	})

	return files, nil
}

func (source *LocalSource) Open(ctx context.Context, name string) (io.ReadCloser, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	normalized := path.Clean(strings.TrimSpace(name))
	if normalized == "." || strings.HasPrefix(normalized, "../") {
		return nil, fmt.Errorf("%w: %s", ErrSourcePathOutsideRoot, name)
	}

	file, err := source.filesystem.Open(normalized)
	if err != nil {
		return nil, err
	}

	readCloser, ok := file.(io.ReadCloser)
	if !ok {
		_ = file.Close()
		return nil, fmt.Errorf("%w: local source cannot open %s", ErrValidation, normalized)
	}

	return readCloser, nil
}

func (source *LocalSource) Kind() string {
	return SourceKindLocal
}

func (source *LocalSource) Close() error {
	return nil
}
