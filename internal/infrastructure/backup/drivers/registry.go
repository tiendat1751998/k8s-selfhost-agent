package drivers

import (
	"fmt"
	"sync"

	"github.com/datdt/k8sselfhost/internal/domain/backup"
	"github.com/datdt/k8sselfhost/internal/pkg/errors"
)

type DriverRegistry struct {
	mu      sync.RWMutex
	drivers map[string]backup.DatabaseDriver
}

func NewDriverRegistry() *DriverRegistry {
	r := &DriverRegistry{
		drivers: make(map[string]backup.DatabaseDriver),
	}
	// Register default drivers
	r.Register(NewPostgresDriver())
	r.Register(NewMySQLDriver())
	r.Register(NewMariaDBDriver())
	r.Register(NewSQLServerDriver("sqlserver"))
	r.Register(NewSQLServerDriver("mssql"))
	r.Register(NewOracleDriver())
	r.Register(NewMongoDBDriver())
	r.Register(NewRedisDriver())
	return r
}

func (r *DriverRegistry) Register(d backup.DatabaseDriver) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.drivers[d.Type()] = d
}

func (r *DriverRegistry) Get(dbType string) (backup.DatabaseDriver, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	d, ok := r.drivers[dbType]
	if !ok {
		return nil, errors.NewNotFound("database_driver", fmt.Sprintf("unsupported db type: %s", dbType))
	}
	return d, nil
}
