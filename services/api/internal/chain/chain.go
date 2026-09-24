// Package chain tiene la poca criptografía de Ethereum que el servidor
// necesita para firmar un voucher que el contrato Distinciones acepte.
//
// Está escrito a mano, y no con `go-ethereum`, por dos razones. La práctica:
// traer el cliente entero son cientos de megas de dependencias para usar dos
// primitivas, y la imagen del servidor entra hoy en 54 MB. Escrito así, el
// paquete le suma 0,8 MB al binario cuando se enlaza. La otra razón, más
// importante: EIP-712 es corto y vale la pena entenderlo. Lo que sigue son
// cuatro operaciones encadenadas —hashear, alinear a 32 bytes, firmar,
// recuperar— y cada una tiene su trampa; esconderlas atrás de una librería es
// perder justo lo que hay que aprender.
//
// Las dos primitivas que sí se importan son las que nadie debería reescribir:
//   - keccak-256, de `golang.org/x/crypto/sha3`;
//   - ECDSA sobre secp256k1, de `github.com/decred/dcrd/dcrec/secp256k1/v4`.
//
// La primera trampa está acá mismo: keccak-256 **no** es SHA3-256. Son el mismo
// algoritmo con distinto relleno, Ethereum se congeló en la versión anterior al
// estándar, y `sha3.New256` devuelve el hash equivocado sin quejarse de nada.
// Por eso en todo el paquete se usa `sha3.NewLegacyKeccak256`, y por eso hay un
// test con un vector conocido: es un error que no avisa.
package chain

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/sha3"
)

// LargoDireccion son los 20 bytes de una dirección de Ethereum.
const LargoDireccion = 20

// Direccion es una dirección de Ethereum: los últimos 20 bytes del hash de una
// clave pública, o de la cuenta que despliega un contrato.
//
// Es un array y no un slice a propósito: así se compara con `==`, se copia sola
// y nunca puede tener un largo que no sea 20.
type Direccion [LargoDireccion]byte

// ErrDireccionInvalida es lo que devuelve ParseDireccion cuando el texto no es
// una dirección: largo equivocado, caracteres que no son hexadecimales.
var ErrDireccionInvalida = errors.New("chain: no es una dirección de 20 bytes")

// ParseDireccion lee una dirección escrita en hexadecimal, con o sin "0x".
//
// No verifica el checksum de EIP-55 —las mayúsculas y minúsculas— porque acá
// las direcciones vienen de la base o de un fixture nuestro, no de alguien
// tipeando. Si algún día entran por HTTP, el checksum se valida ahí.
func ParseDireccion(s string) (Direccion, error) {
	var d Direccion

	crudo := strings.TrimPrefix(strings.TrimPrefix(s, "0x"), "0X")
	if len(crudo) != LargoDireccion*2 {
		return d, fmt.Errorf("%w: %q", ErrDireccionInvalida, s)
	}

	b, err := hex.DecodeString(crudo)
	if err != nil {
		return d, fmt.Errorf("%w: %q", ErrDireccionInvalida, s)
	}

	copy(d[:], b)
	return d, nil
}

// Hex devuelve la dirección en minúsculas y con "0x" adelante.
//
// Siempre en minúsculas: es la forma en que la guarda la base (una dirección,
// una persona, con un UNIQUE encima), y mezclar mayúsculas rompería esa unicidad
// sin que nadie se entere.
func (d Direccion) Hex() string {
	return "0x" + hex.EncodeToString(d[:])
}

func (d Direccion) String() string { return d.Hex() }

// Keccak256 hashea la concatenación de todo lo que se le pase.
//
// Recibe varios trozos porque casi todo lo que hasheamos acá es una
// concatenación de palabras de 32 bytes, y así no hay que armar el buffer
// intermedio en cada llamada.
func Keccak256(trozos ...[]byte) [32]byte {
	h := sha3.NewLegacyKeccak256()
	for _, t := range trozos {
		// sha3.state nunca devuelve error al escribir, y el io.Writer lo
		// obliga a declararlo igual.
		_, _ = h.Write(t)
	}

	var out [32]byte
	copy(out[:], h.Sum(nil))
	return out
}

// palabra es una palabra de `abi.encode`: 32 bytes, siempre.
//
// La regla de ABI para los tipos estáticos es tan simple que se puede escribir
// en tres funciones: todo ocupa exactamente 32 bytes y los números van
// alineados a la derecha (rellenados con ceros por izquierda). Un `uint16` y un
// `uint64` ocupan lo mismo que un `uint256`; lo único que cambia es cuántos
// bytes significativos tienen. Esa uniformidad es lo que hace que el hash del
// struct sea una simple concatenación.
type palabra [32]byte

// palabraDeNumero alinea un entero sin signo a la derecha de la palabra.
func palabraDeNumero(n uint64) palabra {
	var p palabra
	for i := 0; i < 8; i++ {
		p[31-i] = byte(n >> (8 * i))
	}
	return p
}

// palabraDeDireccion alinea una dirección a la derecha: 12 ceros y los 20 bytes.
func palabraDeDireccion(d Direccion) palabra {
	var p palabra
	copy(p[32-LargoDireccion:], d[:])
	return p
}

// palabraDeHash es un bytes32, que ya viene del tamaño justo.
func palabraDeHash(h [32]byte) palabra { return palabra(h) }

// concatenar pega palabras en el orden en que llegan: eso es `abi.encode` para
// una lista de tipos estáticos, sin más.
func concatenar(palabras ...palabra) []byte {
	out := make([]byte, 0, len(palabras)*32)
	for _, p := range palabras {
		out = append(out, p[:]...)
	}
	return out
}
