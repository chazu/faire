package upgrade

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/openpgp"
)

// ProgressHook is called during download with bytes downloaded and total bytes.
type ProgressHook func(downloaded, total int64)

// Downloader downloads and verifies release assets.
type Downloader struct {
	httpClient   *http.Client
	maxRetries   int
	progressHook ProgressHook
}

// NewDownloader creates a new Downloader.
func NewDownloader() *Downloader {
	return &Downloader{
		httpClient: http.DefaultClient,
		maxRetries: 3,
	}
}

// SetProgressHook sets the progress callback.
func (d *Downloader) SetProgressHook(hook ProgressHook) {
	d.progressHook = hook
}

// SetHTTPClient sets the HTTP client (useful for testing).
func (d *Downloader) SetHTTPClient(client *http.Client) {
	d.httpClient = client
}

// Download downloads a file to the destination path.
func (d *Downloader) Download(ctx context.Context, url, destPath string) error {
	for attempt := 0; attempt < d.maxRetries; attempt++ {
		if err := d.downloadAttempt(ctx, url, destPath); err != nil {
			if attempt == d.maxRetries-1 {
				return err
			}
			continue
		}
		return nil
	}
	return nil
}

func (d *Downloader) downloadAttempt(ctx context.Context, url, destPath string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return NewError(ExitNetworkError, "Failed to create request", err)
	}

	req.Header.Set("User-Agent", "svf-upgrade")

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return NewError(ExitNetworkError, "Failed to download", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return NewError(ExitNetworkError, fmt.Sprintf("Download failed with status %d", resp.StatusCode), nil)
	}

	// Create destination directory
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return NewError(ExitGenericError, "Failed to create directory", err)
	}

	// Create destination file
	f, err := os.Create(destPath)
	if err != nil {
		return NewError(ExitGenericError, "Failed to create file", err)
	}
	defer func() { _ = f.Close() }()

	// Download with progress tracking
	total := resp.ContentLength
	var downloaded int64

	if d.progressHook != nil {
		d.progressHook(0, total)
	}

	buf := make([]byte, 32*1024)
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := f.Write(buf[:n]); writeErr != nil {
				return NewError(ExitGenericError, "Failed to write file", writeErr)
			}
			downloaded += int64(n)
			if d.progressHook != nil {
				d.progressHook(downloaded, total)
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return NewError(ExitNetworkError, "Download interrupted", err)
		}
	}

	return nil
}

// VerifyChecksum verifies the SHA256 checksum of a file.
func (d *Downloader) VerifyChecksum(filePath, checksumsPath, expectedBinary string) error {
	if checksumsPath == "" {
		// Checksums file not available, skip verification
		return nil
	}

	// Read checksums file
	checksumsData, err := os.ReadFile(checksumsPath)
	if err != nil {
		return NewError(ExitVerificationError, "Failed to read checksums file", err)
	}

	// Calculate file checksum
	fileHash, err := d.fileSHA256(filePath)
	if err != nil {
		return NewError(ExitVerificationError, "Failed to calculate file checksum", err)
	}

	// Find matching checksum in checksums file
	expectedHash := d.findChecksumForFile(string(checksumsData), expectedBinary)
	if expectedHash == "" {
		// Can't find checksum for this specific file, but we calculated one
		// Skip strict verification
		return nil
	}

	if fileHash != expectedHash {
		return NewError(ExitVerificationError,
			fmt.Sprintf("Checksum mismatch: expected %s, got %s", expectedHash, fileHash), nil)
	}

	return nil
}

// fileSHA256 calculates the SHA256 hash of a file.
func (d *Downloader) fileSHA256(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()

	hash := sha256.New()
	if _, err := io.Copy(hash, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// findChecksumForFile finds the checksum for a specific file in the checksums content.
func (d *Downloader) findChecksumForFile(checksumsContent, filename string) string {
	lines := strings.Split(checksumsContent, "\n")
	basename := filepath.Base(filename)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Format: HASH  filename
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			hash := parts[0]
			file := strings.Join(parts[1:], " ")
			// Check if the filename matches (possibly with quotes or asterisk)
			if strings.Contains(file, basename) || file == basename {
				return hash
			}
		}
	}
	return ""
}

