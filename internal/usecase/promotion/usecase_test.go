package promotion

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/datdt/k8sselfhost/internal/domain/audit"
	domainPromo "github.com/datdt/k8sselfhost/internal/domain/promotion"
	domainDocker "github.com/datdt/k8sselfhost/internal/domain/provider/docker"
	domainerrors "github.com/datdt/k8sselfhost/internal/pkg/errors"
)

type mockPromotionRepo struct {
	items map[string]*domainPromo.Promotion
}

func newMockPromotionRepo() *mockPromotionRepo {
	return &mockPromotionRepo{
		items: make(map[string]*domainPromo.Promotion),
	}
}

func (m *mockPromotionRepo) List(ctx context.Context, status string, limit, offset int) ([]domainPromo.Promotion, int, error) {
	var result []domainPromo.Promotion
	for _, p := range m.items {
		if status == "" || status == "all" || p.Status == status {
			result = append(result, *p)
		}
	}
	return result, len(result), nil
}

func (m *mockPromotionRepo) GetByID(ctx context.Context, id string) (*domainPromo.Promotion, error) {
	p, ok := m.items[id]
	if !ok {
		return nil, domainerrors.NewNotFound("promotion", id)
	}
	// return copy
	cp := *p
	return &cp, nil
}

func (m *mockPromotionRepo) Create(ctx context.Context, p *domainPromo.Promotion) error {
	if p.ID == "" {
		p.ID = fmt.Sprintf("promo-%d", len(m.items)+1)
	}
	m.items[p.ID] = p
	return nil
}

func (m *mockPromotionRepo) Approve(ctx context.Context, id, approver string) error {
	p, ok := m.items[id]
	if !ok {
		return errors.New("not found")
	}
	p.Status = domainPromo.StatusApproved
	p.Approver = approver
	now := time.Now().UTC()
	p.ApprovedAt = &now
	return nil
}

func (m *mockPromotionRepo) Reject(ctx context.Context, id, rejecter string) error {
	p, ok := m.items[id]
	if !ok {
		return errors.New("not found")
	}
	p.Status = domainPromo.StatusRejected
	p.Approver = rejecter
	now := time.Now().UTC()
	p.ApprovedAt = &now
	return nil
}

func (m *mockPromotionRepo) Complete(ctx context.Context, id string) error {
	p, ok := m.items[id]
	if !ok {
		return errors.New("not found")
	}
	p.Status = domainPromo.StatusCompleted
	now := time.Now().UTC()
	p.CompletedAt = &now
	return nil
}

func TestUsecase_Create(t *testing.T) {
	ctx := context.Background()

	t.Run("success create promotion", func(t *testing.T) {
		repo := newMockPromotionRepo()
		uc := NewUsecase(repo)

		input := &domainPromo.Promotion{
			Service:   "payment-service",
			Version:   "v1.2.0",
			FromEnv:   domainPromo.EnvDev,
			ToEnv:     domainPromo.EnvQA,
			Requester: "alice",
		}

		res, err := uc.Create(ctx, input)
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if res.ID == "" {
			t.Errorf("expected generated ID, got empty")
		}
		if res.Status != domainPromo.StatusPending {
			t.Errorf("expected initial status pending, got %s", res.Status)
		}
	})

	t.Run("fails when source and target env are equal", func(t *testing.T) {
		repo := newMockPromotionRepo()
		uc := NewUsecase(repo)

		input := &domainPromo.Promotion{
			Service:   "payment-service",
			Version:   "v1.2.0",
			FromEnv:   domainPromo.EnvStaging,
			ToEnv:     domainPromo.EnvStaging,
			Requester: "alice",
		}

		_, err := uc.Create(ctx, input)
		if err == nil {
			t.Fatal("expected validation error for identical environments, got nil")
		}
		if !domainerrors.Is(err, domainerrors.ErrValidation) {
			t.Errorf("expected ErrValidation, got %v", err)
		}
	})

	t.Run("fails when mandatory fields are missing", func(t *testing.T) {
		repo := newMockPromotionRepo()
		uc := NewUsecase(repo)

		// nil input
		if _, err := uc.Create(ctx, nil); err == nil {
			t.Errorf("expected error for nil input")
		}

		// missing service
		if _, err := uc.Create(ctx, &domainPromo.Promotion{Version: "v1", FromEnv: "dev", ToEnv: "qa"}); err == nil {
			t.Errorf("expected error for missing service")
		}

		// missing version
		if _, err := uc.Create(ctx, &domainPromo.Promotion{Service: "svc", FromEnv: "dev", ToEnv: "qa"}); err == nil {
			t.Errorf("expected error for missing version")
		}

		// missing from_env
		if _, err := uc.Create(ctx, &domainPromo.Promotion{Service: "svc", Version: "v1", ToEnv: "qa"}); err == nil {
			t.Errorf("expected error for missing from_env")
		}

		// missing to_env
		if _, err := uc.Create(ctx, &domainPromo.Promotion{Service: "svc", Version: "v1", FromEnv: "dev"}); err == nil {
			t.Errorf("expected error for missing to_env")
		}
	})
}

