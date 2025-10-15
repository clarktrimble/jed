package docker

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"io"
	"maps"
	"slices"
	"strings"
	//"github.com/ahmetb/dlog"
	//"github.com/joho/godotenv"
	//"github.com/pkg/errors"
	//"sigs.k8s.io/yaml"
)

var empty = map[string]string{}

// Client specifies an http client by which stuff can be sent and received.
type Client interface {
	SendObject(ctx context.Context, method, path string, snd, rcv any) (err error)
	SendJson(ctx context.Context, method, path string, body io.Reader) (data []byte, err error)
}

// Logger specifies a contextual, structured logger.
type Logger interface {
	Info(ctx context.Context, msg string, kv ...any)
	Error(ctx context.Context, msg string, err error, kv ...any)
}

// Condition represents the state of a managed container.
type Condition string

const (
	Undeployed Condition = "undeployed"
	Unchecked  Condition = "unchecked"
	Nominal    Condition = "nominal"
	NotStarted Condition = "not_started"
	NotCreated Condition = "not_created"
)

// Config specifies docker service configuration.
type Config struct {
	Prefix string `json:"prefix" desc:"container name prefix" required:"true"`
}

type Container struct {
	Image      string            `json:"image"`
	Name       string            `json:"name"`
	Env        map[string]string `json:"env,omitempty"`
	EnvForm    string            `json:"env_form"`
	Ports      map[string]string `json:"ports,omitempty"`
	Labels     map[string]string `json:"labels,omitempty"`
	Volumes    map[string]string `json:"volumes,omitempty"`
	Network    string            `json:"network"`
	Restart    string            `json:"restart"`
	AutoDeploy bool              `json:"auto_deploy"`
	Features   []string          `json:"features,omitempty"`
	Id         string            `json:"id,omitempty"`
	Condition  Condition         `json:"condition"`
}

type Status struct {
	Id      string            `json:"Id"`
	Names   []string          `json:"Names"`
	Image   string            `json:"Image"`
	State   string            `json:"State"`
	Status  string            `json:"Status"`
	Labels  map[string]string `json:"Labels"`
	Created int64             `json:"Created"`
}

type Svc struct {
	client     Client
	containers []Container
	logger     Logger
	prefix     string
}

// New creates a docker service from a Client and embedded filesystem.
func (cfg *Config) New(client Client, containersFS embed.FS, lgr Logger) (svc *Svc, err error) {

	containers, err := loadFs(containersFS)
	if err != nil {
		return
	}

	svc = &Svc{
		client:     client,
		containers: containers,
		logger:     lgr,
		prefix:     cfg.Prefix,
	}
	return
}

// Deploy creates and starts a container.
func (svc *Svc) Deploy(ctx context.Context, cntr *Container) (id string, err error) {

	id, err = svc.create(ctx, cntr)
	if err != nil {
		return
	}

	cntr.Id = id

	err = svc.start(ctx, cntr)
	if err != nil {
		return
	}

	cntr.Condition = Unchecked
	return
}

// Undeploy stops and removes a container.
func (svc *Svc) Undeploy(ctx context.Context, cntr *Container) (err error) {

	err = svc.stop(ctx, cntr)
	if err != nil {
		// Todo: easy to get hung up on created but not started, think thru
		//return
		svc.logger.Error(ctx, "failed to stop container", err)
		// best effort
	}

	err = svc.delete(ctx, cntr)
	if err != nil {
		return
	}

	cntr.Id = ""
	cntr.Condition = Undeployed
	return
}

// AutoDeploy deploys all containers marked for auto-deployment.
func (svc *Svc) AutoDeploy(ctx context.Context) (err error) {

	// Todo: sort out how to get started properly plz
	// Todo: this pattern of calls is wide-spread and pathologic
	statuses, err := svc.Statuses(ctx)
	if err != nil {
		return err
	}

	svc.UpdateIds(ctx, statuses)
	svc.Check(ctx, statuses)
	// end sort out how

	for i := range svc.containers {
		cntr := &svc.containers[i]
		if cntr.AutoDeploy && cntr.Condition == Undeployed {

			svc.logger.Info(ctx, "auto-deploying container", "name", cntr.Name)

			_, err = svc.Deploy(ctx, cntr) // Todo: use or loose returned id
			if err != nil {
				return
			}
		}
	}

	return
}

// Redeploy undeploys and then deploys a container.
func (svc *Svc) Redeploy(ctx context.Context, cntr *Container) (id string, err error) {

	err = svc.Undeploy(ctx, cntr)
	if err != nil {
		return
	}

	id, err = svc.Deploy(ctx, cntr)
	return
}

