package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/datdt/k8sselfhost/internal/infrastructure/kubernetes"
	"github.com/datdt/k8sselfhost/internal/infrastructure/provider/docker"
	"github.com/datdt/k8sselfhost/internal/usecase/deployment"
)

func main() {
	dockerHost := os.Getenv("DOCKER_HOST")
	if dockerHost == "" {
		dockerHost = "unix:///var/run/docker.sock"
	}
	dockerRepo, err := docker.NewRealDockerRepo(dockerHost, "")
	if err != nil {
		log.Fatal(err)
	}
	depRepo := kubernetes.NewDeploymentRepo(nil, dockerRepo, nil, nil)
	uc := deployment.NewUsecase(depRepo)
	
	apps, err := uc.ListDeployments(context.Background())
	if err != nil {
		log.Fatal("Error:", err)
	}
	fmt.Printf("Apps: %+v\n", apps)
}
