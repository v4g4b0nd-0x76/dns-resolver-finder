package resolver

import (
	"bufio"
	"context"
	"dns-db/pkg/conf"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/miekg/dns"
)

type Resolver struct {
	IP       string        `json:"ip"`
	Latency  time.Duration `json:"latency"`
	LastTest time.Time     `json:"last_test"`
}

func NewResolver(ip string, duration time.Duration) *Resolver {
	return &Resolver{
		IP:       ip,
		Latency:  duration,
		LastTest: time.Now(),
	}
}

type ResolverService struct {
	conf      *conf.Conf
	resolvers map[string]*Resolver
}

func NewResolverService(conf *conf.Conf) (*ResolverService, error) {
	return &ResolverService{
		conf:      conf,
		resolvers: make(map[string]*Resolver, conf.MaxResolvers),
	}, nil
}

func (r *ResolverService) Run(ctx context.Context) error {
	fmt.Printf("Running resolver service at: %s\n", time.Now().Format(time.DateTime))
	r.fetchResolvers(ctx)
	<-ctx.Done()
	fmt.Printf("ResolverService: context done, shutting down\n")
	return nil
}

func (r *ResolverService) fetchResolvers(ctx context.Context) {
	r.refreshSources()
	tk := time.NewTicker(time.Minute * r.conf.RefreshInterval)
	defer tk.Stop()
	for range tk.C {
		r.refreshSources()
	}
	<-ctx.Done()

}
func (r *ResolverService) refreshSources() {
	sources := r.conf.Sources
	uniqueResources := make(map[string]struct{}, 1000)
	mu := sync.Mutex{}
	wg := sync.WaitGroup{}
	for _, res := range sources {
		wg.Go(func() {
			addrs, err := r.fetchResource(res)
			if err != nil {
				fmt.Printf("failed to fetch %s: %s\n", res, err.Error())
			}
			mu.Lock()
			for _, addr := range addrs {
				if _, ok := uniqueResources[addr]; !ok {
					uniqueResources[addr] = struct{}{}
				}
			}
			mu.Unlock()
		})
	}
	wg.Wait()

	sem := make(chan struct{}, r.conf.MaxResolve)
	for addr, _ := range uniqueResources {
		sem <- struct{}{}
		result, err := r.testAddr(addr)
		if err != nil || result == nil {
			<-sem
			continue
		}
		fmt.Printf("%s is valid with %3fs latency\n", addr, result.Latency.Seconds())
		r.resolvers[addr] = result
		<-sem
	}
}

func (r *ResolverService) fetchResource(source string) ([]string, error) {
	start := time.Now()
	fmt.Printf("fetching %s\n", source)
	resp, err := http.Get(source)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch %s: %s\n", source, err.Error())
	}
	defer resp.Body.Close()
	scanner := bufio.NewScanner(resp.Body)
	addrs := make([]string, 0, 1000)
	for scanner.Scan() {
		addr := scanner.Text()
		// we first validate that the addr is ipv4 or ipv6
		// then we send a dns query to the given test domains from conf and if the addr passed the quorum of the domains we save it in resolvers
		addrs = append(addrs, addr)
	}

	fmt.Printf("fetched and validate %s in %3fs\n", source, time.Since(start).Seconds())
	return addrs, nil
}

var msgPool = sync.Pool{
	New: func() interface{} {
		return new(dns.Msg)
	},
}

var clientPool = sync.Pool{
	New: func() interface{} {
		// Only create the config once
		return &dns.Client{
			Timeout: 2 * time.Second,
			Net:     "udp",
		}
	},
}

func (r *ResolverService) testAddr(addr string) (*Resolver, error) {
	msg := msgPool.Get().(*dns.Msg)
	client := clientPool.Get().(*dns.Client)
	msg.Id = dns.Id()
	msg.RecursionDesired = true
	msg.Question = make([]dns.Question, len(r.conf.TestDomains))
	for i, domain := range r.conf.TestDomains {
		msg.Question[i] = dns.Question{
			Name:   dns.Fqdn(domain),
			Qtype:  dns.TypeA,
			Qclass: dns.ClassINET,
		}
	}
	start := time.Now()
	res, _, err := client.Exchange(msg, net.JoinHostPort(addr, "53"))
	if err != nil || res == nil || len(res.Answer) <= 0 {
		return nil, err
	}
	duration := time.Since(start)
	msgPool.Put(msg)
	clientPool.Put(client)
	resolver := NewResolver(addr, duration)
	return resolver, nil
}

func (r *ResolverService) scanRanges() {

}

func (r *ResolverService) testResolvers() {

}
