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
	go r.fetchResolvers(ctx)
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
		r.expireResolvers()
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
	for addr := range uniqueResources {
		sem <- struct{}{}
		result, err := TestAddr(addr, r.conf.TestDomains)
		if err != nil || result == nil {
			<-sem
			continue
		}
		fmt.Printf("%s is valid with %3fs latency\n", addr, result.Latency.Seconds())
		if len(r.resolvers) >= r.conf.MaxResolvers {
			// run expireResolvers to make room for new resolvers
			r.expireResolvers()
			if len(r.resolvers) >= r.conf.MaxResolvers {
				// remove resolver with more latency than current result
				var worstAddr string
				for addr, resolver := range r.resolvers {
					if resolver.Latency > result.Latency {
						worstAddr = addr
						break
					}
				}
				if worstAddr != "" {
					delete(r.resolvers, worstAddr)
				} else {
					<-sem
					continue
				}
			}
		}
		existing, ok := r.resolvers[addr]
		if !ok {
			r.resolvers[addr] = result
		} else {
			existing.Latency = result.Latency
			existing.LastTest = time.Now()
		}
		<-sem
	}
}
func (r *ResolverService) expireResolvers() {
	expCount := 0
	for addr, resolver := range r.resolvers {
		if time.Since(resolver.LastTest) > (r.conf.RefreshInterval*2)*time.Minute {
			delete(r.resolvers, addr)
			expCount++
		}
	}
}

func (r *ResolverService) fetchResource(source string) ([]string, error) {
	start := time.Now()
	fmt.Printf("fetching %s\n", source)
	resp, err := http.Get(source)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch %s: %s", source, err.Error())
	}
	defer resp.Body.Close()
	scanner := bufio.NewScanner(resp.Body)
	addrs := make([]string, 0, 1000)
	for scanner.Scan() {
		addr := scanner.Text()
		if net.ParseIP(addr) == nil {
			continue
		}
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

func TestAddr(addr string, testDomains []string) (*Resolver, error) {
	msg := msgPool.Get().(*dns.Msg)
	client := clientPool.Get().(*dns.Client)
	msg.Id = dns.Id()
	msg.RecursionDesired = true
	msg.Question = make([]dns.Question, len(testDomains))
	for i, domain := range testDomains {
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
