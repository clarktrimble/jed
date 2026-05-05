package swarm

import (
	"fmt"

	"github.com/clarktrimble/jed"
)

// traefikLabels generates Docker-provider Traefik labels for PathPrefix routing
// at /{name} with TLS on the websecure entrypoint. When PathPrefixStrip is
// true, it adds a stripprefix middleware.
func traefikLabels(name string, t *jed.Traefik) map[string]string {
	labels := map[string]string{
		"traefik.enable": "true",
		fmt.Sprintf("traefik.http.routers.%s.rule", name):                      fmt.Sprintf("PathPrefix(`/%s`)", name),
		fmt.Sprintf("traefik.http.routers.%s.entrypoints", name):               "websecure",
		fmt.Sprintf("traefik.http.routers.%s.tls", name):                       "true",
		fmt.Sprintf("traefik.http.services.%s.loadbalancer.server.port", name): t.Port,
	}

	if t.PathPrefixStrip {
		labels[fmt.Sprintf("traefik.http.routers.%s.middlewares", name)] = fmt.Sprintf("%s-strip", name)
		labels[fmt.Sprintf("traefik.http.middlewares.%s-strip.stripprefix.prefixes", name)] = fmt.Sprintf("/%s", name)
	}

	return labels
}