func TestUsecase_StateTransitions(t *testing.T) {
	ctx := context.Background()

	t.Run("approve pending promotion succeeds", func(t *testing.T) {
		repo := newMockPromotionRepo()
		uc := NewUsecase(repo)

		promo, err := uc.Create(ctx, &domainPromo.Promotion{
			Service: "order-service",
			Version: "v2.0.0",
			FromEnv: domainPromo.EnvQA,
			ToEnv:   domainPromo.EnvStaging,
		})
		if err != nil {
			t.Fatalf("create failed: %v", err)
		}

		err = uc.Approve(ctx, promo.ID, "bob")
		if err != nil {
			t.Fatalf("approve failed: %v", err)
		}

		stored, err := uc.GetByID(ctx, promo.ID)
		if err != nil {
			t.Fatalf("get failed: %v", err)
		}
		if stored.Status != domainPromo.StatusApproved {
			t.Errorf("expected status approved, got %s", stored.Status)
		}
		if stored.Approver != "bob" {
			t.Errorf("expected approver bob, got %s", stored.Approver)
		}
	})

	t.Run("cannot approve already approved or rejected promotion", func(t *testing.T) {
		repo := newMockPromotionRepo()
		uc := NewUsecase(repo)

		promo, _ := uc.Create(ctx, &domainPromo.Promotion{
			Service: "order-service",
			Version: "v2.0.0",
			FromEnv: domainPromo.EnvQA,
			ToEnv:   domainPromo.EnvStaging,
		})

		_ = uc.Approve(ctx, promo.ID, "bob")

		// Re-approving must fail
		err := uc.Approve(ctx, promo.ID, "bob")
		if err == nil {
			t.Fatal("expected error approving already approved promotion, got nil")
		}
		if !domainerrors.Is(err, domainerrors.ErrValidation) {
			t.Errorf("expected ErrValidation, got %v", err)
		}
	})

	t.Run("reject pending promotion succeeds", func(t *testing.T) {
		repo := newMockPromotionRepo()
		uc := NewUsecase(repo)

		promo, _ := uc.Create(ctx, &domainPromo.Promotion{
			Service: "order-service",
			Version: "v2.0.0",
			FromEnv: domainPromo.EnvQA,
			ToEnv:   domainPromo.EnvStaging,
		})

		err := uc.Reject(ctx, promo.ID, "charlie")
		if err != nil {
			t.Fatalf("reject failed: %v", err)
		}

		stored, _ := uc.GetByID(ctx, promo.ID)
		if stored.Status != domainPromo.StatusRejected {
			t.Errorf("expected status rejected, got %s", stored.Status)
		}
	})

	t.Run("cannot reject non-pending promotion", func(t *testing.T) {
		repo := newMockPromotionRepo()
		uc := NewUsecase(repo)

		promo, _ := uc.Create(ctx, &domainPromo.Promotion{
			Service: "order-service",
			Version: "v2.0.0",
			FromEnv: domainPromo.EnvQA,
			ToEnv:   domainPromo.EnvStaging,
		})

		_ = uc.Approve(ctx, promo.ID, "bob")

		// Rejecting approved promotion must fail
		err := uc.Reject(ctx, promo.ID, "charlie")
		if err == nil {
			t.Fatal("expected error rejecting approved promotion, got nil")
		}
		if !domainerrors.Is(err, domainerrors.ErrValidation) {
			t.Errorf("expected ErrValidation, got %v", err)
		}
	})

	t.Run("complete approved promotion succeeds", func(t *testing.T) {
		repo := newMockPromotionRepo()
		uc := NewUsecase(repo)

		promo, _ := uc.Create(ctx, &domainPromo.Promotion{
			Service: "order-service",
			Version: "v2.0.0",
			FromEnv: domainPromo.EnvQA,
			ToEnv:   domainPromo.EnvStaging,
		})

		_ = uc.Approve(ctx, promo.ID, "bob")

		err := uc.Complete(ctx, promo.ID)
		if err != nil {
			t.Fatalf("complete failed: %v", err)
		}

		stored, _ := uc.GetByID(ctx, promo.ID)
		if stored.Status != domainPromo.StatusCompleted {
			t.Errorf("expected status completed, got %s", stored.Status)
		}
	})

	t.Run("cannot complete pending or rejected promotion", func(t *testing.T) {
		repo := newMockPromotionRepo()
		uc := NewUsecase(repo)

		promo, _ := uc.Create(ctx, &domainPromo.Promotion{
			Service: "order-service",
			Version: "v2.0.0",
			FromEnv: domainPromo.EnvQA,
			ToEnv:   domainPromo.EnvStaging,
		})

		// Complete while still pending
		err := uc.Complete(ctx, promo.ID)
		if err == nil {
			t.Fatal("expected error completing pending promotion, got nil")
		}
		if !domainerrors.Is(err, domainerrors.ErrValidation) {
			t.Errorf("expected ErrValidation, got %v", err)
		}

		// Reject then complete
		_ = uc.Reject(ctx, promo.ID, "charlie")
		err = uc.Complete(ctx, promo.ID)
		if err == nil {
			t.Fatal("expected error completing rejected promotion, got nil")
		}
		if !domainerrors.Is(err, domainerrors.ErrValidation) {
			t.Errorf("expected ErrValidation, got %v", err)
		}
	})

	t.Run("validation on empty IDs", func(t *testing.T) {
		repo := newMockPromotionRepo()
		uc := NewUsecase(repo)

		if err := uc.Approve(ctx, ""); err == nil {
			t.Errorf("expected error for empty id in Approve")
		}
		if err := uc.Reject(ctx, ""); err == nil {
			t.Errorf("expected error for empty id in Reject")
		}
		if err := uc.Complete(ctx, ""); err == nil {
			t.Errorf("expected error for empty id in Complete")
		}
		if _, err := uc.GetByID(ctx, ""); err == nil {
			t.Errorf("expected error for empty id in GetByID")
		}
	})

	t.Run("list promotions", func(t *testing.T) {
		repo := newMockPromotionRepo()
		uc := NewUsecase(repo)

		_, _ = uc.Create(ctx, &domainPromo.Promotion{
			Service: "svc-1",
			Version: "v1",
			FromEnv: "dev",
			ToEnv:   "qa",
		})
		_, _ = uc.Create(ctx, &domainPromo.Promotion{
			Service: "svc-2",
			Version: "v2",
			FromEnv: "qa",
			ToEnv:   "staging",
		})

		items, total, err := uc.List(ctx, "", 10, 0)
		if err != nil {
			t.Fatalf("list failed: %v", err)
		}
		if total != 2 || len(items) != 2 {
			t.Errorf("expected 2 items, got total=%d len=%d", total, len(items))
		}

		all, err := uc.ListAll(ctx)
		if err != nil {
			t.Fatalf("listAll failed: %v", err)
		}
		if len(all) != 2 {
			t.Errorf("expected 2 items, got %d", len(all))
		}
	})
}

