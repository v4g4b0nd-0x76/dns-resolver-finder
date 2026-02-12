package resolver

import (
	"context"
	"dns-db/pkg/conf"
	"testing"
	"time"
)

func TestResolverService(t *testing.T) {
	conf, err := conf.NewConf()
	if err != nil {
		t.Fatalf("Failed to create Conf: %v", err)
	}
	service, err := NewResolverService(conf)
	if err != nil {
		t.Fatalf("Failed to create ResolverService: %v", err)
	}
	if service == nil {
		t.Fatalf("Expected non-nil ResolverService")
	}
	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	err = service.Run(ctx)
	if time.Since(start).Seconds() > 11 {
		t.Fatalf("ResolverService.Run took too long: %v", time.Since(start))
	}
	if err != nil {
		t.Fatalf("ResolverService.Run returned error: %v", err)
	}
	if len(service.resolvers) == 0 {
		t.Fatalf("Expected at least one resolver, got 0")
	}
}
