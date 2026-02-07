package lblb

import (
	"container/heap"
	"context"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/traefik/traefik/v3/pkg/config/dynamic"
	"github.com/traefik/traefik/v3/pkg/server/service/loadbalancer"
	"golang.org/x/time/rate"
)

type namedHandler struct {
	http.Handler
	name     string
	burst    int64
	average  int64
	period   time.Duration
	priority int64
	bucket   *rate.Limiter
	canAllow bool
}

// type stickyCookie struct {
// 	name     string
// 	secure   bool
// 	httpOnly bool
// }

// Balancer is a LeakyBucket load balancer.

type LBBalancer struct {
	wantsHealthCheck bool

	mutex    sync.RWMutex
	handlers []*namedHandler
	// curDeadline float64
	// status is a record of which child services of the Balancer are healthy, keyed
	// by name of child service. A service is initially added to the map when it is
	// created via Add, and it is later removed or added to the map as needed,
	// through the SetStatus method.
	status map[string]struct{}
	// updaters is the list of hooks that are run (to update the Balancer
	// parent(s)), whenever the Balancer status changes.
	updaters           []func(bool)
	serverAvailability map[string]time.Time
	sticky             *loadbalancer.Sticky
}

// New creates a new load balancer.
func New(sticky *dynamic.Sticky, wantHealthCheck bool) *LBBalancer {
	balancer := &LBBalancer{
		status:             make(map[string]struct{}),
		serverAvailability: make(map[string]time.Time),
		wantsHealthCheck:   wantHealthCheck,
	}
	if sticky != nil && sticky.Cookie != nil {
		balancer.sticky = loadbalancer.NewSticky(*sticky.Cookie)
	}

	return balancer
}

// Len implements heap.Interface/sort.Interface.
func (b *LBBalancer) Len() int { return len(b.handlers) }

// Less implements heap.Interface/sort.Interface.
// func (b *LBBalancer) Less(i, j int) bool { // to be fixed later
// 	return b.handlers[i].priority < b.handlers[j].priority
// }

func (b *LBBalancer) Less(i, j int) bool {
	return b.handlers[i].priority < b.handlers[j].priority
}

// Swap implements heap.Interface/sort.Interface.
func (b *LBBalancer) Swap(i, j int) {
	b.handlers[i], b.handlers[j] = b.handlers[j], b.handlers[i]
}

// Push implements heap.Interface for pushing an item into the heap.
func (b *LBBalancer) Push(x interface{}) {
	h, ok := x.(*namedHandler)
	if !ok {
		return
	}

	b.handlers = append(b.handlers, h)
}

// Pop implements heap.Interface for popping an item from the heap.
// It panics if b.Len() < 1.
func (b *LBBalancer) Pop() interface{} {
	h := b.handlers[len(b.handlers)-1]
	b.handlers = b.handlers[0 : len(b.handlers)-1]
	return h
}

// SetStatus sets on the balancer that its given child is now of the given
// status. balancerName is only needed for logging purposes.
func (b *LBBalancer) SetStatus(ctx context.Context, childName string, up bool) {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	upBefore := len(b.status) > 0

	status := "DOWN"
	if up {
		status = "UP"
	}

	log.Ctx(ctx).Debug().Msgf("Setting status of %s to %v", childName, status)

	if up {
		b.status[childName] = struct{}{}
	} else {
		delete(b.status, childName)
	}

	upAfter := len(b.status) > 0
	status = "DOWN"
	if upAfter {
		status = "UP"
	}

	// No Status Change
	if upBefore == upAfter {
		// We're still with the same status, no need to propagate
		log.Ctx(ctx).Debug().Msgf("Still %s, no need to propagate", status)
		return
	}

	// Status Change
	log.Ctx(ctx).Debug().Msgf("Propagating new %s status", status)
	for _, fn := range b.updaters {
		fn(upAfter)
	}
}

// RegisterStatusUpdater adds fn to the list of hooks that are run when the
// status of the Balancer changes.
// Not thread safe.
func (b *LBBalancer) RegisterStatusUpdater(fn func(up bool)) error {
	if !b.wantsHealthCheck {
		return errors.New("healthCheck not enabled in config for this leaky bucket service")
	}
	b.updaters = append(b.updaters, fn)
	return nil
}

