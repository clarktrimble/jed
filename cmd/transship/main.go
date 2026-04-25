// Package main implements transship, a CLI for deploying to Docker Swarm
// using service and env definitions from the jed store.
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/alexflint/go-arg"
	"github.com/clarktrimble/giant"
	"github.com/clarktrimble/sabot"
	"github.com/pkg/errors"
	"golang.org/x/term"

	"github.com/clarktrimble/jed/store/bbolt"
	"github.com/clarktrimble/jed/swarm"
)

// Todo: regularize commands "ls-" etc
// Todo: list networks
// Todo: dry run for deploy and ??
// Todo: hairpin Host route thing??
// Todo: resolve "name" arg for set-svc but not set-env, this is awkward and error prone
// Todo: also awkward to specify commit hash in yaml rather than on transship cli??
// Todo: remove version/blah info in help
// Todo: revisit bind (host path) vs named vol issue; need to support both??

// Bootstrap notes
/*
➜  jed git:(swarm) ✗ go run cmd/transship/main.go create-network svc-net
error: failed to create network "svc-net": http POST request to http://localhost /v1.52/networks/create failed: Post "http://localhost/v1.52/networks/create": unexpected status code 409 with body: {"message":"network with name svc-net already exists"}

exit status 1

*/

type DeployCmd struct {
	Name string `arg:"positional,required" help:"service name"`
}

type LsServicesCmd struct{}

type DeleteServiceCmd struct {
	Name string `arg:"positional,required" help:"service name"`
}

type InspectCmd struct {
	Name string `arg:"positional,required" help:"service name"`
}

type TasksCmd struct {
	Service string `arg:"positional,required" help:"service name"`
}

type LogsCmd struct {
	Task string `arg:"positional,required" help:"task ID"`
	Tail string `arg:"-n,--tail" default:"100" help:"number of lines to show"`
}

type LsSecretsCmd struct{}

type CreateSecretCmd struct {
	Name string `arg:"positional,required" help:"secret name (value from stdin)"`
}

type LsConfigsCmd struct{}

type CreateConfigCmd struct {
	Name string `arg:"positional,required" help:"config name"`
	File string `arg:"positional,required" help:"file containing config value"`
}

type CreateNetworkCmd struct {
	Name       string `arg:"positional,required" help:"network name"`
	Attachable bool   `arg:"-a,--attachable" default:"true" help:"allow manual container attachment"`
	Encrypted  bool   `arg:"-e,--encrypted" help:"encrypt overlay traffic"`
}

type EventsCmd struct{}

type args struct {
	Deploy        *DeployCmd        `arg:"subcommand:deploy" help:"deploy/update a swarm service"`
	LsServices    *LsServicesCmd    `arg:"subcommand:ls-services" help:"list swarm services"`
	DeleteService *DeleteServiceCmd `arg:"subcommand:delete-service" help:"delete a swarm service"`
	Inspect       *InspectCmd       `arg:"subcommand:inspect" help:"show service spec"`
	Tasks         *TasksCmd         `arg:"subcommand:tasks" help:"show service tasks"`
	Logs          *LogsCmd          `arg:"subcommand:logs" help:"show service logs"`
	Events        *EventsCmd        `arg:"subcommand:events" help:"stream docker events"`
	LsSecrets     *LsSecretsCmd     `arg:"subcommand:ls-secrets" help:"list secrets"`
	CreateSecret  *CreateSecretCmd  `arg:"subcommand:create-secret" help:"create a secret"`
	LsConfigs     *LsConfigsCmd     `arg:"subcommand:ls-configs" help:"list configs"`
	CreateConfig  *CreateConfigCmd  `arg:"subcommand:create-config" help:"create a config"`
	CreateNetwork *CreateNetworkCmd `arg:"subcommand:create-network" help:"create overlay network"`

	Socket string `arg:"-s,--socket" default:"/var/run/docker.sock" help:"docker socket path"`
	DB     string `arg:"-d,--db" default:"jed.db" help:"path to jed store"`
}

func main() {
	var args args
	p := arg.MustParse(&args)

	if p.Subcommand() == nil {
		p.WriteHelp(os.Stdout)
		os.Exit(0)
	}

	ctx := context.Background()
	deployer := newDeployer(args.Socket)

	switch {
	case args.Deploy != nil:
		store, err := bbolt.New(args.DB)
		fatal(err)
		defer store.Close()
		deploy(ctx, deployer, store, args.Deploy.Name)
	case args.LsServices != nil:
		lsServices(ctx, deployer)
	case args.DeleteService != nil:
		deleteService(ctx, deployer, args.DeleteService.Name)
	case args.Inspect != nil:
		inspect(ctx, deployer, args.Inspect.Name)
	case args.Tasks != nil:
		tasks(ctx, deployer, args.Tasks.Service)
	case args.Logs != nil:
		logs(ctx, deployer, args.Logs.Task, args.Logs.Tail)
	case args.Events != nil:
		events(ctx, deployer)
	case args.LsSecrets != nil:
		lsSecrets(ctx, deployer)
	case args.CreateSecret != nil:
		createSecret(ctx, deployer, args.CreateSecret.Name)
	case args.LsConfigs != nil:
		lsConfigs(ctx, deployer)
	case args.CreateConfig != nil:
		createConfig(ctx, deployer, args.CreateConfig.Name, args.CreateConfig.File)
	case args.CreateNetwork != nil:
		createNetwork(ctx, deployer, args.CreateNetwork)
	}
}

