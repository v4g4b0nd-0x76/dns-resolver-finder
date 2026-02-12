package main

import (
	"context"
	"dns-resolver-finder/internal/resolver"
	"dns-resolver-finder/pkg/conf"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGABRT, syscall.SIGINT, os.Kill)
	defer cancel()
	conf, err := conf.NewConf()
	if err != nil {
		panic(err)
	}
	resolverService, err := resolver.NewResolverService(conf)
	if err != nil {
		panic(err)
	}
	go func() {
		if err := resolverService.Run(ctx); err != nil {
			panic(err)
		}
	}()
	<-ctx.Done()
	fmt.Printf("exiting")
}
