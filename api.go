package jed

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"

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
	rtr.HandleFunc("GET /store/export", h.exportStore)
	rtr.HandleFunc("PUT /store/import", h.importStore)
	rtr.HandleFunc("GET /store/services", h.listServices)
	rtr.HandleFunc("GET /store/services/{name}", h.getService)
	rtr.HandleFunc("PUT /store/services/{name}", h.setService)
	rtr.HandleFunc("DELETE /store/services/{name}", h.delService)
	rtr.HandleFunc("GET /store/envs", h.listEnvs)
	rtr.HandleFunc("GET /store/envs/{name}", h.getEnv)
	rtr.HandleFunc("PUT /store/envs/{name}", h.setEnv)
	rtr.HandleFunc("DELETE /store/envs/{name}", h.delEnv)
	// Todo: add /store/intents routes when the HTTP store API needs intent parity.
}

type apiHandlers struct {
	jed    *Jed
	logger logger.Logger // Todo: why not just use jed.logger as see in swarm/api.go ??
}

type storeExport struct {
	Schema   string    `json:"schema"`
	Services []Service `json:"services"`
	Envs     []Env     `json:"envs"`
	Intents  []Intent  `json:"intents"`
}

type storeImportResponse struct {
	Services int `json:"services"`
	Envs     int `json:"envs"`
	Intents  int `json:"intents"`
}

func (h *apiHandlers) exportStore(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rp := respond.New(w, h.logger)

	services, err := h.jed.store.AllServices(ctx)
	if err != nil {
		rp.NotOk(ctx, http.StatusInternalServerError, err)
		return
	}
	envs, err := h.jed.store.Envs(ctx)
	if err != nil {
		rp.NotOk(ctx, http.StatusInternalServerError, err)
		return
	}
	intents, err := h.jed.store.Intents(ctx)
	if err != nil {
		rp.NotOk(ctx, http.StatusInternalServerError, err)
		return
	}

	// Todo: consider requiring non-nil empty slices in the Store contract tests instead.
	if services == nil {
		services = []Service{}
	}
	if envs == nil {
		envs = []Env{}
	}
	if intents == nil {
		intents = []Intent{}
	}

	sort.Slice(services, func(i, j int) bool {
		if services[i].Name == services[j].Name {
			return services[i].Image < services[j].Image
		}
		return services[i].Name < services[j].Name
	})
	sort.Slice(envs, func(i, j int) bool {
		return envs[i].Name < envs[j].Name
	})
	sort.Slice(intents, func(i, j int) bool {
		return intents[i].Name < intents[j].Name
	})

	rp.WriteObject(ctx, storeExport{
		Schema:   DBSchemaVersion,
		Services: services,
		Envs:     envs,
		Intents:  intents,
	})
}

func (h *apiHandlers) importStore(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rp := respond.New(w, h.logger)

	var payload storeExport
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		rp.NotOk(ctx, http.StatusBadRequest, errors.Wrap(err, "failed to decode store import"))
		return
	}
	if payload.Schema != DBSchemaVersion {
		rp.NotOk(ctx, http.StatusUnprocessableEntity, errors.Errorf("store import schema %q does not match current schema %q", payload.Schema, DBSchemaVersion))
		return
	}

	empty, err := h.storeIsEmpty(ctx)
	if err != nil {
		rp.NotOk(ctx, http.StatusInternalServerError, err)
		return
	}
	if !empty {
		rp.NotOk(ctx, http.StatusConflict, errors.New("store is not empty"))
		return
	}

	serviceKeys := map[string]struct{}{}
	for _, service := range payload.Services {
		if err := service.Validate(); err != nil {
			rp.NotOk(ctx, http.StatusUnprocessableEntity, err)
			return
		}
		key := serviceName(service.Name, service.Image)
		if _, exists := serviceKeys[key]; exists {
			rp.NotOk(ctx, http.StatusUnprocessableEntity, errors.Errorf("duplicate service %q", key))
			return
		}
		serviceKeys[key] = struct{}{}
	}
	envNames := map[string]struct{}{}
	for _, env := range payload.Envs {
		if env.Name == "" {
			rp.NotOk(ctx, http.StatusUnprocessableEntity, errors.New("env name is required"))
			return
		}
		if _, exists := envNames[env.Name]; exists {
			rp.NotOk(ctx, http.StatusUnprocessableEntity, errors.Errorf("duplicate env %q", env.Name))
			return
		}
		envNames[env.Name] = struct{}{}
	}
	intentNames := map[string]struct{}{}
	for _, intent := range payload.Intents {
		if err := intent.Validate(); err != nil {
			rp.NotOk(ctx, http.StatusUnprocessableEntity, err)
			return
		}
		if _, exists := intentNames[intent.Name]; exists {
			rp.NotOk(ctx, http.StatusUnprocessableEntity, errors.Errorf("duplicate intent %q", intent.Name))
			return
		}
		intentNames[intent.Name] = struct{}{}
		if _, ok := serviceKeys[serviceName(intent.Name, intent.Image)]; !ok {
			rp.NotOk(ctx, http.StatusUnprocessableEntity, errors.Errorf("intent %q references missing service image %q", intent.Name, intent.Image))
			return
		}
	}

	for _, service := range payload.Services {
		if err := h.jed.store.SetService(ctx, service); err != nil {
			rp.NotOk(ctx, http.StatusInternalServerError, err)
			return
		}
	}
	for _, env := range payload.Envs {
		if env.Vars == nil {
			env.Vars = map[string]string{}
		}
		if err := h.jed.store.SetEnv(ctx, env); err != nil {
			rp.NotOk(ctx, http.StatusInternalServerError, err)
			return
		}
	}
	for _, intent := range payload.Intents {
		if err := h.jed.store.SetIntent(ctx, intent); err != nil {
			rp.NotOk(ctx, http.StatusInternalServerError, err)
			return
		}
	}

	rp.WriteObject(ctx, storeImportResponse{
		Services: len(payload.Services),
		Envs:     len(payload.Envs),
		Intents:  len(payload.Intents),
	})
}

func (h *apiHandlers) storeIsEmpty(ctx context.Context) (bool, error) {
	services, err := h.jed.store.AllServices(ctx)
	if err != nil {
		return false, err
	}
	envs, err := h.jed.store.Envs(ctx)
	if err != nil {
		return false, err
	}
	intents, err := h.jed.store.Intents(ctx)
	if err != nil {
		return false, err
	}
	return len(services) == 0 && len(envs) == 0 && len(intents) == 0, nil
}

func serviceName(name, image string) string {
	return name + "@" + image
}

func (h *apiHandlers) listServices(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rp := respond.New(w, h.logger)

	// Todo: name in query is bust?
	name := r.URL.Query().Get("name")
	var services []Service
	var err error
	if name == "" {
		services, err = h.jed.store.AllServices(ctx)
	} else {
		services, err = h.jed.store.Services(ctx, name)
	}
	if err != nil {
		rp.NotOk(ctx, http.StatusInternalServerError, err)
		return
	}
	rp.WriteObject(ctx, services)
}

func (h *apiHandlers) getService(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rp := respond.New(w, h.logger)

	// Todo: image in query is bust?
	image := r.URL.Query().Get("image")
	service, err := h.jed.store.GetService(ctx, r.PathValue("name"), image)
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

	// Todo: image in query is bust?
	image := r.URL.Query().Get("image")
	err := h.jed.store.DelService(ctx, r.PathValue("name"), image)
	if err != nil {
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
