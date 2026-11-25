package domain

// ErrorCode identifies a transfer failure independently of its diagnostic text.
type ErrorCode string

func (code ErrorCode) Error() string { return string(code) }

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
