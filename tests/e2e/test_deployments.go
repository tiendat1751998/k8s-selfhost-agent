package main

import (
	"context"
	"fmt"
	"log"

	"github.com/datdt/k8sselfhost/internal/infrastructure/kubernetes"
	"github.com/datdt/k8sselfhost/internal/infrastructure/provider/docker"
	"github.com/datdt/k8sselfhost/internal/usecase/deployment"
)

func main() {
	dockerRepo, err := docker.NewRealDockerRepo("tcp://10.10.10.133:2375", "")
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