// Restart restarts an existing container using Docker's restart API.
func (svc *Svc) Restart(ctx context.Context, cntr *Container) (err error) {

	svc.logger.Info(ctx, "restarting container", "name", cntr.Name)

	path := fmt.Sprintf("/containers/%s/restart", svc.Prefixed(cntr.Name))
	err = svc.client.SendObject(ctx, "POST", path, empty, nil)
	return
}

// GetContainer gets container by name.
func (svc *Svc) GetContainer(name string) (cntr *Container, err error) {

	idx := slices.IndexFunc(svc.containers, func(c Container) bool {
		return c.Name == name
	})
	if idx == -1 {
		err = errors.Errorf("container %s not found", name)
		return
	}

	cntr = &svc.containers[idx]
	return
}

// GetContainerLogs retrieves logs from a container using Docker API.
func (svc *Svc) GetContainerLogs(ctx context.Context, cntr *Container, tail string) (logs []byte, err error) {

	if cntr.Id == "" {
		err = errors.Errorf("container %s has no ID, may not be deployed", cntr.Name)
		return
	}

	svc.logger.Info(ctx, "getting container logs", "name", cntr.Name, "id", cntr.Id)

	// Todo: prolly dont want stderr
	path := fmt.Sprintf("/containers/%s/logs?stdout=true&stderr=true&tail=%s", cntr.Id, tail)

	rawLogs, err := svc.client.SendJson(ctx, "GET", path, nil)
	if err != nil {
		err = errors.Wrapf(err, "failed to get logs for container %s", cntr.Name)
		return
	}

	reader := dlog.NewReader(bytes.NewReader(rawLogs))
	logs, err = io.ReadAll(reader)
	if err != nil {
		err = errors.Wrapf(err, "failed to parse docker logs for container %s", cntr.Name)
		return
	}

	return
}

/*
// parseDockerLogFormat strips Docker's 8-byte header from each log line
func parseDockerLogFormat(rawLogs []byte) []byte {

	// Todo: compare to dlog and decide

	if len(rawLogs) == 0 {
		return rawLogs
	}

	var result []byte
	i := 0

	for i < len(rawLogs) {
		// Each log entry starts with 8-byte header: [STREAM_TYPE, 0, 0, 0, SIZE_BE_UINT32]
		if i+8 > len(rawLogs) {
			break
		}

		// Extract length from bytes 4-7 (big endian uint32)
		length := int(rawLogs[i+4])<<24 | int(rawLogs[i+5])<<16 | int(rawLogs[i+6])<<8 | int(rawLogs[i+7])

		// Skip the 8-byte header
		i += 8

		// Extract the log content
		if i+length > len(rawLogs) {
			break
		}

		logContent := rawLogs[i : i+length]
		result = append(result, logContent...)

		i += length
	}

	return result
}
*/

// Down stops and removes all containers defined in service.
func (svc *Svc) Down(ctx context.Context) (err error) {

	for idx := range svc.containers {

		err = svc.Undeploy(ctx, &svc.containers[idx])
		if err != nil {
			return
		}
	}

	return
}

// PatchEnv updates environment variables for a container.
func (svc *Svc) PatchEnv(ctx context.Context, cntr *Container, env map[string]string) (err error) {

	svc.logger.Info(ctx, "patching container", "name", cntr.Name, "env", env)

	if cntr.Env == nil {
		err = errors.Errorf("cannot patch nil env for container: %s", cntr.Name)
		return
	}

	maps.Copy(cntr.Env, env)
	return
}

func (svc *Svc) Containers() (cntrs []Container) {
	// Todo: only a shallow copy, look at deep or ???
	cntrs = make([]Container, len(svc.containers))
	copy(cntrs, svc.containers)
	return
}

// Statuses returns all containers managed by this service.
func (svc *Svc) Statuses(ctx context.Context) (statii map[string]Status, err error) {

	var all []Status
	err = svc.client.SendObject(ctx, "GET", "/containers/json?all=true", nil, &all)
	if err != nil {
		return
	}

	prefix := "/" + svc.prefix + "-"
	filtered := slices.DeleteFunc(all, func(status Status) bool {
		return !slices.ContainsFunc(status.Names, func(name string) bool {
			return strings.HasPrefix(name, prefix)
		})
	})

	statii = map[string]Status{}
	for _, status := range filtered {
		statii[status.Id] = status
	}

	return
}

func (svc *Svc) Prefixed(name string) string {
	return svc.prefix + "-" + name
}

