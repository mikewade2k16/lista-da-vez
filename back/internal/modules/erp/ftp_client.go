package erp

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	goftp "github.com/jlaffaye/ftp"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

type sftpClientAPI interface {
	ReadDirContext(ctx context.Context, p string) ([]os.FileInfo, error)
	Open(path string) (*sftp.File, error)
	Close() error
}

type ftpListEntry struct {
	Name    string
	Size    int64
	ModTime time.Time
	IsDir   bool
}

type ftpClientAPI interface {
	List(path string) ([]ftpListEntry, error)
	Retr(path string) (io.ReadCloser, error)
	Close() error
}

type SFTPSource struct {
	options SourceOptions
	client  sftpClientAPI
	sshConn *ssh.Client
	mu      sync.Mutex
}

type FTPSource struct {
	options     SourceOptions
	client      ftpClientAPI
	kind        string
	explicitTLS bool
	mu          sync.Mutex
}

type FTPSSource struct {
	*FTPSource
}

func NewSFTPSource(options SourceOptions) (*SFTPSource, error) {
	if strings.TrimSpace(options.Host) == "" || strings.TrimSpace(options.Username) == "" || strings.TrimSpace(options.RemoteDir) == "" {
		return nil, ErrSourceNotConfigured
	}
	if strings.TrimSpace(options.Password) == "" && strings.TrimSpace(options.KeyPath) == "" {
		return nil, ErrSourceNotConfigured
	}
	if options.Port == 0 {
		options.Port = 22
	}
	if options.dialSSH == nil {
		options.dialSSH = defaultSSHClientDialer
	}
	if options.newSFTPClient == nil {
		options.newSFTPClient = defaultSFTPClientFactory
	}

	return &SFTPSource{options: options}, nil
}

func NewFTPSource(options SourceOptions) (*FTPSource, error) {
	return newFTPSource(options, false, SourceKindFTP)
}

func NewFTPSSource(options SourceOptions) (*FTPSSource, error) {
	base, err := newFTPSource(options, true, SourceKindFTPS)
	if err != nil {
		return nil, err
	}
	return &FTPSSource{FTPSource: base}, nil
}

func newFTPSource(options SourceOptions, explicitTLS bool, kind string) (*FTPSource, error) {
	if strings.TrimSpace(options.Host) == "" || strings.TrimSpace(options.Username) == "" || strings.TrimSpace(options.RemoteDir) == "" {
		return nil, ErrSourceNotConfigured
	}
	if strings.TrimSpace(options.Password) == "" {
		return nil, ErrSourceNotConfigured
	}
	if options.Port == 0 {
		options.Port = 21
	}
	if options.dialFTP == nil {
		options.dialFTP = defaultFTPDialer
	}

	return &FTPSource{
		options:     options,
		kind:        kind,
		explicitTLS: explicitTLS,
	}, nil
}

func (source *SFTPSource) List(ctx context.Context, storeCode string) ([]SourceFileInfo, error) {
	if err := source.ensureConnected(ctx); err != nil {
		return nil, err
	}

	entries, err := source.client.ReadDirContext(ctx, source.options.RemoteDir)
	if err != nil {
		return nil, source.wrapError(err)
	}

	normalizedStoreCode := strings.TrimSpace(storeCode)
	files := make([]SourceFileInfo, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(path.Ext(entry.Name()), ".csv") {
			continue
		}
		meta, parseErr := parseCSVFilename(entry.Name())
		if parseErr != nil || meta.StoreCode != normalizedStoreCode {
			continue
		}
		files = append(files, SourceFileInfo{
			Name:    entry.Name(),
			Size:    entry.Size(),
			ModTime: entry.ModTime(),
		})
	}

	sort.Slice(files, func(left int, right int) bool {
		return files[left].Name < files[right].Name
	})

	return files, nil
}

func (source *FTPSource) List(ctx context.Context, storeCode string) ([]SourceFileInfo, error) {
	if err := source.ensureConnected(ctx); err != nil {
		return nil, err
	}

	entries, err := source.client.List(source.options.RemoteDir)
	if err != nil {
		return nil, source.wrapError(err)
	}

	normalizedStoreCode := strings.TrimSpace(storeCode)
	files := make([]SourceFileInfo, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir || !strings.EqualFold(path.Ext(entry.Name), ".csv") {
			continue
		}
		meta, parseErr := parseCSVFilename(entry.Name)
		if parseErr != nil || meta.StoreCode != normalizedStoreCode {
			continue
		}
		files = append(files, SourceFileInfo{
			Name:    entry.Name,
			Size:    entry.Size,
			ModTime: entry.ModTime,
		})
	}

	sort.Slice(files, func(left int, right int) bool {
		return files[left].Name < files[right].Name
	})

	return files, nil
}

