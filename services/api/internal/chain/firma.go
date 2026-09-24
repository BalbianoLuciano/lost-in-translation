package chain

import (
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/decred/dcrd/dcrec/secp256k1/v4/ecdsa"
)

// LargoFirma son los 65 bytes del formato de Ethereum: R ‖ S ‖ V.
const LargoFirma = 65

// Firma es una firma ECDSA en el formato que espera Solidity: 32 bytes de R,
// 32 de S y uno de V.
//
// Los 64 primeros bytes son la firma propiamente dicha; el byte 65 es el que
// sobra en todos los otros protocolos y acá hace falta. ECDSA, por sí sola, no
// permite saber quién firmó: permite verificar contra una clave pública que ya
// tenés. Ethereum no la tiene —el contrato sólo guarda una dirección, que es un
// hash— así que usa `ecrecover`, que **deduce** la clave pública desde la firma.
// Para eso hacen falta dos bits de ayuda, porque la ecuación tiene hasta cuatro
// soluciones: eso es V.
type Firma [LargoFirma]byte

var (
	// ErrFirmaInvalida: la firma no tiene la forma que Ethereum espera.
	ErrFirmaInvalida = errors.New("chain: la firma no tiene el formato R||S||V de 65 bytes")
	// ErrFirmaMaleable: la firma trae la mitad alta de S. Ver normalizar.
	ErrFirmaMaleable = errors.New("chain: la firma tiene la S alta y es maleable")
	// ErrNoSeRecupera: la firma es sintácticamente válida pero no sale de ella
	// ninguna clave pública.
	ErrNoSeRecupera = errors.New("chain: de esa firma no se recupera ninguna clave")
)

// El orden del grupo de secp256k1 y su mitad, escritos acá y no leídos de la
// librería porque son parte del razonamiento que sigue y conviene tenerlos a la
// vista. Son constantes del protocolo: no cambian nunca.
var (
	ordenSecp256k1 = must(new(big.Int).SetString(
		"fffffffffffffffffffffffffffffffebaaedce6af48a03bbfd25e8cd0364141", 16))
	mitadDelOrden = new(big.Int).Rsh(ordenSecp256k1, 1)
)

func must(n *big.Int, ok bool) *big.Int {
	if !ok {
		panic("chain: no se pudo leer una constante de secp256k1")
	}
	return n
}

// Firmar produce la firma del digest en formato de Ethereum.
//
// El nonce de la firma es determinista (RFC 6979): firmar dos veces el mismo
// digest con la misma clave da exactamente los mismos bytes. No es un detalle
// de comodidad —es lo que permite que el fixture del test cruzado sea un golden
// file— y además evita el accidente más caro de ECDSA, que es repetir un nonce
// al azar y filtrar la clave privada.
func Firmar(priv *secp256k1.PrivateKey, digest [32]byte) (Firma, error) {
	var f Firma

	// SignCompact devuelve el formato de Bitcoin: el byte de recuperación
	// **adelante**, y con un 27 ya sumado (más 4 si la clave era comprimida,
	// que acá no aplica porque Ethereum siempre usa la larga).
	compacta := ecdsa.SignCompact(priv, digest[:], false)
	if len(compacta) != LargoFirma {
		return f, fmt.Errorf("chain: SignCompact devolvió %d bytes", len(compacta))
	}

	// Ethereum quiere el mismo byte, pero al final. Es toda la diferencia entre
	// los dos formatos, y confundirse de punta es de los errores que se
	// descubren recién cuando el contrato recupera una dirección cualquiera.
	copy(f[0:64], compacta[1:65])
	f[64] = compacta[0]

	return normalizar(f), nil
}

// normalizar deja la firma con la S baja, que es la única que acepta el
// contrato.
//
// Por qué: si (r, s) es una firma válida, (r, n-s) también lo es, con el mismo
// r y verificando igual. Dos firmas distintas del mismo mensaje con la misma
// clave. Eso es la maleabilidad, y en un contrato que usara la firma como clave
// de "esto ya se usó" sería un agujero: la misma autorización entra dos veces
// con dos hashes distintos.
//
// La convención de Ethereum resuelve el empate quedándose siempre con la mitad
// baja, y el `tryRecover` de OpenZeppelin —que es el que usa Distinciones.sol—
// directamente rechaza cualquier S por encima de n/2. Así que esto no es
// prolijidad: una firma sin normalizar la mitad de las veces no sirve.
//
// Al dar vuelta la S hay que dar vuelta también la paridad de V: el punto R de
// la curva sigue siendo el mismo, pero es el otro de los dos que comparten esa
// coordenada x.
func normalizar(f Firma) Firma {
	s := new(big.Int).SetBytes(f[32:64])
	if s.Cmp(mitadDelOrden) <= 0 {
		return f
	}

	s.Sub(ordenSecp256k1, s)

	salida := f
	// FillBytes mantiene el alineado a la derecha en 32 bytes, que es
	// lo que Solidity va a leer como bytes32.
	s.FillBytes(salida[32:64])
	salida[64] = darVueltaV(f[64])

	return salida
}

