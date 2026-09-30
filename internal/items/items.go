// Package items: elementos del listado del dashboard (modelo, validación, repositorio y handlers).
package items

import (
	"context"
	"errors"
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

// Changes son los campos editables de un item (el dueño y la fecha no cambian).
type Changes struct {
	Title       string
	Description string
}

// ErrNotFound: el item no existe. ErrForbidden: existe pero es de otro usuario (solo el dueño
// puede editarlo o borrarlo).
var (
	ErrNotFound  = errors.New("items: no existe")
	ErrForbidden = errors.New("items: de otro usuario")
)

// Limits salen de la config (ITEM_TITLE_MAX, ITEM_DESCRIPTION_MAX, ITEMS_PAGE_LIMIT).
type Limits struct {
	TitleMax       int
	DescriptionMax int
	PageLimit      int
}

type Repository interface {
	List(ctx context.Context, limit int) ([]Item, error)
	Create(ctx context.Context, in NewItem) (Item, error)
	Update(ctx context.Context, id int64, owner string, c Changes) (Item, error)
	Delete(ctx context.Context, id int64, owner string) error
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