func (source *SFTPSource) Open(ctx context.Context, name string) (io.ReadCloser, error) {
	if err := source.ensureConnected(ctx); err != nil {
		return nil, err
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	normalized := path.Clean(strings.TrimSpace(name))
	if normalized == "." || strings.HasPrefix(normalized, "../") {
		return nil, fmt.Errorf("%w: %s", ErrSourcePathOutsideRoot, name)
	}

	file, err := source.client.Open(path.Join(source.options.RemoteDir, normalized))
	if err != nil {
		return nil, source.wrapError(err)
	}

	return file, nil
}

func (source *FTPSource) Open(ctx context.Context, name string) (io.ReadCloser, error) {
	if err := source.ensureConnected(ctx); err != nil {
		return nil, err
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	normalized := path.Clean(strings.TrimSpace(name))
	if normalized == "." || strings.HasPrefix(normalized, "../") {
		return nil, fmt.Errorf("%w: %s", ErrSourcePathOutsideRoot, name)
	}

	reader, err := source.client.Retr(path.Join(source.options.RemoteDir, normalized))
	if err != nil {
		return nil, source.wrapError(err)
	}

	return reader, nil
}

func (source *SFTPSource) Kind() string {
	return SourceKindSFTP
}

func (source *FTPSource) Kind() string {
	return source.kind
}

func (source *SFTPSource) Close() error {
	source.mu.Lock()
	defer source.mu.Unlock()

	var firstErr error
	if source.client != nil {
		if err := source.client.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
		source.client = nil
	}
	if source.sshConn != nil {
		if err := source.sshConn.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
		source.sshConn = nil
	}
	return firstErr
}

func (source *FTPSource) Close() error {
	source.mu.Lock()
	defer source.mu.Unlock()

	if source.client == nil {
		return nil
	}
	err := source.client.Close()
	source.client = nil
	return err
}

func (source *SFTPSource) ensureConnected(ctx context.Context) error {
	source.mu.Lock()
	defer source.mu.Unlock()

	if source.client != nil {
		return nil
	}

	clientConfig, err := source.buildSSHConfig()
	if err != nil {
		return err
	}

	address := net.JoinHostPort(source.options.Host, strconv.Itoa(source.options.Port))
	backoffs := []time.Duration{0, time.Second, 4 * time.Second, 16 * time.Second}
	var lastErr error

	for _, backoff := range backoffs {
		if backoff > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
		}

		sshConn, err := source.options.dialSSH(ctx, "tcp", address, clientConfig)
		if err != nil {
			lastErr = err
			continue
		}

		sftpClient, err := source.options.newSFTPClient(sshConn)
		if err != nil {
			lastErr = err
			_ = sshConn.Close()
			continue
		}

		source.sshConn = sshConn
		source.client = sftpClient
		return nil
	}

	return source.wrapError(lastErr)
}

func (source *FTPSource) ensureConnected(ctx context.Context) error {
	source.mu.Lock()
	defer source.mu.Unlock()

	if source.client != nil {
		return nil
	}

	backoffs := []time.Duration{0, time.Second, 4 * time.Second, 16 * time.Second}
	var lastErr error

	for _, backoff := range backoffs {
		if backoff > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
		}

		client, err := source.options.dialFTP(ctx, source.options, source.explicitTLS)
		if err != nil {
			lastErr = err
			continue
		}

		source.client = client
		return nil
	}

	return source.wrapError(lastErr)
}

func (source *SFTPSource) buildSSHConfig() (*ssh.ClientConfig, error) {
	authMethods, err := source.buildAuthMethods()
	if err != nil {
		return nil, err
	}

	hostKeyCallback, err := buildHostKeyCallback(source.options.Environment, source.options.HostKeyFingerprint)
	if err != nil {
		return nil, err
	}

	return &ssh.ClientConfig{
		User:            source.options.Username,
		Auth:            authMethods,
		HostKeyCallback: hostKeyCallback,
		Timeout:         15 * time.Second,
	}, nil
}

