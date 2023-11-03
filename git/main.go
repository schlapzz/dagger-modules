package main

import (
	"context"
	"fmt"

	gu "github.com/whilp/git-urls"
)

type Git struct {
	RepositoryUrl string
	Branch        string
	Username      string
	Email         string
	SshKey        *File
}

// - echo "setting up Git push access"
//- git config --global user.email "${GIT_USER_EMAIL}"
//- git config --global user.name "${GIT_USER_NAME}"
//- mkdir -p /tmp/.ssh/
//- cat $SSH_KEY > /tmp/.ssh/id_rsa
//- chmod 400 /tmp/.ssh/id_rsa
//- printf "\nHost ${GIT_SSH_HOST}\n  UpdateHostKeys no" > /tmp/.ssh/config
//- eval $(ssh-agent -s)
//- ssh-add
//- ssh-keyscan ${GIT_SSH_HOST} >> /tmp/.ssh/known_hosts
//- chmod 644 /tmp/.ssh/known_hosts

// example usage: "dagger call grep-dir --directory-arg . --pattern GrepDir"

func (m *Git) WithRepoUrl(url string, branch Optional[string]) {
	m.RepositoryUrl = url
	m.Branch = branch.GetOr("main")
}

func (m *Git) WithUserInfo(username string, email string) {
	m.Username = username
	m.Email = email
}

func (m *Git) WithSshKey(key *File) {
	m.SshKey = key
}

func (m *Git) GrepDir(ctx context.Context) (string, error) {

	url, err := gu.Parse(m.RepositoryUrl)
	if err != nil {
		return "", err
	}

	return dag.Container().
		From("registry.puzzle.ch/cicd/alpine-base:latest").
		WithWorkdir("/tmp").
		WithFile("/tmp/ssh/", m.SshKey, ContainerWithFileOpts{Permissions: 0400}).
		WithExec([]string{"eval", "$(ssh-agent -s)"}).
		WithExec([]string{"ssh-add"}).
		WithExec([]string{"sh", "-c", fmt.Sprintf("ssh-keyscan %s >> /tmp/.ssh/known_hosts", url.Host)}).
		WithExec([]string{"git", "config", "--global", "user.name", m.Username}).
		WithExec([]string{"git", "config", "--global", "user.email", m.Username}).
		WithExec([]string{"git", "clone", m.RepositoryUrl, "-b", m.Branch, "."}).
		WithExec([]string{"sh", "-c", "echo yolo >> oink.txt"}).
		WithExec([]string{"git", "add", "."}).
		WithExec([]string{"git", "commit", "-m", "oinkoink"}).
		WithExec([]string{"git", "push"}).
		Stdout(ctx)
}
