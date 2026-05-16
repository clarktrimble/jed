package jed

import (
	"encoding/json"
	"net/http"

	"github.com/clarktrimble/delish/respond"
	"github.com/clarktrimble/jed/logger"
	"github.com/pkg/errors"
)

// Router specifies a router ala stdlib http.ServeMux.
type Router interface {
	HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request))
}

// Register registers Jed store API routes with rtr.
// See paths.yaml for openapi snippet.
func (j *Jed) Register(rtr Router) {
	h := &apiHandlers{jed: j, logger: j.logger}
	rtr.HandleFunc("GET /store/services", h.listServices)
	rtr.HandleFunc("GET /store/services/{name}", h.getService)
	rtr.HandleFunc("PUT /store/services/{name}", h.setService)
	rtr.HandleFunc("DELETE /store/services/{name}", h.delService)
	rtr.HandleFunc("GET /store/envs", h.listEnvs)
	rtr.HandleFunc("GET /store/envs/{name}", h.getEnv)
	rtr.HandleFunc("PUT /store/envs/{name}", h.setEnv)
	rtr.HandleFunc("DELETE /store/envs/{name}", h.delEnv)
}

type apiHandlers struct {
	jed    *Jed
	logger logger.Logger // Todo: why not just use jed.logger as see in swarm/api.go ??
}

func (h *apiHandlers) listServices(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rp := respond.New(w, h.logger)

	services, err := h.jed.store.Services(ctx)
	if err != nil {
		rp.NotOk(ctx, http.StatusInternalServerError, err)
		return
	}
	rp.WriteObject(ctx, services)
}

func (h *apiHandlers) getService(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rp := respond.New(w, h.logger)

	service, err := h.jed.store.GetService(ctx, r.PathValue("name"))
	if err != nil {
		var notFound NotFoundError
		if errors.As(err, &notFound) {
			rp.NotOk(ctx, http.StatusNotFound, err)
			return
		}
		rp.NotOk(ctx, http.StatusInternalServerError, err)
		return
	}
	rp.WriteObject(ctx, service)
}

func (h *apiHandlers) setService(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rp := respond.New(w, h.logger)
	name := r.PathValue("name")

	var service Service
	if err := json.NewDecoder(r.Body).Decode(&service); err != nil {
		rp.NotOk(ctx, http.StatusBadRequest, errors.Wrap(err, "failed to decode service"))
		return
	}
	if service.Name == "" {
		service.Name = name
	}
	if service.Name != name {
		rp.NotOk(ctx, http.StatusUnprocessableEntity, errors.Errorf("service name %q does not match path name %q", service.Name, name))
		return
	}
	if err := service.Validate(); err != nil {
		rp.NotOk(ctx, http.StatusUnprocessableEntity, err)
		return
	}
	if err := h.jed.store.SetService(ctx, service); err != nil {
		rp.NotOk(ctx, http.StatusInternalServerError, err)
		return
	}
	rp.WriteObject(ctx, service)
}

func (h *apiHandlers) delService(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rp := respond.New(w, h.logger)

	if err := h.jed.store.DelService(ctx, r.PathValue("name")); err != nil {
		rp.NotOk(ctx, http.StatusInternalServerError, err)
		return
	}
	rp.Ok(ctx)
}

func (h *apiHandlers) listEnvs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rp := respond.New(w, h.logger)

	envs, err := h.jed.store.Envs(ctx)
	if err != nil {
		rp.NotOk(ctx, http.StatusInternalServerError, err)
		return
	}
	rp.WriteObject(ctx, envs)
}

func (h *apiHandlers) getEnv(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rp := respond.New(w, h.logger)

	env, err := h.jed.store.GetEnv(ctx, r.PathValue("name"))
	if err != nil {
		rp.NotOk(ctx, http.StatusInternalServerError, err)
		return
	}
	rp.WriteObject(ctx, env)
}

func (h *apiHandlers) setEnv(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rp := respond.New(w, h.logger)
	name := r.PathValue("name")

	var env Env
	if err := json.NewDecoder(r.Body).Decode(&env); err != nil {
		rp.NotOk(ctx, http.StatusBadRequest, errors.Wrap(err, "failed to decode env"))
		return
	}
	if env.Name == "" {
		env.Name = name
	}
	if env.Name != name {
		rp.NotOk(ctx, http.StatusUnprocessableEntity, errors.Errorf("env name %q does not match path name %q", env.Name, name))
		return
	}
	if env.Vars == nil {
		env.Vars = map[string]string{}
	}
	if err := h.jed.store.SetEnv(ctx, env); err != nil {
		rp.NotOk(ctx, http.StatusInternalServerError, err)
		return
	}
	rp.WriteObject(ctx, env)
}

func (h *apiHandlers) delEnv(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rp := respond.New(w, h.logger)

	if err := h.jed.store.DelEnv(ctx, r.PathValue("name")); err != nil {
		rp.NotOk(ctx, http.StatusInternalServerError, err)
		return
	}
	rp.Ok(ctx)
}
