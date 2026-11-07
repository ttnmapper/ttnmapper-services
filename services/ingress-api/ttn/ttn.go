package ttn

import (
	"github.com/go-chi/chi"
	"ttnmapper-ingress-api/types"
)

type Context struct {
	// ValidateEmail overrides DNS-backed validation when supplied; nil uses the production validator.
	ValidateEmail  func(string) error
	PublishChannel chan types.TtnMapperUplinkMessage
}

func TtnRoutes(publishChannel chan types.TtnMapperUplinkMessage) *chi.Mux {
	context := &Context{PublishChannel: publishChannel}

	router := chi.NewRouter()

	router.Post("/v2", context.PostTtnV2)
	router.Get("/v2", context.GetTtnV2)

	return router
}
