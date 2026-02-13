package telegram

import (
	"context"
	"dns-resolver-finder/internal/resolver"
	"dns-resolver-finder/pkg/conf"
	"time"
)

type TelegramService struct {
	conf            *conf.Conf
	resolverService *resolver.ResolverService
}

func NewTelegramService(conf *conf.Conf, resolverService *resolver.ResolverService) (*TelegramService, error) {
	return &TelegramService{
		conf:            conf,
		resolverService: resolverService,
	}, nil
}
func (ts *TelegramService) Run(ctx context.Context) error {
	tk := time.NewTicker(ts.conf.RefreshInterval * time.Minute)
	defer tk.Stop()
	for range tk.C {
		ts.sendToTelegram()
	}
	<-ctx.Done()
	return nil
}

func (ts *TelegramService) sendToTelegram() {
	resolvers := ts.resolverService.GetResolvers()
	if len(resolvers) == 0 {
		return
	}
}