// Mocks for Docker and Audit domain interfaces
type mockDockerRepoForUsecase struct {
	services          []domainDocker.Service
	containers        []domainDocker.Container
	updatedServices   map[string]string
	updatedContainers map[string]string
	updateErr         error
	containerErr      error
}

func newMockDockerRepoForUsecase() *mockDockerRepoForUsecase {
	return &mockDockerRepoForUsecase{
		updatedServices:   make(map[string]string),
		updatedContainers: make(map[string]string),
	}
}

func (m *mockDockerRepoForUsecase) ListContainers(ctx context.Context) ([]domainDocker.Container, error) {
	return m.containers, nil
}
func (m *mockDockerRepoForUsecase) ListNodes(ctx context.Context) ([]domainDocker.Node, error) {
	return nil, nil
}
func (m *mockDockerRepoForUsecase) ListServices(ctx context.Context) ([]domainDocker.Service, error) {
	return m.services, nil
}
func (m *mockDockerRepoForUsecase) ScaleService(ctx context.Context, serviceID string, replicas int) error {
	return nil
}
func (m *mockDockerRepoForUsecase) UpdateNodeAvailability(ctx context.Context, nodeID string, availability string) error {
	return nil
}
func (m *mockDockerRepoForUsecase) ToggleContainer(ctx context.Context, containerID string, action string) error {
	return nil
}
func (m *mockDockerRepoForUsecase) DeleteService(ctx context.Context, serviceID string) error {
	return nil
}
func (m *mockDockerRepoForUsecase) RestartService(ctx context.Context, serviceID string) error {
	return nil
}
func (m *mockDockerRepoForUsecase) CreateService(ctx context.Context, name string, image string, replicas int, port int) error {
	return nil
}
func (m *mockDockerRepoForUsecase) GetLogs(ctx context.Context, targetID string, targetType string) (string, error) {
	return "", nil
}
func (m *mockDockerRepoForUsecase) GetLogsWithOptions(ctx context.Context, targetID string, targetType string, tail string, since string) (string, error) {
	return "", nil
}
func (m *mockDockerRepoForUsecase) UpdateServiceImage(ctx context.Context, serviceID string, image string) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	m.updatedServices[serviceID] = image
	return nil
}
func (m *mockDockerRepoForUsecase) UpdateContainerImage(ctx context.Context, containerID string, image string) error {
	if m.containerErr != nil {
		return m.containerErr
	}
	m.updatedContainers[containerID] = image
	return nil
}
func (m *mockDockerRepoForUsecase) UpdateServiceResources(ctx context.Context, serviceID string, memoryLimitBytes int64, memoryReservBytes int64, nanoCPUs int64, replicas int) error {
	return nil
}
func (m *mockDockerRepoForUsecase) GetSwarmJoinTokens(ctx context.Context) (*domainDocker.SwarmTokens, error) {
	return nil, nil
}
func (m *mockDockerRepoForUsecase) DrainNode(ctx context.Context, nodeID string) error {
	return nil
}
func (m *mockDockerRepoForUsecase) ActivateNode(ctx context.Context, nodeID string) error {
	return nil
}
func (m *mockDockerRepoForUsecase) RemoveNode(ctx context.Context, nodeID string, force bool) error {
	return nil
}
func (m *mockDockerRepoForUsecase) GetNodeDetails(ctx context.Context, nodeID string) (*domainDocker.NodeDetails, error) {
	return nil, nil
}
func (m *mockDockerRepoForUsecase) GetSwarmInfo(ctx context.Context) (*domainDocker.SwarmInfo, error) {
	return nil, nil
}

