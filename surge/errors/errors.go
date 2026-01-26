package errors

import (
	"errors"
	"fmt"
	"time"
)

var (
	ErrQueueNotFound = errors.New("queue not found")
	ErrJobNotFound   = errors.New("job not found")
	ErrInvalidConfig = errors.New("invalid configuration")
)

type QueuePausedError struct {
	Namespace string
	Queue     string
}

func (e *QueuePausedError) Error() string {
	return fmt.Sprintf("queue %s:%s is paused", e.Namespace, e.Queue)
}

func IsQueuePaused(err error) bool {
	var qpe *QueuePausedError
	return errors.As(err, &qpe)
}

type UniquenessViolationError struct {
	UniqueKey string
}

func (e *UniquenessViolationError) Error() string {
	return fmt.Sprintf("job already enqueued within uniqueness window: %s", e.UniqueKey)
}

func IsUniquenessViolation(err error) bool {
	var uve *UniquenessViolationError
	return errors.As(err, &uve)
}

type HandlerNotFoundError struct {
	Topic string
}

func (e *HandlerNotFoundError) Error() string {
	return fmt.Sprintf("no handler found for topic: %s", e.Topic)
}

func IsHandlerNotFound(err error) bool {
	var hnf *HandlerNotFoundError
	return errors.As(err, &hnf)
}

type BackendConnectionError struct {
	Backend string
	Err     error
}

func (e *BackendConnectionError) Error() string {
	return fmt.Sprintf("failed to connect to %s backend: %v", e.Backend, e.Err)
}

func (e *BackendConnectionError) Unwrap() error {
	return e.Err
}

func IsBackendConnection(err error) bool {
	var bce *BackendConnectionError
	return errors.As(err, &bce)
}

type BackendOperationError struct {
	Operation string
	Err       error
}

func (e *BackendOperationError) Error() string {
	return fmt.Sprintf("backend operation %s failed: %v", e.Operation, e.Err)
}

func (e *BackendOperationError) Unwrap() error {
	return e.Err
}

func IsBackendOperation(err error) bool {
	var boe *BackendOperationError
	return errors.As(err, &boe)
}

type JobNotFoundError struct {
	JobID string
}

func (e *JobNotFoundError) Error() string {
	return fmt.Sprintf("job not found: %s", e.JobID)
}

func IsJobNotFound(err error) bool {
	var jnf *JobNotFoundError
	return errors.As(err, &jnf)
}

type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("validation failed for field %s: %s", e.Field, e.Message)
	}
	return fmt.Sprintf("validation failed: %s", e.Message)
}

func IsValidation(err error) bool {
	var ve *ValidationError
	return errors.As(err, &ve)
}

type BatchError struct {
	TotalJobs     int
	FailedJobs    int
	SucceededJobs int
	FirstError    error
}

func (e *BatchError) Error() string {
	return fmt.Sprintf("batch operation failed: %d/%d jobs failed. First error: %v",
		e.FailedJobs, e.TotalJobs, e.FirstError)
}

func (e *BatchError) Unwrap() error {
	return e.FirstError
}

func IsBatchError(err error) bool {
	var be *BatchError
	return errors.As(err, &be)
}

type TimeoutError struct {
	Operation string
	Timeout   time.Duration
}

func (e *TimeoutError) Error() string {
	return fmt.Sprintf("operation %s timed out after %v", e.Operation, e.Timeout)
}

func IsTimeout(err error) bool {
	var te *TimeoutError
	return errors.As(err, &te)
}
