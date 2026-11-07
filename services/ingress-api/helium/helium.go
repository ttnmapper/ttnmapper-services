package helium

import (
	"github.com/go-chi/chi"
	"ttnmapper-ingress-api/types"
)

type Context struct {
	// ValidateEmail overrides DNS-backed validation when supplied; nil uses the production validator.
	ValidateEmail  func(string) error
	PublishChannel chan types.TtnMapperUplinkMessage
}

func HeliumRoutes(publishChannel chan types.TtnMapperUplinkMessage) *chi.Mux {
	context := &Context{PublishChannel: publishChannel}

	router := chi.NewRouter()

	router.Post("/v1", context.PostHelium)
	router.Get("/v1", context.GetHelium)

	return router
}
