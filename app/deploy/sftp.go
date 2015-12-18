package deploy

import (
	"io"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/go-xiaohei/pugo-static/app/builder"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"gopkg.in/inconshreveable/log15.v2"
)

const (
	TYPE_SFTP = "sftp"
)

var (
// _ DeployTask = new(SftpTask)
)

type (
	SftpTask struct {
		opt *SftpOption
	}
	SftpOption struct {
		url       *url.URL
		Address   string
		User      string
		Password  string
		Directory string
	}
)

// new sftp task with section
func (ft *SftpTask) New(conf string) (DeployTask, error) {
	conf = strings.TrimLeft(conf, "sftp://")
	confData := strings.Split(conf, "@")
	if len(confData) != 2 {
		return nil, ErrDeployConfFormatError
	}
	userData := strings.Split(confData[0], ":")
	if len(userData) != 2 {
		return nil, ErrDeployConfFormatError
	}
	f := &SftpTask{
		opt: &SftpOption{
			User:     userData[0],
			Password: userData[1],
			Address:  confData[1],
		},
	}
	if u, err := url.Parse("ssh://" + f.opt.Address); err != nil {
		return nil, err
	} else {
		f.opt.url = u
	}
	p := f.opt.url.Path
	if strings.HasPrefix(p, "/~") {
		f.opt.Directory = strings.TrimPrefix(p, "/~/")
	} else {
		f.opt.Directory = p
	}
	return f, nil
}

// sftp task's name
func (ft *SftpTask) Name() string {
	return TYPE_SFTP
}

// is sftp task's name
func (ft *SftpTask) Is(conf string) bool {
	return strings.HasPrefix(conf, "sftp://")
}

// sftp task's build directory
func (ft *SftpTask) Dir() string {
	return path.Base(ft.opt.Directory)
}

// sftp task do action
func (ft *SftpTask) Do(b *builder.Builder, ctx *builder.Context) error {
	conn, client, err := connectSftp(ft.opt)
	if err != nil {
		return err
	}
	defer client.Close()
	defer conn.Close()
	log15.Debug("Deploy.[" + ft.opt.Address + "].Connected")

	// just make directory, ignore error
	makeSftpDir(client, getDirs(ft.opt.Directory))

	if b.Count < 2 {
		log15.Debug("Deploy.[" + ft.opt.Address + "].UploadAll")
		return ft.uploadAllFiles(client, ctx)
	}

	log15.Debug("Deploy.[" + ft.opt.Address + "].UploadDiff")
	return ft.uploadDiffFiles(client, ctx)
}

func (ft *SftpTask) uploadAllFiles(client *sftp.Client, ctx *builder.Context) error {
	var (
		createdDirs = make(map[string]bool)
		err         error
	)
	return ctx.Diff.Walk(func(name string, entry *builder.DiffEntry) error {
		rel, _ := filepath.Rel(ctx.DstDir, name)
		rel = filepath.ToSlash(rel)

		if entry.Behavior == builder.DIFF_REMOVE {
			log15.Debug("Deploy.Sftp.Delete", "file", rel)
			return client.Remove(path.Join(ft.opt.Directory, rel))
		}

		// create directory recursive
		dirs := getDirs(path.Dir(rel))
		if len(dirs) > 0 {
			for i := len(dirs) - 1; i >= 0; i-- {
				dir := dirs[i]
				if !createdDirs[dir] {
					if err = client.Mkdir(path.Join(ft.opt.Directory, dir)); err == nil {
						createdDirs[dir] = true
					}
				}
			}
		}

		// upload file
		f, err := os.Open(name)
		if err != nil {
			return err
		}
		defer f.Close()

		f2, err := client.Create(path.Join(ft.opt.Directory, rel))
		if err != nil {
			return err
		}
		defer f2.Close()

		if _, err = io.Copy(f2, f); err != nil {
			return err
		}
		log15.Debug("Deploy.Sftp.Stor", "file", rel)
		return nil
	})
}

func (ft *SftpTask) uploadDiffFiles(client *sftp.Client, ctx *builder.Context) error {
	return ctx.Diff.Walk(func(name string, entry *builder.DiffEntry) error {
		rel, _ := filepath.Rel(ctx.DstDir, name)
		rel = filepath.ToSlash(rel)

		if entry.Behavior == builder.DIFF_REMOVE {
			log15.Debug("Deploy.Sftp.Delete", "file", rel)
			return client.Remove(path.Join(ft.opt.Directory, rel))
		}

		target := path.Join(ft.opt.Directory, rel)
		if entry.Behavior == builder.DIFF_KEEP {
			if fi, _ := client.Stat(target); fi != nil {
				// entry file should be older than uploaded file
				if entry.Time.Sub(fi.ModTime()).Seconds() < 0 {
					return nil
				}
			}
		}

		dirs := getDirs(path.Dir(rel))
		for i := len(dirs) - 1; i >= 0; i-- {
			client.Mkdir(path.Join(ft.opt.Directory, dirs[i]))
		}

		// upload file
		f, err := os.Open(name)
		if err != nil {
			return err
		}
		defer f.Close()

		f2, err := client.Create(target)
		if err != nil {
			return err
		}
		defer f2.Close()

		if _, err = io.Copy(f2, f); err != nil {
			return err
		}
		log15.Debug("Deploy.Sftp.Stor", "file", rel)
		return nil
	})
}

// connect to sftp, get ssh connection and sftp client
func connectSftp(opt *SftpOption) (*ssh.Client, *sftp.Client, error) {
	conf := &ssh.ClientConfig{
		User: opt.User,
		Auth: []ssh.AuthMethod{
			ssh.Password(opt.Password),
		},
	}
	client, err := ssh.Dial("tcp", opt.url.Host, conf)
	if err != nil {
		return nil, nil, err
	}
	s, err := sftp.NewClient(client)
	return client, s, err
}

func makeSftpDir(client *sftp.Client, dirs []string) error {
	for i := len(dirs) - 1; i >= 0; i-- {
		if err := client.Mkdir(dirs[i]); err != nil {
			return err
		}
	}
	return nil
}