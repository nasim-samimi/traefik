package lblb

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/traefik/traefik/v3/pkg/config/dynamic"
)

func TestLBBalancer(t *testing.T) {
	balancer := New(nil, false)

	balancer.Add("first", http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.Header().Set("server", "first")
		rw.WriteHeader(http.StatusOK)
	}), Int(1), Int(1), Int(3), Int(1))

	balancer.Add("second", http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.Header().Set("server", "second")
		rw.WriteHeader(http.StatusOK)
	}), Int(1), Int(1), Int(2), Int(2))

	balancer.Add("Third", http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.Header().Set("server", "Third")
		rw.WriteHeader(http.StatusOK)
	}), Int(2), Int(1), Int(1), Int(3))

	recorder := &responseRecorder{ResponseRecorder: httptest.NewRecorder(), save: map[string]int{}}
	for i := 0; i < 4; i++ {
		time.Sleep(500 * time.Millisecond)
		balancer.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	}

	assert.Equal(t, 1, recorder.save["first"])
	assert.Equal(t, 1, recorder.save["second"])
	assert.Equal(t, 2, recorder.save["Third"])
}

func TestLBBalancerNoService(t *testing.T) {
	balancer := New(nil, false)

	recorder := httptest.NewRecorder()
	balancer.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))

	assert.Equal(t, http.StatusServiceUnavailable, recorder.Result().StatusCode)
}

func TestLBBalancerOneServerZeroBurst(t *testing.T) {
	balancer := New(nil, false)

	balancer.Add("first", http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.Header().Set("server", "first")
		rw.WriteHeader(http.StatusOK)
	}), Int(1), Int(2), Int(1), Int(1))

	balancer.Add("second", http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.Header().Set("server", "second")
		rw.WriteHeader(http.StatusOK)
	}), Int(0), Int(1), Int(1), Int(2))

	recorder := &responseRecorder{ResponseRecorder: httptest.NewRecorder(), save: map[string]int{}}
	for i := 0; i < 3; i++ {
		time.Sleep(200 * time.Millisecond)
		balancer.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	}

	assert.Equal(t, 3, recorder.save["first"])
	assert.Equal(t, 0, recorder.save["second"])
}

func TestLBBalancerOneServerZeroRate(t *testing.T) {
	balancer := New(nil, false)

	balancer.Add("first", http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.Header().Set("server", "first")
		rw.WriteHeader(http.StatusOK)
	}), Int(1), Int(1), Int(1), Int(1))

	balancer.Add("second", http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.Header().Set("server", "second")
		rw.WriteHeader(http.StatusOK)
	}), Int(1), Int(1), Int(1), Int(2))

	recorder := &responseRecorder{ResponseRecorder: httptest.NewRecorder(), save: map[string]int{}}
	for i := 0; i < 3; i++ {
		balancer.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	}

	assert.Equal(t, 1, recorder.save["first"])
	assert.Equal(t, 2, recorder.save["second"])
}

type key string

const serviceName key = "serviceName"

func TestLBBalancerNoServiceUp(t *testing.T) {
	balancer := New(nil, false)

	balancer.Add("first", http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.WriteHeader(http.StatusInternalServerError)
	}), Int(1), Int(1), Int(1), Int(1))

	balancer.Add("second", http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.WriteHeader(http.StatusInternalServerError)
	}), Int(1), Int(1), Int(1), Int(2))

	balancer.SetStatus(context.WithValue(context.Background(), serviceName, "parent"), "first", false)
	balancer.SetStatus(context.WithValue(context.Background(), serviceName, "parent"), "second", false)

	recorder := httptest.NewRecorder()
	balancer.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))

	assert.Equal(t, http.StatusServiceUnavailable, recorder.Result().StatusCode)
}

func TestLBBalancerOneServerDown(t *testing.T) {
	balancer := New(nil, false)

	balancer.Add("first", http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.Header().Set("server", "first")
		rw.WriteHeader(http.StatusOK)
	}), Int(1), Int(1), Int(1), Int(1))

	balancer.Add("second", http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.WriteHeader(http.StatusInternalServerError)
	}), Int(1), Int(1), Int(1), Int(2))
	balancer.SetStatus(context.WithValue(context.Background(), serviceName, "parent"), "second", false)

	recorder := &responseRecorder{ResponseRecorder: httptest.NewRecorder(), save: map[string]int{}}
	for i := 0; i < 3; i++ {
		balancer.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	}

	assert.Equal(t, 3, recorder.save["first"])
}