// VerifySignature verifies the GPG signature of a file.
// The signature file should contain a detached signature for the target file.
// publicKeyPath is the path to the public key file used for verification.
// If publicKeyPath is empty, verification is skipped.
// If the signature file is empty or doesn't exist, verification is skipped.
func (d *Downloader) VerifySignature(filePath, signaturePath, publicKeyPath string) error {
	// Skip verification if no public key provided
	if publicKeyPath == "" {
		return nil
	}

	// Skip verification if signature file doesn't exist or is empty
	if signaturePath == "" {
		return nil
	}

	sigInfo, err := os.Stat(signaturePath)
	if err != nil || sigInfo.Size() == 0 {
		// Signature file not available, skip verification
		return nil
	}

	// Read the public key
	keyFile, err := os.Open(publicKeyPath)
	if err != nil {
		return NewError(ExitVerificationError, "Failed to open public key file", err)
	}
	defer func() { _ = keyFile.Close() }()

	keyring, err := openpgp.ReadArmoredKeyRing(keyFile)
	if err != nil {
		return NewError(ExitVerificationError, "Failed to read public key", err)
	}

	if len(keyring) == 0 {
		return NewError(ExitVerificationError, "No public keys found in key file", nil)
	}

	// Read the signature file
	sigFile, err := os.Open(signaturePath)
	if err != nil {
		return NewError(ExitVerificationError, "Failed to open signature file", err)
	}
	defer func() { _ = sigFile.Close() }()

	// Read the file to be verified
	dataFile, err := os.Open(filePath)
	if err != nil {
		return NewError(ExitVerificationError, "Failed to open file to verify", err)
	}
	defer func() { _ = dataFile.Close() }()

	// Verify the detached signature
	_, err = openpgp.CheckDetachedSignature(keyring, dataFile, sigFile)
	if err != nil {
		return NewError(ExitVerificationError, "Signature verification failed", err)
	}

	return nil
}

// VerifySignatureArmored verifies an armored GPG signature of a file.
// This is used when the signature file is ASCII-armored (e.g., checksums.txt.sig).
// publicKeyPath is the path to the public key file used for verification.
// If publicKeyPath is empty, verification is skipped.
func (d *Downloader) VerifySignatureArmored(signedFilePath, signaturePath, publicKeyPath string) error {
	// Skip verification if no public key provided
	if publicKeyPath == "" {
		return nil
	}

	// Skip verification if signature file doesn't exist
	if signaturePath == "" {
		return nil
	}

	if _, err := os.Stat(signaturePath); os.IsNotExist(err) {
		// Signature file not available, skip verification
		return nil
	}

	// Read the public key
	keyFile, err := os.Open(publicKeyPath)
	if err != nil {
		return NewError(ExitVerificationError, "Failed to open public key file", err)
	}
	defer func() { _ = keyFile.Close() }()

	keyring, err := openpgp.ReadArmoredKeyRing(keyFile)
	if err != nil {
		return NewError(ExitVerificationError, "Failed to read public key", err)
	}

	if len(keyring) == 0 {
		return NewError(ExitVerificationError, "No public keys found in key file", nil)
	}

	// Read the signed file
	signedFile, err := os.Open(signedFilePath)
	if err != nil {
		return NewError(ExitVerificationError, "Failed to open signed file", err)
	}
	defer func() { _ = signedFile.Close() }()

	// Read the signature file
	sigFile, err := os.Open(signaturePath)
	if err != nil {
		return NewError(ExitVerificationError, "Failed to open signature file", err)
	}
	defer func() { _ = sigFile.Close() }()

	// Verify the armored detached signature
	signer, err := openpgp.CheckArmoredDetachedSignature(keyring, signedFile, sigFile)
	if err != nil {
		return NewError(ExitVerificationError, "Signature verification failed", err)
	}

	if signer == nil {
		return NewError(ExitVerificationError, "Invalid signature: no signer found", nil)
	}

	return nil
}
