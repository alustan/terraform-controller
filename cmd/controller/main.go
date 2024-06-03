package main

import (
	"log"

	"github.com/gin-gonic/gin"
	"controller/pkg/controller"
)

func main() {
	r := gin.Default()
	ctrl := controller.NewInClusterController() // Use NewInClusterController
	go ctrl.Reconcile() // Start the reconciliation loop in a separate goroutine
	r.POST("/sync", ctrl.ServeHTTP)

	log.Println("Starting server on port 8080...")
	if err := r.Run(":8080"); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

