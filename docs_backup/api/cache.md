# Cache API

The `cache` package provides caching with DragonflyDB/Redis.

## Cache Interface

```go
type Cache interface {
    Get(ctx context.Context, key string) ([]byte, error)
    Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
    Delete(ctx context.Context, key string) error
    Exists(ctx context.Context, key string) (bool, error)
}
```

## DragonflyCache

```go
type DragonflyCache struct{}

type Config struct {
    Address  string
    Password string
    Database int
}

func New(cfg Config) (*DragonflyCache, error)
func (c *DragonflyCache) Get(ctx context.Context, key string) ([]byte, error)
func (c *DragonflyCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
func (c *DragonflyCache) Delete(ctx context.Context, key string) error
func (c *DragonflyCache) Exists(ctx context.Context, key string) (bool, error)
func (c *DragonflyCache) Client() *redis.Client
```

## Typed Cache

```go
func GetTyped[T any](ctx context.Context, c Cache, key string) (T, error)
func SetTyped[T any](ctx context.Context, c Cache, key string, value T, ttl time.Duration) error
```
