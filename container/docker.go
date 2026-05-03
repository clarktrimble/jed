package container

import (
	"context"
	"fmt"
)

var (
	empty = map[string]string{}
)

func (rt *Runtime) create(ctx context.Context, name string, cfg containerConfig) (id string, err error) {

	rt.logger.Info(ctx, "creating container", "name", name)

	var response struct {
		Id string `json:"Id"`
	}

	path := fmt.Sprintf("/containers/create?name=%s", name)
	err = rt.client.SendObject(ctx, "POST", path, cfg, &response)
	if err != nil {
		return
	}

	id = response.Id
	rt.logger.Info(ctx, "created container", "name", name, "id", id)
	return
}

func (rt *Runtime) containers(ctx context.Context) (containers Containers, err error) {

	path := `/containers/json?all=true&filters={"label":["managed_by=jed"]}`
	err = rt.client.SendObject(ctx, "GET", path, nil, &containers)
	return
}

func (rt *Runtime) start(ctx context.Context, name string) (err error) {

	rt.logger.Info(ctx, "starting container", "name", name)

	path := fmt.Sprintf("/containers/%s/start", name)
	err = rt.client.SendObject(ctx, "POST", path, empty, nil)
	return
}

func (rt *Runtime) stop(ctx context.Context, name string) (err error) {

	rt.logger.Info(ctx, "stopping container", "name", name)

	path := fmt.Sprintf("/containers/%s/stop", name)
	err = rt.client.SendObject(ctx, "POST", path, empty, nil)
	return
}

func (rt *Runtime) delete(ctx context.Context, name string) (err error) {

	rt.logger.Info(ctx, "deleting container", "name", name)

	path := fmt.Sprintf("/containers/%s", name)
	err = rt.client.SendObject(ctx, "DELETE", path, nil, nil)
	return
}
