package tenant

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// TenantService is the thin domain service for tenant lifecycle operations.
// All persistence goes through EntityRepository — no raw SQL here.
type TenantService interface {
	// Create provisions a new tenant record in PENDING status.
	Create(ctx context.Context, input CreateInput) (*TenantDTO, error)

	// GetByID returns a tenant by its UUID primary key.
	GetByID(ctx context.Context, id uuid.UUID) (*TenantDTO, error)

	// GetBySlug returns a tenant by its URL slug.
	GetBySlug(ctx context.Context, slug string) (*TenantDTO, error)

	// Activate transitions a tenant from PENDING to ACTIVE.
	Activate(ctx context.Context, id uuid.UUID) error

	// Suspend transitions a tenant from ACTIVE to SUSPENDED.
	Suspend(ctx context.Context, id uuid.UUID, reason string) error

	// Archive transitions a tenant to the terminal ARCHIVED state.
	Archive(ctx context.Context, id uuid.UUID) error

	// List returns all tenants (platform-admin view only).
	List(ctx context.Context, opts ListOptions) ([]*TenantDTO, error)
}

// CreateInput holds the fields required to create a new tenant.
type CreateInput struct {
	Name         string
	Slug         string
	ContactEmail string
	Plan         string
	Country      string
	Locale       string
	Timezone     string
	Currency     string
}

// ListOptions controls pagination and filtering for tenant list queries.
type ListOptions struct {
	Status string
	Limit  int
	Offset int
}

// TenantDTO is the read model returned by TenantService methods.
type TenantDTO struct {
	ID       uuid.UUID
	Name     string
	Slug     string
	Status   string
	Plan     string
	Country  string
	Locale   string
	Timezone string
	Currency string
}

// stubTenantService is the stub implementation. Replace with a real
// implementation that calls EntityRepository methods.
type stubTenantService struct{}

// NewTenantService returns a stub TenantService.
// TODO: inject EntityRepository[platform_tenant] when implementing for real.
func NewTenantService() TenantService {
	return &stubTenantService{}
}

func (s *stubTenantService) Create(_ context.Context, _ CreateInput) (*TenantDTO, error) {
	return nil, fmt.Errorf("not implemented: TenantService.Create")
}

func (s *stubTenantService) GetByID(_ context.Context, _ uuid.UUID) (*TenantDTO, error) {
	return nil, fmt.Errorf("not implemented: TenantService.GetByID")
}

func (s *stubTenantService) GetBySlug(_ context.Context, _ string) (*TenantDTO, error) {
	return nil, fmt.Errorf("not implemented: TenantService.GetBySlug")
}

func (s *stubTenantService) Activate(_ context.Context, _ uuid.UUID) error {
	return fmt.Errorf("not implemented: TenantService.Activate")
}

func (s *stubTenantService) Suspend(_ context.Context, _ uuid.UUID, _ string) error {
	return fmt.Errorf("not implemented: TenantService.Suspend")
}

func (s *stubTenantService) Archive(_ context.Context, _ uuid.UUID) error {
	return fmt.Errorf("not implemented: TenantService.Archive")
}

func (s *stubTenantService) List(_ context.Context, _ ListOptions) ([]*TenantDTO, error) {
	return nil, fmt.Errorf("not implemented: TenantService.List")
}
