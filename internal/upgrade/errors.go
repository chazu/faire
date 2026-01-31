package upgrade

import "fmt"

// Exit codes for upgrade command.
const (
	ExitSuccess           = 0 // Success or already up-to-date
	ExitGenericError      = 1 // Generic error
	ExitNetworkError      = 2 // Network error (couldn't reach GitHub)
	ExitVerificationError = 3 // Verification failed (checksum/signature mismatch)
	ExitInstallError      = 4 // Installation failed (rollback attempted)
	ExitAlreadyLatest     = 5 // Already on latest version (with --check-only)
)

// UpgradeError represents an upgrade operation error.
type UpgradeError struct {
	Code    int
	Message string
	Cause   error
}

func (e *UpgradeError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%d] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%d] %s", e.Code, e.Message)
}

func (e *UpgradeError) Unwrap() error {
	return e.Cause
}

// NewError creates a new UpgradeError.
func NewError(code int, message string, cause error) *UpgradeError {
	return &UpgradeError{
		Code:    code,
		Message: message,
		Cause:   cause,
	}
}

// Predefined errors.
var (
	ErrNetworkFailure     = NewError(ExitNetworkError, "Network error", nil)
	ErrVerificationFailed = NewError(ExitVerificationError, "Verification failed", nil)
	ErrInstallFailed      = NewError(ExitInstallError, "Installation failed", nil)
)