type auditRecord struct {
	Actor      string
	Action     string
	TargetType string
	TargetID   string
	TargetName string
	Result     string
	Details    map[string]interface{}
}

type mockAuditRepoForUsecase struct {
	records []auditRecord
}

func (m *mockAuditRepoForUsecase) ListFindings(ctx context.Context, status string) ([]audit.AuditFinding, error) {
	return nil, nil
}
func (m *mockAuditRepoForUsecase) GetFinding(ctx context.Context, id string) (*audit.AuditFinding, error) {
	return nil, nil
}
func (m *mockAuditRepoForUsecase) ResolveFinding(ctx context.Context, id string) error {
	return nil
}
func (m *mockAuditRepoForUsecase) RecordRun(ctx context.Context, run *audit.AuditRun) error {
	return nil
}
func (m *mockAuditRepoForUsecase) GetLastRun(ctx context.Context) (*audit.AuditRun, error) {
	return nil, nil
}
func (m *mockAuditRepoForUsecase) RecordAction(ctx context.Context, actor, action, targetType, targetID, targetName, result string, details map[string]interface{}, ipAddress, userAgent string) error {
	m.records = append(m.records, auditRecord{
		Actor:      actor,
		Action:     action,
		TargetType: targetType,
		TargetID:   targetID,
		TargetName: targetName,
		Result:     result,
		Details:    details,
	})
	return nil
}