func (source *SFTPSource) buildAuthMethods() ([]ssh.AuthMethod, error) {
	if password := strings.TrimSpace(source.options.Password); password != "" {
		return []ssh.AuthMethod{ssh.Password(password)}, nil
	}

	keyBytes, err := os.ReadFile(source.options.KeyPath)
	if err != nil {
		return nil, err
	}
	privateKey, err := ssh.ParsePrivateKey(keyBytes)
	if err != nil {
		return nil, err
	}
	return []ssh.AuthMethod{ssh.PublicKeys(privateKey)}, nil
}

func (source *SFTPSource) wrapError(err error) error {
	if err == nil {
		return nil
	}
	return redactedError{cause: err, secrets: []string{source.options.Password}}
}

func (source *FTPSource) wrapError(err error) error {
	if err == nil {
		return nil
	}
	return redactedError{cause: err, secrets: []string{source.options.Password}}
}

func defaultSSHClientDialer(ctx context.Context, network string, address string, config *ssh.ClientConfig) (*ssh.Client, error) {
	dialer := &net.Dialer{Timeout: config.Timeout}
	connection, err := dialer.DialContext(ctx, network, address)
	if err != nil {
		return nil, err
	}

	clientConn, channels, requests, err := ssh.NewClientConn(connection, address, config)
	if err != nil {
		_ = connection.Close()
		return nil, err
	}

	return ssh.NewClient(clientConn, channels, requests), nil
}

func defaultSFTPClientFactory(client *ssh.Client) (sftpClientAPI, error) {
	return sftp.NewClient(client)
}

func defaultFTPDialer(ctx context.Context, options SourceOptions, explicitTLS bool) (ftpClientAPI, error) {
	address := net.JoinHostPort(options.Host, strconv.Itoa(options.Port))
	dialOptions := []goftp.DialOption{
		goftp.DialWithContext(ctx),
		goftp.DialWithTimeout(15 * time.Second),
		goftp.DialWithDisabledEPSV(true),
	}
	if explicitTLS {
		dialOptions = append(dialOptions, goftp.DialWithExplicitTLS(&tls.Config{
			MinVersion: tls.VersionTLS12,
			ServerName: options.Host,
		}))
	}

	conn, err := goftp.Dial(address, dialOptions...)
	if err != nil {
		return nil, err
	}
	if err := conn.Login(options.Username, options.Password); err != nil {
		_ = conn.Quit()
		return nil, err
	}

	return &goFTPClient{conn: conn}, nil
}

type goFTPClient struct {
	conn *goftp.ServerConn
}

func (client *goFTPClient) List(path string) ([]ftpListEntry, error) {
	entries, err := client.conn.List(path)
	if err != nil {
		return nil, err
	}

	result := make([]ftpListEntry, 0, len(entries))
	for _, entry := range entries {
		if entry == nil {
			continue
		}
		result = append(result, ftpListEntry{
			Name:    entry.Name,
			Size:    int64(entry.Size),
			ModTime: entry.Time,
			IsDir:   entry.Type == goftp.EntryTypeFolder,
		})
	}

	return result, nil
}

func (client *goFTPClient) Retr(path string) (io.ReadCloser, error) {
	return client.conn.Retr(path)
}

func (client *goFTPClient) Close() error {
	return client.conn.Quit()
}

func buildHostKeyCallback(environment string, fingerprint string) (ssh.HostKeyCallback, error) {
	normalizedFingerprint := strings.TrimSpace(fingerprint)
	if normalizedFingerprint == "" {
		if strings.EqualFold(strings.TrimSpace(environment), "production") {
			return nil, ErrSourceHostKeyRequired
		}
		return ssh.InsecureIgnoreHostKey(), nil
	}

	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		if ssh.FingerprintSHA256(key) != normalizedFingerprint {
			return fmt.Errorf("%w: unexpected host key for %s", ErrValidation, hostname)
		}
		return nil
	}, nil
}

type redactedError struct {
	cause   error
	secrets []string
}

func (err redactedError) Error() string {
	message := err.cause.Error()
	for _, secret := range err.secrets {
		trimmed := strings.TrimSpace(secret)
		if trimmed == "" {
			continue
		}
		message = strings.ReplaceAll(message, trimmed, "[REDACTED]")
	}
	return message
}

func (err redactedError) Unwrap() error {
	return err.cause
}
