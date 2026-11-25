package domain

// ErrorCode identifies a transfer failure independently of its diagnostic text.
type ErrorCode string

func (code ErrorCode) Error() string { return string(code) }

// NamedError adds the affected name without changing the stable error code.
type NamedError struct {
	Code  ErrorCode
	Name  string
	Cause error
}

func (e NamedError) Error() string { return string(e.Code) + ": " + e.Name }
func (e NamedError) Unwrap() []error {
	if e.Cause != nil {
		return []error{e.Code, e.Cause}
	}
	return []error{e.Code}
}

const (
	ErrInvalidHeaders      ErrorCode = "INVALID_HEADERS"
	ErrManagedHeader       ErrorCode = "MANAGED_HEADER"
	ErrUnknownProxyMode    ErrorCode = "UNKNOWN_PROXY_MODE"
	ErrUnknownDNSMode      ErrorCode = "UNKNOWN_DNS_MODE"
	ErrInvalidProxyAddress ErrorCode = "INVALID_PROXY_ADDRESS"
	ErrInvalidProxyValue   ErrorCode = "INVALID_PROXY_VALUE"
	ErrLinksRequired       ErrorCode = "LINKS_REQUIRED"
	ErrDiskRange           ErrorCode = "DISK_RANGE"
	ErrInvalidAllocation   ErrorCode = "INVALID_ALLOCATION"
)
