package configuration

import (
	"fmt"
)

type CorruptedConfigurationError struct {
	Keys []string
}

func NewCorruptedConfigurationError(keys []string) *CorruptedConfigurationError {
	return &CorruptedConfigurationError{
		Keys: keys,
	}
}

func (e *CorruptedConfigurationError) Error() string {
	return fmt.Sprintf("corrupted %d configuration keys", len(e.Keys))
}
