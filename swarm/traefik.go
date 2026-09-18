package swarm

import (
	"fmt"

	"github.com/clarktrimble/jed"
)

const authMiddleware = "ingress-auth"

// traefikLabels generates Docker-provider Traefik labels for PathPrefix routing
func traefikLabels(name string, trfk *jed.Traefik) map[string]string {

	routerRuleKey := fmt.Sprintf("traefik.http.routers.%s.rule", name)
	routerEntrypointsKey := fmt.Sprintf("traefik.http.routers.%s.entrypoints", name)
	routerTLSKey := fmt.Sprintf("traefik.http.routers.%s.tls", name)
	routerMiddlewaresKey := fmt.Sprintf("traefik.http.routers.%s.middlewares", name)
	servicePortKey := fmt.Sprintf("traefik.http.services.%s.loadbalancer.server.port", name)
	stripMiddlewareKey := fmt.Sprintf("traefik.http.middlewares.%s-strip.stripprefix.prefixes", name)

	labels := map[string]string{
		"traefik.enable":     "true",
		routerRuleKey:        fmt.Sprintf("PathPrefix(`/%s`)", name),
		routerEntrypointsKey: "websecure",
		routerTLSKey:         "true",
		servicePortKey:       trfk.Port,
	}

	if !trfk.PreservePathPrefix {
		labels[stripMiddlewareKey] = "/" + name
	}

	switch {
	case trfk.BypassAuth && trfk.PreservePathPrefix:
		// Neither middleware is needed.
	case trfk.BypassAuth:
		labels[routerMiddlewaresKey] = fmt.Sprintf("%s-strip", name)
	case trfk.PreservePathPrefix:
		labels[routerMiddlewaresKey] = authMiddleware
	default:
		labels[routerMiddlewaresKey] = fmt.Sprintf("%s,%s-strip", authMiddleware, name)
	}

	return labels
}
