package wallet

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/chain"
)

// EIP-4361, "Sign-In with Ethereum", que es un formato de **texto** y no de
// firma.
//
// La firma de un mensaje personal (chain.HashPersonal) no tiene dominio: nada
// en el hash dice para qué sitio vale. Si el servidor pidiera firmar "hola", un
// atacante podría hacer que la misma persona firme "hola" en otro sitio y traer
// esa firma acá. SIWE cierra ese agujero metiendo en el propio texto lo que a
// la firma le falta: quién lo pide (`domain`), qué dirección dice ser
// (`address`), un nonce que el servidor emitió, y hasta cuándo vale.
//
// Por eso verificar SIWE son dos preguntas separadas y las dos hacen falta:
//
//  1. ¿la firma cierra contra la dirección que el texto declara? (ecrecover)
//  2. ¿el texto dice lo que tenía que decir? (dominio, nonce, vencimiento)
//
// Contestar sólo la primera es el error clásico: la firma es impecable y el
// mensaje lo escribió otro.
//
// Acá se implementa el subconjunto del ABNF de EIP-4361 que este servidor
// **emite**. Es a propósito: el parser no tiene que aceptar cualquier mensaje
// SIWE del mundo, tiene que aceptar exactamente los que emitió este servidor y
// rechazar todo lo demás. Un parser tolerante, en un lugar donde lo que se
// parsea es la prueba de identidad, es una superficie de ataque y no una
// comodidad.

const (
	// statement es lo último que la persona lee antes de firmar. Dice qué va a
	// la cadena, porque el aviso va **antes** de firmar y no después
	// (docs/sdd-distinciones.md §8): on-chain viajan una dirección y un número
	// de pieza, nada más, y eso queda para siempre.
	statement = "Link this wallet to your Lost in Translation account. " +
		"Only your address and a piece number ever reach the blockchain — no email, no name, no progress. " +
		"What is minted cannot be deleted, not even if you delete your account. " +
		"This signature costs nothing and authorizes no transaction."

	// VidaDelDesafio: lo que dura un nonce. Corto porque lo único que hay que
	// hacer con él es apretar "firmar" en la billetera.
	VidaDelDesafio = 10 * time.Minute

	largoNonce = 16 // caracteres hexadecimales: 64 bits de azar, de sobra
)

var (
	// ErrMensajeInvalido: el texto no tiene la forma que este servidor emite.
	ErrMensajeInvalido = errors.New("el mensaje no tiene el formato SIWE que emitió este servidor")
	// ErrDominioAjeno: el mensaje es de otro sitio. Es el ataque que SIWE existe
	// para frenar, así que tiene su propio error.
	ErrDominioAjeno = errors.New("el mensaje SIWE fue emitido para otro dominio")
	// ErrCadenaAjena: el mensaje declara otra cadena.
	ErrCadenaAjena = errors.New("el mensaje SIWE fue emitido para otra cadena")
	// ErrMensajeVencido: el propio texto dice que ya no vale.
	ErrMensajeVencido = errors.New("el mensaje SIWE venció")
)

// Mensaje es un mensaje SIWE, ya partido en sus campos.
type Mensaje struct {
	Dominio    string
	Direccion  chain.Direccion
	Statement  string
	URI        string
	Version    string
	ChainID    uint64
	Nonce      string
	IssuedAt   time.Time
	Expiration time.Time
}

// ArmarMensaje escribe el texto EIP-4361 que la persona va a firmar.
//
// La dirección va **en minúsculas** y no en EIP-55 a propósito. El estándar
// pide el checksum, pero acá el servidor todavía no sabe qué dirección va a
// firmar —el desafío se emite antes de que la billetera diga quién es—, así que
// el campo lo completa el cliente con la dirección que la billetera le dé. El
// servidor sólo lo lee.
func ArmarMensaje(m Mensaje) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s wants you to sign in with your Ethereum account:\n", m.Dominio)
	fmt.Fprintf(&b, "%s\n\n", m.Direccion.Hex())
	fmt.Fprintf(&b, "%s\n\n", m.Statement)
	fmt.Fprintf(&b, "URI: %s\n", m.URI)
	fmt.Fprintf(&b, "Version: %s\n", m.Version)
	fmt.Fprintf(&b, "Chain ID: %d\n", m.ChainID)
	fmt.Fprintf(&b, "Nonce: %s\n", m.Nonce)
	fmt.Fprintf(&b, "Issued At: %s\n", m.IssuedAt.UTC().Format(time.RFC3339))
	fmt.Fprintf(&b, "Expiration Time: %s", m.Expiration.UTC().Format(time.RFC3339))
	return b.String()
}

