package main
import(
	"context"
	"strconv"

	"github.com/redis/go/go-redis/v9"
)
//Cache wraps Redis, buffering access counts off the Postgres hot path
type Cache struct {
	rdb *redis.Client
}
func NewCache(addr string) (*Cache, error){
	rdb := redis.NewClient(&redis.Options{
		Addr: addr,
	})
	// Ping to verify the connection works now, not later mid request
	if err := rdb.Ping(context.Background()).Err(); err!= nil{
		return nil, err
	}
	return &Cache{rdb: rdb}, nil
}
func (c *Cache) Close() error{
	return c.rdb.Close()
}

//RecordAccess increments the access counter for a key in Redis 
//Fast, in-memory, atomic, replaces the per GET Postgres write.
func (c *Cache) RecordAccess(key string) error{
	ctx := context.Background()
	return c.rdb.Incr(ctx, "access:"+key).Err()
}