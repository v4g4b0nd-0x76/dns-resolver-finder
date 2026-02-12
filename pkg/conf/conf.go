package conf

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

type Conf struct {
	TestDomains     []string      `mapstructure:"test_domains"`
	Sources         []string      `mapstructure:"sources"`
	MaxResolvers    int           `mapstructure:"max_resolvers"`
	ScanRanges      []string      `mapstructure:"scan_ranges"`
	RefreshInterval time.Duration `mapstructure:"refresh_interval"`
	MaxResolve      int           `mapstructure:"max_resolve"`
}

func NewConf() (*Conf, error) {
	v := viper.New()
	v.SetConfigName("conf")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	v.WatchConfig()
	v.SetDefault("test_domains", []string{"google.com", "github.com", "cloudflare.com", "chatgpt.com", "openai.com", "chat.openai.com"})
	v.SetDefault("max_resolvers", 100)
	// v.SetDefault("sources", []string{"https://public-dns.info/nameservers.txt", "https://raw.githubusercontent.com/janmasarik/resolvers/master/resolvers.txt", "https://raw.githubusercontent.com/trickest/resolvers/main/resolvers.txt", "https://github.com/trickest/resolvers/blob/main/resolvers.txt"})
	v.SetDefault("sources", []string{"https://raw.githubusercontent.com/janmasarik/resolvers/master/resolvers.txt", "https://public-dns.info/nameservers.txt"})
	v.SetDefault("scan_ranges", []string{"2.188.21.0/24"})
	v.SetDefault("refresh_interval", 1) // TODO: increase this to 60 minute later
	v.SetDefault("max_resolve", 100)
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var cfg Conf
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}
	return &cfg, nil
}
