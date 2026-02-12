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
func (s *APIServer) Run(ctx context.Context) error {
	server := echo.New()
	server.GET("/resolvers", func(c *echo.Context) error {
		resolvers := s.resolverService.GetResolvers()
		c.Set("Cache-Control", "no-cache, no-store, must-revalidate")
		c.Set("Content-Type", "application/json")
		return c.JSON(200, resolvers)
	})
	return server.Start(":8080")
}