func (svc *Svc) UpdateIds(ctx context.Context, statuses map[string]Status) {

	// Todo: want this apart from Check??
	//       need to untangle now that we have a use case

	for _, status := range statuses {
		for _, dockerName := range status.Names {
			for i, cntr := range svc.containers {
				if dockerName == "/"+svc.Prefixed(cntr.Name) {
					svc.containers[i].Id = status.Id
				}
			}
		}
	}
}

// Check validates that our managed state matches Docker reality.
func (svc *Svc) Check(ctx context.Context, statuses map[string]Status) {

	// Work with a copy to avoid modifying the input
	statii := maps.Clone(statuses)

	deployed := map[string]*Container{}
	for idx, cntr := range svc.containers {
		if cntr.Id != "" {
			deployed[cntr.Id] = &svc.containers[idx]
		}
	}

	for id, status := range statii {
		cntr, ok := deployed[id]
		if ok {
			if status.State == "running" {
				cntr.Condition = Nominal
			} else {
				cntr.Condition = NotStarted
				err := errors.Errorf("container not running")
				svc.logger.Error(ctx, "deployed container not started", err, "name", cntr.Name, "status", status)
			}
			delete(statii, id)
			delete(deployed, id)
		}
	}

	for _, status := range statii {
		err := errors.Errorf("unexpected container")
		svc.logger.Error(ctx, "unmanged container found with managed prefix", err, "prefix", svc.prefix, "status", status)
		// Todo: think about adding to svc.containers as "unmanaged", but what then?
	}

	for _, cntr := range deployed {
		cntr.Condition = NotCreated
		err := errors.Errorf("expected deployment")
		svc.logger.Error(ctx, "deployed container not found in docker", err, "name", cntr.Name)
	}
}

// unexported

func (cntr *Container) validate() error {
	var issues []string

	if cntr.Image == "" {
		issues = append(issues, "image is required")
	}
	if cntr.Name == "" {
		issues = append(issues, "name is required")
	}
	if cntr.EnvForm == "" {
		issues = append(issues, "env_form is required")
	}
	if cntr.Network == "" {
		issues = append(issues, "network is required")
	}
	if cntr.Restart == "" {
		issues = append(issues, "restart policy is required")
	}

	if len(issues) > 0 {
		return errors.Errorf("container %s invalid: %s", cntr.Name, strings.Join(issues, ", "))
	}

	return nil
}

func (svc *Svc) create(ctx context.Context, cntr *Container) (id string, err error) {

	svc.logger.Info(ctx, "creating container", "name", cntr.Name)
	// Todo: use prefixed name in logs??

	err = cntr.validate()
	if err != nil {
		return
	}

	err = svc.checkImage(ctx, cntr.Image)
	if err != nil {
		return
	}

	exposedPorts, portBindings := buildPortConfig(cntr.Ports)

	hostConfig := map[string]any{
		"PortBindings": portBindings,
		"Binds":        buildVolumeConfig(cntr.Volumes),
		"RestartPolicy": map[string]any{
			"Name": cntr.Restart,
		},
	}

	config := map[string]any{
		"Image":        cntr.Image,
		"Env":          envLines(cntr.Env),
		"Labels":       cntr.Labels,
		"ExposedPorts": exposedPorts,
		"HostConfig":   hostConfig,
		"NetworkingConfig": map[string]any{
			"EndpointsConfig": map[string]any{
				cntr.Network: map[string]any{},
			},
		},
	}

	var response struct {
		Id string `json:"Id"`
	}

	path := fmt.Sprintf("/containers/create?name=%s", svc.Prefixed(cntr.Name))
	err = svc.client.SendObject(ctx, "POST", path, config, &response)
	if err != nil {
		return
	}

	id = response.Id
	svc.logger.Info(ctx, "created container", "name", cntr.Name, "id", id)
	return
}

func (svc *Svc) start(ctx context.Context, cntr *Container) (err error) {

	svc.logger.Info(ctx, "starting container", "name", cntr.Name)

	path := fmt.Sprintf("/containers/%s/start", svc.Prefixed(cntr.Name))
	err = svc.client.SendObject(ctx, "POST", path, empty, nil)
	return
}

func (svc *Svc) stop(ctx context.Context, cntr *Container) (err error) {

	svc.logger.Info(ctx, "stopping container", "name", cntr.Name)

	path := fmt.Sprintf("/containers/%s/stop", svc.Prefixed(cntr.Name))
	err = svc.client.SendObject(ctx, "POST", path, empty, nil)
	return
}

