package terraform_test

import (
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	gogit "github.com/go-git/go-git/v5"
	"controller/pkg/terraform"
)

// MockRepository is a mock implementation of the git.Repository
type MockRepository struct {
	mock.Mock
}

func (m *MockRepository) Worktree() (*gogit.Worktree, error) {
	args := m.Called()
	return args.Get(0).(*gogit.Worktree), args.Error(1)
}

func (m *MockRepository) Pull(options *gogit.PullOptions) error {
	args := m.Called(options)
	return args.Error(0)
}

// MockWorktree is a mock implementation of the git.Worktree
type MockWorktree struct {
	mock.Mock
}

func (m *MockWorktree) Pull(options *gogit.PullOptions) error {
	args := m.Called(options)
	return args.Error(0)
}

func TestCloneOrPullRepo(t *testing.T) {
	repoURL := "git@github.com:user/repo.git"
	branch := "main"
	repoDir := "/tmp/repo"
	sshKey := "test-ssh-key"

	t.Run("clone repository successfully", func(t *testing.T) {
		os.RemoveAll(repoDir)
		err := terraform.CloneOrPullRepo(repoURL, branch, repoDir, sshKey)
		assert.NoError(t, err)

		_, err = os.Stat(repoDir)
		assert.False(t, os.IsNotExist(err), "repoDir should exist after cloning")
	})

	t.Run("pull repository successfully", func(t *testing.T) {
		os.Mkdir(repoDir, 0755)
		mockRepo := new(MockRepository)
		mockWorktree := new(MockWorktree)

		// Backup original function
		origPlainOpen := terraform.PlainOpen
		defer func() { terraform.PlainOpen = origPlainOpen }()

		// Mock PlainOpen and Worktree
		terraform.PlainOpen = func(path string) (*gogit.Repository, error) {
			return mockRepo, nil
		}
		mockRepo.On("Worktree").Return(mockWorktree, nil)

		// Mock Pull
		mockWorktree.On("Pull", mock.Anything).Return(nil)

		err := terraform.CloneOrPullRepo(repoURL, branch, repoDir, sshKey)
		assert.NoError(t, err)

		mockRepo.AssertExpectations(t)
		mockWorktree.AssertExpectations(t)
	})

	t.Run("handle SSH key error", func(t *testing.T) {
		invalidSSHKey := "invalid-ssh-key"
		err := terraform.CloneOrPullRepo(repoURL, branch, repoDir, invalidSSHKey)
		assert.Error(t, err)
	})

	t.Run("handle clone error", func(t *testing.T) {
		os.RemoveAll(repoDir)
		invalidRepoURL := "git@github.com:invalid/repo.git"
		err := terraform.CloneOrPullRepo(invalidRepoURL, branch, repoDir, sshKey)
		assert.Error(t, err)
	})

	t.Run("handle pull error", func(t *testing.T) {
		os.Mkdir(repoDir, 0755)
		mockRepo := new(MockRepository)
		mockWorktree := new(MockWorktree)

		// Backup original function
		origPlainOpen := terraform.PlainOpen
		defer func() { terraform.PlainOpen = origPlainOpen }()

		// Mock PlainOpen and Worktree
		terraform.PlainOpen = func(path string) (*gogit.Repository, error) {
			return mockRepo, nil
		}
		mockRepo.On("Worktree").Return(mockWorktree, nil)

		// Mock Pull with error
		mockWorktree.On("Pull", mock.Anything).Return(errors.New("pull error"))

		err := terraform.CloneOrPullRepo(repoURL, branch, repoDir, sshKey)
		assert.Error(t, err)

		mockRepo.AssertExpectations(t)
		mockWorktree.AssertExpectations(t)
	})
}
