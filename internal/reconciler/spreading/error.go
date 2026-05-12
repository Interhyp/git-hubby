package spreading

import (
	"errors"
	"fmt"
	"time"
)

type RequiresSpreadError struct {
	RequeueAfter time.Duration
}

func (e RequiresSpreadError) Error() string {
	return fmt.Sprintf("GitHub rate limit exceeded, reset time: %v", e.RequeueAfter)
}

func (e RequiresSpreadError) Is(err error) bool {
	var rateLimitedError *RequiresSpreadError
	return errors.As(err, &rateLimitedError)
}
