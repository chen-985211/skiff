package application

import (
	"errors"
	"fmt"

	"skiff/internal/contexts/transfer/domain"
)

type ErrorCode = domain.ErrorCode

// Domain validation errors keep their identity across the application boundary.
const (
	ErrInvalidHeaders      = domain.ErrInvalidHeaders
	ErrManagedHeader       = domain.ErrManagedHeader
	ErrUnknownProxyMode    = domain.ErrUnknownProxyMode
	ErrUnknownDNSMode      = domain.ErrUnknownDNSMode
	ErrInvalidProxyAddress = domain.ErrInvalidProxyAddress
	ErrInvalidProxyValue   = domain.ErrInvalidProxyValue
	ErrLinksRequired       = domain.ErrLinksRequired
	ErrDiskRange           = domain.ErrDiskRange
	ErrInvalidAllocation   = domain.ErrInvalidAllocation
)

// Application and adapter errors exposed by the transfer use cases.
const (
	ErrInvalidMirrors             ErrorCode = "INVALID_MIRRORS"
	ErrTorrentNotFound            ErrorCode = "TORRENT_NOT_FOUND"
	ErrVerifyUnsupported          ErrorCode = "VERIFY_UNSUPPORTED"
	ErrTorrentDirectory           ErrorCode = "TORRENT_DIRECTORY"
	ErrPauseBeforeMoving          ErrorCode = "PAUSE_BEFORE_MOVING"
	ErrTaskChanged                ErrorCode = "TASK_CHANGED"
	ErrDestinationInUse           ErrorCode = "DESTINATION_IN_USE"
	ErrEngineMissing              ErrorCode = "ENGINE_MISSING"
	ErrTorrentCreationUnsupported ErrorCode = "TORRENT_CREATION_UNSUPPORTED"
	ErrTorrentPathsRequired       ErrorCode = "TORRENT_PATHS_REQUIRED"
	ErrInvalidTracker             ErrorCode = "INVALID_TRACKER"
	ErrTorrentSeed                ErrorCode = "TORRENT_SEED_FAILED"
	ErrNegativeRate               ErrorCode = "NEGATIVE_RATE"
	ErrTaskNotFound               ErrorCode = "TASK_NOT_FOUND"
	ErrSingleCURLRequired         ErrorCode = "SINGLE_CURL_REQUIRED"
	ErrCURLQuoting                ErrorCode = "CURL_QUOTING"
	ErrCURLMultipleURLs           ErrorCode = "CURL_MULTIPLE_URLS"
	ErrCURLUnsupportedOperation   ErrorCode = "CURL_UNSUPPORTED_OPERATION"
	ErrCURLMissingValue           ErrorCode = "CURL_MISSING_VALUE"
	ErrCURLRequiresGET            ErrorCode = "CURL_REQUIRES_GET"
	ErrCURLMissingHeader          ErrorCode = "CURL_MISSING_HEADER"
	ErrCURLMissingCookie          ErrorCode = "CURL_MISSING_COOKIE"
	ErrCURLMissingLogin           ErrorCode = "CURL_MISSING_LOGIN"
	ErrCURLMissingFileURL         ErrorCode = "CURL_MISSING_FILE_URL"
	ErrBTUnsupported              ErrorCode = "BT_UNSUPPORTED"
	ErrTorrentSelection           ErrorCode = "TORRENT_SELECTION"
	ErrStaleTorrentSelection      ErrorCode = "STALE_TORRENT_SELECTION"
	ErrDuplicateTorrent           ErrorCode = "DUPLICATE_TORRENT"
	ErrBatchRemaining             ErrorCode = "BATCH_REMAINING"
	ErrConnectionLimit            ErrorCode = "CONNECTION_LIMIT"
	ErrInvalidOutputName          ErrorCode = "INVALID_OUTPUT_NAME"
	ErrInvalidDirectory           ErrorCode = "INVALID_DIRECTORY"
	ErrSingleMirrorTask           ErrorCode = "SINGLE_MIRROR_TASK"
	ErrMixedHeaderOrigins         ErrorCode = "MIXED_HEADER_ORIGINS"
	ErrInvalidDefaultDirectory    ErrorCode = "INVALID_DEFAULT_DIRECTORY"
	ErrUnknownTheme               ErrorCode = "UNKNOWN_THEME"
	ErrConcurrencyRange           ErrorCode = "CONCURRENCY_RANGE"
	ErrApplyConcurrency           ErrorCode = "APPLY_CONCURRENCY_FAILED"
	ErrInvalidSeedPolicy          ErrorCode = "INVALID_SEED_POLICY"
	ErrBTPortRange                ErrorCode = "BT_PORT_RANGE"
	ErrBadSubscriptionURL         ErrorCode = "BAD_SUBSCRIPTION_URL"
	ErrSubscriptionsUnsupported   ErrorCode = "SUBSCRIPTIONS_UNSUPPORTED"
	ErrReadPeers                  ErrorCode = "READ_PEERS_FAILED"
	ErrBTPeerLimit                ErrorCode = "BT_PEER_LIMIT"
	ErrCommandUnavailable         ErrorCode = "COMMAND_UNAVAILABLE"
	ErrSaveQueue                  ErrorCode = "SAVE_QUEUE_FAILED"
	ErrFTPLoginUnneeded           ErrorCode = "FTP_LOGIN_UNNEEDED"
	ErrInvalidFTPLogin            ErrorCode = "INVALID_FTP_LOGIN"
	ErrCorruptQueue               ErrorCode = "CORRUPT_QUEUE"
	ErrQueueVersion               ErrorCode = "QUEUE_VERSION_UNSUPPORTED"
	ErrEnginePlatform             ErrorCode = "ENGINE_PLATFORM_UNSUPPORTED"
	ErrEngineDownload             ErrorCode = "ENGINE_DOWNLOAD_FAILED"
	ErrEnginePackage              ErrorCode = "ENGINE_PACKAGE_INVALID"
	ErrEngineStart                ErrorCode = "ENGINE_START_FAILED"
	ErrEngineTimeout              ErrorCode = "ENGINE_START_TIMEOUT"
	ErrEngineExited               ErrorCode = "ENGINE_EXITED"
	ErrEngineConnection           ErrorCode = "ENGINE_CONNECTION_FAILED"
	ErrFolderOpen                 ErrorCode = "FOLDER_OPEN_FAILED"
	ErrFileMissing                ErrorCode = "FILE_MISSING"
	ErrFileDialog                 ErrorCode = "FILE_DIALOG_FAILED"
	ErrTorrentRead                ErrorCode = "TORRENT_READ_FAILED"
	ErrTorrentInvalid             ErrorCode = "TORRENT_INVALID"
	ErrTorrentVersion             ErrorCode = "TORRENT_VERSION_UNSUPPORTED"
	ErrMagnetInvalid              ErrorCode = "MAGNET_INVALID"
	ErrMagnetMetadata             ErrorCode = "MAGNET_METADATA_FAILED"
	ErrTorrentFiles               ErrorCode = "TORRENT_FILE_VERIFICATION_FAILED"
	ErrUnexpected                 ErrorCode = "UNEXPECTED_ERROR"
)

