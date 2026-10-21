package domain

import (
	"context"
	"time"
)

// Announcement representa um comunicado Rich Media no mural ou modal de boas-vindas
type Announcement struct {
	ID                 string    `json:"id"`
	Title              string    `json:"title"`
	Category           string    `json:"category"`
	Content            string    `json:"content"`
	ImageURL           string    `json:"imageUrl"`
	CTALink            string    `json:"ctaLink"`
	CTAText            string    `json:"ctaText"`
	ShowInWelcomeModal bool      `json:"showInWelcomeModal"`
	BadgeColor         string    `json:"badgeColor"`
	Active             bool      `json:"active"`
	ViewsCount         int       `json:"viewsCount"`
	CreatedAt          time.Time `json:"createdAt"`
	UpdatedAt          time.Time `json:"updatedAt"`
}

// AnnouncementRepository define o contrato de persistência para Comunicados Rich Media
type AnnouncementRepository interface {
	GetAll(ctx context.Context) ([]*Announcement, error)
	GetActive(ctx context.Context) ([]*Announcement, error)
	GetByID(ctx context.Context, id string) (*Announcement, error)
	Create(ctx context.Context, ann *Announcement) error
	Update(ctx context.Context, ann *Announcement) error
	ToggleActive(ctx context.Context, id string) (*Announcement, error)
	Delete(ctx context.Context, id string) error
	IncrementViews(ctx context.Context, id string) error
}
