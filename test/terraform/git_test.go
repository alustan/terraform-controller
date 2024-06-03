package terraform_test

import (
	"controller/pkg/terraform"
	"testing"
	"os"

	"github.com/stretchr/testify/assert"
)

func TestCloneGitRepo(t *testing.T) {
	repoDir, err := os.MkdirTemp("", "repo")
	assert.NoError(t, err)
	defer os.RemoveAll(repoDir)

	err = terraform.CloneGitRepo("https://github.com/alustan/platform-template.git", "main", "", repoDir)
	assert.NoError(t, err)

	files, err := os.ReadDir(repoDir)
	assert.NoError(t, err)
	assert.True(t, len(files) > 0)
}
