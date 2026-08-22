// Package promotion coordinates deployment promotion business logic and state transitions.
package promotion

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/domain/audit"
	"github.com/datdt/k8sselfhost/internal/domain/promotion"
	domainDocker "github.com/datdt/k8sselfhost/internal/domain/provider/docker"
	domainerrors "github.com/datdt/k8sselfhost/internal/pkg/errors"
	"github.com/datdt/k8sselfhost/internal/pkg/logger"
)

// Usecase coordinates deployment promotion business logic and state transitions.
type Usecase struct {
	repo       promotion.Repository
	dockerRepo domainDocker.Repository
	auditRepo  audit.Repository
	logger     *zap.Logger
}

// NewUsecase creates a new promotion usecase with the injected repository dependency and optional adapters.
func NewUsecase(repo promotion.Repository, args ...interface{}) *Usecase {
	u := &Usecase{
		repo:   repo,
		logger: logger.Get(),
	}
	for _, arg := range args {
		switch v := arg.(type) {
		case domainDocker.Repository:
			u.dockerRepo = v
		case audit.Repository:
			u.auditRepo = v
		case *zap.Logger:
			if v != nil {
				u.logger = v
			}
		}
	}
	return u
}

// Create validates source/target environments and mandatory fields, sets the initial pending status, and persists the record.
func (u *Usecase) Create(ctx context.Context, input *promotion.Promotion) (*promotion.Promotion, error) {
	if input == nil {
		return nil, domainerrors.NewValidation("promotion", "promotion data cannot be nil")
	}

	if strings.TrimSpace(input.Service) == "" {
		return nil, domainerrors.NewValidation("service", "service is required")
	}
	if strings.TrimSpace(input.Version) == "" {
		return nil, domainerrors.NewValidation("version", "version is required")
	}
	if strings.TrimSpace(string(input.FromEnv)) == "" {
		return nil, domainerrors.NewValidation("from_env", "from_env is required")
	}
	if strings.TrimSpace(string(input.ToEnv)) == "" {
		return nil, domainerrors.NewValidation("to_env", "to_env is required")
	}

	// Business rule: source and target environments must be different
	if input.FromEnv == input.ToEnv {
		return nil, domainerrors.NewValidation("to_env", "source and target environments must be different")
	}

	// Enforce initial status to pending
	input.Status = promotion.StatusPending
	if input.CreatedAt.IsZero() {
		input.CreatedAt = time.Now().UTC()
	}

	if err := u.repo.Create(ctx, input); err != nil {
		return nil, fmt.Errorf("creating promotion: %w", err)
	}

	return input, nil
}

// Approve validates that the promotion is in Pending status before transitioning to Approved.
func (u *Usecase) Approve(ctx context.Context, id string, approver ...string) error {
	if strings.TrimSpace(id) == "" {
		return domainerrors.NewValidation("id", "promotion id is required")
	}

	promo, err := u.repo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("getting promotion %s: %w", id, err)
	}
	if promo == nil {
		return domainerrors.NewNotFound("promotion", id)
	}

	// Business rule: can only approve from Pending status
	if !promo.CanApprove() {
		return domainerrors.NewValidation("status", fmt.Sprintf("cannot approve promotion in '%s' status (must be pending)", promo.Status))
	}

	var app string
	if len(approver) > 0 {
		app = approver[0]
	}

	if err := u.repo.Approve(ctx, id, app); err != nil {
		return fmt.Errorf("approving promotion: %w", err)
	}
	return nil
}

// Reject validates that the promotion is in Pending status before transitioning to Rejected.
func (u *Usecase) Reject(ctx context.Context, id string, rejecter ...string) error {
	if strings.TrimSpace(id) == "" {
		return domainerrors.NewValidation("id", "promotion id is required")
	}

	promo, err := u.repo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("getting promotion %s: %w", id, err)
	}
	if promo == nil {
		return domainerrors.NewNotFound("promotion", id)
	}

	// Business rule: can only reject from Pending status
	if !promo.CanReject() {
		return domainerrors.NewValidation("status", fmt.Sprintf("cannot reject promotion in '%s' status (must be pending)", promo.Status))
	}

	var rej string
	if len(rejecter) > 0 {
		rej = rejecter[0]
	}

	if err := u.repo.Reject(ctx, id, rej); err != nil {
		return fmt.Errorf("rejecting promotion: %w", err)
	}
	return nil
}

