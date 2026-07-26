package communications

import (
	"context"
	"time"
)

type AccessContext struct {
	UserID              string
	AccountID           string
	Role                string
	StoreIDs            []string
	Permissions         []string
	PermissionsResolved bool
}

type Communication struct {
	ID               string
	AccountID        string
	Title            string
	Excerpt          string
	Body             string
	StartsAt         *time.Time
	EndsAt           *time.Time
	IsPublished      bool
	DisplayOrder     int
	TargetsAllStores bool
	StoreIDs         []string
	CreatedBy        string
	UpdatedBy        string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type CommunicationView struct {
	ID               string     `json:"id"`
	Title            string     `json:"title"`
	Excerpt          string     `json:"excerpt"`
	Body             string     `json:"body"`
	StartsAt         *time.Time `json:"startsAt,omitempty"`
	EndsAt           *time.Time `json:"endsAt,omitempty"`
	IsPublished      bool       `json:"isPublished"`
	DisplayOrder     int        `json:"displayOrder"`
	TargetsAllStores bool       `json:"targetsAllStores"`
	StoreIDs         []string   `json:"storeIds"`
	CreatedAt        time.Time  `json:"createdAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`
}

type UpsertInput struct {
	Title            string     `json:"title"`
	Excerpt          string     `json:"excerpt"`
	Body             string     `json:"body"`
	StartsAt         *time.Time `json:"startsAt"`
	EndsAt           *time.Time `json:"endsAt"`
	IsPublished      bool       `json:"isPublished"`
	DisplayOrder     int        `json:"displayOrder"`
	TargetsAllStores bool       `json:"targetsAllStores"`
	StoreIDs         []string   `json:"storeIds"`
}

type ListFilter struct {
	StoreID       string
	PublishedOnly bool
}

type ListResponse struct {
	Items []CommunicationView `json:"items"`
}

type Repository interface {
	List(ctx context.Context, accountID string, filter ListFilter) ([]Communication, error)
	Get(ctx context.Context, accountID, communicationID string) (Communication, error)
	StoresBelongToAccount(ctx context.Context, accountID string, storeIDs []string) (bool, error)
	Create(ctx context.Context, communication Communication) (Communication, error)
	Update(ctx context.Context, communication Communication) (Communication, error)
	Archive(ctx context.Context, accountID, communicationID, updatedBy string) error
}

func communicationView(item Communication) CommunicationView {
	storeIDs := append([]string{}, item.StoreIDs...)
	if storeIDs == nil {
		storeIDs = []string{}
	}
	return CommunicationView{
		ID:               item.ID,
		Title:            item.Title,
		Excerpt:          item.Excerpt,
		Body:             item.Body,
		StartsAt:         item.StartsAt,
		EndsAt:           item.EndsAt,
		IsPublished:      item.IsPublished,
		DisplayOrder:     item.DisplayOrder,
		TargetsAllStores: item.TargetsAllStores,
		StoreIDs:         storeIDs,
		CreatedAt:        item.CreatedAt,
		UpdatedAt:        item.UpdatedAt,
	}
}
