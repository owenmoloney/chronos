package store
import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
)

func NewPool(ctx context.Context, databaseURL string ) (*pgxpool.Pool, error) {

	pool, err := pgxpool.New(ctx, databaseURL)

	if err != nil{
		return nil, err
	}

	err = pool.Ping(ctx)

	if err != nil{
		pool.Close()
		return nil, err
	}

	return pool, nil
}