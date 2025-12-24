package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	log.Println("Starting Sandbox Agent...")

	// This agent runs INSIDE each sandbox container/VM
	// It handles:
	// - Code execution requests
	// - File operations
	// - Process management
	// - Resource monitoring

	// TODO: Start gRPC server for receiving commands
	// TODO: Start metrics collection

	log.Println("Sandbox Agent is running")

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Sandbox Agent shutting down...")
}
