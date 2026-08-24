package scheduler

import(
	"context"
	"time"
	"log"
	"github.com/owenmoloney/chronos/internal/store"
	"github.com/owenmoloney/chronos/internal/leader"
	"github.com/owenmoloney/chronos/internal/cron"	
	"github.com/owenmoloney/chronos/internal/observe"
)

type Scheduler struct{
	store	*store.Store
	elector *leader.Elector
}

func New(store *store.Store, elector *leader.Elector) *Scheduler{
	return &Scheduler{store: store, elector: elector}
}

func (s *Scheduler) Run(ctx context.Context){
	ticker := time.NewTicker(30 * time.Second)

	defer ticker.Stop()

	for{
		select{
		case<-ctx.Done():
				return
			
		case<-ticker.C: 
				s.tick(ctx)              
		}
	}
}

func (s *Scheduler) tick(ctx context.Context){
	if !s.elector.IsLeader(){
		observe.Leader.Set(0)
		return
	}

	observe.Leader.Set(1)



	defs, err := s.store.ListEnabledCronDefinitions(ctx)
	if err!=nil{
		log.Printf("cron list: %v", err)
		return
	}

	now := time.Now()

	for _, def := range defs {
		due, err := cron.IsDue(def.CronExpr, def.Timezone, def.LastEnqueuedAt, now)
		if err != nil{
			log.Printf("cron %d IsDue:: %v", def.ID, err)
			continue
		}
		if !due{
			continue
		}
		err = s.enqueueDue(ctx, def, now, def.LastEnqueuedAt)
		if err!= nil{
			log.Printf("cron %d enqueue: %v", def.ID, err)
			continue
		}
		log.Printf("enqueue cron %d", def.ID)
	}	
}