func TestLBBalancerDownThenUp(t *testing.T) {
	balancer := New(nil, false)

	balancer.Add("first", http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.Header().Set("server", "first")
		rw.WriteHeader(http.StatusOK)
	}), Int(1), Int(1), Int(2), Int(1))

	balancer.Add("second", http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.Header().Set("server", "second")
		rw.WriteHeader(http.StatusOK)
	}), Int(1), Int(1), Int(1), Int(2))
	balancer.SetStatus(context.WithValue(context.Background(), serviceName, "parent"), "second", false)

	recorder := &responseRecorder{ResponseRecorder: httptest.NewRecorder(), save: map[string]int{}}
	for i := 0; i < 3; i++ {
		balancer.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	}
	assert.Equal(t, 3, recorder.save["first"])

	balancer.SetStatus(context.WithValue(context.Background(), serviceName, "parent"), "second", true)
	recorder = &responseRecorder{ResponseRecorder: httptest.NewRecorder(), save: map[string]int{}}
	for i := 0; i < 2; i++ {
		time.Sleep(1000 * time.Millisecond)
		balancer.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	}
	assert.Equal(t, 1, recorder.save["first"])
	assert.Equal(t, 1, recorder.save["second"])
}

func TestLBBalancerPropagate(t *testing.T) {
	balancer1 := New(nil, true)

	balancer1.Add("first", http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.Header().Set("server", "first")
		rw.WriteHeader(http.StatusOK)
	}), Int(1), Int(1), Int(1), Int(1))
	balancer1.Add("second", http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.Header().Set("server", "second")
		rw.WriteHeader(http.StatusOK)
	}), Int(1), Int(1), Int(1), Int(2))

	balancer2 := New(nil, true)
	balancer2.Add("third", http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.Header().Set("server", "third")
		rw.WriteHeader(http.StatusOK)
	}), Int(1), Int(1), Int(1), Int(3))
	balancer2.Add("fourth", http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.Header().Set("server", "fourth")
		rw.WriteHeader(http.StatusOK)
	}), Int(1), Int(1), Int(1), Int(4))

	topBalancer := New(nil, true)
	topBalancer.Add("balancer1", balancer1, Int(1), Int(4), Int(1), Int(1))
	_ = balancer1.RegisterStatusUpdater(func(up bool) {
		topBalancer.SetStatus(context.WithValue(context.Background(), serviceName, "top"), "balancer1", up)
		// TODO(mpl): if test gets flaky, add channel or something here to signal that
		// propagation is done, and wait on it before sending request.
	})
	topBalancer.Add("balancer2", balancer2, Int(1), Int(4), Int(1), Int(1))
	_ = balancer2.RegisterStatusUpdater(func(up bool) {
		topBalancer.SetStatus(context.WithValue(context.Background(), serviceName, "top"), "balancer2", up)
	})

	recorder := &responseRecorder{ResponseRecorder: httptest.NewRecorder(), save: map[string]int{}}
	for i := 0; i < 8; i++ {
		time.Sleep(250 * time.Millisecond)
		topBalancer.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	}
	assert.Equal(t, 2, recorder.save["first"])
	assert.Equal(t, 2, recorder.save["second"])
	assert.Equal(t, 2, recorder.save["third"])
	assert.Equal(t, 2, recorder.save["fourth"])
	wantStatus := []int{200, 200, 200, 200, 200, 200, 200, 200}
	assert.Equal(t, wantStatus, recorder.status)

	// fourth gets downed, but balancer2 still up since third is still up.
	balancer2.SetStatus(context.WithValue(context.Background(), serviceName, "top"), "fourth", false)
	recorder = &responseRecorder{ResponseRecorder: httptest.NewRecorder(), save: map[string]int{}}
	for i := 0; i < 8; i++ {
		time.Sleep(200 * time.Millisecond)
		topBalancer.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	}
	assert.Equal(t, 2, recorder.save["first"])
	assert.Equal(t, 2, recorder.save["second"])
	assert.Equal(t, 4, recorder.save["third"])
	assert.Equal(t, 0, recorder.save["fourth"])
	wantStatus = []int{200, 200, 200, 200, 200, 200, 200, 200}
	assert.Equal(t, wantStatus, recorder.status)

	// third gets downed, and the propagation triggers balancer2 to be marked as
	// down as well for topBalancer.
	balancer2.SetStatus(context.WithValue(context.Background(), serviceName, "top"), "third", false)
	recorder = &responseRecorder{ResponseRecorder: httptest.NewRecorder(), save: map[string]int{}}
	for i := 0; i < 8; i++ {
		topBalancer.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	}
	assert.Equal(t, 4, recorder.save["first"])
	assert.Equal(t, 4, recorder.save["second"])
	assert.Equal(t, 0, recorder.save["third"])
	assert.Equal(t, 0, recorder.save["fourth"])
	wantStatus = []int{200, 200, 200, 200, 200, 200, 200, 200}
	assert.Equal(t, wantStatus, recorder.status)
}

