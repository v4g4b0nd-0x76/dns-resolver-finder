package telegram

import (
	"bytes"
	"context"
	"dns-resolver-finder/internal/resolver"
	"dns-resolver-finder/pkg/conf"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
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
	fmt.Printf("Sending top 10 resolvers to telegram\n")
	resolvers := ts.resolverService.GetResolvers()
	if len(resolvers) == 0 {
		return
	}

	botToken := ts.conf.TelegramToken
	chatID := ts.conf.TelegramChatID

	limit := min(len(resolvers), 10)

	header := "🚀 *Top DNS Resolvers*\n\n"
	var body strings.Builder
	for i, r := range resolvers[:limit] {
		fmt.Fprintf(&body, "%d. `%s` — *%dms*\n", i+1, r.IP, r.Latency.Milliseconds())
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)

	payload, _ := json.Marshal(map[string]string{
		"chat_id":    chatID,
		"text":       header + body.String(),
		"parse_mode": "Markdown",
	})

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(payload))
	if err != nil {
		fmt.Printf("Telegram error: %v\n", err)
		return
	}
	defer resp.Body.Close()
}
