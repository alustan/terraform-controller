package terraform

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
)

func CloneGitRepo(gitRepoURL, branch, sshKey, repoDir string) error {
	var gitSSHCommand string
	if sshKey != "" {
		sshKeyPath := filepath.Join(repoDir, "id_rsa")
		err := ioutil.WriteFile(sshKeyPath, []byte(sshKey), 0600)
		if err != nil {
			return fmt.Errorf("failed to write SSH key: %v", err)
		}
		gitSSHCommand = fmt.Sprintf("ssh -i %s", sshKeyPath)
	}

	cmd := exec.Command("git", "clone", "-b", branch, gitRepoURL, repoDir)
	if gitSSHCommand != "" {
		cmd.Env = append(os.Environ(), fmt.Sprintf("GIT_SSH_COMMAND=%s", gitSSHCommand))
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Printf("Error: %s\nOutput: %s\n", err.Error(), string(output))
		return err
	}

	fmt.Printf("Output: %s\n", string(output))
	return nil
}
