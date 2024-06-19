package main

import (
	"log"
	"os"
	"github.com/alustan/terraform-controller/pkg/terraform"
)

func main() {
	repoURL := os.Getenv("REPO_URL")
	branch := os.Getenv("BRANCH")
	repoDir := os.Getenv("REPO_DIR")
	sshKey := os.Getenv("SSH_KEY")

	if repoURL == "" || branch == "" || repoDir == "" {
		log.Fatal("Environment variables REPO_URL, BRANCH, and REPO_DIR must be set")
	}

	if err := terraform.CloneOrPullRepo(repoURL, branch, repoDir, sshKey); err != nil {
		log.Fatalf("Failed to clone or pull repository: %v", err)
	} else {
		log.Println("Repository cloned or pulled successfully.")
	}
}



