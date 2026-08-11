package job

import(
	"testing"
	"time"
)


func TestRetryDelay(t *testing.T){

	cases := []struct{
		attempt int
	  }{
		{1},
		{2},
		{3},
		{4},
	  }
	
	  for _, tc := range cases{
		d := RetryDelay(tc.attempt)

		shift := tc.attempt - 1
		min := 2 * time.Second * time.Duration(1<<shift) // same as production for attempts 1–4
		max := min + time.Second + time.Millisecond

		if d < min || d > max {
    		t.Fatalf("attempt %d: delay %v not in [%v, %v]", tc.attempt, d, min, max)
		}
	  }
	
}
