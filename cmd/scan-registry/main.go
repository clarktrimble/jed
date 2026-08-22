package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/clarktrimble/giant"
	"github.com/clarktrimble/jed/scanner"
	"github.com/clarktrimble/sabot"
)

func main() {
	registry := "https://registry.bastille.cloud"
	if len(os.Args) > 1 {
		registry = os.Args[1]
	}

	lgrCfg := sabot.Config{MaxLen: 999}
	lgr := lgrCfg.New(os.Stderr)
	client := (&giant.Config{
		BaseUri: registry,
		Headers: []string{
			"Accept", "application/vnd.oci.image.index.v1+json, application/vnd.oci.image.manifest.v1+json, application/vnd.docker.distribution.manifest.list.v2+json, application/vnd.docker.distribution.manifest.v2+json",
		},
	}).NewWithTrippers(lgr)

	images, err := scanner.New(client).Images(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, "scan registry: %v\n", err)
		os.Exit(1)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(images); err != nil {
		fmt.Fprintf(os.Stderr, "write json: %v\n", err)
		os.Exit(1)
	}
}
