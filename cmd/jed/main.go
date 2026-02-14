// Package main implements jed, a CLI for managing service and env definitions in the store.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/alexflint/go-arg"
	"github.com/joho/godotenv"
	"github.com/pkg/errors"
	"sigs.k8s.io/yaml"

	"github.com/clarktrimble/jed"
	"github.com/clarktrimble/jed/store/bbolt"
)

type LsSvcCmd struct{}

type GetSvcCmd struct {
	Name string `arg:"positional,required" help:"service name"`
}

type SetSvcCmd struct {
	File string `arg:"positional,required" help:"YAML file with service definition"`
}

type LsEnvCmd struct{}

type GetEnvCmd struct {
	Name string `arg:"positional,required" help:"service name"`
}

type SetEnvCmd struct {
	Name string `arg:"positional,required" help:"service name"`
	File string `arg:"positional,required" help:".env file"`
}

type args struct {
	LsSvc  *LsSvcCmd  `arg:"subcommand:ls-svc" help:"list services"`
	GetSvc *GetSvcCmd `arg:"subcommand:get-svc" help:"get a service"`
	SetSvc *SetSvcCmd `arg:"subcommand:set-svc" help:"set a service from YAML"`
	LsEnv  *LsEnvCmd  `arg:"subcommand:ls-env" help:"list envs"`
	GetEnv *GetEnvCmd `arg:"subcommand:get-env" help:"get env for a service"`
	SetEnv *SetEnvCmd `arg:"subcommand:set-env" help:"set env from .env file"`

	DB string `arg:"-d,--db" default:"jed.db" help:"path to bbolt database"`
}

func main() {
	var args args
	p := arg.MustParse(&args)

	if p.Subcommand() == nil {
		p.WriteHelp(os.Stdout)
		os.Exit(0)
	}

	ctx := context.Background()
	store, err := bbolt.New(args.DB)
	fatal(err)
	defer store.Close()

	switch {
	case args.LsSvc != nil:
		lsSvc(ctx, store)
	case args.GetSvc != nil:
		getSvc(ctx, store, args.GetSvc.Name)
	case args.SetSvc != nil:
		setSvc(ctx, store, args.SetSvc.File)
	case args.LsEnv != nil:
		lsEnv(ctx, store)
	case args.GetEnv != nil:
		getEnv(ctx, store, args.GetEnv.Name)
	case args.SetEnv != nil:
		setEnv(ctx, store, args.SetEnv.Name, args.SetEnv.File)
	}
}

func lsSvc(ctx context.Context, store *bbolt.Store) {
	services, err := store.Services(ctx)
	fatal(err)

	for _, svc := range services {
		fmt.Printf("%s\t%s\n", svc.Name, svc.Image)
	}
}

func getSvc(ctx context.Context, store *bbolt.Store, name string) {
	svc, err := store.GetService(ctx, name)
	fatal(err)

	data, err := json.MarshalIndent(svc, "", "  ")
	fatal(err)

	fmt.Println(string(data))
}

func setSvc(ctx context.Context, store *bbolt.Store, file string) {
	data, err := os.ReadFile(file)
	fatal(err)

	var svc jed.Service
	err = yaml.Unmarshal(data, &svc)
	fatal(errors.Wrapf(err, "failed to parse %s", file))

	if svc.Name == "" {
		fatal(errors.Errorf("service name is required in %s", file))
	}

	err = store.SetService(ctx, svc)
	fatal(err)

	fmt.Printf("set service %s\n", svc.Name)
}

func lsEnv(ctx context.Context, store *bbolt.Store) {
	envs, err := store.Envs(ctx)
	fatal(err)

	for _, env := range envs {
		fmt.Printf("%s\t%d vars\n", env.Name, len(env.Vars))
	}
}

func getEnv(ctx context.Context, store *bbolt.Store, name string) {
	env, err := store.GetEnv(ctx, name)
	fatal(err)

	for key, val := range env.Vars {
		fmt.Printf("%s=%s\n", key, val)
	}
}

func setEnv(ctx context.Context, store *bbolt.Store, name, file string) {
	data, err := os.ReadFile(file)
	fatal(err)

	vars, err := godotenv.Unmarshal(string(data))
	fatal(errors.Wrapf(err, "failed to parse %s", file))

	err = store.SetEnv(ctx, jed.Env{Name: name, Vars: vars})
	fatal(err)

	fmt.Printf("set env %s (%d vars)\n", name, len(vars))
}

func fatal(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
