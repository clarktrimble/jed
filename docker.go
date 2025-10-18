package jed

import (
	"context"
	"fmt"
)

var (
	empty = map[string]string{}
)

func (svc *Svc) create(ctx context.Context, name string, cfg containerConfig) (id string, err error) {

	svc.logger.Info(ctx, "creating container", "name", name)

	var response struct {
		Id string `json:"Id"`
	}

	path := fmt.Sprintf("/containers/create?name=%s", name)
	err = svc.client.SendObject(ctx, "POST", path, cfg, &response)
	if err != nil {
		return
	}

	id = response.Id
	svc.logger.Info(ctx, "created container", "name", name, "id", id)
	return
}

func (svc *Svc) containers(ctx context.Context) (statuses []Status, err error) {

	path := `/containers/json?all=true&filters={"label":["managed_by=jed"]}`
	err = svc.client.SendObject(ctx, "GET", path, nil, &statuses)
	return
}

func (svc *Svc) start(ctx context.Context, name string) (err error) {

	svc.logger.Info(ctx, "starting container", "name", name)

	path := fmt.Sprintf("/containers/%s/start", name)
	err = svc.client.SendObject(ctx, "POST", path, empty, nil)
	return
}

func (svc *Svc) stop(ctx context.Context, name string) (err error) {

	svc.logger.Info(ctx, "stopping container", "name", name)

	path := fmt.Sprintf("/containers/%s/stop", name)
	err = svc.client.SendObject(ctx, "POST", path, empty, nil)
	return
}

func (svc *Svc) delete(ctx context.Context, name string) (err error) {

	svc.logger.Info(ctx, "deleting container", "name", name)

	path := fmt.Sprintf("/containers/%s", name)
	err = svc.client.SendObject(ctx, "DELETE", path, nil, nil)
	return
}

func (svc *Svc) checkImage(ctx context.Context, name string) (err error) {

	path := fmt.Sprintf("/images/%s/json", name)
	err = svc.client.SendObject(ctx, "GET", path, nil, nil)
	return
}