func newDeployer(socket string) *swarm.Swarm {
	lgrCfg := sabot.Config{MaxLen: 999}
	lgr := lgrCfg.New(os.Stderr)

	clientCfg := &giant.Config{
		BaseUri:    "http://localhost",
		UnixSocket: socket,
	}
	client := clientCfg.NewWithTrippers(lgr)

	return swarm.New(client, lgr)
}

func deploy(ctx context.Context, deployer *swarm.Swarm, store *bbolt.Store, name string) {
	svc, err := store.GetService(ctx, name)
	fatal(errors.Wrapf(err, "failed to get service %q from store", name))

	err = svc.Validate()
	fatal(errors.Wrapf(err, "failed to validate service %q", name))

	env, err := store.GetEnv(ctx, name)
	fatal(errors.Wrapf(err, "failed to get env %q from store", name))

	fmt.Printf("deploying %s (%s)\n", svc.Name, svc.Image)
	if len(svc.Secrets) > 0 {
		fmt.Printf("  secrets: %v\n", svc.Secrets)
	}
	if len(svc.Configs) > 0 {
		fmt.Printf("  configs: %v\n", svc.Configs)
	}
	fmt.Printf("  env: %d vars\n", len(env.Vars))

	id, err := deployer.Deploy(ctx, svc, env)
	fatal(err)

	if id != "" {
		fmt.Printf("created %s (%s)\n", name, id[:12])
	} else {
		fmt.Printf("updated %s\n", name)
	}
}

func lsServices(ctx context.Context, deployer *swarm.Swarm) {
	svcs, err := deployer.ListServices(ctx)
	fatal(err)

	for _, s := range svcs {
		fmt.Printf("%s\t%s\n", s.ID[:12], s.Name)
	}
}

func deleteService(ctx context.Context, deployer *swarm.Swarm, name string) {
	err := deployer.DeleteService(ctx, name)
	fatal(err)

	fmt.Printf("deleted service %s\n", name)
}

func inspect(ctx context.Context, deployer *swarm.Swarm, name string) {
	svc, err := deployer.GetService(ctx, name)
	fatal(err)

	var buf bytes.Buffer
	err = json.Indent(&buf, svc.Spec, "", "  ")
	fatal(err)

	fmt.Println(buf.String())
}

func tasks(ctx context.Context, deployer *swarm.Swarm, service string) {
	tasks, err := deployer.ServiceTasks(ctx, service)
	fatal(err)

	// Sort by timestamp, newest first (ISO format sorts correctly as strings)
	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].Timestamp > tasks[j].Timestamp
	})

	for _, t := range tasks {
		ts := t.Timestamp
		if len(ts) > 19 {
			ts = ts[:19] // trim nanoseconds
		}
		fmt.Printf("%s\t%s\t%s\t%s\t%s\n", ts, t.ID[:12], t.State, t.Image, t.Error)
	}
}

func logs(ctx context.Context, deployer *swarm.Swarm, task, tail string) {
	data, err := deployer.TaskLogs(ctx, task, tail)
	fatal(err)

	fmt.Print(string(data))
}

func events(ctx context.Context, deployer *swarm.Swarm) {
	evts, err := deployer.Events(ctx)
	fatal(err)

	for event := range evts {
		fmt.Printf("%s %s %s: %s\n", event.Time.Format("15:04:05"), event.Service, event.Type, string(event.Payload))
	}
}

func lsSecrets(ctx context.Context, deployer *swarm.Swarm) {
	secrets, err := deployer.ListSecrets(ctx)
	fatal(err)

	for _, s := range secrets {
		fmt.Printf("%s\t%s\n", s.ID[:12], s.Name)
	}
}

func createSecret(ctx context.Context, deployer *swarm.Swarm, name string) {
	var data []byte
	var err error

	if term.IsTerminal(int(os.Stdin.Fd())) {
		fmt.Print("Enter secret value: ")
		reader := bufio.NewReader(os.Stdin)
		data, err = reader.ReadBytes('\n')
		data = bytes.TrimRight(data, "\n")
	} else {
		data, err = io.ReadAll(os.Stdin)
	}
	fatal(err)

	id, err := deployer.CreateSecret(ctx, name, data)
	fatal(err)

	fmt.Printf("created secret %s: %s\n", name, id)
}

func lsConfigs(ctx context.Context, deployer *swarm.Swarm) {
	configs, err := deployer.ListConfigs(ctx)
	fatal(err)

	for _, c := range configs {
		fmt.Printf("%s\t%s\n", c.ID[:12], c.Name)
	}
}

func createConfig(ctx context.Context, deployer *swarm.Swarm, name, file string) {
	data, err := os.ReadFile(file)
	fatal(err)

	id, err := deployer.CreateConfig(ctx, name, data)
	fatal(err)

	fmt.Printf("created config %s: %s\n", name, id)
}

func createNetwork(ctx context.Context, deployer *swarm.Swarm, cmd *CreateNetworkCmd) {
	id, err := deployer.CreateNetwork(ctx, cmd.Name, cmd.Attachable, cmd.Encrypted)
	fatal(err)

	fmt.Printf("created network %s: %s\n", cmd.Name, id)
}

func fatal(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
