package main

import (
	"log"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/alustan/terraform-controller/pkg/util"
	"github.com/alustan/terraform-controller/pkg/controller" 
	
)

// Variables to be set by ldflags
var (
	version  string
	commit   string
	date     string
	builtBy  string
)

func main() {
	fmt.Printf("Version: %s\n", version)
	fmt.Printf("Commit: %s\n", commit)
	fmt.Printf("Date: %s\n", date)
	fmt.Printf("Built by: %s\n", builtBy)
	
	r := gin.Default()
	ctrl := controller.NewInClusterController()
	syncInterval := util.GetSyncInterval()
	log.Printf("Sync interval is set to %v", syncInterval)
	go ctrl.Reconcile(syncInterval) // Start the reconciliation loop in a separate goroutine

	r.POST("/sync", ctrl.ServeHTTP)

	log.Println("Starting server on port 8080...")
	if err := r.Run(":8080"); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