func TestUsecase_Complete_DockerSwarmServiceUpdate_ExactName(t *testing.T) {
	ctx := context.Background()
	repo := newMockPromotionRepo()
	dockerMock := newMockDockerRepoForUsecase()
	auditMock := &mockAuditRepoForUsecase{}

	dockerMock.services = []domainDocker.Service{
		{ID: "svc-redis-id", Name: "tiki_redis", Image: "redis:7.0-alpine"},
	}

	uc := NewUsecase(repo, dockerMock, auditMock)

	promo, err := uc.Create(ctx, &domainPromo.Promotion{
		Service:   "tiki_redis",
		Version:   "redis:7.2-alpine",
		FromEnv:   domainPromo.EnvStaging,
		ToEnv:     domainPromo.EnvProduction,
		Requester: "alice",
	})
	if err != nil {
		t.Fatalf("create failed: %v", err)
	}

	_ = uc.Approve(ctx, promo.ID, "bob")

	err = uc.Complete(ctx, promo.ID, "admin-user")
	if err != nil {
		t.Fatalf("complete failed: %v", err)
	}

	// Verify service image was updated
	if dockerMock.updatedServices["svc-redis-id"] != "redis:7.2-alpine" {
		t.Errorf("expected updated image 'redis:7.2-alpine', got '%s'", dockerMock.updatedServices["svc-redis-id"])
	}

	// Verify audit log
	if len(auditMock.records) != 1 {
		t.Fatalf("expected 1 audit record, got %d", len(auditMock.records))
	}
	rec := auditMock.records[0]
	if rec.Action != "promote_docker_service" || rec.Result != "success" || rec.TargetName != "tiki_redis" {
		t.Errorf("unexpected audit record: %+v", rec)
	}
	if rec.Actor != "admin-user" {
		t.Errorf("expected actor 'admin-user', got '%s'", rec.Actor)
	}
}

func TestUsecase_Complete_DockerSwarmServiceUpdate_StackPrefixAndTags(t *testing.T) {
	ctx := context.Background()
	repo := newMockPromotionRepo()
	dockerMock := newMockDockerRepoForUsecase()
	auditMock := &mockAuditRepoForUsecase{}

	dockerMock.services = []domainDocker.Service{
		{ID: "svc-redis-1", Name: "tiki_redis", Image: "redis:7.0-alpine"},
		{ID: "svc-nginx-1", Name: "my-nginx", Image: "nginx:1.24"},
		{ID: "svc-traefik-1", Name: "tiki_traefik", Image: "traefik:v2.10"},
	}

	uc := NewUsecase(repo, dockerMock, auditMock)

	// 1. Match "redis" to "tiki_redis" and tag "7.2-alpine" -> "redis:7.2-alpine"
	p1, _ := uc.Create(ctx, &domainPromo.Promotion{
		Service: "redis",
		Version: "7.2-alpine",
		FromEnv: domainPromo.EnvDev,
		ToEnv:   domainPromo.EnvQA,
	})
	_ = uc.Approve(ctx, p1.ID, "bob")
	if err := uc.Complete(ctx, p1.ID); err != nil {
		t.Fatalf("complete p1 failed: %v", err)
	}
	if dockerMock.updatedServices["svc-redis-1"] != "redis:7.2-alpine" {
		t.Errorf("expected redis:7.2-alpine, got %s", dockerMock.updatedServices["svc-redis-1"])
	}

	// 2. Match "my-nginx" directly with version "1.25.4" -> "nginx:1.25.4"
	p2, _ := uc.Create(ctx, &domainPromo.Promotion{
		Service: "my-nginx",
		Version: "1.25.4",
		FromEnv: domainPromo.EnvDev,
		ToEnv:   domainPromo.EnvQA,
	})
	_ = uc.Approve(ctx, p2.ID, "bob")
	if err := uc.Complete(ctx, p2.ID); err != nil {
		t.Fatalf("complete p2 failed: %v", err)
	}
	if dockerMock.updatedServices["svc-nginx-1"] != "nginx:1.25.4" {
		t.Errorf("expected nginx:1.25.4, got %s", dockerMock.updatedServices["svc-nginx-1"])
	}

	// 3. Match "traefik" to "tiki_traefik" with full image "traefik:v3.0"
	p3, _ := uc.Create(ctx, &domainPromo.Promotion{
		Service: "traefik",
		Version: "traefik:v3.0",
		FromEnv: domainPromo.EnvDev,
		ToEnv:   domainPromo.EnvQA,
	})
	_ = uc.Approve(ctx, p3.ID, "bob")
	if err := uc.Complete(ctx, p3.ID); err != nil {
		t.Fatalf("complete p3 failed: %v", err)
	}
	if dockerMock.updatedServices["svc-traefik-1"] != "traefik:v3.0" {
		t.Errorf("expected traefik:v3.0, got %s", dockerMock.updatedServices["svc-traefik-1"])
	}
}

