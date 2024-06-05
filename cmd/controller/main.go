package main

import (
	"log"
	"fmt"

	"github.com/gin-gonic/gin"
	"controller/pkg/util"
	"controller/pkg/controller" 
	_ "controller/plugin/aws"
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
	go ctrl.Reconcile(syncInterval) // Start the reconciliation loop in a separate goroutine
	r.POST("/sync", ctrl.ServeHTTP)

	log.Println("Starting server on port 8080...")
	if err := r.Run(":8080"); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

