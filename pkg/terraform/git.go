package terraform

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"golang.org/x/crypto/ssh"
	"gopkg.in/src-d/go-git.v4/plumbing/transport"
	gitssh "gopkg.in/src-d/go-git.v4/plumbing/transport/ssh"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// getSSHKeyFromSecret retrieves the SSH key from a Kubernetes Secret
func getSSHKeyFromSecret(clientset *kubernetes.Clientset, namespace, secretName, keyName string) (string, error) {
	secret, err := clientset.CoreV1().Secrets(namespace).Get(context.Background(), secretName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to get secret: %v", err)
	}

	sshKey, ok := secret.Data[keyName]
	if !ok {
		return "", fmt.Errorf("key %s not found in secret %s", keyName, secretName)
	}

	return string(sshKey), nil
}

// cloneOrPullRepo clones the repository if it does not exist, or pulls the latest changes if it does.
// It uses the SSH key for authentication if provided.
func cloneOrPullRepo(repoURL, branch, repoDir, sshKey string) error {
	var repo *git.Repository
	var err error
	var auth transport.AuthMethod

	// If an SSH key is provided, set up the authentication
	if sshKey != "" {
		signer, err := ssh.ParsePrivateKey([]byte(sshKey))
		if err != nil {
			return fmt.Errorf("failed to parse SSH key: %v", err)
		}

		auth = &gitssh.PublicKeys{
			User:   "git",
			Signer: signer,
		}
	}

	if _, err = os.Stat(repoDir); os.IsNotExist(err) {
		// Clone the repository
		repo, err = git.PlainClone(repoDir, false, &git.CloneOptions{
			URL:           repoURL,
			ReferenceName: plumbing.NewBranchReferenceName(branch),
			Auth:          auth,
		})
		if err != nil {
			return fmt.Errorf("failed to clone repository: %v", err)
		}
	} else {
		// Open the existing repository and pull the latest changes
		repo, err = git.PlainOpen(repoDir)
		if err != nil {
			return fmt.Errorf("failed to open repository: %v", err)
		}

		worktree, err := repo.Worktree()
		if err != nil {
			return fmt.Errorf("failed to get worktree: %v", err)
		}

		err = worktree.Pull(&git.PullOptions{
			ReferenceName: plumbing.NewBranchReferenceName(branch),
			Auth:          auth,
			Force:         true,
		})
		if err != nil && err != git.NoErrAlreadyUpToDate {
			return fmt.Errorf("failed to pull repository: %v", err)
		}
	}

	return nil
}