var errNoAvailableServer = errors.New("no available server")

func (b *LBBalancer) nextServer() (*namedHandler, error) {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	if len(b.handlers) == 0 || len(b.status) == 0 {
		return nil, errNoAvailableServer
	}

	var handler *namedHandler
	poppedHandlers := []*namedHandler{}
	for {
		if b.Len() == 0 {
			for _, handler := range poppedHandlers {
				heap.Push(b, handler)
			}
			return nil, errNoAvailableServer
		}
		// Pick handler with highest priority.
		handler = heap.Pop(b).(*namedHandler)
		log.Debug().Msgf("Handler poped: %s", handler.name)
		admissionStart := time.Now()
		handler.canAllow = handler.bucket.Allow()
		log.Debug().Msgf("admission decision: %s allow=%t in %d us", handler.name, handler.canAllow, time.Since(admissionStart).Microseconds())
		poppedHandlers = append(poppedHandlers, handler)
		// heap.Push(b, handler) // not to be immediately pushed back

		if _, ok := b.status[handler.name]; ok && handler.canAllow {
			break
		}
		log.Debug().Msgf("Service bucket not allowed: %s", handler.name)

	}
	for _, handler := range poppedHandlers {
		heap.Push(b, handler)
	}
	log.Debug().Msgf("Service selected by LB: %s", handler.name)
	return handler, nil
}

// func (b *LBBalancer) bucketDelay(handler *namedHandler, delay time.Duration) {
// 	b.mutex.Lock()
// 	defer b.mutex.Unlock()
// 	b.serverAvailability[handler.name] = time.Now().Add(delay)
// }

func (b *LBBalancer) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	// Start timing for load balancer overhead
	lbStart := time.Now()

	if len(b.handlers) == 0 || len(b.status) == 0 {
		http.Error(w, errNoAvailableServer.Error(), http.StatusServiceUnavailable)
		return
	}
	server, err := b.nextServer()
	
	// Measure load balancer duration (without OpenTelemetry overhead)
	lbDuration := time.Since(lbStart)
	
	if err != nil {
		if errors.Is(err, errNoAvailableServer) {
			http.Error(w, errNoAvailableServer.Error(), http.StatusServiceUnavailable)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)

		}
		return
	}
	
	log.Debug().Msgf("load balancer response time: %d us (server=%s)", lbDuration.Microseconds(), server.name)

	// res := server.bucket.Reserve()
	// if !res.OK() {
	// 	http.Error(w, errNoAvailableServer.Error(), http.StatusServiceUnavailable)
	// 	return
	// }
	// b.bucketDelay(server, res.Delay())
	server.ServeHTTP(w, req)

}

// AddServer adds a handler with a server.
func (b *LBBalancer) AddServer(name string, handler http.Handler, server dynamic.Server) {
	b.Add(name, handler, server.Burst, server.Average, server.Period, server.Priority)
}

// Add adds a handler.
// A handler with a non-positive values is ignored.
func (b *LBBalancer) Add(name string, handler http.Handler, burst *int, average *int, period *int, priority *int) {
	bu := 1
	if burst != nil {
		bu = *burst
	}

	if bu <= 1 {
		bu = 1
	}

	a := 1
	if average != nil {
		a = *average
	}

	if a <= 0 {
		return
	}

	p := 1
	if period != nil {
		p = *period
	}

	if p <= 0 {
		p = 1
	}

	prio := 1
	if priority != nil {
		prio = *priority
	}

	if prio <= 0 {
		prio = 1
	}

	bucket := rate.NewLimiter(rate.Every((time.Millisecond*time.Duration(p))/time.Duration(a)), bu)
	canAllow := true
	h := &namedHandler{Handler: handler, name: name, burst: int64(bu), average: int64(a), period: time.Millisecond * time.Duration(p), priority: int64(prio), bucket: bucket, canAllow: canAllow}

	b.mutex.Lock()
	heap.Push(b, h)
	b.status[name] = struct{}{}
	b.mutex.Unlock()
}
