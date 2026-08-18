package leader

import (
	"context"
	"time"
	"log"
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
	held 	   bool
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

	val, err := e.client.Get(ctx, e.key).Result()

	if err == redis.Nil{
		return false, nil
	}

	if err != nil {
		return false, err
	}

	if val != e.instanceID {
		return false, nil
	}

	err = e.client.Expire(ctx, e.key, e.ttl).Err()

	if err != nil{
		return false, err
	}

	return true, nil
}

func (e *Elector) Run(ctx context.Context){
	ticker:= time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	
	for {
		select{
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !e.held{
				ok, err := e.TryAcquire(ctx)
				if err != nil {
					log.Printf("leader acquire: %v", err)
				} else if ok {
					e.held = true
					log.Printf("became leader %s", e.instanceID)
				}				
			} else {
				ok, err := e.Renew(ctx)
				if err != nil {
					log.Printf("leader renew: %v", err)
					e.held = false
				} else if !ok {
					e.held = false
					log.Printf("lost leadership %s", e.instanceID)
				}
			}
		}
	}
}

func (e *Elector) IsLeader() bool{
	return e.held
}