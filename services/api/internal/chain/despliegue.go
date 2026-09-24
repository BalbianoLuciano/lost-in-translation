package chain

// La dirección de un contrato desplegado con CREATE.
//
// Esto existe por un problema muy concreto del test cruzado, y no por gusto de
// implementar RLP. El separador de dominio incluye `verifyingContract`, así que
// para firmar hay que saber **antes** en qué dirección va a estar el contrato.
// En producción eso es fácil: se despliega y se configura la dirección. En un
// test que corre de los dos lados no hay despliegue previo que consultar, y
// hardcodear la dirección la deja atada a un detalle que cualquiera puede
// cambiar sin darse cuenta.
//
// La salida es que la dirección de CREATE no es aleatoria: sale de quién
// despliega y con qué nonce, y nada más. Con un deployer fijo y un nonce fijo,
// Go la puede calcular y el test de Foundry puede reproducirla con `vm.prank` y
// `vm.setNonce`. Los dos lados llegan al mismo número sin hablarse.

// DireccionDeContrato calcula la dirección de un contrato desplegado con CREATE
// por `deployer` con ese `nonce`.
//
//	direccion = keccak256(rlp([deployer, nonce]))[12:]
//
// Es el mismo cálculo que hace el EVM. Notar lo que implica: el mismo deployer
// con el mismo nonce siempre da la misma dirección, en cualquier cadena. Es lo
// que permite tener el mismo contrato en la misma dirección en varias redes, y
// también por qué una cuenta no puede desplegar dos veces en el mismo lugar.
func DireccionDeContrato(deployer Direccion, nonce uint64) Direccion {
	h := Keccak256(rlpListaDeDeployerYNonce(deployer, nonce))

	var d Direccion
	copy(d[:], h[32-LargoDireccion:])
	return d
}

// rlpListaDeDeployerYNonce arma el RLP de `[deployer, nonce]`.
//
// RLP es la serialización de Ethereum y es más chica de lo que su fama sugiere:
// sólo sabe de bytes y de listas de bytes. Acá hacen falta tres de sus reglas,
// y ninguna más:
//
//   - una cadena de 1 a 55 bytes se escribe 0x80+largo y después los bytes
//     (la dirección: 0x94 y los 20 bytes);
//   - un byte solo menor a 0x80 se escribe tal cual, sin prefijo;
//   - una lista cuyo contenido mide menos de 56 bytes se escribe 0xc0+largo y
//     después el contenido.
//
// La trampa es el nonce: RLP no tiene números, tiene bytes, y un número se
// escribe con la menor cantidad posible de bytes —sin ceros adelante—. El cero
// no se escribe como 0x00 sino como la cadena vacía, 0x80. Ponerle el cero da
// otra dirección.
func rlpListaDeDeployerYNonce(deployer Direccion, nonce uint64) []byte {
	contenido := make([]byte, 0, 1+LargoDireccion+1+8)

	contenido = append(contenido, 0x80+LargoDireccion)
	contenido = append(contenido, deployer[:]...)
	contenido = append(contenido, rlpNumero(nonce)...)

	// El contenido acá mide como mucho 30 bytes, así que siempre entra en la
	// forma corta de lista y no hace falta la larga.
	return append([]byte{0xc0 + byte(len(contenido))}, contenido...)
}

// rlpNumero escribe un entero como cadena RLP en su forma mínima.
func rlpNumero(n uint64) []byte {
	if n == 0 {
		return []byte{0x80} // la cadena vacía, no un cero
	}
	if n < 0x80 {
		return []byte{byte(n)} // un byte chico se escribe solo
	}

	var bytes []byte
	for ; n > 0; n >>= 8 {
		bytes = append([]byte{byte(n)}, bytes...)
	}

	return append([]byte{0x80 + byte(len(bytes))}, bytes...)
}
