package util

import (
	"io"
	"os"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/mauricioscastro/hcreport/pkg/util/log"
	"go.uber.org/zap"
)

var logger = log.Logger().Named("git-util")

func publicKey(keyFile string) (*ssh.PublicKeys, error) {
	var publicKey *ssh.PublicKeys
	sshKey, _ := os.ReadFile(keyFile)
	publicKey, err := ssh.NewPublicKeys("git", sshKey, "")
	if err != nil {
		return nil, err
	}
	return publicKey, err
}

func GitClone(url, dir string, pkFile string) error {
	logger.Sugar().Debugf("cloning %s into %s", url, dir)
	auth, keyErr := publicKey(pkFile)
	if keyErr != nil {
		return keyErr
	}
	r, err := git.PlainClone(dir, false, &git.CloneOptions{
		Progress: io.Discard,
		URL:      url,
		Auth:     auth,
	})
	if err != nil {
		if err == git.ErrRepositoryAlreadyExists {
			logger.Info("already cloned")
			r, err = git.PlainOpen(dir)
			if err != nil {
				return err
			}
		} else {
			logger.Error("clone git repo error", zap.Error(err))
			return err
		}
	}

	// w, err := r.Worktree()
	// if err != nil {
	// 	fmt.Printf("%v", err)
	// 	return
	// }

	// // Create new file
	// filePath := "my-new-ififif.txt"
	// newFile, err := fs.Create(filePath)
	// if err != nil {
	// 	return
	// }
	// newFile.Write([]byte("My new file"))
	// newFile.Close()

	// // Run git status before adding the file to the worktree
	// fmt.Println(w.Status())

	// // git add $filePath
	// w.Add(filePath)

	// // Run git status after the file has been added adding to the worktree
	// fmt.Println(w.Status())

	// // git commit -m $message
	// w.Commit("Added my new file", &git.CommitOptions{})

	// //Push the code to the remote
	// err = r.Push(&git.PushOptions{
	// 	RemoteName: "origin",
	// 	Auth:       auth,
	// })
	// if err != nil {
	// 	return
	// }
	// fmt.Println("Remote updated.", filePath)

	//////////////////////////////////////////////////////

	// headRef, err := r.Head()
	// if err != nil {
	// 	return nil, err
	// }

	// ref := plumbing.NewHashReference("refs/heads/my-branch", headRef.Hash())

	// err = r.Storer.SetReference(ref)
	// if err != nil {
	// 	return nil, err
	// }

	w, err := r.Worktree()
	if err != nil {
		logger.Error("Worktree", zap.Error(err))
		return err
	}

	_, err = w.Add("./")
	if err != nil {
		logger.Error("Add", zap.Error(err))
		return err
	}

	commit, err := w.Commit("example go-git commit", &git.CommitOptions{
		Author: &object.Signature{
			Name:  "hcreport",
			Email: "macastro@redhat.com",
			When:  time.Now(),
		},
	})
	if err != nil {
		logger.Error("Commit", zap.Error(err))
		return err
	}

	_, err = r.CommitObject(commit)
	if err != nil {
		logger.Error("CommitObject", zap.Error(err))
		return err
	}

	err = r.Push(&git.PushOptions{
		Auth: auth,
	})
	if err != nil {
		logger.Error("Push", zap.Error(err))
		return err
	}

	return nil
}
