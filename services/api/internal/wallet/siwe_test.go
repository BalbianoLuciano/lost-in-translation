package wallet

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/chain"
)

var (
	emitido   = time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	direccion = mustDireccion("0x70997970c51812dc3a010c7d01b50e0d17dc79c8")
)

func mustDireccion(s string) chain.Direccion {
	d, err := chain.ParseDireccion(s)
	if err != nil {
		panic(err)
	}
	return d
}

func mensajeDePrueba() Mensaje {
	return Mensaje{
		Dominio:    "localhost:5173",
		Direccion:  direccion,
		Statement:  statement,
		URI:        "http://localhost:5173",
		Version:    "1",
		ChainID:    84532,
		Nonce:      "c0ffee00c0ffee00",
		IssuedAt:   emitido,
		Expiration: emitido.Add(VidaDelDesafio),
	}
}

// Armar y volver a parsear tiene que dar lo mismo. Es lo que garantiza que el
// servidor pueda leer exactamente lo que emitió, que es lo único que acepta.
func TestArmarYParsearDanLaVuelta(t *testing.T) {
	quiero := mensajeDePrueba()
	texto := ArmarMensaje(quiero)

	got, err := ParseMensaje(texto)
	if err != nil {
		t.Fatalf("no pudo parsear lo que él mismo escribió:\n%s\n%v", texto, err)
	}
	if got != quiero {
		t.Fatalf("dio la vuelta distinto:\n%+v\n%+v", got, quiero)
	}
	// Y el texto es EIP-4361 de verdad, no algo parecido.
	if !strings.HasPrefix(texto, "localhost:5173 wants you to sign in with your Ethereum account:\n") {
		t.Fatalf("la primera línea no es la del estándar:\n%s", texto)
	}
	if !strings.Contains(texto, "\nNonce: c0ffee00c0ffee00\n") {
		t.Fatalf("falta el nonce:\n%s", texto)
	}
}

// El aviso del §8 se lee **antes** de firmar, así que tiene que estar adentro
// del texto y no en una nota al pie de la pantalla.
func TestElStatementAvisaQueLaCadenaNoSeBorra(t *testing.T) {
	texto := ArmarMensaje(mensajeDePrueba())
	for _, frase := range []string{"cannot be deleted", "no email", "authorizes no transaction"} {
		if !strings.Contains(texto, frase) {
			t.Fatalf("el statement no dice %q:\n%s", frase, texto)
		}
	}
}

func TestParseMensajeRechazaLoQueNoEmitio(t *testing.T) {
	bueno := ArmarMensaje(mensajeDePrueba())

	tests := []struct {
		name  string
		texto string
	}{
		{"vacío", ""},
		{"sólo la primera línea", strings.SplitN(bueno, "\n", 2)[0]},
		{"sin el encabezado", strings.Replace(bueno, " wants you to sign in with your Ethereum account:", ":", 1)},
		{"sin dominio", strings.TrimPrefix(bueno, "localhost:5173")},
		{"la dirección no es una dirección", strings.Replace(bueno, direccion.Hex(), "0xnope", 1)},
		{"falta el Nonce", strings.Replace(bueno, "Nonce: c0ffee00c0ffee00", "Nonce:", 1)},
		{"campos en otro orden", strings.Replace(bueno, "URI: http://localhost:5173\nVersion: 1",
			"Version: 1\nURI: http://localhost:5173", 1)},
		{"Chain ID no es un número", strings.Replace(bueno, "Chain ID: 84532", "Chain ID: base", 1)},
		{"la fecha no es RFC 3339", strings.Replace(bueno, "Issued At: 2026-09-24T12:00:00Z", "Issued At: ayer", 1)},
		{"una línea de más", bueno + "\nAlgo: otra cosa"},
		{
			// \r\n no se normaliza a propósito: lo que se firmó son los bytes
			// que llegaron. "Arreglarlo" acá recuperaría otra dirección.
			name:  "con saltos de Windows",
			texto: strings.ReplaceAll(bueno, "\n", "\r\n"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := ParseMensaje(tt.texto); !errors.Is(err, ErrMensajeInvalido) {
				t.Fatalf("err = %v, want ErrMensajeInvalido", err)
			}
		})
	}
}

// La mitad no criptográfica de la verificación, que es la que existe justamente
// porque una firma personal no tiene dominio.
func TestCoincide(t *testing.T) {
	const (
		dominio = "localhost:5173"
		uri     = "http://localhost:5173"
		cadena  = uint64(84532)
	)
	ahora := emitido.Add(time.Minute)

	tests := []struct {
		name   string
		toca   func(*Mensaje)
		cuando time.Time
		want   error
	}{
		{"el que emitió este servidor", func(*Mensaje) {}, ahora, nil},
		{
			// El ataque que SIWE existe para frenar: la firma es impecable y el
			// texto lo escribió otro sitio.
			name:   "otro dominio",
			toca:   func(m *Mensaje) { m.Dominio = "distinciones-gratis.example" },
			cuando: ahora, want: ErrDominioAjeno,
		},
		{"otra URI", func(m *Mensaje) { m.URI = "https://otro.example" }, ahora, ErrDominioAjeno},
		{"otra cadena", func(m *Mensaje) { m.ChainID = 1 }, ahora, ErrCadenaAjena},
		{"otra versión de SIWE", func(m *Mensaje) { m.Version = "2" }, ahora, ErrMensajeInvalido},
		{
			// Si el statement cambió, no firmó lo que creemos que firmó: el
			// aviso de que la cadena no se borra puede no haber estado.
			name:   "otro statement",
			toca:   func(m *Mensaje) { m.Statement = "Sign here, trust me" },
			cuando: ahora, want: ErrMensajeInvalido,
		},
		{"vencido", func(*Mensaje) {}, emitido.Add(VidaDelDesafio), ErrMensajeVencido},
		{"vencido hace rato", func(*Mensaje) {}, emitido.Add(24 * time.Hour), ErrMensajeVencido},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := mensajeDePrueba()
			tt.toca(&m)
			err := m.Coincide(dominio, uri, cadena, tt.cuando)
			if tt.want == nil && err != nil {
				t.Fatalf("err = %v, want nil", err)
			}
			if tt.want != nil && !errors.Is(err, tt.want) {
				t.Fatalf("err = %v, want %v", err, tt.want)
			}
		})
	}
}
