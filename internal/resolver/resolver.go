package resolver

import (
	"bufio"
	"context"
	"dns-resolver-finder/pkg/conf"
	"dns-resolver-finder/pkg/radix"
	"dns-resolver-finder/pkg/types"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/miekg/dns"
)

func NewResolver(ip string, duration time.Duration) *types.Resolver {
	return &types.Resolver{
		IP:       ip,
		Latency:  duration,
		LastTest: time.Now(),
	}
}

type ResolverService struct {
	conf       *conf.Conf
	resolvers  radix.IPTree
	mu         sync.Mutex
	checkedIps map[string]struct{}
}

func NewResolverService(conf *conf.Conf) (*ResolverService, error) {
	return &ResolverService{
		conf:       conf,
		resolvers:  *radix.NewIPTree(conf.MaxResolvers),
		mu:         sync.Mutex{},
		checkedIps: make(map[string]struct{}, 100_000),
	}, nil
}

func (r *ResolverService) Run(ctx context.Context) error {
	fmt.Printf("Running resolver service at: %s\n", time.Now().Format(time.DateTime))
	go r.fetchResolvers(ctx)
	go r.scanRanges(ctx)
	<-ctx.Done()
	fmt.Printf("ResolverService: context done, shutting down\n")
	return nil
}

func (r *ResolverService) fetchResolvers(ctx context.Context) {
	r.refreshSources()
	tk := time.NewTicker(time.Second * 5)
	defer tk.Stop()
	for range tk.C {
		go r.refreshSources()
		go r.expireResolvers()
		go r.reEvaluateResolvers()
	}
	<-ctx.Done()

}
func (r *ResolverService) refreshSources() {
	sources := r.conf.Sources
	uniqueResources := make(map[string]struct{})
	var fetchMu sync.Mutex
	var fetchWg sync.WaitGroup

	for _, res := range sources {
		fetchWg.Add(1)
		go func(url string) {
			defer fetchWg.Done()
			addrs, err := r.fetchResource(url)
			if err != nil {
				return
			}
			fetchMu.Lock()
			for _, addr := range addrs {
				uniqueResources[addr] = struct{}{}
			}
			fetchMu.Unlock()
		}(res)
	}
	fetchWg.Wait()

	sem := make(chan struct{}, r.conf.MaxResolve)
	var testWg sync.WaitGroup
	resultsChan := make(chan *types.Resolver, len(uniqueResources))
	go func() {
		for result := range resultsChan {
			r.mu.Lock()

			if r.resolvers.Len() >= r.conf.MaxResolvers {
				r.expireResolvers()
				if r.resolvers.Len() >= r.conf.MaxResolvers {
					r.resolvers.ReplaceWorst(result)
					continue
				}
			}

			r.resolvers.Insert(result)
			fmt.Printf("added resolver %s with latency %3fs, current resolvers length: %d\n", result.IP, result.Latency.Seconds(), r.resolvers.Len())

			r.mu.Unlock()
		}
	}()

	for addr := range uniqueResources {
		r.mu.Lock()
		if _, checked := r.checkedIps[addr]; checked {
			r.mu.Unlock()
			continue
		}
		r.mu.Unlock()
		testWg.Add(1)
		go func(address string) {
			r.mu.Lock()
			r.checkedIps[address] = struct{}{}
			r.mu.Unlock()
			defer testWg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			result, err := TestAddr(address, r.conf.TestDomains)
			if err != nil || result == nil {
				return
			}
			resultsChan <- result

		}(addr)
	}
	testWg.Wait()
	close(resultsChan)
}

