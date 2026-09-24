package httpapi

import "testing"

func TestGate(t *testing.T) {
	tests := []struct {
		name  string
		gate  *Gate
		email string
		want  bool
	}{
		{
			name:  "en la lista",
			gate:  NewGate([]string{"uno@example.com", "dos@example.com"}, false),
			email: "dos@example.com",
			want:  true,
		},
		{
			name:  "fuera de la lista",
			gate:  NewGate([]string{"uno@example.com"}, false),
			email: "cualquiera@example.com",
		},
		{
			name:  "las mayúsculas y los espacios no deciden nada",
			gate:  NewGate([]string{" Uno@Example.com "}, false),
			email: "uno@example.com",
			want:  true,
		},
		{
			name:  "con altas abiertas entra cualquiera",
			gate:  NewGate(nil, true),
			email: "cualquiera@example.com",
			want:  true,
		},
		{
			name:  "cerrada y sin lista no entra nadie nuevo",
			gate:  NewGate(nil, false),
			email: "cualquiera@example.com",
		},
		{
			name:  "sin puerta configurada no se frena a nadie",
			email: "cualquiera@example.com",
			want:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.gate.CanRegister(tt.email); got != tt.want {
				t.Fatalf("CanRegister(%q) = %v, want %v", tt.email, got, tt.want)
			}
		})
	}
}
