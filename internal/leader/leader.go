package leader

import (
	"context"
	"time"
	"sync/atomic"
	"log"
	"fmt"
	"github.com/redis/go-redis/v9"
)

const (
	ChronosKey     = "chronos:leader"
	ConfigLeaseTTL = 10 * time.Second
)

type Elector struct {
	client     *redis.Client
	instanceID string
	key        string
	ttl        time.Duration
	held 	   atomic.Bool
}

func New(redisURL, instanceID string) (*Elector, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, err
	}

	client := redis.NewClient(opts)

	return &Elector{
		client:     client,
		instanceID: instanceID,
		key:        ChronosKey,
		ttl:        ConfigLeaseTTL,
	}, nil
}

func (e *Elector) TryAcquire(ctx context.Context) (bool, error) {
	ok, err := e.client.SetNX(ctx, e.key, e.instanceID, e.ttl).Result()
	if err != nil {
		return false, err
	}
	return ok, nil
}


func (e *Elector) Renew(ctx context.Context) (bool, error){

	
	script := `if redis.call("GET", KEYS[1]) == ARGV[1] then
		return redis.call("EXPIRE", KEYS[1], ARGV[2])
	else
		return 0
	end`

	res, err := e.client.Eval(ctx, script, []string{e.key}, e.instanceID,int(e.ttl.Seconds())).Result()

	if err != nil {
		return false, err
	}

	n, ok := res.(int64)
	if !ok {
		return false,fmt.Errorf("leader renew: unexpected result %v", res)
	}

	return n >0 ,nil


}

func (e *Elector) Run(ctx context.Context){
	ticker:= time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	
	for {
		select{
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !e.held.Load(){
				ok, err := e.TryAcquire(ctx)
				if err != nil {
					log.Printf("leader acquire: %v", err)
				} else if ok {
					e.held.Store(true)
					log.Printf("became leader %s", e.instanceID)
				}				
			} else {
				ok, err := e.Renew(ctx)
				if err != nil {
					log.Printf("leader renew: %v", err)
					e.held.Store(false)
				} else if !ok {
					e.held.Store(false)
					log.Printf("lost leadership %s", e.instanceID)
				}
			}
		}
	}
}

func (e *Elector) IsLeader() bool{
	return e.held.Load()
}