package jed

import (
	"context"
	"fmt"
)

var (
	empty = map[string]string{}
)

func (jed *Jed) create(ctx context.Context, name string, cfg containerConfig) (id string, err error) {

	jed.logger.Info(ctx, "creating container", "name", name)

	var response struct {
		Id string `json:"Id"`
	}

	path := fmt.Sprintf("/containers/create?name=%s", name)
	err = jed.client.SendObject(ctx, "POST", path, cfg, &response)
	if err != nil {
		return
	}

	id = response.Id
	jed.logger.Info(ctx, "created container", "name", name, "id", id)
	return
}

func (jed *Jed) containers(ctx context.Context) (containers []Container, err error) {

	path := `/containers/json?all=true&filters={"label":["managed_by=jed"]}`
	err = jed.client.SendObject(ctx, "GET", path, nil, &containers)
	return
}

func (jed *Jed) start(ctx context.Context, name string) (err error) {

	jed.logger.Info(ctx, "starting container", "name", name)

	path := fmt.Sprintf("/containers/%s/start", name)
	err = jed.client.SendObject(ctx, "POST", path, empty, nil)
	return
}

func (jed *Jed) stop(ctx context.Context, name string) (err error) {

	jed.logger.Info(ctx, "stopping container", "name", name)

	path := fmt.Sprintf("/containers/%s/stop", name)
	err = jed.client.SendObject(ctx, "POST", path, empty, nil)
	return
}

func (jed *Jed) delete(ctx context.Context, name string) (err error) {

	jed.logger.Info(ctx, "deleting container", "name", name)

	path := fmt.Sprintf("/containers/%s", name)
	err = jed.client.SendObject(ctx, "DELETE", path, nil, nil)
	return
}

func (jed *Jed) checkImage(ctx context.Context, name string) (err error) {

	path := fmt.Sprintf("/images/%s/json", name)
	err = jed.client.SendObject(ctx, "GET", path, nil, nil)
	return
}