func CodeOf(err error) ErrorCode {
	var code ErrorCode
	if errors.As(err, &code) {
		return code
	}
	return ErrUnexpected
}

func IsExpected(code ErrorCode) bool {
	switch code {
	case ErrInvalidHeaders, ErrManagedHeader, ErrUnknownProxyMode, ErrUnknownDNSMode,
		ErrInvalidProxyAddress, ErrInvalidProxyValue, ErrLinksRequired, ErrDiskRange,
		ErrInvalidAllocation, ErrInvalidMirrors, ErrTorrentNotFound, ErrVerifyUnsupported,
		ErrTorrentDirectory, ErrPauseBeforeMoving, ErrTaskChanged, ErrDestinationInUse,
		ErrEngineMissing, ErrTorrentCreationUnsupported, ErrTorrentPathsRequired, ErrInvalidTracker,
		ErrNegativeRate, ErrTaskNotFound, ErrSingleCURLRequired, ErrCURLQuoting,
		ErrCURLMultipleURLs, ErrCURLUnsupportedOperation, ErrCURLMissingValue, ErrCURLRequiresGET,
		ErrCURLMissingHeader, ErrCURLMissingCookie, ErrCURLMissingLogin, ErrCURLMissingFileURL,
		ErrBTUnsupported, ErrTorrentSelection, ErrStaleTorrentSelection, ErrDuplicateTorrent,
		ErrBatchRemaining, ErrConnectionLimit, ErrInvalidOutputName, ErrInvalidDirectory,
		ErrSingleMirrorTask, ErrMixedHeaderOrigins, ErrInvalidDefaultDirectory, ErrUnknownTheme,
		ErrConcurrencyRange, ErrInvalidSeedPolicy, ErrBTPortRange, ErrBadSubscriptionURL,
		ErrSubscriptionsUnsupported, ErrBTPeerLimit, ErrCommandUnavailable, ErrFTPLoginUnneeded,
		ErrInvalidFTPLogin, ErrEnginePlatform, ErrFileMissing, ErrTorrentInvalid,
		ErrTorrentVersion, ErrMagnetInvalid, ErrTorrentFiles:
		return true
	default:
		return false
	}
}

// Wrap adds a stable code while preserving the original cause for inspection.
func Wrap(code ErrorCode, cause error) error {
	if cause == nil {
		return nil
	}
	return fmt.Errorf("%w: %w", code, cause)
}

// Task failure values already persisted in the queue keep their stable identity.
const (
	FailureResumeUnsupported    = domain.ResumeUnsupported
	FailureSourceUnverified     = domain.SourceUnverified
	FailureCheckpointUnverified = domain.CheckpointUnverified
	FailureFTPLoginRequired     = domain.FTPLoginRequired
	FailureSourceChanged        = domain.SourceChanged
	FailureSourceUnavailable    = domain.SourceUnavailable
)