func (r *ResolverService) expireResolvers() {
	expCount := 0
	for _, resolver := range r.resolvers.GetAll() {
		if time.Since(resolver.LastTest) > (r.conf.RefreshInterval*2)*time.Minute {
			r.resolvers.Delete(resolver.IP)
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
	New: func() any {
		return new(dns.Msg)
	},
}

var clientPool = sync.Pool{
	New: func() any {
		// Only create the config once
		return &dns.Client{
			Timeout: time.Second,
			Net:     "udp",
		}
	},
}

func TestAddr(addr string, testDomains []string) (*types.Resolver, error) {
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

func (r *ResolverService) reEvaluateResolvers() {
	var wg sync.WaitGroup
	sem := make(chan struct{}, r.conf.MaxResolve)
	for _, resolver := range r.resolvers.GetAll() {
		if time.Since(resolver.LastTest) < r.conf.RefreshInterval*time.Minute {
			continue
		}
		wg.Add(1)
		go func(address string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			result, err := TestAddr(address, r.conf.TestDomains)
			if err != nil || result == nil {
				return
			}
			r.mu.Lock()
			if existing := r.resolvers.Get(address); existing != nil {
				existing.Latency = result.Latency
				existing.LastTest = time.Now()
			}
			r.mu.Unlock()
		}(resolver.IP)
	}
	wg.Wait()
}
func (r *ResolverService) scanRange(ctx context.Context, cidr string, ipChan chan<- string) {
	ip, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return
	}

	for ip := ip.Mask(ipnet.Mask); ipnet.Contains(ip); incIP(ip) {
		select {
		case <-ctx.Done():
			return
		default:
			tempIP := make(net.IP, len(ip))
			copy(tempIP, ip)

			ones, _ := ipnet.Mask.Size()
			if ones < 31 {
				if tempIP.Equal(ipnet.IP) || isBroadcast(tempIP, ipnet) {
					continue
				}
			}
			ipChan <- tempIP.String()
		}
	}
}

func (r *ResolverService) scanRanges(ctx context.Context) {
	ticker := time.NewTicker(r.conf.RefreshInterval * time.Minute)
	defer ticker.Stop()
	resultChan := make(chan *types.Resolver, 1000)
	go func() {

		for result := range resultChan {
			r.mu.Lock()
			if r.resolvers.Len() >= r.conf.MaxResolvers {
				r.expireResolvers()
				if r.resolvers.Len() >= r.conf.MaxResolvers {
					r.resolvers.ReplaceWorst(result)
					r.mu.Unlock()
					continue
				}
			}
			r.resolvers.Insert(result)
			fmt.Printf("added resolver %s with latency %3fs from scan, current resolvers length: %d\n", result.IP, result.Latency.Seconds(), r.resolvers.Len())
			r.mu.Unlock()
		}
	}()

	for {
		maxWorkers := r.conf.MaxResolve
		if maxWorkers == 0 {
			maxWorkers = 100
		}

		ipChan := make(chan string, maxWorkers)
		var wg sync.WaitGroup

		for i := 0; i < maxWorkers; i++ {
			wg.Go(func() {
				for ip := range ipChan {
					r.mu.Lock()
					if _, checked := r.checkedIps[ip]; checked {
						r.mu.Unlock()
						continue
					}
					r.checkedIps[ip] = struct{}{}
					r.mu.Unlock()

					result, err := TestAddr(ip, r.conf.TestDomains)
					if err != nil || result == nil {
						continue
					}

					resultChan <- result
				}
			})
		}

		for _, rng := range r.conf.ScanRanges {
			r.scanRange(ctx, rng, ipChan)
		}
		close(ipChan)
		wg.Wait()

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.mu.Lock()
			r.checkedIps = make(map[string]struct{})
			r.mu.Unlock()
		}
	}
}
func isBroadcast(ip net.IP, ipnet *net.IPNet) bool {
	broadcast := make(net.IP, len(ipnet.IP))
	for i := 0; i < len(ipnet.IP); i++ {
		broadcast[i] = ipnet.IP[i] | ^ipnet.Mask[i]
	}
	return ip.Equal(broadcast)
}

func incIP(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}

func (r *ResolverService) GetResolvers() []*types.Resolver {
	return r.resolvers.GetAllSortedByLatency()
}

func (r *ResolverService) GetResolver(ip string) *types.Resolver {
	return r.resolvers.Get(ip)
}
