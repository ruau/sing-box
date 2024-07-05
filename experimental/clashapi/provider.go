package clashapi

import (
	"context"
	"net/http"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing/common/json/badjson"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
)

func proxyProviderRouter(server *Server, router adapter.Router) http.Handler {
	r := chi.NewRouter()
	r.Get("/", getProviders(server, router))

	r.Route("/{name}", func(r chi.Router) {
		r.Use(parseProviderName, findProviderByName(router))
		r.Get("/", getProvider(server))
		r.Put("/", updateProvider)
		r.Get("/healthcheck", healthCheckProvider)
	})
	return r
}

func getProviders(server *Server, router adapter.Router) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		providers := router.OutboundProviders()
		m := render.M{}
		for _, provider := range providers {
			m[provider.Tag()] = proxyProviderInfo(server, provider)
		}
		render.JSON(w, r, render.M{
			"providers": m,
		})
	}
}

func getProvider(server *Server) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		provider := r.Context().Value(CtxKeyProvider).(adapter.OutboundProvider)
		render.JSON(w, r, proxyInfo(server, provider))
	}
}

func updateProvider(w http.ResponseWriter, r *http.Request) {
	provider := r.Context().Value(CtxKeyProvider).(adapter.OutboundProvider)
	if err := provider.Update(r.Context()); err != nil {
		render.Status(r, http.StatusInternalServerError)
		render.JSON(w, r, newError(err.Error()))
		return
	}
	render.NoContent(w, r)
}

func healthCheckProvider(w http.ResponseWriter, r *http.Request) {
	render.NoContent(w, r)
}

func parseProviderName(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := getEscapeParam(r, "name")
		ctx := context.WithValue(r.Context(), CtxKeyProviderName, name)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func findProviderByName(router adapter.Router) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			name := r.Context().Value(CtxKeyProviderName).(string)
			provider, exist := router.OutboundProvider(name)
			if !exist {
				render.Status(r, http.StatusNotFound)
				render.JSON(w, r, ErrNotFound)
				return
			}
			ctx := context.WithValue(r.Context(), CtxKeyProvider, provider)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func proxyProviderInfo(server *Server, provider adapter.OutboundProvider) *badjson.JSONObject {
	var info badjson.JSONObject
	info.Put("name", provider.Tag())
	info.Put("type", "Proxy")
	info.Put("vehicleType", "HTTP")
	subscriptionInfo := render.M{}
	providerInfo := provider.ProviderInfo()
	subscriptionInfo["Download"] = providerInfo.Download
	subscriptionInfo["Upload"] = providerInfo.Upload
	subscriptionInfo["Total"] = providerInfo.Total
	subscriptionInfo["Expire"] = providerInfo.Expired.Unix()
	info.Put("subscriptionInfo", subscriptionInfo)
	info.Put("updatedAt", providerInfo.LastUpdated)
	outbounds := provider.BasicOutbounds()
	proxies := make([]*badjson.JSONObject, 0, len(outbounds))
	for _, out := range outbounds {
		proxies = append(proxies, proxyInfo(server, out))
	}
	info.Put("proxies", proxies)
	return &info
}
