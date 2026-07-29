package swarm

import (
	"context"
	"io"
	"net/http"

	"github.com/clarktrimble/delish/respond"
	"github.com/pkg/errors"
)

// Router specifies a router ala stdlib http.ServeMux.
type Router interface {
	HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request))
}

// Register registers Swarm API routes with rtr.
// See paths.yaml for openapi snippet.
func (d *Swarm) Register(rtr Router) {
	h := &apiHandlers{swarm: d}

	rtr.HandleFunc("GET /swarm/secrets", h.listSecrets)
	rtr.HandleFunc("POST /swarm/secrets/{name}", h.createSecret)
	rtr.HandleFunc("GET /swarm/configs", h.listConfigs)
	rtr.HandleFunc("GET /swarm/configs/{name}", h.getConfig)
	rtr.HandleFunc("POST /swarm/configs/{name}", h.createConfig)
	rtr.HandleFunc("DELETE /swarm/configs/by_id/{id}", h.deleteConfig)
}

type apiHandlers struct {
	swarm *Swarm
}

func (h *apiHandlers) listSecrets(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rp := respond.New(w, h.swarm.logger)

	secrets, err := h.swarm.ListSecrets(ctx)
	if err != nil {
		h.writeError(ctx, rp, err)
		return
	}
	rp.WriteObject(ctx, secrets)
}

func (h *apiHandlers) createSecret(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rp := respond.New(w, h.swarm.logger)

	data, err := io.ReadAll(r.Body)
	if err != nil {
		rp.NotOk(ctx, http.StatusBadRequest, err)
		return
	}

	id, err := h.swarm.CreateSecret(ctx, r.PathValue("name"), data)
	if err != nil {
		h.writeError(ctx, rp, err)
		return
	}
	rp.WriteObject(ctx, IDResponse{ID: id})
}

func (h *apiHandlers) listConfigs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rp := respond.New(w, h.swarm.logger)

	configs, err := h.swarm.ListConfigs(ctx)
	if err != nil {
		h.writeError(ctx, rp, err)
		return
	}
	rp.WriteObject(ctx, configs)
}

func (h *apiHandlers) getConfig(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rp := respond.New(w, h.swarm.logger)

	config, err := h.swarm.GetLatestConfig(ctx, r.PathValue("name"))
	if err != nil {
		h.writeError(ctx, rp, err)
		return
	}
	rp.WriteObject(ctx, config)
}

func (h *apiHandlers) createConfig(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rp := respond.New(w, h.swarm.logger)

	data, err := io.ReadAll(r.Body)
	if err != nil {
		rp.NotOk(ctx, http.StatusBadRequest, err)
		return
	}

	id, err := h.swarm.CreateConfig(ctx, r.PathValue("name"), data)
	if err != nil {
		h.writeError(ctx, rp, err)
		return
	}
	rp.WriteObject(ctx, IDResponse{ID: id})
}

func (h *apiHandlers) writeError(ctx context.Context, rp *respond.Respond, err error) {
	if errors.Is(err, ErrNotFound) {
		rp.NotOk(ctx, http.StatusNotFound, err)
		return
	}
	rp.NotOk(ctx, http.StatusInternalServerError, err)
}

func (h *apiHandlers) deleteConfig(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rp := respond.New(w, h.swarm.logger)

	if err := h.swarm.DeleteConfig(ctx, r.PathValue("id")); err != nil {
		h.writeError(ctx, rp, err)
		return
	}
	rp.Ok(ctx)
}
