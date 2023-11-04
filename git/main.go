package main

import (
	"context"
	"fmt"
	"time"

	"github.com/xanzy/go-gitlab"
	"github.com/xyproto/randomstring"
	"gopkg.in/yaml.v3"
)

const WorkDir = "/tmp/repo/"

type GitActions struct {
}

type GitActionRepository struct {
	RepoUrl string
	SshKey  *File
}

func (m *GitActions) WithRepository(ctx context.Context, repoUrl string, sshKey *File) *GitActionRepository {
	return &GitActionRepository{
		RepoUrl: repoUrl,
		SshKey:  sshKey,
	}
}

// "git@ssh.gitlab.puzzle.ch:cschlatter/clone-test.git"
func (m *GitActionRepository) CloneSsh(ctx context.Context) (*Directory, error) {

	if m.RepoUrl == "" || m.SshKey == nil {
		return nil, fmt.Errorf("Repo URL and SSH Key must be set")
	}

	c, err := prepareContainer(m.SshKey).
		WithExec([]string{"git", "clone", m.RepoUrl, "."}).
		Sync(ctx)

	if err != nil {
		return nil, err
	}

	dir := c.Directory(WorkDir)

	return dir, nil
}

func (m *GitActionRepository) Push(ctx context.Context, dir *Directory, prBranch Optional[string]) error {

	c := prepareContainer(m.SshKey).
		WithDirectory(WorkDir, dir)

	if prBranch.isSet {
		c = c.WithExec([]string{"git", "checkout", "-b", prBranch.GetOr("main")})
	}

	_, err := c.WithExec([]string{"git", "add", "."}).
		WithExec([]string{"git", "commit", "-m", "oinkoink"}).
		WithExec([]string{"git", "push"}).
		Sync(ctx)

	return err
}

func prepareContainer(key *File) *Container {
	return dag.Container().
		From("registry.puzzle.ch/cicd/alpine-base:latest").
		WithWorkdir(WorkDir).
		WithFile("/tmp/.ssh/id", key, ContainerWithFileOpts{Permissions: 0400}).
		WithEnvVariable("GIT_SSH_COMMAND", "ssh -i /tmp/.ssh/id -o StrictHostKeyChecking=no").
		WithEnvVariable("CACHE_BUSTER", time.Now().String()).
		WithExec([]string{"git", "config", "--global", "user.name", "dagger-bot"}).
		WithExec([]string{"git", "config", "--global", "user.email", "cicd@puzzle.ch"}).
		WithExec([]string{"git", "config", "--global", "--add", "--bool", "push.autoSetupRemote", "true"})
}

// pitc-cicd-helm-demo-prod
func (m *GitActionRepository) UpdateHelmRevision(ctx context.Context, envName string, revision string, pushBranch Optional[string]) error {

	gitDir, err := m.CloneSsh(ctx)
	if err != nil {
		return err
	}

	mod := dag.Container().From("registry.puzzle.ch/cicd/alpine-base:latest").
		WithDirectory(WorkDir, gitDir).
		WithWorkdir(WorkDir).
		WithExec([]string{"sh", "-c", fmt.Sprintf("yq eval '.environments |= map(select(.name == \"%s\").argocd.helm.targetRevision=\"%s\")' -i argocd/values.yaml", envName, revision)}).
		Directory(WorkDir)

	return m.Push(ctx, mod, pushBranch)

}

func (m *GitActionRepository) UpdateImageTagHelm(ctx context.Context, key *File, valuesFile string, jsonPath string, revision string, createPr Optional[bool]) error {
	gitDir, err := m.CloneSsh(ctx)
	if err != nil {
		return err
	}

	mod := dag.Container().From("registry.puzzle.ch/cicd/alpine-base:latest").
		WithDirectory(WorkDir, gitDir).
		WithWorkdir(WorkDir).
		WithExec([]string{"sh", "-c", fmt.Sprintf("yq eval '%s=\"%s\"' -i %s", jsonPath, revision, valuesFile)}).
		Directory(WorkDir)

	fmt.Printf("yq eval '%s=\"%s\"' -i %s", jsonPath, revision, valuesFile)

	prBranch := Optional[string]{}

	if createPr.GetOr(false) {
		rand := randomstring.HumanFriendlyEnglishString(6)
		prBranch = Opt[string](fmt.Sprintf("update/helm-revision-%s-%s", revision, rand))
	}

	return m.Push(ctx, mod, prBranch)

}

type MergeRequest struct {
	Title        string
	Description  string
	SourceBranch string
	TargetBranch string
	ProjectPath  string
	ApiUrl       string
	AccessToken  string
}

func (m *GitActions) WithAPI(ctx context.Context, apiUrl string, accessToken string) *MergeRequest {
	return &MergeRequest{
		AccessToken: accessToken,
		ApiUrl:      apiUrl,
	}
}

func (m *MergeRequest) WithMergeRequest(ctx context.Context, projectPath string, sourceBranch string, targetBranch string, title Optional[string], descripton Optional[string]) *MergeRequest {

	m.Title = title.GetOr("Dagger Bot MR")
	m.Description = descripton.GetOr("No description provided")
	m.SourceBranch = sourceBranch
	m.TargetBranch = targetBranch
	m.ProjectPath = projectPath

	return m
}

func (m *MergeRequest) createGitLabMR(ctx context.Context) error {

	glClient, err := gitlab.NewClient(m.AccessToken, gitlab.WithBaseURL(m.ApiUrl))
	if err != nil {
		return err
	}

	_, _, err = glClient.MergeRequests.CreateMergeRequest(m.ProjectPath, &gitlab.CreateMergeRequestOptions{
		Title:        &m.Title,
		Description:  &m.Description,
		SourceBranch: &m.SourceBranch,
		TargetBranch: &m.TargetBranch,
		Labels:       &gitlab.Labels{"auto"},
	})

	return err
}

func StringPtr(s string) *string {
	return &s
}

type MrConfig struct {
	OpsRepository string   `yaml:"opsRepository"`
	SourceBranch  string   `yaml:"sourceBranch"`
	TargetBranch  string   `yaml:"targetBranch"`
	Tags          []string `yaml:"tags"`
}

func (m *GitActions) Run(ctx context.Context, config *File, key *File, apiToken string, version string) error {

	content, err := config.Contents(ctx)
	if err != nil {
		return err
	}

	mrConfig := &MrConfig{}
	err = yaml.Unmarshal([]byte(content), mrConfig)
	if err != nil {
		return err
	}

	rand := randomstring.HumanFriendlyEnglishString(6)
	prBranch := Opt[string](fmt.Sprintf("update/helm-revision-%s-%s", version, rand))

	action := m.WithRepository(ctx, "git@ssh.gitlab.puzzle.ch:cschlatter/clone-test.git", key)
	err = action.
		UpdateHelmRevision(ctx, "pitc-cicd-helm-demo-prod", version, prBranch)
	if err != nil {
		return err
	}

	return m.WithAPI(ctx, "https://gitlab.puzzle.ch", apiToken).
		WithMergeRequest(ctx, "cschlatter/clone-test", prBranch.value, "main", Opt[string]("YOLO"), Opt[string]("YOLO")).
		createGitLabMR(ctx)
}