// Complete validates that the promotion is in Approved or Promoting status before transitioning to Completed.
// When completed, if Docker Swarm client is available, it checks if the service exists in Docker Swarm and updates its container image.
// If not a Swarm service (or standalone container), it logs the promotion event in the audit log.
func (u *Usecase) Complete(ctx context.Context, id string, completer ...string) error {
	if strings.TrimSpace(id) == "" {
		return domainerrors.NewValidation("id", "promotion id is required")
	}

	promo, err := u.repo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("getting promotion %s: %w", id, err)
	}
	if promo == nil {
		return domainerrors.NewNotFound("promotion", id)
	}

	// Business rule: can only complete from Approved status (or promoting)
	if !promo.CanComplete() {
		return domainerrors.NewValidation("status", fmt.Sprintf("cannot complete promotion in '%s' status (must be approved)", promo.Status))
	}

	if err := u.repo.Complete(ctx, id); err != nil {
		return fmt.Errorf("completing promotion: %w", err)
	}

	actor := "system"
	if len(completer) > 0 && strings.TrimSpace(completer[0]) != "" {
		actor = completer[0]
	} else if promo.Approver != "" {
		actor = promo.Approver
	} else if promo.Requester != "" {
		actor = promo.Requester
	}

	u.handlePostComplete(ctx, promo, actor)
	return nil
}

func (u *Usecase) handlePostComplete(ctx context.Context, promo *promotion.Promotion, actor string) {
	details := map[string]interface{}{
		"service":   promo.Service,
		"version":   promo.Version,
		"from_env":  string(promo.FromEnv),
		"to_env":    string(promo.ToEnv),
		"status":    "completed",
		"requester": promo.Requester,
	}

	if u.dockerRepo != nil {
		services, err := u.dockerRepo.ListServices(ctx)
		if err == nil {
			if matchedSvc := matchSwarmService(services, promo.Service); matchedSvc != nil {
				targetImage := resolveTargetImage(matchedSvc.Image, promo.Version)
				details["swarm_service_id"] = matchedSvc.ID
				details["swarm_service_name"] = matchedSvc.Name
				details["target_image"] = targetImage
				details["previous_image"] = matchedSvc.Image

				updateErr := u.dockerRepo.UpdateServiceImage(ctx, matchedSvc.ID, targetImage)
				if updateErr != nil {
					details["docker_update_error"] = updateErr.Error()
					if u.logger != nil {
						u.logger.Error("failed to update swarm service image upon promotion completion",
							zap.String("service_id", matchedSvc.ID),
							zap.String("target_image", targetImage),
							zap.Error(updateErr))
					}
					if u.auditRepo != nil {
						if recErr := u.auditRepo.RecordAction(ctx, actor, "promote_docker_service", "docker_service", matchedSvc.ID, matchedSvc.Name, "failed", details, "", ""); recErr != nil {
							if u.logger != nil {
								u.logger.Warn("failed to record audit action for failed docker service promotion", zap.Error(recErr))
							}
						}
					}
					return
				}

				if u.auditRepo != nil {
					if recErr := u.auditRepo.RecordAction(ctx, actor, "promote_docker_service", "docker_service", matchedSvc.ID, matchedSvc.Name, "success", details, "", ""); recErr != nil {
						if u.logger != nil {
							u.logger.Warn("failed to record audit action for successful docker service promotion", zap.Error(recErr))
						}
					}
				}
				return
			}
		}

		// Check if it exists as a standalone container
		containers, err := u.dockerRepo.ListContainers(ctx)
		if err == nil {
			if matchedCnt := matchContainer(containers, promo.Service); matchedCnt != nil {
				targetImage := resolveTargetImage(matchedCnt.Image, promo.Version)
				details["standalone_container"] = true
				details["container_id"] = matchedCnt.ID
				details["container_name"] = matchedCnt.Name
				details["target_image"] = targetImage
				details["previous_image"] = matchedCnt.Image

				updateErr := u.dockerRepo.UpdateContainerImage(ctx, matchedCnt.ID, targetImage)
				if updateErr != nil {
					details["docker_update_error"] = updateErr.Error()
					if u.logger != nil {
						u.logger.Error("failed to update standalone container image upon promotion completion",
							zap.String("container_id", matchedCnt.ID),
							zap.String("target_image", targetImage),
							zap.Error(updateErr))
					}
					if u.auditRepo != nil {
						if recErr := u.auditRepo.RecordAction(ctx, actor, "promote_docker_container", "docker_container", matchedCnt.ID, matchedCnt.Name, "failed", details, "", ""); recErr != nil {
							if u.logger != nil {
								u.logger.Warn("failed to record audit action for failed docker container promotion", zap.Error(recErr))
							}
						}
					}
					return
				}

				if u.auditRepo != nil {
					if recErr := u.auditRepo.RecordAction(ctx, actor, "promote_docker_container", "docker_container", matchedCnt.ID, matchedCnt.Name, "success", details, "", ""); recErr != nil {
						if u.logger != nil {
							u.logger.Warn("failed to record audit action for successful docker container promotion", zap.Error(recErr))
						}
					}
				}
				return
			}
		}
	}

	// Not a Swarm service (or standalone container / external provider): log promotion event in audit log
	if u.auditRepo != nil {
		if recErr := u.auditRepo.RecordAction(ctx, actor, "complete", "promotion", promo.ID, promo.Service, "success", details, "", ""); recErr != nil {
			if u.logger != nil {
				u.logger.Warn("failed to record audit action for completed promotion", zap.Error(recErr))
			}
		}
	}
}

