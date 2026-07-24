// Package main implements jed, a CLI for managing service and env definitions in the store.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/alexflint/go-arg"
	"github.com/joho/godotenv"
	"github.com/pkg/errors"
	"sigs.k8s.io/yaml"

	"github.com/clarktrimble/jed"
	"github.com/clarktrimble/jed/store/bbolt"
)

var (
	version = "dev"
	release = "untagged"
)

type lsSvcCmd struct{}

type getSvcCmd struct {
	Name  string `arg:"positional,required" help:"service name"`
	Image string `arg:"positional,required" help:"service image"`
}

type setSvcCmd struct {
	File string `arg:"positional,required" help:"YAML file with service definition"`
}

type delSvcCmd struct {
	Name  string `arg:"positional,required" help:"service name"`
	Image string `arg:"positional,required" help:"service image"`
}

type lsEnvCmd struct{}

type getEnvCmd struct {
	Name string `arg:"positional,required" help:"service name"`
}

type setEnvCmd struct {
	Name string `arg:"positional,required" help:"service name"`
	File string `arg:"positional,required" help:".env file"`
}

type delEnvCmd struct {
	Name string `arg:"positional,required" help:"service name"`
}

type args struct {
	LsSvc  *lsSvcCmd  `arg:"subcommand:ls-svc" help:"list services"`
	GetSvc *getSvcCmd `arg:"subcommand:get-svc" help:"get a service"`
	SetSvc *setSvcCmd `arg:"subcommand:set-svc" help:"set a service from YAML"`
	DelSvc *delSvcCmd `arg:"subcommand:del-svc" help:"delete a service"`
	LsEnv  *lsEnvCmd  `arg:"subcommand:ls-env" help:"list envs"`
	GetEnv *getEnvCmd `arg:"subcommand:get-env" help:"get env for a service"`
	SetEnv *setEnvCmd `arg:"subcommand:set-env" help:"set env from .env file"`
	DelEnv *delEnvCmd `arg:"subcommand:del-env" help:"delete env for a service"`

	DB              string `arg:"-d,--db" default:"jed.db" help:"path to bbolt database"`
	SkipSchemaCheck bool   `arg:"--skip-schema-check" help:"open database without validating schema version"`
}

func (args) Version() string {
	return fmt.Sprintf("jed %s (%s), db schema %s", release, version, jed.DBSchemaVersion)
}

func main() {
	var args args
	p := arg.MustParse(&args)

	if p.Subcommand() == nil {
		p.WriteHelp(os.Stdout)
		os.Exit(0)
	}

	ctx := context.Background()
	store, err := (&bbolt.Config{
		Path:            args.DB,
		SkipSchemaCheck: args.SkipSchemaCheck,
	}).New()
	fatal(err)
	defer store.Close()

	switch {
	case args.LsSvc != nil:
		lsSvc(ctx, store)
	case args.GetSvc != nil:
		getSvc(ctx, store, args.GetSvc.Name, args.GetSvc.Image)
	case args.SetSvc != nil:
		setSvc(ctx, store, args.SetSvc.File)
	case args.DelSvc != nil:
		delSvc(ctx, store, args.DelSvc.Name, args.DelSvc.Image)
	case args.LsEnv != nil:
		lsEnv(ctx, store)
	case args.GetEnv != nil:
		getEnv(ctx, store, args.GetEnv.Name)
	case args.SetEnv != nil:
		setEnv(ctx, store, args.SetEnv.Name, args.SetEnv.File)
	case args.DelEnv != nil:
		delEnv(ctx, store, args.DelEnv.Name)
	}
}

func lsSvc(ctx context.Context, store *bbolt.Store) {
	services, err := store.AllServices(ctx)
	fatal(err)

	for _, svc := range services {
		fmt.Printf("%s\t%s\n", svc.Name, svc.Image)
	}
}

func getSvc(ctx context.Context, store *bbolt.Store, name, image string) {
	svc, err := store.GetService(ctx, name, image)
	fatal(err)

	data, err := yaml.Marshal(svc)
	fatal(err)

	fmt.Printf("---\n%s", data)
}

func setSvc(ctx context.Context, store *bbolt.Store, file string) {
	data, err := os.ReadFile(file)
	fatal(err)

	var svc jed.Service
	err = yaml.Unmarshal(data, &svc)
	fatal(errors.Wrapf(err, "failed to parse %s", file))

	err = svc.Validate()
	fatal(errors.Wrapf(err, "failed to validate service from %s", file))

	err = store.SetService(ctx, svc)
	fatal(err)

	fmt.Printf("set service %s\n", svc.Name)
}

func delSvc(ctx context.Context, store *bbolt.Store, name, image string) {
	_, err := store.GetService(ctx, name, image)
	fatal(err)

	err = store.DelService(ctx, name, image)
	fatal(err)

	fmt.Printf("deleted service %s %s\n", name, image)
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

func delEnv(ctx context.Context, store *bbolt.Store, name string) {
	err := store.DelEnv(ctx, name)
	fatal(err)

	fmt.Printf("deleted env %s\n", name)
}

func fatal(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
