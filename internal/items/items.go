// Package items: elementos del listado del dashboard (modelo, validación, repositorio y handlers).
package items

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

type Item struct {
	ID          int64     `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	CreatedBy   string    `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
}

type NewItem struct {
	Title       string
	Description string
	CreatedBy   string
}

// Limits salen de la config (ITEM_TITLE_MAX, ITEM_DESCRIPTION_MAX, ITEMS_PAGE_LIMIT).
type Limits struct {
	TitleMax       int
	DescriptionMax int
	PageLimit      int
}

type Repository interface {
	List(ctx context.Context, limit int) ([]Item, error)
	Create(ctx context.Context, in NewItem) (Item, error)
}

// Validate devuelve un mensaje por campo inválido, o nil. Los límites cuentan caracteres, no bytes.
func Validate(title, description string, l Limits) map[string]string {
	errs := map[string]string{}
	switch t := strings.TrimSpace(title); {
	case t == "":
		errs["title"] = "es obligatorio"
	case utf8.RuneCountInString(t) > l.TitleMax:
		errs["title"] = fmt.Sprintf("máximo %d caracteres", l.TitleMax)
	}
	if utf8.RuneCountInString(strings.TrimSpace(description)) > l.DescriptionMax {
		errs["description"] = fmt.Sprintf("máximo %d caracteres", l.DescriptionMax)
	}
	if len(errs) == 0 {
		return nil
	}
	return errs
}
