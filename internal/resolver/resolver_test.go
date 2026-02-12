package resolver

import (
	"context"
	"dns-resolver-finder/pkg/conf"
	"sync"
	"testing"
	"time"
)

func TestResolverService_Functional(t *testing.T) {
	c := &conf.Conf{
		Sources:         []string{"https://public-dns.info/nameservers.txt"},
		MaxResolvers:    10,
		MaxResolve:      50,
		TestDomains:     []string{"google.com"},
		RefreshInterval: 1,
		ScanRanges:      []string{"8.8.8.0/30"},
	}

	service, err := NewResolverService(c)
	if err != nil {
		t.Fatalf("Failed to create ResolverService: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	go func() {
		_ = service.Run(ctx)
	}()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	found := false
	for range 20 {
		<-ticker.C
		resolvers := service.GetResolvers()
		if len(resolvers) > 0 {
			found = true
			break
		}
	}

	if !found {
		t.Error("Expected resolvers to be populated, but length is 0")
	}
}

func TestResolverService_Race(t *testing.T) {
	c := &conf.Conf{
		MaxResolvers:    100,
		MaxResolve:      10,
		RefreshInterval: 1,
		TestDomains:     []string{"google.com"},
	}
	service, _ := NewResolverService(c)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var wg sync.WaitGroup

	wg.Go(func() {
		service.refreshSources()
	})

	wg.Go(func() {
		for range 50 {
			_ = service.GetResolvers()
			time.Sleep(10 * time.Millisecond)
		}
	})

	wg.Go(func() {
		service.reEvaluateResolvers()
	})

	wg.Go(func() {
		ipChan := make(chan string, 10)
		go func() {
			service.scanRange(ctx, "1.1.1.0/24", ipChan)
			close(ipChan)
		}()
		for range ipChan {
		}
	})

	wg.Wait()
}
