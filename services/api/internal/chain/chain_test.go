package chain

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

// TestKeccak256VectoresConocidos usa vectores publicados, no valores que salgan
// de esta misma implementación: un test que compara el código contra sí mismo
// no prueba nada.
func TestKeccak256VectoresConocidos(t *testing.T) {
	casos := []struct {
		nombre   string
		entrada  string
		esperado string
	}{
		{
			nombre:   "la cadena vacía",
			entrada:  "",
			esperado: "c5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470",
		},
		{
			nombre:   "abc",
			entrada:  "abc",
			esperado: "4e03657aea45a94fc7d47ba826c8d667c0d1e6e33a64a036ec44f58fa12d6c45",
		},
		{
			// El tipo del voucher: si este valor cambia, cambió el tipo, y todas
			// las firmas viejas dejaron de servir.
			nombre:   "el tipo del voucher",
			entrada:  tipoDistincion,
			esperado: "71b24eb9344ae018a42000d2cc1c9c69c9f9a48950c442d8a83e61d310660823",
		},
		{
			nombre:   "el tipo del dominio",
			entrada:  tipoDominio,
			esperado: "8b73c3c69bb8fe3d512ecc4cf759cc79239f7b179b0ffacaa9a75d522b39400f",
		},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			got := Keccak256([]byte(c.entrada))
			if hex.EncodeToString(got[:]) != c.esperado {
				t.Fatalf("keccak256(%q) = %x, esperaba %s", c.entrada, got, c.esperado)
			}
		})
	}
}

// TestKeccakNoEsSHA3 deja escrita la trampa clásica.
//
// Los dos algoritmos son la misma permutación con distinto byte de relleno.
// Usar `sha3.New256` en lugar de `sha3.NewLegacyKeccak256` compila, corre, no
// da ningún error y devuelve el hash equivocado; el único síntoma es que el
// contrato recupera una dirección cualquiera. Este test existe para que, si
// alguien cambia el hash "por el que parece más moderno", se entere acá.
func TestKeccakNoEsSHA3(t *testing.T) {
	keccak := Keccak256(nil)
	sha3estandar := sha256.Sum256(nil) // no es SHA3, pero alcanza para el punto

	if keccak == sha3estandar {
		t.Fatal("keccak-256 y SHA-256 dieron lo mismo: algo está muy mal")
	}

	// El valor de SHA3-256("") es a7ffc6f8... y el de keccak-256(""), c5d24601...
	if hex.EncodeToString(keccak[:]) ==
		"a7ffc6f8bf1ed76651c14756a061d662f580ff4de43b49fa82d80a4b80f8434a" {
		t.Fatal("esto es SHA3-256, no keccak-256: revisá que sea NewLegacyKeccak256")
	}
}

func TestKeccak256ConcatenaLosTrozos(t *testing.T) {
	junto := Keccak256([]byte("hola mundo"))
	partido := Keccak256([]byte("hola "), []byte("mundo"))

	if junto != partido {
		t.Fatalf("hashear en partes dio distinto: %x vs %x", junto, partido)
	}
}

func TestParseDireccion(t *testing.T) {
	const canonica = "0xf39fd6e51aad88f6f4ce6ab8827279cfffb92266"

	casos := []struct {
		nombre  string
		entrada string
		falla   bool
	}{
		{nombre: "con 0x", entrada: canonica},
		{nombre: "sin 0x", entrada: canonica[2:]},
		{nombre: "en mayúsculas", entrada: "0xF39FD6E51AAD88F6F4CE6AB8827279CFFFB92266"},
		{nombre: "vacía", entrada: "", falla: true},
		{nombre: "corta", entrada: "0xf39fd6e5", falla: true},
		{nombre: "con basura", entrada: "0xzzzzd6e51aad88f6f4ce6ab8827279cfffb92266", falla: true},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			d, err := ParseDireccion(c.entrada)
			if c.falla {
				if err == nil {
					t.Fatalf("esperaba error para %q y devolvió %s", c.entrada, d)
				}
				return
			}
			if err != nil {
				t.Fatalf("no esperaba error para %q: %v", c.entrada, err)
			}
			// Hex siempre en minúsculas, entre otras cosas, porque la base tiene
			// un UNIQUE sobre la dirección.
			if d.Hex() != canonica {
				t.Fatalf("Hex() = %s, esperaba %s", d.Hex(), canonica)
			}
		})
	}
}

// TestPalabrasAlineanADerecha prueba lo único que hace falta saber de la ABI
// para este paquete: todo ocupa 32 bytes y los números van a la derecha.
func TestPalabrasAlineanADerecha(t *testing.T) {
	p := palabraDeNumero(17)
	esperado := "0000000000000000000000000000000000000000000000000000000000000011"
	if hex.EncodeToString(p[:]) != esperado {
		t.Fatalf("palabraDeNumero(17) = %x", p)
	}

	d, err := ParseDireccion("0x70997970c51812dc3a010c7d01b50e0d17dc79c8")
	if err != nil {
		t.Fatal(err)
	}
	pd := palabraDeDireccion(d)
	esperado = "00000000000000000000000070997970c51812dc3a010c7d01b50e0d17dc79c8"
	if hex.EncodeToString(pd[:]) != esperado {
		t.Fatalf("palabraDeDireccion = %x", pd)
	}

	// Un uint16 y un uint64 con el mismo valor tienen que dar la misma palabra:
	// es justamente lo que hace que `pieza` y `deadline` se codifiquen igual.
	if palabraDeNumero(uint64(uint16(300))) != palabraDeNumero(300) {
		t.Fatal("un uint16 y un uint64 con el mismo valor deberían dar la misma palabra")
	}
}

// TestDireccionDeContrato usa los vectores de CREATE que están en la
// documentación de Ethereum: el mismo deployer con distintos nonces.
func TestDireccionDeContrato(t *testing.T) {
	deployer, err := ParseDireccion("0x6ac7ea33f8831ea9dcc53393aaa88b25a785dbf0")
	if err != nil {
		t.Fatal(err)
	}

	casos := []struct {
		nonce    uint64
		esperado string
	}{
		// nonce 0: en RLP se escribe 0x80, la cadena vacía, y no 0x00.
		{nonce: 0, esperado: "0xcd234a471b72ba2f1ccf0a70fcaba648a5eecd8d"},
		// nonce 1: un byte suelto menor a 0x80 va sin prefijo.
		{nonce: 1, esperado: "0x343c43a37d37dff08ae8c4a11544c718abb4fcf8"},
		// nonce 128: ya no entra suelto y necesita el prefijo de largo.
		{nonce: 128, esperado: "0x08e190dcb7b73f5fcdabb43e102215c83659a76d"},
	}

	for _, c := range casos {
		got := DireccionDeContrato(deployer, c.nonce)
		if got.Hex() != c.esperado {
			t.Fatalf("nonce %d: dio %s, esperaba %s", c.nonce, got, c.esperado)
		}
	}
}
