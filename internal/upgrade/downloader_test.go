package upgrade

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestVerifySignature_NoPublicKey tests that verification is skipped when no public key is provided.
func TestVerifySignature_NoPublicKey(t *testing.T) {
	d := NewDownloader()
	err := d.VerifySignature("dummy.txt", "sig.txt", "")
	assert.NoError(t, err, "Verification should be skipped when no public key is provided")
}

// TestVerifySignature_NoSignatureFile tests that verification is skipped when no signature file exists.
func TestVerifySignature_NoSignatureFile(t *testing.T) {
	d := NewDownloader()

	// Create a temporary directory
	tmpDir := t.TempDir()
	dataFile := filepath.Join(tmpDir, "data.txt")
	require.NoError(t, os.WriteFile(dataFile, []byte("test data"), 0644))

	// Non-existent signature file
	sigFile := filepath.Join(tmpDir, "nonexistent.sig")
	err := d.VerifySignature(dataFile, sigFile, "key.pub")
	assert.NoError(t, err, "Verification should be skipped when signature file doesn't exist")
}

// TestVerifySignature_InvalidPublicKey tests that verification fails with an invalid public key.
func TestVerifySignature_InvalidPublicKey(t *testing.T) {
	d := NewDownloader()

	// Create a temporary directory
	tmpDir := t.TempDir()
	dataFile := filepath.Join(tmpDir, "data.txt")
	sigFile := filepath.Join(tmpDir, "sig.txt")
	keyFile := filepath.Join(tmpDir, "key.pub")

	require.NoError(t, os.WriteFile(dataFile, []byte("test data"), 0644))
	require.NoError(t, os.WriteFile(sigFile, []byte("fake signature"), 0644))
	require.NoError(t, os.WriteFile(keyFile, []byte("invalid key"), 0644))

	err := d.VerifySignature(dataFile, sigFile, keyFile)
	assert.Error(t, err, "Verification should fail with invalid public key")
}

// TestFindChecksumForFile tests the checksum finding logic.
func TestFindChecksumForFile(t *testing.T) {
	d := NewDownloader()

	tests := []struct {
		name           string
		checksums      string
		filename       string
		expectedHash   string
		shouldMatch    bool
	}{
		{
			name:     "standard format",
			checksums: "abc123def456  svf_1.0.0_darwin_arm64.tar.gz\n",
			filename: "svf_1.0.0_darwin_arm64.tar.gz",
			expectedHash: "abc123def456",
			shouldMatch: true,
		},
		{
			name:     "with asterisk",
			checksums: "abc123def456 *svf_1.0.0_darwin_arm64.tar.gz\n",
			filename: "svf_1.0.0_darwin_arm64.tar.gz",
			expectedHash: "abc123def456",
			shouldMatch: true,
		},
		{
			name:     "multiple entries",
			checksums: "abc123  svf_1.0.0_darwin_arm64.tar.gz\ndef456  svf_1.0.0_linux_amd64.tar.gz\n",
			filename: "svf_1.0.0_linux_amd64.tar.gz",
			expectedHash: "def456",
			shouldMatch: true,
		},
		{
			name:     "file not found",
			checksums: "abc123  other_file.tar.gz\n",
			filename: "svf_1.0.0_darwin_arm64.tar.gz",
			expectedHash: "",
			shouldMatch: false,
		},
		{
			name:     "full path in filename",
			checksums: "abc123  svf_1.0.0_darwin_arm64.tar.gz\n",
			filename: "/tmp/svf_1.0.0_darwin_arm64.tar.gz",
			expectedHash: "abc123",
			shouldMatch: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := d.findChecksumForFile(tt.checksums, tt.filename)
			if tt.shouldMatch {
				assert.Equal(t, tt.expectedHash, result)
			} else {
				assert.Equal(t, "", result)
			}
		})
	}
}

// TestFileSHA256 tests the SHA256 calculation.
func TestFileSHA256(t *testing.T) {
	d := NewDownloader()

	// Create a temporary file with known content
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	testContent := "hello world"
	require.NoError(t, os.WriteFile(testFile, []byte(testContent), 0644))

	// Known SHA256 hash for "hello world"
	expectedHash := "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9"

	hash, err := d.fileSHA256(testFile)
	require.NoError(t, err)
	assert.Equal(t, expectedHash, hash)
}

// TestVerifyChecksum tests checksum verification.
func TestVerifyChecksum(t *testing.T) {
	d := NewDownloader()

	t.Run("valid checksum", func(t *testing.T) {
		tmpDir := t.TempDir()
		dataFile := filepath.Join(tmpDir, "data.txt")
		checksumsFile := filepath.Join(tmpDir, "checksums.txt")

		// Create test file
		testContent := "test data"
		require.NoError(t, os.WriteFile(dataFile, []byte(testContent), 0644))

		// Calculate SHA256
		expectedHash, err := d.fileSHA256(dataFile)
		require.NoError(t, err)

		// Create checksums file
		checksumsContent := expectedHash + "  data.txt\n"
		require.NoError(t, os.WriteFile(checksumsFile, []byte(checksumsContent), 0644))

		// Verify
		err = d.VerifyChecksum(dataFile, checksumsFile, "data.txt")
		assert.NoError(t, err)
	})

	t.Run("invalid checksum", func(t *testing.T) {
		tmpDir := t.TempDir()
		dataFile := filepath.Join(tmpDir, "data.txt")
		checksumsFile := filepath.Join(tmpDir, "checksums.txt")

		// Create test file
		require.NoError(t, os.WriteFile(dataFile, []byte("test data"), 0644))

		// Create checksums file with wrong hash
		checksumsContent := "wronghash123  data.txt\n"
		require.NoError(t, os.WriteFile(checksumsFile, []byte(checksumsContent), 0644))

		// Verify should fail
		err := d.VerifyChecksum(dataFile, checksumsFile, "data.txt")
		assert.Error(t, err)
	})

	t.Run("no checksums file", func(t *testing.T) {
		tmpDir := t.TempDir()
		dataFile := filepath.Join(tmpDir, "data.txt")

		require.NoError(t, os.WriteFile(dataFile, []byte("test data"), 0644))

		// Empty checksums path should skip verification
		err := d.VerifyChecksum(dataFile, "", "data.txt")
		assert.NoError(t, err)
	})
}
