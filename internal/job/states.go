package job

type State string

const (
	StatePending State = "pending"
	StateRunnable State = "runnable"
	StateRunning State = "running"
	StateSucceded State = "succeeded"
	StateFailedRetrying State = "failed_retrying"
	StateDeadLettered State = "dead_lettered"
	StateCanceled State = "canceled"
)

var allowedTransitions = map[State][]State{
	StatePending: 			{StateRunnable, StateCanceled},
	StateRunnable: 			{StateRunning, StateCanceled},
	StateRunning:    		{StateSucceded, StateRunnable, StateFailedRetrying, StateDeadLettered, StateCanceled},
	StateSucceded: 			{StateRunnable, StateDeadLettered, StateCanceled},
	StateFailedRetrying:   	{StateRunnable, StateDeadLettered},
	StateDeadLettered: 		{},
	StateCanceled: 			{},
}

func ValidTransition(from State, to State) bool{
	var allowed = allowedTransitions[from];

	for _, candidate := range allowed{
		if candidate == to{ 
			return true;
		}
	}
	return false;
}

