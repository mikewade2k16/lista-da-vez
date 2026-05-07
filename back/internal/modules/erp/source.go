package erp

import (
	"context"
	"io"
	"io/fs"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

const (
	SourceKindLocal = "local"
	SourceKindFTP   = "ftp"
	SourceKindSFTP  = "sftp"
	SourceKindFTPS  = "ftps"
)

type SourceFileInfo struct {
	Name    string
	Size    int64
	ModTime time.Time
}

type ErpSource interface {
	List(ctx context.Context, storeCode string) ([]SourceFileInfo, error)
	Open(ctx context.Context, name string) (io.ReadCloser, error)
	Kind() string
	Close() error
}

type sshClientDialer func(ctx context.Context, network string, address string, config *ssh.ClientConfig) (*ssh.Client, error)

type sftpClientFactory func(client *ssh.Client) (sftpClientAPI, error)

type ftpClientDialer func(ctx context.Context, options SourceOptions, explicitTLS bool) (ftpClientAPI, error)

type SourceOptions struct {
	Kind               string
	Environment        string
	LocalDir           string
	RemoteDir          string
	Host               string
	Port               int
	Username           string
	Password           string
	KeyPath            string
	HostKeyFingerprint string
	LocalFS            fs.FS
	dialSSH            sshClientDialer
	newSFTPClient      sftpClientFactory
	dialFTP            ftpClientDialer
}

func NewSource(options SourceOptions) (ErpSource, error) {
	switch strings.TrimSpace(strings.ToLower(options.Kind)) {
	case "", SourceKindLocal:
		return NewLocalSource(options)
	case SourceKindFTP:
		return NewFTPSource(options)
	case SourceKindSFTP:
		return NewSFTPSource(options)
	case SourceKindFTPS:
		return NewFTPSSource(options)
	default:
		return nil, ErrSourceNotConfigured
	}
}
