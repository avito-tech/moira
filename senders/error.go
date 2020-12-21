package senders

import (
	"fmt"
)

type ErrSendEvents struct {
	Reason error
	Fatal  bool
}

func (err ErrSendEvents) Error() string {
	var retry string
	if err.Fatal {
		retry = "can't retry"
	} else {
		retry = "can retry"
	}
	return fmt.Sprintf("%v [%s]", err.Reason, retry)
}