func TestUsecase_Complete_StandaloneContainer_AuditLogged(t *testing.T) {
	ctx := context.Background()
	repo := newMockPromotionRepo()
	dockerMock := newMockDockerRepoForUsecase()
	auditMock := &mockAuditRepoForUsecase{}

	dockerMock.containers = []domainDocker.Container{
		{ID: "cnt-standalone-1", Name: "/my-container", Image: "alpine:3.19"},
	}

	uc := NewUsecase(repo, dockerMock, auditMock)

	promo, _ := uc.Create(ctx, &domainPromo.Promotion{
		Service:   "my-container",
		Version:   "alpine:3.20",
		FromEnv:   domainPromo.EnvQA,
		ToEnv:     domainPromo.EnvProduction,
		Requester: "alice",
	})
	_ = uc.Approve(ctx, promo.ID, "bob")

	err := uc.Complete(ctx, promo.ID, "admin-user")
	if err != nil {
		t.Fatalf("complete failed: %v", err)
	}

	// Swarm service update should NOT be triggered
	if len(dockerMock.updatedServices) != 0 {
		t.Errorf("expected 0 updated swarm services, got %d", len(dockerMock.updatedServices))
	}

	// Container update SHOULD be triggered
	if dockerMock.updatedContainers["cnt-standalone-1"] != "alpine:3.20" {
		t.Errorf("expected standalone container image 'alpine:3.20', got '%s'", dockerMock.updatedContainers["cnt-standalone-1"])
	}

	// Audit record should be logged
	if len(auditMock.records) != 1 {
		t.Fatalf("expected 1 audit record, got %d", len(auditMock.records))
	}
	rec := auditMock.records[0]
	if rec.Action != "promote_docker_container" || rec.Result != "success" || rec.TargetName != "/my-container" {
		t.Errorf("unexpected audit record: %+v", rec)
	}
	if isStandalone, ok := rec.Details["standalone_container"].(bool); !ok || !isStandalone {
		t.Errorf("expected standalone_container detail to be true, got %v", rec.Details["standalone_container"])
	}
}

func TestUsecase_Complete_StandaloneContainer_Error_Handled(t *testing.T) {
	ctx := context.Background()
	repo := newMockPromotionRepo()
	dockerMock := newMockDockerRepoForUsecase()
	auditMock := &mockAuditRepoForUsecase{}

	dockerMock.containers = []domainDocker.Container{
		{ID: "cnt-err-1", Name: "/my-container", Image: "alpine:3.19"},
	}
	dockerMock.containerErr = errors.New("cannot create container: name collision")

	uc := NewUsecase(repo, dockerMock, auditMock)

	promo, _ := uc.Create(ctx, &domainPromo.Promotion{
		Service: "my-container",
		Version: "alpine:3.20",
		FromEnv: domainPromo.EnvQA,
		ToEnv:   domainPromo.EnvProduction,
	})
	_ = uc.Approve(ctx, promo.ID, "bob")

	err := uc.Complete(ctx, promo.ID)
	if err != nil {
		t.Fatalf("complete returned unexpected error: %v", err)
	}

	if len(auditMock.records) != 1 {
		t.Fatalf("expected 1 audit record, got %d", len(auditMock.records))
	}
	rec := auditMock.records[0]
	if rec.Action != "promote_docker_container" || rec.Result != "failed" {
		t.Errorf("expected failed docker container audit record, got %+v", rec)
	}
}

