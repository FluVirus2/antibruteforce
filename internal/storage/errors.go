package storage

import (
	"errors"
	"fmt"
)

var ErrCacheMiss = errors.New("cache miss")

type UnexpectedDataFormatError struct {
	Key   string
	Cause error
}

func (e *UnexpectedDataFormatError) Error() string {
	return fmt.Sprintf("unexpected data format in cache for key %q: %v", e.Key, e.Cause)
}

func (e *UnexpectedDataFormatError) Unwrap() error {
	return e.Cause
}