func TestLBBalancerAllServersZeroWeight(t *testing.T) {
	balancer := New(nil, false)

	balancer.Add("test", http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {}), Int(0), Int(0), Int(0), Int(1))
	balancer.Add("test2", http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {}), Int(0), Int(0), Int(0), Int(2))

	recorder := httptest.NewRecorder()
	balancer.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))

	assert.Equal(t, http.StatusServiceUnavailable, recorder.Result().StatusCode)
}

func TestSticky(t *testing.T) {
	balancer := New(&dynamic.Sticky{
		Cookie: &dynamic.Cookie{Name: "test"},
	}, false)

	balancer.Add("first", http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.Header().Set("server", "first")
		rw.WriteHeader(http.StatusOK)
	}), Int(1), Int(1), Int(1), Int(1))

	balancer.Add("second", http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.Header().Set("server", "second")
		rw.WriteHeader(http.StatusOK)
	}), Int(2), Int(1), Int(1), Int(2))

	recorder := &responseRecorder{ResponseRecorder: httptest.NewRecorder(), save: map[string]int{}}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	for i := 0; i < 3; i++ {
		for _, cookie := range recorder.Result().Cookies() {
			req.AddCookie(cookie)
		}
		recorder.ResponseRecorder = httptest.NewRecorder()

		balancer.ServeHTTP(recorder, req)
	}

	assert.Equal(t, 0, recorder.save["first"])
	assert.Equal(t, 3, recorder.save["second"])
}

// TestBalancerBias makes sure that the WRR algorithm spreads elements evenly right from the start,
// and that it does not "over-favor" the high-weighted ones with a biased start-up regime.
func TestLBBalancerBias(t *testing.T) {
	balancer := New(nil, false)

	balancer.Add("first", http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.Header().Set("server", "A")
		rw.WriteHeader(http.StatusOK)
	}), Int(11), Int(1), Int(1), Int(1))

	balancer.Add("second", http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.Header().Set("server", "B")
		rw.WriteHeader(http.StatusOK)
	}), Int(3), Int(1), Int(1), Int(2))

	recorder := &responseRecorder{ResponseRecorder: httptest.NewRecorder(), save: map[string]int{}}

	for i := 0; i < 14; i++ {
		balancer.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	}

	wantSequence := []string{"A", "A", "A", "B", "A", "A", "A", "A", "B", "A", "A", "A", "B", "A"}

	assert.Equal(t, wantSequence, recorder.sequence)
}

func Int(v int) *int { return &v }

type responseRecorder struct {
	*httptest.ResponseRecorder
	save     map[string]int
	sequence []string
	status   []int
}

func (r *responseRecorder) WriteHeader(statusCode int) {
	r.save[r.Header().Get("server")]++
	r.sequence = append(r.sequence, r.Header().Get("server"))
	r.status = append(r.status, statusCode)
	r.ResponseRecorder.WriteHeader(statusCode)
}