// matchSwarmService matches a service name/ID against available Swarm services.
func matchSwarmService(services []domainDocker.Service, serviceNameOrID string) *domainDocker.Service {
	cleanName := strings.TrimSpace(serviceNameOrID)
	if cleanName == "" {
		return nil
	}

	// 1. Exact ID or Name match
	for i := range services {
		if services[i].ID == cleanName || services[i].Name == cleanName {
			return &services[i]
		}
	}

	// 2. Case-insensitive Name match
	for i := range services {
		if strings.EqualFold(services[i].Name, cleanName) {
			return &services[i]
		}
	}

	// 3. Stack prefix / suffix match (e.g. "tiki_redis" matches "redis" or vice versa)
	for i := range services {
		sName := services[i].Name
		if strings.HasSuffix(sName, "_"+cleanName) || strings.HasSuffix(sName, "-"+cleanName) {
			return &services[i]
		}
		if strings.HasSuffix(cleanName, "_"+sName) || strings.HasSuffix(cleanName, "-"+sName) {
			return &services[i]
		}
	}

	return nil
}

// matchContainer matches a container name/ID against available standalone Docker containers.
func matchContainer(containers []domainDocker.Container, serviceNameOrID string) *domainDocker.Container {
	cleanName := strings.TrimSpace(serviceNameOrID)
	if cleanName == "" {
		return nil
	}

	for i := range containers {
		cName := strings.TrimPrefix(containers[i].Name, "/")
		if containers[i].ID == cleanName || cName == cleanName || strings.EqualFold(cName, cleanName) {
			return &containers[i]
		}
		if strings.HasSuffix(cName, "_"+cleanName) || strings.HasSuffix(cName, "-"+cleanName) {
			return &containers[i]
		}
	}
	return nil
}

// resolveTargetImage constructs the full image tag from current image and promoted version.
func resolveTargetImage(currentImage, version string) string {
	version = strings.TrimSpace(version)
	if version == "" {
		return currentImage
	}

	// If version contains a colon or slash, treat as explicit repository/tag
	if strings.Contains(version, ":") || strings.Contains(version, "/") {
		return version
	}

	// If currentImage is present and contains tag separator, replace the tag
	if currentImage != "" {
		if idx := strings.LastIndex(currentImage, ":"); idx != -1 {
			return currentImage[:idx] + ":" + version
		}
		return currentImage + ":" + version
	}

	return version
}

// List retrieves promotions matching the optional status and pagination parameters.
func (u *Usecase) List(ctx context.Context, status string, limit, offset int) ([]promotion.Promotion, int, error) {
	return u.repo.List(ctx, status, limit, offset)
}

// ListAll retrieves all promotions.
func (u *Usecase) ListAll(ctx context.Context) ([]promotion.Promotion, error) {
	items, _, err := u.repo.List(ctx, "", 0, 0)
	return items, err
}

// GetByID retrieves a promotion by its ID.
func (u *Usecase) GetByID(ctx context.Context, id string) (*promotion.Promotion, error) {
	if strings.TrimSpace(id) == "" {
		return nil, domainerrors.NewValidation("id", "promotion id is required")
	}
	return u.repo.GetByID(ctx, id)
}