func (svc *Svc) delete(ctx context.Context, cntr *Container) (err error) {

	svc.logger.Info(ctx, "deleting container", "name", cntr.Name)

	path := fmt.Sprintf("/containers/%s", svc.Prefixed(cntr.Name))
	err = svc.client.SendObject(ctx, "DELETE", path, nil, nil)
	return
}

func (svc *Svc) checkImage(ctx context.Context, name string) (err error) {

	err = svc.client.SendObject(ctx, "GET", fmt.Sprintf("/images/%s/json", name), nil, nil)
	return
}

func loadFs(fs embed.FS) (containers []Container, err error) {

	// Todo: demajic
	//file := "containers/containers.yaml"
	file := "containers.yaml"
	data, err := fs.ReadFile(file)
	if err != nil {
		err = errors.Wrapf(err, "cannot read %s", file)
		return
	}

	err = yaml.Unmarshal(data, &containers)
	if err != nil {
		err = errors.Wrapf(err, "cannot unmarshal %s", file)
		return
	}

	for i, container := range containers {
		err = container.validate()
		if err != nil {
			return
		}

		var env map[string]string
		env, err = loadEnv(fs, container.Name)
		if err != nil {
			return
			// Todo: option to skip an env file??
		}
		containers[i].Env = env
		containers[i].Condition = Undeployed

		/*
			// Todo: valid is good yeah
			// Validate container configuration
			err = containers[i].validate()
			if err != nil {
				err = errors.Wrapf(err, "invalid container configuration for %s", container.Name)
				return
			}
		*/
	}

	return
}

func loadEnv(fs embed.FS, name string) (env map[string]string, err error) {

	// Todo: demajic
	//file := fmt.Sprintf("containers/%s.env", name)
	file := fmt.Sprintf("%s.env", name)
	envData, err := fs.ReadFile(file)
	if err != nil {
		err = errors.Wrapf(err, "cannot read %s", file)
		return
	}

	env, err = godotenv.Unmarshal(string(envData))
	if err != nil {
		err = errors.Wrapf(err, "cannot unmarshal %s", file)
		return
	}

	return
}

func envLines(env map[string]string) (lines []string) {

	// godotenv.Marshal mangled the vals, maybe with quotes? anyway ..

	for key, value := range env {
		lines = append(lines, fmt.Sprintf("%s=%s", key, value))
	}

	return
}

func buildPortConfig(ports map[string]string) (exposedPorts map[string]any, portBindings map[string][]map[string]string) {
	exposedPorts = make(map[string]any)
	portBindings = make(map[string][]map[string]string)

	for containerPort, hostPort := range ports {
		exposedPorts[containerPort] = map[string]any{}
		portBindings[containerPort] = []map[string]string{
			{"HostPort": hostPort},
		}
	}

	return
}

func buildVolumeConfig(volumes map[string]string) []string {
	var binds []string
	for hostPath, containerPath := range volumes {
		binds = append(binds, fmt.Sprintf("%s:%s", hostPath, containerPath))
	}
	return binds
}

//
// encapsulate bad things
//

/*
// Service is specific to whatever intadmin wants
// Todo: and therefore a little misplaced here, hopefully not to hard to fix :/
type Service struct {
}

func (svc *Svc) FreshenedContainers(ctx context.Context) (map[string]Status, []Container) {

	// Todo: understand side effects and straighten

	statuses, err := svc.Statuses(ctx)
	if err != nil {
		// Todo: do we really want to handle here?
		svc.logger.Error(ctx, "failed to get container statuses", err)
		statuses = make(map[string]Status)
	}

	svc.UpdateIds(ctx, statuses)
	svc.Check(ctx, statuses)

	return statuses, svc.Containers()
}

func (svc *Svc) apiUrl(name string) (url string, err error) {

	// Todo: GetContainer by name is a little pathologic
	//       maybe store these by name or ??

	container, err := svc.GetContainer(name)
	if err != nil {
		return
	}

	// Todo: find a better way or at least untangle

	if rule, ok := container.Labels["traefik.http.routers."+name+".rule"]; ok {
		// Extract hostname from rule like "Host(`axis.local`)"
		if strings.Contains(rule, "Host(`") {
			start := strings.Index(rule, "Host(`") + len("Host(`")
			end := strings.Index(rule[start:], "`)")
			if end > 0 {
				hostname := rule[start : start+end]
				url = "https://" + hostname
				// Todo: what about tls ja
				return
			}
		}
	}

	err = errors.Errorf("unable to find proxy hostname from labels for %s", name)
	return
}
*/