func TestUsecase_Complete_NoDockerProvider_AuditLogged(t *testing.T) {
	ctx := context.Background()
	repo := newMockPromotionRepo()
	auditMock := &mockAuditRepoForUsecase{}

	uc := NewUsecase(repo, auditMock)

	promo, _ := uc.Create(ctx, &domainPromo.Promotion{
		Service:   "payment-api",
		Version:   "v3.0.0",
		FromEnv:   domainPromo.EnvStaging,
		ToEnv:     domainPromo.EnvProduction,
		Requester: "charlie",
	})
	_ = uc.Approve(ctx, promo.ID, "lead")

	err := uc.Complete(ctx, promo.ID)
	if err != nil {
		t.Fatalf("complete failed: %v", err)
	}

	if len(auditMock.records) != 1 {
		t.Fatalf("expected 1 audit record, got %d", len(auditMock.records))
	}
	rec := auditMock.records[0]
	if rec.Action != "complete" || rec.TargetType != "promotion" {
		t.Errorf("unexpected audit record: %+v", rec)
	}
	if rec.Actor != "lead" {
		t.Errorf("expected fallback actor 'lead' from approver, got '%s'", rec.Actor)
	}
}

func TestUsecase_Complete_DockerUpdateError_Handled(t *testing.T) {
	ctx := context.Background()
	repo := newMockPromotionRepo()
	dockerMock := newMockDockerRepoForUsecase()
	auditMock := &mockAuditRepoForUsecase{}

	dockerMock.services = []domainDocker.Service{
		{ID: "svc-err-1", Name: "tiki_redis", Image: "redis:7.0-alpine"},
	}
	dockerMock.updateErr = errors.New("docker daemon connection refused")

	uc := NewUsecase(repo, dockerMock, auditMock)

	promo, _ := uc.Create(ctx, &domainPromo.Promotion{
		Service: "tiki_redis",
		Version: "redis:7.2-alpine",
		FromEnv: domainPromo.EnvDev,
		ToEnv:   domainPromo.EnvQA,
	})
	_ = uc.Approve(ctx, promo.ID, "bob")

	err := uc.Complete(ctx, promo.ID)
	if err != nil {
		t.Fatalf("complete returned unexpected error: %v", err)
	}

	if len(auditMock.records) != 1 {
		t.Fatalf("expected 1 audit record, got %d", len(auditMock.records))
	}
	rec := auditMock.records[0]
	if rec.Action != "promote_docker_service" || rec.Result != "failed" {
		t.Errorf("expected failed docker service audit record, got %+v", rec)
	}
}

func TestResolveTargetImage(t *testing.T) {
	tests := []struct {
		current  string
		version  string
		expected string
	}{
		{"redis:6.2-alpine", "redis:7.2-alpine", "redis:7.2-alpine"},
		{"redis:6.2-alpine", "7.2-alpine", "redis:7.2-alpine"},
		{"nginx:1.24", "1.25.3", "nginx:1.25.3"},
		{"docker.io/myorg/myapp:v1.0.0", "v1.1.0", "docker.io/myorg/myapp:v1.1.0"},
		{"myorg/myapp", "v1.1.0", "myorg/myapp:v1.1.0"},
		{"", "redis:7.0", "redis:7.0"},
		{"", "v1.0.0", "v1.0.0"},
	}

	for _, tt := range tests {
		res := resolveTargetImage(tt.current, tt.version)
		if res != tt.expected {
			t.Errorf("resolveTargetImage(%q, %q) = %q; want %q", tt.current, tt.version, res, tt.expected)
		}
	}
}

