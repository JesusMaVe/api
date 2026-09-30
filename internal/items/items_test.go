package items

import (
	"strings"
	"testing"
)

var limits = Limits{TitleMax: 5, DescriptionMax: 8, PageLimit: 50}

func TestValidate(t *testing.T) {
	cases := []struct {
		name, title, description string
		wantFields               []string
	}{
		{"válido", "Zelda", "", nil},
		{"título vacío", "", "x", []string{"title"}},
		{"título solo espacios", "   ", "x", []string{"title"}},
		{"título largo", "Zeldas", "", []string{"title"}},
		{"el límite cuenta caracteres, no bytes", "ñáéíó", "", nil},
		{"emojis cuentan como un carácter", "🎮🎮🎮🎮🎮", "", nil},
		{"descripción larga", "Zelda", strings.Repeat("a", 9), []string{"description"}},
		{"los dos", "", strings.Repeat("a", 9), []string{"title", "description"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Validate(tc.title, tc.description, limits)
			if len(got) != len(tc.wantFields) {
				t.Fatalf("errores = %v, se esperaban en %v", got, tc.wantFields)
			}
			for _, f := range tc.wantFields {
				if got[f] == "" {
					t.Errorf("falta el error de %s: %v", f, got)
				}
			}
		})
	}
}
