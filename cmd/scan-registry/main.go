package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/clarktrimble/giant"
	"github.com/clarktrimble/jed"
	"github.com/clarktrimble/jed/extractor"
	"github.com/clarktrimble/jed/logger"
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

	scn, err := scanner.New(client, lgr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "new scanner: %v\n", err)
		os.Exit(1)
	}

	images, err := scn.Scan(context.Background())
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

	if os.Getenv("JED_EXTRACT") != "" {
		err = extract(context.Background(), lgr, images)
		if err != nil {
			fmt.Fprintf(os.Stderr, "extract: %v\n", err)
			os.Exit(1)
		}
	}
}

func extract(ctx context.Context, lgr logger.Logger, images []jed.Image) (err error) {
	platform := os.Getenv("JED_PLATFORM")
	if platform == "" {
		platform = "linux/amd64"
	}

	socket := os.Getenv("DOCKER_SOCKET")
	if socket == "" {
		socket = "/var/run/docker.sock"
	}

	client := (&giant.Config{
		BaseUri:    "http://localhost",
		UnixSocket: socket,
	}).NewWithTrippers(lgr)

	ext := extractor.New(client, lgr)
	for imageRef, cfg := range jed.Images(images).Configs(platform) {
		paths := extractionPaths(cfg)
		if len(paths) == 0 {
			continue
		}

		files, err := ext.Files(ctx, imageRef, paths...)
		if err != nil {
			return err
		}

		for _, path := range paths {
			fmt.Fprintf(os.Stderr, "\n# %s %s\n%s", imageRef, path, files[path])
		}
		return nil
	}

	return nil
}

func extractionPaths(cfg jed.ImageConfig) (paths []string) {
	labels := cfg.Labels
	if labels == nil {
		return nil
	}

	for _, label := range []string{
		"org.bastille.bip.service.spec.path",
		"org.bastille.bip.service.env.path",
	} {
		if path := labels[label]; path != "" {
			paths = append(paths, path)
		}
	}
	return paths
}
