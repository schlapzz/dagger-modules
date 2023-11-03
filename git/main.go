package main

import (
	"context"
	"fmt"

	gu "github.com/whilp/git-urls"
)

type Git struct{}

func (m *Git) Push(ctx context.Context, url string, username string, email string, branch Optional[string], key *File) (string, error) {
	urlParts, err := gu.Parse(url)
	if err != nil {
		return "", err
	}

	return dag.Container().
		From("registry.puzzle.ch/cicd/alpine-base:latest").
		WithWorkdir("/tmp").
		WithFile("/tmp/ssh/", key, ContainerWithFileOpts{Permissions: 0400}).
		WithExec([]string{"eval", "$(ssh-agent -s)"}).
		WithExec([]string{"ssh-add"}).
		WithExec([]string{"sh", "-c", fmt.Sprintf("ssh-keyscan %s >> /tmp/.ssh/known_hosts", urlParts.Host)}).
		WithExec([]string{"git", "config", "--global", "user.name", username}).
		WithExec([]string{"git", "config", "--global", "user.email", email}).
		WithExec([]string{"git", "clone", url, "-b", branch.GetOr("main"), "."}).
		WithExec([]string{"sh", "-c", "echo yolo >> oink.txt"}).
		WithExec([]string{"git", "add", "."}).
		WithExec([]string{"git", "commit", "-m", "oinkoink"}).
		WithExec([]string{"git", "push"}).
		Stdout(ctx)
}
