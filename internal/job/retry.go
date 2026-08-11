package job

import (
	"math/rand"
	"time"
)

func RetryDelay(attempt int) time.Duration{
	if attempt <1 {
		attempt = 1
	}

	base := 2 * time.Second
	cap := 5 * time.Minute

	shift := attempt -1
	if shift > 20{
		shift = 20
	}

	backoff := base * time.Duration(1<<shift)

	if backoff > cap {
		backoff =cap
	}

	jitter := time.Duration(rand.Int63n(int64(time.Second)+1))

	return backoff + jitter
}