// darVueltaV cambia 27 por 28 y al revés.
//
// Parece una pavada y no lo es: el atajo obvio, `v ^ 1`, está mal. Los bits de
// abajo de 27 y 28 son 011 y 100, así que el XOR con 1 da 26 y 29, dos valores
// que no existen. Hay que sacar el 27 primero, dar vuelta el bit de paridad del
// código de recuperación —que es lo único que realmente cambia— y volver a
// sumarlo.
func darVueltaV(v byte) byte {
	if v != 27 && v != 28 {
		// No es un V de Ethereum; devolverlo intacto es menos malo que inventar
		// uno, y Recuperar lo va a rechazar igual.
		return v
	}
	return 27 + ((v - 27) ^ 1)
}

// Recuperar devuelve la dirección que firmó el digest.
//
// Es el mismo `ecrecover` que hace el contrato, de este lado: sirve para
// probar, y para que el servidor pueda verificar una firma de billetera cuando
// llegue la vinculación con SIWE. Rechaza lo mismo que rechaza OpenZeppelin, y
// eso es a propósito: si acá una firma pasa, allá también.
func Recuperar(digest [32]byte, f Firma) (Direccion, error) {
	var d Direccion

	if f[64] != 27 && f[64] != 28 {
		return d, fmt.Errorf("%w: V es %d y tiene que ser 27 o 28", ErrFirmaInvalida, f[64])
	}

	s := new(big.Int).SetBytes(f[32:64])
	if s.Cmp(mitadDelOrden) > 0 {
		return d, ErrFirmaMaleable
	}

	// De vuelta al formato compacto de Bitcoin: el byte de recuperación adelante.
	compacta := make([]byte, LargoFirma)
	compacta[0] = f[64]
	copy(compacta[1:], f[0:64])

	pub, _, err := ecdsa.RecoverCompact(compacta, digest[:])
	if err != nil {
		return d, fmt.Errorf("%w: %v", ErrNoSeRecupera, err)
	}

	return DireccionDeClave(pub), nil
}

// DireccionDeClave deriva la dirección de Ethereum de una clave pública.
//
// La receta es corta y tiene una sola trampa: se hashean los 64 bytes de la
// clave sin el 0x04 del principio. Ese byte es un prefijo del formato de
// serialización SEC —dice "acá viene la forma larga, con las dos
// coordenadas"—, no es parte de la clave, y dejarlo adentro da una dirección
// perfectamente válida que no es la de nadie.
//
// Después se toman los últimos 20 bytes de los 32 del hash. Que sobren 12 es lo
// que hace que una dirección no sea reversible a una clave pública, y también
// lo que hace que `abi.encode` de una dirección venga con 12 ceros adelante.
func DireccionDeClave(pub *secp256k1.PublicKey) Direccion {
	sinPrefijo := pub.SerializeUncompressed()[1:]
	h := Keccak256(sinPrefijo)

	var d Direccion
	copy(d[:], h[32-LargoDireccion:])
	return d
}

// ClaveDesdeHex lee una clave privada escrita en hexadecimal, con o sin "0x".
func ClaveDesdeHex(s string) (*secp256k1.PrivateKey, error) {
	crudo := strings.TrimPrefix(strings.TrimPrefix(s, "0x"), "0X")

	b, err := hex.DecodeString(crudo)
	if err != nil {
		return nil, fmt.Errorf("chain: la clave privada no es hexadecimal: %w", err)
	}
	if len(b) != 32 {
		return nil, fmt.Errorf("chain: la clave privada tiene %d bytes y no 32", len(b))
	}

	// Una clave tiene que estar entre 1 y n-1. PrivKeyFromBytes no se queja de
	// un cero ni de algo por encima del orden: lo reduce en silencio, y una
	// clave en cero firmaría con la dirección de nadie.
	n := new(big.Int).SetBytes(b)
	if n.Sign() == 0 || n.Cmp(ordenSecp256k1) >= 0 {
		return nil, errors.New("chain: la clave privada está fuera del rango de secp256k1")
	}

	return secp256k1.PrivKeyFromBytes(b), nil
}

// Hex devuelve la firma en hexadecimal con "0x", que es como viaja por JSON y
// como la lee `vm.parseJsonBytes` del lado de Foundry.
func (f Firma) Hex() string { return "0x" + hex.EncodeToString(f[:]) }

// ParseFirma lee una firma de 65 bytes en hexadecimal.
func ParseFirma(s string) (Firma, error) {
	var f Firma

	crudo := strings.TrimPrefix(strings.TrimPrefix(s, "0x"), "0X")
	if len(crudo) != LargoFirma*2 {
		return f, fmt.Errorf("%w: son %d caracteres", ErrFirmaInvalida, len(crudo))
	}

	b, err := hex.DecodeString(crudo)
	if err != nil {
		return f, fmt.Errorf("%w: no es hexadecimal", ErrFirmaInvalida)
	}

	copy(f[:], b)
	return f, nil
}
