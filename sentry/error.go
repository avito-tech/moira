package sentry

import (
	"fmt"
)

var (
	ErrAlreadyInit = fmt.Errorf("sentry is already initialized")
	ErrNotInit     = fmt.Errorf("sentry is not initialized yet")
)
