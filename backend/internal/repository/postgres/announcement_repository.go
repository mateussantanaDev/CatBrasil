package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"catbrasil/backend/internal/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AnnouncementRepository struct {
	pool *pgxpool.Pool
}

func NewAnnouncementRepository(pool *pgxpool.Pool) *AnnouncementRepository {
	return &AnnouncementRepository{pool: pool}
}

const announcementSelectFields = `
	id, title, category, content, image_url, cta_link, cta_text,
	show_in_welcome_modal, badge_color, active, views_count, created_at, updated_at
`

func scanAnnouncement(row pgx.Row) (*domain.Announcement, error) {
	var ann domain.Announcement
	err := row.Scan(
		&ann.ID, &ann.Title, &ann.Category, &ann.Content, &ann.ImageURL, &ann.CTALink, &ann.CTAText,
		&ann.ShowInWelcomeModal, &ann.BadgeColor, &ann.Active, &ann.ViewsCount, &ann.CreatedAt, &ann.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrAnnouncementNotFound
		}
		return nil, fmt.Errorf("erro ao mapear comunicado: %w", err)
	}
	return &ann, nil
}

func (r *AnnouncementRepository) GetAll(ctx context.Context) ([]*domain.Announcement, error) {
	query := fmt.Sprintf("SELECT %s FROM announcements ORDER BY created_at DESC", announcementSelectFields)
	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("falha ao listar todos os comunicados: %w", err)
	}
	defer rows.Close()

	var list []*domain.Announcement
	for rows.Next() {
		ann, err := scanAnnouncement(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, ann)
	}

	return list, nil
}

func (r *AnnouncementRepository) GetActive(ctx context.Context) ([]*domain.Announcement, error) {
	query := fmt.Sprintf("SELECT %s FROM announcements WHERE active = TRUE ORDER BY created_at DESC", announcementSelectFields)
	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("falha ao listar comunicados ativos: %w", err)
	}
	defer rows.Close()

	var list []*domain.Announcement
	for rows.Next() {
		ann, err := scanAnnouncement(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, ann)
	}

	return list, nil
}

func (r *AnnouncementRepository) GetByID(ctx context.Context, id string) (*domain.Announcement, error) {
	query := fmt.Sprintf("SELECT %s FROM announcements WHERE id = $1", announcementSelectFields)
	row := r.pool.QueryRow(ctx, query, id)
	return scanAnnouncement(row)
}

func (r *AnnouncementRepository) Create(ctx context.Context, ann *domain.Announcement) error {
	query := `
		INSERT INTO announcements (
			id, title, category, content, image_url, cta_link, cta_text,
			show_in_welcome_modal, badge_color, active, views_count, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7,
			$8, $9, $10, $11, $12, $13
		)
	`

	if ann.CreatedAt.IsZero() {
		ann.CreatedAt = time.Now()
	}
	ann.UpdatedAt = time.Now()

	_, err := r.pool.Exec(ctx, query,
		ann.ID, ann.Title, ann.Category, ann.Content, ann.ImageURL, ann.CTALink, ann.CTAText,
		ann.ShowInWelcomeModal, ann.BadgeColor, ann.Active, ann.ViewsCount, ann.CreatedAt, ann.UpdatedAt,
	)
	return err
}

func (r *AnnouncementRepository) Update(ctx context.Context, ann *domain.Announcement) error {
	query := `
		UPDATE announcements SET
			title = $1, category = $2, content = $3, image_url = $4,
			cta_link = $5, cta_text = $6, show_in_welcome_modal = $7,
			badge_color = $8, active = $9, updated_at = $10
		WHERE id = $11
	`

	ann.UpdatedAt = time.Now()

	cmdTag, err := r.pool.Exec(ctx, query,
		ann.Title, ann.Category, ann.Content, ann.ImageURL,
		ann.CTALink, ann.CTAText, ann.ShowInWelcomeModal,
		ann.BadgeColor, ann.Active, ann.UpdatedAt, ann.ID,
	)
	if err != nil {
		return err
	}
	if cmdTag.RowsAffected() == 0 {
		return domain.ErrAnnouncementNotFound
	}
	return nil
}

func (r *AnnouncementRepository) ToggleActive(ctx context.Context, id string) (*domain.Announcement, error) {
	query := fmt.Sprintf(`
		UPDATE announcements SET
			active = NOT active,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = $1
		RETURNING %s
	`, announcementSelectFields)

	row := r.pool.QueryRow(ctx, query, id)
	return scanAnnouncement(row)
}

func (r *AnnouncementRepository) Delete(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, "DELETE FROM announcements WHERE id = $1", id)
	return err
}

func (r *AnnouncementRepository) IncrementViews(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, "UPDATE announcements SET views_count = views_count + 1 WHERE id = $1", id)
	return err
}
