package subnet

import (
	"context"
	"fmt"
	"log/slog"
	"net"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	WhitelistTypeID int = 1
	BlacklistTypeID int = 2
)

type Repository struct {
	pool   *pgxpool.Pool
	logger *slog.Logger
}

func NewRepository(pool *pgxpool.Pool, logger *slog.Logger) *Repository {
	return &Repository{
		pool:   pool,
		logger: logger,
	}
}

func (r *Repository) Add(ctx context.Context, listType int, cidr string) error {
	if _, _, err := net.ParseCIDR(cidr); err != nil {
		return fmt.Errorf("invalid cidr %q: %w", cidr, err)
	}

	_, err := r.pool.Exec(ctx,
		`INSERT INTO subnets (subnet_type, subnet) VALUES ($1, $2)
         ON CONFLICT (subnet_type, subnet) DO NOTHING`,
		listType, cidr)
	if err != nil {
		return fmt.Errorf("failed to add subnet %q to list type %d: %w", cidr, listType, err)
	}

	return nil
}

func (r *Repository) Remove(ctx context.Context, listType int, cidr string) (deletedCount int64, err error) {
	if _, _, err := net.ParseCIDR(cidr); err != nil {
		return 0, fmt.Errorf("invalid cidr %q: %w", cidr, err)
	}

	cmdTag, err := r.pool.Exec(ctx,
		`DELETE FROM subnets WHERE subnet_type = $1 AND subnet = $2`,
		listType, cidr)
	if err != nil {
		return 0, fmt.Errorf("failed to remove subnet %q from list type %d: %w", cidr, listType, err)
	}

	return cmdTag.RowsAffected(), nil
}

func (r *Repository) ListWithOffsetLimit(ctx context.Context, listType int, offset, limit uint64) ([]string, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT subnet::text FROM subnets WHERE subnet_type = $1
         ORDER BY subnet
         OFFSET $2 LIMIT $3`, listType, offset, limit)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to query subnets for list type %d (offset=%d, limit=%d): %w",
			listType, offset, limit, err)
	}
	defer rows.Close()

	var subnets []string
	for rows.Next() {
		var cidr string
		if err := rows.Scan(&cidr); err != nil {
			return nil, fmt.Errorf("failed to scan subnet row for list type %d: %w", listType, err)
		}
		subnets = append(subnets, cidr)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating subnets for list type %d: %w", listType, err)
	}

	return subnets, nil
}

func (r *Repository) List(ctx context.Context, listType int) ([]string, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT subnet::text FROM subnets WHERE subnet_type = $1 ORDER BY subnet`,
		listType)
	if err != nil {
		return nil, fmt.Errorf("failed to query all subnets for list type %d: %w", listType, err)
	}
	defer rows.Close()

	var subnets []string
	for rows.Next() {
		var cidr string
		if err := rows.Scan(&cidr); err != nil {
			return nil, fmt.Errorf("failed to scan subnet row for list type %d: %w", listType, err)
		}
		subnets = append(subnets, cidr)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating all subnets for list type %d: %w", listType, err)
	}

	return subnets, nil
}

func (r *Repository) GetBothLists(ctx context.Context) (whitelistSubnets, blacklistSubnets []string, err error) {
	rows, err := r.pool.Query(ctx,
		`SELECT subnet_type, subnet::text FROM subnets 
		 WHERE subnet_type IN ($1, $2) 
		 ORDER BY subnet_type, subnet`,
		WhitelistTypeID, BlacklistTypeID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to query both subnet lists: %w", err)
	}
	defer rows.Close()

	var whitelist, blacklist []string
	for rows.Next() {
		var listType int
		var cidr string
		if err := rows.Scan(&listType, &cidr); err != nil {
			return nil, nil, fmt.Errorf("failed to scan subnet row: %w", err)
		}

		switch listType {
		case WhitelistTypeID:
			whitelist = append(whitelist, cidr)
		case BlacklistTypeID:
			blacklist = append(blacklist, cidr)
		default:
			return nil, nil, fmt.Errorf("invalid subnet type %d", listType)
		}
	}

	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("error iterating both subnet lists: %w", err)
	}

	return whitelist, blacklist, nil
}

//nolint:lll
func (r *Repository) CheckIPInBothLists(ctx context.Context, ip string) (inWhitelist bool, inBlacklist bool, err error) {
	err = r.pool.QueryRow(ctx,
		`SELECT
			EXISTS(SELECT 1 FROM subnets WHERE subnet_type = $1 AND subnet >> $3::inet LIMIT 1) AS in_whitelist,
			EXISTS(SELECT 1 FROM subnets WHERE subnet_type = $2 AND subnet >> $3::inet LIMIT 1) AS in_blacklist`,
		WhitelistTypeID, BlacklistTypeID, ip).Scan(&inWhitelist, &inBlacklist)
	if err != nil {
		return false, false, fmt.Errorf("failed to check IP %q in both lists: %w", ip, err)
	}

	return inWhitelist, inBlacklist, nil
}
