package main

import (
	"context"
	"fmt"
	"os"
	"strings"
)

type HelmPush struct {
}

func (h *HelmPush) PackagePush(ctx context.Context, d *Directory, registry string, username string, password string) error {
	fmt.Fprintf(os.Stdout, "☸️ Helm package and Push")
	c := dag.Container().From("registry.puzzle.ch/cicd/alpine-base:latest").WithDirectory("/helm", d).WithWorkdir("/helm")
	version, err := c.WithExec([]string{"sh", "-c", "helm show chart . | yq eval '.version' -"}).Stdout(ctx)
	if err != nil {
		return err
	}

	version = strings.TrimSpace(version)

	name, err := c.WithExec([]string{"sh", "-c", "helm show chart . | yq eval '.name' -"}).Stdout(ctx)
	if err != nil {
		return err
	}

	name = strings.TrimSpace(name)

	c, err = c.WithExec([]string{"helm", "registry", "login", registry, "-u", username, "-p", password}).Sync(ctx)
	if err != nil {
		return err
	}

	return nil
}
