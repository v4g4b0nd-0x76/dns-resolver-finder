package main

import (
	"context"
	"dns-resolver-finder/internal/api"
	"dns-resolver-finder/internal/resolver"
	"dns-resolver-finder/internal/telegram"
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
	apiServer, err := api.NewAPIServer(conf, resolverService)
	if err != nil {
		panic(err)
	}
	go func() {
		if err := resolverService.Run(ctx); err != nil {
			panic(err)
		}
	}()
	go func() {
		if err := apiServer.Run(ctx); err != nil {
			panic(err)
		}
	}()
	if conf.TelegramToken != "" && conf.TelegramChatID != "" {
		telegramService, err := telegram.NewTelegramService(conf, resolverService)
		if err != nil {
			panic(err)
		}
		go func() {
			if err := telegramService.Run(ctx); err != nil {
				panic(err)
			}
		}()
	}
	<-ctx.Done()
	fmt.Printf("exiting")
}
