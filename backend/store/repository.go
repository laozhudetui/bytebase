package store

import (
	"context"
	"fmt"
	"strings"

	"github.com/bytebase/bytebase"
	"github.com/bytebase/bytebase/api"
	"go.uber.org/zap"
)

var (
	_ api.RepositoryService = (*RepositoryService)(nil)
)

// RepositoryService represents a service for managing repository.
type RepositoryService struct {
	l  *zap.Logger
	db *DB
}

// NewRepositoryService returns a new instance of RepositoryService.
func NewRepositoryService(logger *zap.Logger, db *DB) *RepositoryService {
	return &RepositoryService{l: logger, db: db}
}

// CreateRepository creates a new repository.
func (s *RepositoryService) CreateRepository(ctx context.Context, create *api.RepositoryCreate) (*api.Repository, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, FormatError(err)
	}
	defer tx.Rollback()

	repository, err := createRepository(ctx, tx, create)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, FormatError(err)
	}

	return repository, nil
}

// FindRepositoryList retrieves a list of repositorys based on find.
func (s *RepositoryService) FindRepositoryList(ctx context.Context, find *api.RepositoryFind) ([]*api.Repository, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, FormatError(err)
	}
	defer tx.Rollback()

	list, err := findRepositoryList(ctx, tx, find)
	if err != nil {
		return []*api.Repository{}, err
	}

	return list, nil
}

// FindRepository retrieves a single repository based on find.
// Returns ENOTFOUND if no matching record.
// Returns the first matching one and prints a warning if finding more than 1 matching records.
func (s *RepositoryService) FindRepository(ctx context.Context, find *api.RepositoryFind) (*api.Repository, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, FormatError(err)
	}
	defer tx.Rollback()

	list, err := findRepositoryList(ctx, tx, find)
	if err != nil {
		return nil, err
	} else if len(list) == 0 {
		return nil, &bytebase.Error{Code: bytebase.ENOTFOUND, Message: fmt.Sprintf("repository not found: %v", find)}
	} else if len(list) > 1 {
		s.l.Warn(fmt.Sprintf("found mulitple repositories: %d, expect 1", len(list)))
	}
	return list[0], nil
}

// PatchRepository updates an existing repository by ID.
// Returns ENOTFOUND if repository does not exist.
func (s *RepositoryService) PatchRepository(ctx context.Context, patch *api.RepositoryPatch) (*api.Repository, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, FormatError(err)
	}
	defer tx.Rollback()

	repository, err := patchRepository(ctx, tx, patch)
	if err != nil {
		return nil, FormatError(err)
	}

	if err := tx.Commit(); err != nil {
		return nil, FormatError(err)
	}

	return repository, nil
}

// DeleteRepository deletes an existing repository by ID.
// Returns ENOTFOUND if repository does not exist.
func (s *RepositoryService) DeleteRepository(ctx context.Context, delete *api.RepositoryDelete) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return FormatError(err)
	}
	defer tx.Rollback()

	err = deleteRepository(ctx, tx, delete)
	if err != nil {
		return FormatError(err)
	}

	if err := tx.Commit(); err != nil {
		return FormatError(err)
	}

	return nil
}

// createRepository creates a new repository.
func createRepository(ctx context.Context, tx *Tx, create *api.RepositoryCreate) (*api.Repository, error) {
	// Insert row into database.
	row, err := tx.QueryContext(ctx, `
		INSERT INTO repo (
		    creator_id,
		    updater_id,
			vcs_id,
			project_id,
			name,
			full_path,
			web_url,
			base_directory,
			branch_filter,
			external_id,
			webhook_id
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		RETURNING id, creator_id, created_ts, updater_id, updated_ts, vcs_id, project_id, name, full_path, web_url, base_directory, branch_filter, external_id, webhook_id
	`,
		create.CreatorId,
		create.CreatorId,
		create.VCSId,
		create.ProjectId,
		create.Name,
		create.FullPath,
		create.WebURL,
		create.BaseDirectory,
		create.BranchFilter,
		create.ExternalId,
		create.WebhookId,
	)

	if err != nil {
		return nil, FormatError(err)
	}
	defer row.Close()

	row.Next()
	var repository api.Repository
	if err := row.Scan(
		&repository.ID,
		&repository.CreatorId,
		&repository.CreatedTs,
		&repository.UpdaterId,
		&repository.UpdatedTs,
		&repository.VCSId,
		&repository.ProjectId,
		&repository.Name,
		&repository.FullPath,
		&repository.WebURL,
		&repository.BaseDirectory,
		&repository.BranchFilter,
		&repository.ExternalId,
		&repository.WebhookId,
	); err != nil {
		return nil, FormatError(err)
	}

	return &repository, nil
}

