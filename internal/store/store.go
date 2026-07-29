package store

import (
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct{
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) (* Store){
		if pool == nil {
			panic("store: nil pool")
		}
	return &Store{pool: pool}
}