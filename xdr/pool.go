package xdr

import "sync"

// Pool provides type-safe object pooling for XDR types.
//
// Objects retrieved from the pool may contain data from previous use.
// The next decode operation will overwrite all fields, so explicit
// clearing is not required.
//
// Thread Safety:
//   - Pool.Get() and Pool.Put() are safe for concurrent use
//   - However, the objects themselves are NOT thread-safe
//   - Do not access an object after calling Put()
//   - Do not call Put() while another goroutine is using the object
//
// Example usage:
//
//	var lcmPool = xdr.NewPool[LedgerCloseMeta]()
//
//	func HandleRequest(data []byte) (*Result, error) {
//	    lcm := lcmPool.Get()
//	    defer lcmPool.Put(lcm)
//
//	    if _, err := xdr.SafeUnmarshal(data, lcm); err != nil {
//	        return nil, err
//	    }
//	    return process(lcm), nil
//	}
type Pool[T any] struct {
	pool sync.Pool
}

// NewPool creates a new pool for type T.
func NewPool[T any]() *Pool[T] {
	return &Pool[T]{
		pool: sync.Pool{
			New: func() any { return new(T) },
		},
	}
}

// Get retrieves an object from the pool.
//
// The returned object may contain data from previous use. The caller
// should decode into it, which will overwrite all fields.
//
// The caller is responsible for calling Put() when done, unless
// ownership is transferred to another goroutine (in which case that
// goroutine must call Put()).
func (p *Pool[T]) Get() *T {
	return p.pool.Get().(*T)
}

// Put returns an object to the pool for reuse.
//
// WARNING: Do not use the object after calling Put().
// WARNING: Do not call Put() if the object is being used by another goroutine.
func (p *Pool[T]) Put(v *T) {
	p.pool.Put(v)
}
