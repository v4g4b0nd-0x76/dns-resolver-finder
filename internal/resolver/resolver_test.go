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
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*1)
	defer cancel()
	err = service.Run(ctx)
	if time.Since(start).Seconds() > 2 {
		t.Fatalf("ResolverService.Run took too long: %v", time.Since(start))
	}
	if err != nil {
		t.Fatalf("ResolverService.Run returned error: %v", err)
	}
}
