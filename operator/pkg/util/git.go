package util

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/mauricioscastro/hcreport/pkg/util/log"
	"go.uber.org/zap"
)

var logger = log.Logger().Named("git-util")

func publicKey(keyFile string) (*ssh.PublicKeys, error) {
	var publicKey *ssh.PublicKeys
	sshKey, _ := os.ReadFile(keyFile)
	publicKey, err := ssh.NewPublicKeys("git", []byte(sshKey), "")
	if err != nil {
		return nil, err
	}
	return publicKey, err
}

func GitClone(url, dir string, pkFile string) (*git.Repository, error) {
	logger.Sugar().Debugf("cloning %s into %s", url, dir)
	auth, keyErr := publicKey(pkFile)
	if keyErr != nil {
		return nil, keyErr
	}
	r, err := git.PlainClone(dir, false, &git.CloneOptions{
		Progress: io.Discard,
		URL:      url,
		Auth:     auth,
	})
	if err != nil {
		if err == git.ErrRepositoryAlreadyExists {
			logger.Info("already cloned")
			return git.PlainOpen(dir)
		} else {
			logger.Error("clone git repo error", zap.Error(err))
			return nil, err
		}
	}

	headRef, err := r.Head()
	if err != nil {
		return nil, err
	}

	ref := plumbing.NewHashReference("refs/heads/my-branch", headRef.Hash())

	err = r.Storer.SetReference(ref)
	if err != nil {
		return nil, err
	}

	w, err := r.Worktree()
	if err != nil {
		return nil, err
	}

	_, err = w.Add("hello.txt")
	if err != nil {
		logger.Error("AddGlob", zap.Error(err))
		return nil, err
	}

	commit, err := w.Commit("example go-git commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "hcreport",
			Email: "macastro@redhat.com",
			When:  time.Now(),
		},
	})
	if err != nil {
		return nil, err
	}

	obj, err := r.CommitObject(commit)
	if err != nil {
		return nil, err
	}
	fmt.Println(obj)

	err = r.Push(&git.PushOptions{})
	if err != nil {
		return nil, err
	}

	return r, nil
}
