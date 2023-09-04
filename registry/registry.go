package registry

import (
	"log"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

type GGTRegistry struct {
	timeout time.Duration
	mu      sync.Mutex
	// key:  addr, value: ServerItem
	servers map[string]*ServerItem
}

type ServerItem struct {
	Addr      string
	startTime time.Time
}

const (
	defaultPath    = "/_ggt-rpc_/ggt-registry"
	defaultTimeout = time.Minute * 5
)

func New(timeout time.Duration) *GGTRegistry {
	return &GGTRegistry{
		timeout: timeout,
		servers: make(map[string]*ServerItem),
	}
}

var DefaultGGTRegistry = New(defaultTimeout)

func (r *GGTRegistry) putServer(addr string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s := r.servers[addr]
	if s == nil {
		r.servers[addr] = &ServerItem{
			Addr:      addr,
			startTime: time.Now(),
		}
	} else {
		s.startTime = time.Now()
	}
}

func (r *GGTRegistry) aliveServers() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	var alive []string
	for addr, s := range r.servers {
		if r.timeout == 0 || s.startTime.Add(r.timeout).After(time.Now()) {
			alive = append(alive, addr)
		} else {
			delete(r.servers, addr)
		}
	}
	sort.Strings(alive)
	return alive
}

func sendHeartbeat(registry, addr string) error {
	log.Println(addr, "send heart beat to registry", registry)
	httpClient := &http.Client{}
	req, _ := http.NewRequest("POST", registry, nil)
	req.Header.Set("X-GGT-RPC-SERVER", addr)
	if _, err := httpClient.Do(req); err != nil {
		log.Println("rpc server: heart beat err:", err)
		return err
	}
	return nil

}

func Heartbeat(registry, addr string, duration time.Duration) {
	if duration == 0 {
		duration = defaultTimeout - time.Duration(1)*time.Minute
	}
	var err error
	err = sendHeartbeat(registry, addr)
	go func() {
		t := time.NewTicker(duration)
		for err == nil {
			<-t.C
			err = sendHeartbeat(registry, addr)
		}
	}()

}

func (r *GGTRegistry) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case "GET":
		w.Header().Set("X-GGT-RPC-SERVER-LIST", strings.Join(r.aliveServers(), ","))
	case "POST":
		addr := req.Header.Get("X-GGT-RPC-SERVER")
		if addr == "" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		r.putServer(addr)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (r *GGTRegistry) HandleHTTP(registryPath string) {
	http.Handle(registryPath, r)
	log.Println("rpc registry path:", registryPath)
}

func HandleHTTP() {
	DefaultGGTRegistry.HandleHTTP(defaultPath)
}
