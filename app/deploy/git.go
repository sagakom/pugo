package deploy

import (
	"errors"
	"path"
	"strings"
	"time"

	"github.com/Unknwon/com"
	"github.com/go-xiaohei/pugo-static/app/builder"
	"gopkg.in/inconshreveable/log15.v2"
)

const (
	TYPE_GIT = "git"
)

var (
	// _ DeployTask = new(GitTask)

	ErrGitNotRepo      = errors.New("destination directory is not a git repository")
	ErrGitNoBranch     = errors.New("can not read git respository's branch")
	gitMessageReplacer = strings.NewReplacer("{now}", time.Now().Format(time.RFC3339))
)

type (
	// Git Deployment task
	GitTask struct {
		name      string
		opt       *GitOption
		directory string
	}
	// git options
	GitOption struct {
		Branch  string // remote repository branch name
		Message string // commit message, only support {now} time string
	}
)

// New GitTask with name and ini.Section options
func (gt *GitTask) New(conf string) (DeployTask, error) {
	// create a new GitTask
	g := &GitTask{
		name: "git",
		opt: &GitOption{
			Message: "Site Updated at {now}",
		},
	}
	dir := strings.TrimPrefix(conf, "git://")
	if dir == "" {
		return nil, errors.New("git deploy conf need be git://git_repository_directory")
	}
	g.directory = dir
	return g, nil
}

// GitTask's name
func (g *GitTask) Name() string {
	return TYPE_GIT
}

// GitTask's destination directory
func (g *GitTask) Dir() string {
	return g.directory
}

// is GitTask
func (g *GitTask) Is(conf string) bool {
	return strings.HasPrefix(conf, "git://")
}

// readRepo branch
func (g *GitTask) readRepo(dest string) error {
	content, _, err := com.ExecCmdDir(dest, "git", []string{"branch"}...)
	if err != nil {
		return err
	}
	contentData := strings.Split(content, "\n")
	for _, cnt := range contentData {
		if strings.HasPrefix(cnt, "*") {
			cntData := strings.Split(cnt, " ")
			g.opt.Branch = cntData[len(cntData)-1]
			return nil
		}
	}
	return nil
}

// Git deployment action
func (g *GitTask) Do(b *builder.Builder, ctx *builder.Context) error {
	gitDir := path.Join(ctx.DstDir, ".git")
	if !com.IsDir(gitDir) {
		return ErrGitNotRepo
	}
	var err error
	if err = g.readRepo(ctx.DstDir); err != nil {
		return err
	}
	if g.opt.Branch == "" {
		return ErrGitNoBranch
	}

	// add files
	if _, stderr, err := com.ExecCmdDir(ctx.DstDir, "git", []string{"add", "--all"}...); err != nil {
		log15.Error("Deploy.Git.Error", "error", stderr)
		return err
	}
	log15.Debug("Deploy.Git.[" + g.opt.Branch + "].AddFiles")

	// commit message
	message := gitMessageReplacer.Replace(g.opt.Message)
	if _, stderr, err := com.ExecCmdDir(ctx.DstDir, "git", []string{"commit", "-m", message}...); err != nil {
		log15.Error("Deploy.Git.Error", "error", stderr)
		return err
	}
	log15.Debug("Deploy.Git.[" + g.opt.Branch + "].Commit.'" + message + "'")

	// push to repo
	_, stderr, err := com.ExecCmdDir(ctx.DstDir, "git", []string{
		"push", "--force", "origin", g.opt.Branch}...)
	if err != nil {
		log15.Error("Deploy.Git.Error", "error", stderr)
		if stderr != "" {
			return errors.New(stderr)
		}
		return err
	}
	log15.Debug("Deploy.Git.[" + g.opt.Branch + "].Push")
	return nil
	/*
		opt := g.opt
		if opt.Directory == "" {
			opt.Directory = ctx.DstDir // use context destination directory as default
		}
		// check git repo
		gitDir := path.Join(opt.Directory, ".git")
		if !com.IsDir(gitDir) {
			return ErrGitNotRepo
		}
		// add files
		if _, stderr, err := com.ExecCmdDir(
			ctx.DstDir,
			"git",
			[]string{"add", "--all"}...); err != nil {
			log15.Error("Deploy.Git.Error", "error", stderr)
			return err
		}
		log15.Debug("Deploy.[" + g.opt.RepoUrl + "].AddAll")

		// commit message
		message := gitMessageReplacer.Replace(opt.Message)
		if _, stderr, err := com.ExecCmdDir(
			ctx.DstDir, "git", []string{"commit", "-m", message}...); err != nil {
			log15.Error("Deploy.Git.Error", "error", stderr)
			return err
		}
		log15.Debug("Deploy.[" + g.opt.RepoUrl + "].Commit.'" + message + "'")

		// change remote url
		if _, stderr, err := com.ExecCmdDir(ctx.DstDir, "git", []string{
			"remote", "set-url", "origin", opt.remoteUrl(),
		}...); err != nil {
			log15.Error("Deploy.Git.Error", "error", stderr)
			return err
		}
		// push to repo
		if _, stderr, err := com.ExecCmdDir(ctx.DstDir, "git", []string{
			"push", "--force", "origin", opt.Branch}...); err != nil {
			log15.Error("Deploy.Git.Error", "error", stderr)
			if stderr != "" {
				return errors.New(stderr)
			}
			return err
		}
		log15.Debug("Deploy.[" + g.opt.RepoUrl + "].Push")
		return nil
	*/
}