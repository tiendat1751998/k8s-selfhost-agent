package postgres

import (
	"context"
	"fmt"

	"github.com/datdt/k8sselfhost/internal/domain/search"
)

type searchRepo struct {
	db      DBTX
	builder *SearchQueryBuilder
}

// NewSearchRepo creates a new Postgres-backed Search repository.
func NewSearchRepo(db DBTX) search.Repository {
	return &searchRepo{
		db:      db,
		builder: NewSearchQueryBuilder(),
	}
}

func (r *searchRepo) getDB(ctx context.Context) DBTX {
	return ExtractTx(ctx, r.db)
}

// allowedSearchTypes retains internal package-level backward compatibility.
var allowedSearchTypes = AllowedSearchTypes

func (r *searchRepo) Search(ctx context.Context, q string, searchType string) ([]search.Result, error) {
	queries, err := r.builder.BuildQueries(ctx, q, searchType)
	if err != nil {
		return nil, err
	}

	var results []search.Result
	for _, querySpec := range queries {
		rows, queryErr := r.getDB(ctx).Query(ctx, querySpec.SQL, querySpec.Args...)
		if queryErr != nil {
			return nil, fmt.Errorf("searching %s: %w", querySpec.Domain, queryErr)
		}

		scanErr := func() error {
			defer rows.Close()
			for rows.Next() {
				var res search.Result
				if err := rows.Scan(&res.Type, &res.Title, &res.Desc); err != nil {
					return fmt.Errorf("scanning %s row: %w", querySpec.Domain, err)
				}
				results = append(results, res)
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterating %s rows: %w", querySpec.Domain, err)
			}
			return nil
		}()

		if scanErr != nil {
			return nil, scanErr
		}
	}

	return results, nil
}