// ParseMensaje lee el texto que devolvió la billetera.
//
// Es estricto con el orden y con los saltos de línea: el mensaje que se parsea
// tiene que poder reescribirse igual byte a byte, porque es lo que se hasheó.
func ParseMensaje(texto string) (Mensaje, error) {
	var m Mensaje

	// \r\n aparece cuando el texto pasó por un campo de formulario o por
	// Windows. Normalizarlo acá sería tentador y está mal: lo que se firmó son
	// los bytes que llegaron, y si difieren, la firma no cierra igual. Mejor
	// rechazarlo con un mensaje claro que recuperar la dirección equivocada.
	// Once líneas exactas: encabezado, dirección, blanco, statement, blanco y
	// los seis campos con nombre. Ni una más ni una menos.
	const lineasEsperadas = 11
	lineas := strings.Split(texto, "\n")
	if len(lineas) != lineasEsperadas {
		return m, fmt.Errorf("%w: tiene %d líneas y espera %d", ErrMensajeInvalido, len(lineas), lineasEsperadas)
	}

	encabezado, ok := strings.CutSuffix(lineas[0], " wants you to sign in with your Ethereum account:")
	if !ok || encabezado == "" {
		return m, fmt.Errorf("%w: la primera línea no declara un dominio", ErrMensajeInvalido)
	}
	m.Dominio = encabezado

	dir, err := chain.ParseDireccion(lineas[1])
	if err != nil {
		return m, fmt.Errorf("%w: la segunda línea no es una dirección", ErrMensajeInvalido)
	}
	m.Direccion = dir

	if lineas[2] != "" || lineas[4] != "" {
		return m, fmt.Errorf("%w: faltan las líneas en blanco alrededor del statement", ErrMensajeInvalido)
	}
	m.Statement = lineas[3]

	etiquetas := []string{"URI: ", "Version: ", "Chain ID: ", "Nonce: ", "Issued At: ", "Expiration Time: "}
	valores := make([]string, len(etiquetas))
	for i, etiqueta := range etiquetas {
		v, ok := strings.CutPrefix(lineas[5+i], etiqueta)
		if !ok || v == "" {
			return m, fmt.Errorf("%w: falta el campo %q", ErrMensajeInvalido, strings.TrimSuffix(etiqueta, ": "))
		}
		valores[i] = v
	}
	m.URI, m.Version, m.Nonce = valores[0], valores[1], valores[3]

	if m.ChainID, err = strconv.ParseUint(valores[2], 10, 64); err != nil {
		return m, fmt.Errorf("%w: Chain ID no es un número", ErrMensajeInvalido)
	}
	if m.IssuedAt, err = time.Parse(time.RFC3339, valores[4]); err != nil {
		return m, fmt.Errorf("%w: Issued At no es una fecha RFC 3339", ErrMensajeInvalido)
	}
	if m.Expiration, err = time.Parse(time.RFC3339, valores[5]); err != nil {
		return m, fmt.Errorf("%w: Expiration Time no es una fecha RFC 3339", ErrMensajeInvalido)
	}

	return m, nil
}

// Coincide chequea todo lo que el texto tiene que decir, que es la mitad de la
// verificación que no es criptográfica.
//
// El nonce no se compara acá: eso lo hace la base, borrándolo, porque es la
// única forma de que "de un solo uso" sea cierto con dos pedidos simultáneos.
func (m Mensaje) Coincide(dominio, uri string, chainID uint64, ahora time.Time) error {
	if m.Dominio != dominio {
		return fmt.Errorf("%w: dice %q y este servidor es %q", ErrDominioAjeno, m.Dominio, dominio)
	}
	if m.URI != uri {
		return fmt.Errorf("%w: la URI dice %q", ErrDominioAjeno, m.URI)
	}
	if m.Version != "1" {
		return fmt.Errorf("%w: versión %q", ErrMensajeInvalido, m.Version)
	}
	if m.ChainID != chainID {
		return fmt.Errorf("%w: dice %d y la cadena de este servidor es %d", ErrCadenaAjena, m.ChainID, chainID)
	}
	if m.Statement != statement {
		// El statement es lo que la persona leyó antes de aceptar. Si cambió,
		// no firmó lo que creemos que firmó.
		return fmt.Errorf("%w: el statement no es el que emite este servidor", ErrMensajeInvalido)
	}
	if !ahora.Before(m.Expiration) {
		return fmt.Errorf("%w: venció el %s", ErrMensajeVencido, m.Expiration.UTC().Format(time.RFC3339))
	}
	return nil
}
