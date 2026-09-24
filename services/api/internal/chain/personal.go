package chain

import "strconv"

// EIP-191, que es la otra forma de firmar de Ethereum y la que usan las
// billeteras cuando alguien aprieta "firmar mensaje".
//
// EIP-712 —el otro archivo— es para lo que firma el servidor: un struct tipado
// que el contrato verifica. Esto es para lo que firma la persona: un texto
// plano que lee con sus ojos antes de aceptar. Son primos: los dos empiezan con
// 0x19, y lo que cambia es el byte de versión. El 0x01 de EIP-712 dice "atrás
// viene un dominio y un hash de struct"; acá la versión es el carácter `\x45`
// —una "E"— que introduce el prefijo de texto de siempre.
//
// La diferencia de forma esconde una de fondo: un mensaje personal **no** tiene
// dominio. Nada en el hash dice para qué sitio vale, así que si el texto no lo
// dice, una firma pedida en un lado sirve en cualquier otro. De ahí que exista
// EIP-4361 (SIWE), que no es un formato de firma sino un formato de **texto**,
// con el dominio y un nonce adentro. Verificar SIWE es, entonces, dos cosas
// separadas: que la firma cierre (esto) y que el texto diga lo que tenía que
// decir (internal/wallet/siwe.go).

// prefijoPersonal es el texto que Ethereum antepone a todo mensaje firmado a
// mano. Existe para que un mensaje firmado en una billetera nunca pueda ser
// también una transacción válida: ninguna transacción RLP empieza con 0x19.
const prefijoPersonal = "\x19Ethereum Signed Message:\n"

// HashPersonal es el digest de `personal_sign`: el prefijo, el largo del
// mensaje escrito en decimal, y el mensaje.
//
//	keccak256("\x19Ethereum Signed Message:\n" ‖ len(mensaje) ‖ mensaje)
//
// El largo va en **bytes** y en decimal como texto, no como número binario: un
// mensaje de 137 bytes aporta los caracteres '1', '3', '7'. Es una rareza del
// formato, y contar caracteres en vez de bytes es la forma de equivocarse
// apenas alguien firma un texto con un acento.
func HashPersonal(mensaje []byte) [32]byte {
	return Keccak256(
		[]byte(prefijoPersonal),
		[]byte(strconv.Itoa(len(mensaje))),
		mensaje,
	)
}
