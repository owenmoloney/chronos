package cron 

import(
	"github.com/robfig/cron/v3"
	"time"
)

func IsDue(expr, timezone string, lastEnqueuedAt, now time.Time) (bool, error){

	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return false, err
	}

	last := lastEnqueuedAt.In(loc)
	now = now.In(loc)

	sched, err := cron.ParseStandard(expr)

	if err != nil{
		return false, err
	}

	next := sched.Next(last)

	if next.IsZero(){
		return false, nil
	}

	return !next.After(now), nil
}