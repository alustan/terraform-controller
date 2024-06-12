package main

import (
	"log"
	"os"
	"time"
	"controller/pkg/terraform"
)

func main() {
	repoURL := os.Getenv("REPO_URL")
	branch := os.Getenv("BRANCH")
	repoDir := os.Getenv("REPO_DIR")
	sshKey := os.Getenv("SSH_KEY")

	if repoURL == "" || branch == "" || repoDir == "" {
		log.Fatal("Environment variables REPO_URL, BRANCH, and REPO_DIR must be set")
	}

	const maxRetries = 5
	const retryInterval = 30 * time.Second

	for i := 0; i < maxRetries; i++ {
		if err := terraform.CloneOrPullRepo(repoURL, branch, repoDir, sshKey); err != nil {
			if i == maxRetries-1 {
				log.Fatalf("Failed to clone or pull repository after %d attempts: %v", maxRetries, err)
			} else {
				log.Printf("Failed to clone or pull repository: %v. Retrying in %v...", err, retryInterval)
				time.Sleep(retryInterval)
			}
		} else {
			log.Println("Repository cloned or pulled successfully.")
			break
		}
	}
}
