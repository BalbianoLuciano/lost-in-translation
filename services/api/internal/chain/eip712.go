package chain

// EIP-712, entero, en un archivo.
//
// La idea del estándar es que lo que se firma no sea un hexadecimal opaco sino
// un dato con forma: un tipo con nombres de campo, dentro de un dominio que
// dice para qué contrato y para qué cadena vale la firma. De ahí sale la
// propiedad que acá importa: una firma hecha para este contrato en esta cadena
// **no sirve en ningún otro lado**. Ni en mainnet si se hizo para la testnet,
// ni en otro contrato, ni para otro tipo de mensaje.
//
// El digest final tiene tres capas, y se arman de adentro hacia afuera:
//
//	hashStruct       = keccak256(TIPO ‖ campos alineados a 32 bytes)
//	separadorDominio = keccak256(TIPO_DOMINIO ‖ campos del dominio)
//	digest           = keccak256(0x19 ‖ 0x01 ‖ separadorDominio ‖ hashStruct)
//
// Lo que se firma es el digest. Las tres tienen que dar exactamente lo mismo
// que calcula el contrato, byte por byte; si una sola difiere, la dirección que
// el contrato recupera es otra cualquiera y el mint revierte con FirmaInvalida
// sin decir por qué. De ahí el test cruzado: es la única forma de saber que los
// dos lados están de acuerdo.

// tipoDominio es la firma del dominio de EIP-712, tal cual: sin espacios, en el
// orden en que aparecen los campos. El texto es parte del protocolo, no un
// comentario; cambiar un espacio cambia el hash.
const tipoDominio = "EIP712Domain(string name,string version,uint256 chainId,address verifyingContract)"

// tipoDistincion es el tipo del voucher. Tiene que coincidir carácter por
// carácter con la constante TIPO_DISTINCION de Distinciones.sol.
//
// El orden de los campos es parte del tipo: mover `pieza` y `deadline` de lugar
// da otro hash y deja de servir. Por eso el test cruzado firma justamente esto.
const tipoDistincion = "Distincion(address to,uint16 pieza,uint64 deadline)"

// Dominio es el contexto en el que vale una firma.
//
// Los cuatro campos son los que el contrato declara en su constructor
// (`EIP712("Lost in Translation", "1")`) más los dos que salen de dónde está
// desplegado. VerifyingContract es el que trae la complicación práctica: hay
// que conocer la dirección del contrato **antes** de firmar, y en el test eso
// se resuelve desplegando desde una cuenta fija con un nonce fijo.
type Dominio struct {
	Nombre            string
	Version           string
	ChainID           uint64
	VerifyingContract Direccion
}

// DominioDistinciones es el dominio del contrato, con los dos campos fijos ya
// puestos: lo único que cambia entre despliegues es en qué cadena y en qué
// dirección está.
func DominioDistinciones(chainID uint64, contrato Direccion) Dominio {
	return Dominio{
		Nombre:            "Lost in Translation",
		Version:           "1",
		ChainID:           chainID,
		VerifyingContract: contrato,
	}
}

// Separador devuelve el separador de dominio.
//
// La trampa acá son las strings: `abi.encode` de un `string` no mete el texto,
// mete su hash. Por eso el nombre y la versión entran como keccak256 de sus
// bytes y no crudos. Es el error clásico de la primera implementación a mano, y
// no avisa: da un hash perfectamente válido que no coincide con el del
// contrato.
func (d Dominio) Separador() [32]byte {
	return Keccak256(concatenar(
		palabraDeHash(Keccak256([]byte(tipoDominio))),
		palabraDeHash(Keccak256([]byte(d.Nombre))),
		palabraDeHash(Keccak256([]byte(d.Version))),
		palabraDeNumero(d.ChainID),
		palabraDeDireccion(d.VerifyingContract),
	))
}

// Voucher es el permiso que el servidor firma: esta persona puede acuñar esta
// pieza, hasta esta hora.
//
// No lleva nonce ni id: el contrato calcula el id del token con
// `(to << 16) | pieza`, así que reclamar dos veces choca con un token que ya
// tiene dueño. El voucher no necesita ser de un solo uso porque lo que protege
// ya es único.
type Voucher struct {
	To       Direccion
	Pieza    uint16
	Deadline uint64
}

// HashStruct es el hash del voucher según EIP-712.
//
// Los tres campos son estáticos, así que esto es el hash del tipo seguido de
// tres palabras de 32 bytes. Notar que `pieza` es uint16 y `deadline` uint64 y
// los dos ocupan 32 bytes igual: la ABI alinea todo al mismo ancho. Escribirlos
// con su tamaño real —dos bytes, ocho bytes— es la otra forma clásica de que no
// coincida con Solidity.
func (v Voucher) HashStruct() [32]byte {
	return Keccak256(concatenar(
		palabraDeHash(Keccak256([]byte(tipoDistincion))),
		palabraDeDireccion(v.To),
		palabraDeNumero(uint64(v.Pieza)),
		palabraDeNumero(v.Deadline),
	))
}

// Digest es lo que se firma: `0x19 0x01 ‖ separadorDominio ‖ hashStruct`.
//
// El `0x19` es el prefijo de EIP-191 para "esto no es una transacción", y el
// `0x01` dice "la versión estructurada, la de EIP-712". Juntos evitan que algo
// firmado acá pueda interpretarse como una transacción RLP: ninguna transacción
// válida empieza con 0x19.
func Digest(d Dominio, v Voucher) [32]byte {
	sep := d.Separador()
	hs := v.HashStruct()

	crudo := make([]byte, 0, 2+32+32)
	crudo = append(crudo, 0x19, 0x01)
	crudo = append(crudo, sep[:]...)
	crudo = append(crudo, hs[:]...)

	return Keccak256(crudo)
}
