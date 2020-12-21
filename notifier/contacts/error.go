package contacts

import (
	"fmt"
)

type ErrGroupIsEmpty struct {
	group string
}

func (err ErrGroupIsEmpty) Error() string {
	return fmt.Sprintf("Group %s doesn't contain any user", err.group)
}

type ErrNobodyOnDuty struct {
	service string
}

func (err ErrNobodyOnDuty) Error() string {
	return fmt.Sprintf("Nobody on duty, service %s", err.service)
}

type ErrNoDeployers struct{}

func (err ErrNoDeployers) Error() string {
	return fmt.Sprintf("No deployers and no fallback value")
}

type ErrNoServiceChannels struct{}

func (err ErrNoServiceChannels) Error() string {
	return fmt.Sprintf("No service channels and no fallback value")
}
