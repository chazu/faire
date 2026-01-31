package upgrade

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Installer handles binary installation with backup and rollback.
type Installer struct {
	currentBinaryPath string
	backupPath        string
	tempDir           string
}

// NewInstaller creates a new Installer.
func NewInstaller(currentBinaryPath string) *Installer {
	return &Installer{
		currentBinaryPath: currentBinaryPath,
	}
}

// Install installs a new binary from an archive.
func (i *Installer) Install(archivePath string) error {
	// Create temp directory for extraction
	tempDir, err := os.MkdirTemp("", "svf-upgrade-*")
	if err != nil {
		return NewError(ExitInstallError, "Failed to create temp directory", err)
	}
	i.tempDir = tempDir
	defer os.RemoveAll(tempDir)

	// Extract binary from archive
	binaryPath, err := i.extractBinary(archivePath, tempDir)
	if err != nil {
		return err
	}

	// Backup current binary
	if err := i.Backup(); err != nil {
		return NewError(ExitInstallError, "Failed to backup current binary", err)
	}

	// Replace binary
	if err := i.replaceBinary(binaryPath); err != nil {
		i.Rollback()
		return err
	}

	// Verify new binary works
	if err := i.Verify(); err != nil {
		i.Rollback()
		return NewError(ExitInstallError, "New binary verification failed", err)
	}

	// Clean up backup on success
	i.cleanupBackup()

	return nil
}

// extractBinary extracts the binary from an archive.
func (i *Installer) extractBinary(archivePath, destDir string) (string, error) {
	if strings.HasSuffix(archivePath, ".tar.gz") {
		return i.extractTarGz(archivePath, destDir)
	}
	if strings.HasSuffix(archivePath, ".zip") {
		return i.extractZip(archivePath, destDir)
	}
	return "", NewError(ExitInstallError, "Unsupported archive format", nil)
}

// extractTarGz extracts a tar.gz archive.
func (i *Installer) extractTarGz(archivePath, destDir string) (string, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return "", NewError(ExitInstallError, "Failed to open archive", err)
	}
	defer f.Close()

	gzr, err := gzip.NewReader(f)
	if err != nil {
		return "", NewError(ExitInstallError, "Failed to read gzip", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", NewError(ExitInstallError, "Failed to read tar", err)
		}

		// Skip directories and non-regular files
		if header.Typeflag != tar.TypeReg {
			continue
		}

		// Check if this is the binary (executable file)
		// The binary name matches the base name without extension
		binaryName := filepath.Base(i.currentBinaryPath)
		if filepath.Base(header.Name) != binaryName {
			continue
		}

		// Extract the file
		destPath := filepath.Join(destDir, binaryName)
		if err := extractFile(tr, destPath, os.FileMode(header.Mode)); err != nil {
			return "", err
		}

		return destPath, nil
	}

	return "", NewError(ExitInstallError, "Binary not found in archive", nil)
}

// extractZip extracts a zip archive.
func (i *Installer) extractZip(archivePath, destDir string) (string, error) {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return "", NewError(ExitInstallError, "Failed to open zip", err)
	}
	defer r.Close()

	binaryName := filepath.Base(i.currentBinaryPath)

	for _, f := range r.File {
		if f.FileInfo().IsDir() {
			continue
		}

		if filepath.Base(f.Name) != binaryName {
			continue
		}

		// Extract the file
		destPath := filepath.Join(destDir, binaryName)
		rc, err := f.Open()
		if err != nil {
			return "", NewError(ExitInstallError, "Failed to open file in zip", err)
		}

		if err := extractFile(rc, destPath, f.Mode()); err != nil {
			rc.Close()
			return "", err
		}
		rc.Close()

		return destPath, nil
	}

	return "", NewError(ExitInstallError, "Binary not found in archive", nil)
}

// extractFile writes a file to disk.
func extractFile(src io.Reader, destPath string, mode os.FileMode) error {
	f, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return NewError(ExitInstallError, "Failed to create file", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, src); err != nil {
		return NewError(ExitInstallError, "Failed to write file", err)
	}

	return nil
}

// Backup creates a backup of the current binary.
func (i *Installer) Backup() error {
	// Check if current binary exists
	if _, err := os.Stat(i.currentBinaryPath); os.IsNotExist(err) {
		return NewError(ExitInstallError, "Current binary not found", err)
	}

	// Create backup path
	i.backupPath = i.currentBinaryPath + ".bak"

	// Copy current binary to backup
	src, err := os.Open(i.currentBinaryPath)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.OpenFile(i.backupPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return err
	}

	return nil
}

// replaceBinary replaces the current binary with the new one.
func (i *Installer) replaceBinary(newBinaryPath string) error {
	// On Unix systems, we can't replace the running binary directly
	// We need to rename it, then copy the new one, then remove the old one
	// For simplicity, we'll just copy over it

	// Remove current binary
	if err := os.Remove(i.currentBinaryPath); err != nil {
		return fmt.Errorf("failed to remove current binary: %w", err)
	}

	// Copy new binary to current location
	if err := copyFile(newBinaryPath, i.currentBinaryPath, 0755); err != nil {
		return err
	}

	return nil
}

// copyFile copies a file from src to dst.
func copyFile(src, dst string, perm os.FileMode) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return err
	}

	return nil
}

// Verify verifies that the new binary works.
func (i *Installer) Verify() error {
	// The new binary should be executable
	// We can't exec it from within the running process,
	// but we can check that it exists and is executable
	info, err := os.Stat(i.currentBinaryPath)
	if err != nil {
		return err
	}

	// Check if executable
	if runtime.GOOS != "windows" {
		if info.Mode().Perm()&0111 == 0 {
			return fmt.Errorf("binary is not executable")
		}
	}

	return nil
}

// Rollback restores the backup if installation failed.
func (i *Installer) Rollback() {
	if i.backupPath == "" {
		return
	}

	// Remove failed binary
	os.Remove(i.currentBinaryPath)

	// Restore backup
	copyFile(i.backupPath, i.currentBinaryPath, 0755)

	// Clean up backup
	i.cleanupBackup()
}

// cleanupBackup removes the backup file.
func (i *Installer) cleanupBackup() {
	if i.backupPath != "" {
		os.Remove(i.backupPath)
		i.backupPath = ""
	}
}

// Cleanup removes any temporary files.
func (i *Installer) Cleanup() {
	if i.tempDir != "" {
		os.RemoveAll(i.tempDir)
		i.tempDir = ""
	}
	i.cleanupBackup()
}
