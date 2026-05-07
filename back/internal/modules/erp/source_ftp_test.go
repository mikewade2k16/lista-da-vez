package erp

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"
)

func TestFTPSourceListAndOpen(t *testing.T) {
	client := &fakeFTPClient{
		entries: []ftpListEntry{
			{Name: "184-12583959000186-customer-20260505010059.csv", Size: 10, ModTime: time.Date(2026, 5, 5, 1, 1, 0, 0, time.UTC)},
			{Name: "184-12583959000186-order-20260505010059.csv", Size: 20, ModTime: time.Date(2026, 5, 5, 1, 1, 0, 0, time.UTC)},
			{Name: "907-12583959000186-order-20260505010059.csv", Size: 30, ModTime: time.Date(2026, 5, 5, 1, 1, 0, 0, time.UTC)},
			{Name: "nested", IsDir: true},
		},
		files: map[string]string{
			"extract_files/184-12583959000186-order-20260505010059.csv": "body",
		},
	}

	source, err := NewFTPSource(SourceOptions{
		Host:      "ftp.example.org",
		Username:  "user",
		Password:  "secret",
		RemoteDir: "extract_files",
		dialFTP: func(ctx context.Context, options SourceOptions, explicitTLS bool) (ftpClientAPI, error) {
			if explicitTLS {
				t.Fatalf("expected plain ftp source")
			}
			return client, nil
		},
	})
	if err != nil {
		t.Fatalf("NewFTPSource() error = %v", err)
	}

	files, err := source.List(context.Background(), "184")
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if client.listPath != "extract_files" {
		t.Fatalf("expected remote dir extract_files, got %q", client.listPath)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}
	if files[0].Name != "184-12583959000186-customer-20260505010059.csv" {
		t.Fatalf("unexpected first file %q", files[0].Name)
	}

	reader, err := source.Open(context.Background(), "184-12583959000186-order-20260505010059.csv")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer reader.Close()

	body, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if string(body) != "body" {
		t.Fatalf("unexpected body %q", string(body))
	}
	if len(client.retrPaths) != 1 || client.retrPaths[0] != "extract_files/184-12583959000186-order-20260505010059.csv" {
		t.Fatalf("unexpected retr paths %#v", client.retrPaths)
	}
	if err := source.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if !client.closed {
		t.Fatal("expected ftp client to be closed")
	}
}

func TestFTPSourceRejectsParentTraversal(t *testing.T) {
	source, err := NewFTPSource(SourceOptions{
		Host:      "ftp.example.org",
		Username:  "user",
		Password:  "secret",
		RemoteDir: "extract_files",
		dialFTP: func(ctx context.Context, options SourceOptions, explicitTLS bool) (ftpClientAPI, error) {
			return &fakeFTPClient{}, nil
		},
	})
	if err != nil {
		t.Fatalf("NewFTPSource() error = %v", err)
	}

	_, err = source.Open(context.Background(), "../secret.csv")
	if err == nil || !strings.Contains(err.Error(), ErrSourcePathOutsideRoot.Error()) {
		t.Fatalf("expected path outside root error, got %v", err)
	}
}

type fakeFTPClient struct {
	entries    []ftpListEntry
	files      map[string]string
	listPath   string
	retrPaths  []string
	closed     bool
	listErr    error
	retrErr    error
	closeError error
}

func (client *fakeFTPClient) List(path string) ([]ftpListEntry, error) {
	client.listPath = path
	if client.listErr != nil {
		return nil, client.listErr
	}
	return append([]ftpListEntry{}, client.entries...), nil
}

func (client *fakeFTPClient) Retr(path string) (io.ReadCloser, error) {
	client.retrPaths = append(client.retrPaths, path)
	if client.retrErr != nil {
		return nil, client.retrErr
	}
	return io.NopCloser(strings.NewReader(client.files[path])), nil
}

func (client *fakeFTPClient) Close() error {
	client.closed = true
	return client.closeError
}