func findRepositoryList(ctx context.Context, tx *Tx, find *api.RepositoryFind) (_ []*api.Repository, err error) {
	// Build WHERE clause.
	where, args := []string{"1 = 1"}, []interface{}{}
	if v := find.ID; v != nil {
		where, args = append(where, "id = ?"), append(args, *v)
	}

	rows, err := tx.QueryContext(ctx, `
		SELECT 
		    id,
		    creator_id,
		    created_ts,
		    updater_id,
		    updated_ts,
			vcs_id,
			project_id,
			name,
			full_path,
			web_url,
			base_directory,
			branch_filter,
			external_id,
			webhook_id
		FROM repo
		WHERE `+strings.Join(where, " AND "),
		args...,
	)
	if err != nil {
		return nil, FormatError(err)
	}
	defer rows.Close()

	// Iterate over result set and deserialize rows into list.
	list := make([]*api.Repository, 0)
	for rows.Next() {
		var repository api.Repository
		if err := rows.Scan(
			&repository.ID,
			&repository.CreatorId,
			&repository.CreatedTs,
			&repository.UpdaterId,
			&repository.UpdatedTs,
			&repository.VCSId,
			&repository.ProjectId,
			&repository.Name,
			&repository.FullPath,
			&repository.WebURL,
			&repository.BaseDirectory,
			&repository.BranchFilter,
			&repository.ExternalId,
			&repository.WebhookId,
		); err != nil {
			return nil, FormatError(err)
		}

		list = append(list, &repository)
	}
	if err := rows.Err(); err != nil {
		return nil, FormatError(err)
	}

	return list, nil
}

// patchRepository updates a repository by ID. Returns the new state of the repository after update.
func patchRepository(ctx context.Context, tx *Tx, patch *api.RepositoryPatch) (*api.Repository, error) {
	// Build UPDATE clause.
	set, args := []string{"updater_id = ?"}, []interface{}{patch.UpdaterId}
	if v := patch.BaseDirectory; v != nil {
		set, args = append(set, "base_directory = ?"), append(args, *v)
	}
	if v := patch.BranchFilter; v != nil {
		set, args = append(set, "branch_filter = ?"), append(args, *v)
	}

	args = append(args, patch.ID)

	// Execute update query with RETURNING.
	row, err := tx.QueryContext(ctx, `
		UPDATE repo
		SET `+strings.Join(set, ", ")+`
		WHERE id = ?
		RETURNING id, creator_id, created_ts, updater_id, updated_ts, vcs_id, project_id, name, full_path, web_url, base_directory, branch_filter, external_id, webhook_id
	`,
		args...,
	)
	if err != nil {
		return nil, FormatError(err)
	}
	defer row.Close()

	if row.Next() {
		var repository api.Repository
		if err := row.Scan(
			&repository.ID,
			&repository.CreatorId,
			&repository.CreatedTs,
			&repository.UpdaterId,
			&repository.UpdatedTs,
			&repository.VCSId,
			&repository.ProjectId,
			&repository.Name,
			&repository.FullPath,
			&repository.WebURL,
			&repository.BaseDirectory,
			&repository.BranchFilter,
			&repository.ExternalId,
			&repository.WebhookId,
		); err != nil {
			return nil, FormatError(err)
		}

		return &repository, nil
	}

	return nil, &bytebase.Error{Code: bytebase.ENOTFOUND, Message: fmt.Sprintf("repository ID not found: %d", patch.ID)}
}

// deleteRepository permanently deletes a repository by ID.
func deleteRepository(ctx context.Context, tx *Tx, delete *api.RepositoryDelete) error {
	// Remove row from database.
	result, err := tx.ExecContext(ctx, `DELETE FROM repo WHERE id = ?`, delete.ID)
	if err != nil {
		return FormatError(err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return &bytebase.Error{Code: bytebase.ENOTFOUND, Message: fmt.Sprintf("repository ID not found: %d", delete.ID)}
	}

	return nil
}