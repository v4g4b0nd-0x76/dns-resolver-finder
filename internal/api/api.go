package api

import (
	"context"
	"dns-resolver-finder/internal/resolver"
	"dns-resolver-finder/pkg/conf"

	"github.com/labstack/echo/v5"
)

type APIServer struct {
	conf            *conf.Conf
	resolverService *resolver.ResolverService
}

func NewAPIServer(conf *conf.Conf, resolverService *resolver.ResolverService) (*APIServer, error) {
	return &APIServer{
		conf:            conf,
		resolverService: resolverService,
	}, nil
}

type ResolverResponse struct {
	IP      string `json:"ip"`
	Latency int64  `json:"latency_ms"`
}

func (s *APIServer) Run(ctx context.Context) error {
	server := echo.New()

	server.GET("/health", func(c *echo.Context) error {
		return c.String(200, "OK")
	})
	server.GET("/resolvers", func(c *echo.Context) error {
		resolvers := s.resolverService.GetResolvers()
		c.Set("Cache-Control", "no-cache, no-store, must-revalidate")
		c.Set("Content-Type", "application/json")
		resp := make([]ResolverResponse, len(resolvers))
		for i, r := range resolvers {
			resp[i] = ResolverResponse{
				IP:      r.IP,
				Latency: r.Latency.Milliseconds(),
			}
		}
		return c.JSON(200, resp)
	})
	server.GET("/resolvers/:ip", func(c *echo.Context) error {
		ip := c.Param("ip")
		resolver := s.resolverService.GetResolver(ip)
		if resolver == nil {
			return c.JSON(404, map[string]string{"error": "resolver not found"})
		}
		c.Set("Cache-Control", "no-cache, no-store, must-revalidate")
		c.Set("Content-Type", "application/json")
		resp := ResolverResponse{
			IP:      resolver.IP,
			Latency: resolver.Latency.Milliseconds(),
		}
		return c.JSON(200, resp)
	})

	return server.Start(":8080")
}
