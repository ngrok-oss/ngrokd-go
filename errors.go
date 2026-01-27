package ngrokd

import (
	"errors"
	"fmt"
)

// Sentinel errors for common failure cases
var (
	ErrEndpointNotFound = errors.New("endpoint not found")
	ErrDialFailed       = errors.New("dial failed")
	ErrUpgradeFailed    = errors.New("binding upgrade failed")
	ErrClosed           = errors.New("dialer is closed")
)

// EndpointNotFoundError is returned when an endpoint is not in cache
type EndpointNotFoundError struct {
	Hostname string
}

func (e *EndpointNotFoundError) Error() string {
	return fmt.Sprintf("endpoint not found: %s", e.Hostname)
}

func (e *EndpointNotFoundError) Is(target error) bool {
	return target == ErrEndpointNotFound
}

// DialError wraps connection establishment failures
type DialError struct {
	Address string
	Cause   error
}

func (e *DialError) Error() string {
	return fmt.Sprintf("dial %s: %v", e.Address, e.Cause)
}

func (e *DialError) Unwrap() error {
	return e.Cause
}

func (e *DialError) Is(target error) bool {
	return target == ErrDialFailed
}

// UpgradeError wraps binding protocol upgrade failures
type UpgradeError struct {
	Hostname string
	Port     int
	Message  string
	Cause    error
}

func (e *UpgradeError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("upgrade failed for %s:%d: %s", e.Hostname, e.Port, e.Message)
	}
	if e.Cause != nil {
		return fmt.Sprintf("upgrade failed for %s:%d: %v", e.Hostname, e.Port, e.Cause)
	}
	return fmt.Sprintf("upgrade failed for %s:%d", e.Hostname, e.Port)
}

func (e *UpgradeError) Unwrap() error {
	return e.Cause
}

func (e *UpgradeError) Is(target error) bool {
	return target == ErrUpgradeFailed
}
