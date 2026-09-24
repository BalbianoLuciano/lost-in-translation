package chain

import (
	"encoding/hex"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"
)

// El test cruzado: Go firma, Solidity verifica.
//
// Este archivo produce `contracts/test/fixtures/voucher.json`, que está
// commiteado y que el test de Foundry lee con `vm.parseJson`. Los dos lados
// nunca se ejecutan juntos: lo único que comparten es ese archivo.
//
// Es el test que más vale de toda la etapa, porque es el único que puede
// agarrar un desacuerdo entre las dos implementaciones. Cada lado, por su
// cuenta, puede estar perfectamente de acuerdo consigo mismo y equivocado: si
// Go y Solidity ordenan distinto los campos del struct, o uno mete el nombre
// del dominio crudo y el otro hasheado, los dos pasan sus propios tests y el
// mint revierte en producción con un FirmaInvalida que no explica nada.
//
// Y es golden a propósito. Si alguien toca el orden de un campo, el separador
// de dominio o el formato de V, el archivo cambia, este test falla acá y no hay
// que ir a leer un stack trace de la EVM para entender qué pasó.
//
// Para regenerarlo, cuando el cambio es deliberado:
//
//	go test ./internal/chain -actualizar-fixture

var actualizarFixture = flag.Bool("actualizar-fixture", false,
	"reescribe contracts/test/fixtures/voucher.json en lugar de comparar contra él")

// Los datos del voucher de prueba. Son fijos: el fixture tiene que ser el mismo
// byte por byte en cada corrida y en cada máquina.
const (
	// FixtureClavePrivada es la cuenta 0 de anvil, la clave de prueba más
	// difundida que existe. Está acá justamente porque no es un secreto: nadie
	// puede confundirla con la clave del servidor, que vive en una variable de
	// entorno y nunca en el repo.
	FixtureClavePrivada = "0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
	// FixtureFirmante es la dirección que sale de esa clave.
	FixtureFirmante = "0xf39fd6e51aad88f6f4ce6ab8827279cfffb92266"

	// FixtureChainID es el de anvil y el que usa Foundry por defecto.
	FixtureChainID = uint64(31337)

	// FixtureDeployer y FixtureNonce son la clave de todo el asunto: con ellos
	// la dirección del contrato se puede calcular de los dos lados sin
	// desplegar nada. Ver despliegue.go y el comentario de abajo.
	FixtureDeployer = "0x1234567890abcdef1234567890abcdef12345678"
	FixtureNonce    = uint64(7)
	// FixtureVerifyingContract es DireccionDeContrato(FixtureDeployer, 7).
	FixtureVerifyingContract = "0x7fa04a5a7fd31c6215c1d980514db5e173d09e69"

	// El voucher en sí. La pieza 17 y no la 0 para que un error de alineado en
	// la palabra de 32 bytes se note; el deadline, una fecha de 2033.
	FixtureTo       = "0x70997970c51812dc3a010c7d01b50e0d17dc79c8"
	FixturePieza    = uint16(17)
	FixtureDeadline = uint64(2000000000)

	// FixtureDigest es el que tiene que dar el contrato. Está duplicado en el
	// fixture y afirmado en el test de Foundry: si los dos digests no coinciden,
	// falla ahí, que es mucho más claro que un "la firma no es del firmante".
	FixtureDigest = "0x300581ea13cd57177c335db6984ede680ce211522783cdbed49d592f54247c76"
)

// voucherFixture es lo que se escribe en el JSON.
//
// Lleva más de lo que el test de Foundry estrictamente necesita: van también el
// deployer, el nonce y la clave, para que el archivo se pueda reproducir entero
// sin leer este código. Un fixture que no se puede regenerar es un valor mágico
// commiteado.
type voucherFixture struct {
	Nota string `json:"nota"`

	// El dominio.
	Name              string `json:"name"`
	Version           string `json:"version"`
	ChainID           uint64 `json:"chainId"`
	VerifyingContract string `json:"verifyingContract"`

	// Cómo llegar a esa dirección sin haber desplegado nada.
	Deployer string `json:"deployer"`
	Nonce    uint64 `json:"nonce"`

	// Quién firma.
	ClavePrivada string `json:"clavePrivada"`
	Firmante     string `json:"firmante"`

	// El voucher.
	To       string `json:"to"`
	Pieza    uint16 `json:"pieza"`
	Deadline uint64 `json:"deadline"`

	// El resultado.
	Digest string `json:"digest"`
	Firma  string `json:"firma"`
}

const rutaFixture = "../../../../contracts/test/fixtures/voucher.json"

// TestFixtureDelVoucher regenera el fixture y lo compara con el commiteado.
func TestFixtureDelVoucher(t *testing.T) {
	generado := generarFixture(t)

	nuevo, err := json.MarshalIndent(generado, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	nuevo = append(nuevo, '\n')

	if *actualizarFixture {
		if err := os.MkdirAll(filepath.Dir(rutaFixture), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(rutaFixture, nuevo, 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("fixture reescrito en %s", rutaFixture)
		return
	}

	viejo, err := os.ReadFile(rutaFixture)
	if err != nil {
		t.Fatalf("no se pudo leer el fixture (%v).\n"+
			"Si es la primera vez, generalo con:\n"+
			"    go test ./internal/chain -actualizar-fixture", err)
	}

	if string(viejo) != string(nuevo) {
		t.Fatalf("el fixture del test cruzado cambió.\n\n"+
			"Esto no es un test flaky: algo de la firma dejó de dar lo mismo que antes.\n"+
			"Antes de regenerarlo, averiguá qué cambió —el orden de un campo del\n"+
			"struct, el separador de dominio, el formato de V— y si el cambio es\n"+
			"deliberado. Ojo: cualquier firma emitida con el formato viejo deja de\n"+
			"servir en el contrato ya desplegado.\n\n"+
			"Commiteado:\n%s\nGenerado ahora:\n%s\n\n"+
			"Si el cambio es a propósito:\n"+
			"    go test ./internal/chain -actualizar-fixture",
			viejo, nuevo)
	}
}

// generarFixture arma el fixture desde cero y, de paso, chequea las dos cosas
// que tienen que valer para que el test de Foundry pueda pasar: que la
// dirección del contrato sea la que se calcula del deployer y el nonce, y que
// la firma la haga el firmante que el contrato va a esperar.
func generarFixture(t *testing.T) voucherFixture {
	t.Helper()

	deployer, err := ParseDireccion(FixtureDeployer)
	if err != nil {
		t.Fatal(err)
	}

	// Acá está la maniobra que hace posible todo el test cruzado. El separador
	// de dominio incluye `verifyingContract`, así que Go tiene que saber en qué
	// dirección va a estar el contrato **antes** de que exista. Con CREATE eso
	// se puede: la dirección sale del deployer y del nonce. Del lado de Foundry
	// se reproduce con vm.setNonce + vm.prank, y el test afirma que coincide.
	contrato := DireccionDeContrato(deployer, FixtureNonce)
	if contrato.Hex() != FixtureVerifyingContract {
		t.Fatalf("la dirección del contrato dio %s y la constante dice %s",
			contrato, FixtureVerifyingContract)
	}

	to, err := ParseDireccion(FixtureTo)
	if err != nil {
		t.Fatal(err)
	}

	dominio := DominioDistinciones(FixtureChainID, contrato)
	voucher := Voucher{To: to, Pieza: FixturePieza, Deadline: FixtureDeadline}
	digest := Digest(dominio, voucher)

	priv, err := ClaveDesdeHex(FixtureClavePrivada)
	if err != nil {
		t.Fatal(err)
	}
	firmante := DireccionDeClave(priv.PubKey())

	firma, err := Firmar(priv, digest)
	if err != nil {
		t.Fatal(err)
	}

	// Lo mismo que va a hacer el contrato, pero de este lado: si acá no da el
	// firmante, no hace falta ir hasta Foundry para saber que está mal.
	recuperado, err := Recuperar(digest, firma)
	if err != nil {
		t.Fatal(err)
	}
	if recuperado != firmante {
		t.Fatalf("la firma recuperó %s en vez de %s", recuperado, firmante)
	}

	return voucherFixture{
		Nota: "Generado por services/api/internal/chain (TestFixtureDelVoucher). " +
			"No editar a mano: se regenera con `go test ./internal/chain -actualizar-fixture`. " +
			"La clave privada es la cuenta 0 de anvil, de prueba y sin fondos.",
		Name:              dominio.Nombre,
		Version:           dominio.Version,
		ChainID:           dominio.ChainID,
		VerifyingContract: contrato.Hex(),
		Deployer:          deployer.Hex(),
		Nonce:             FixtureNonce,
		ClavePrivada:      FixtureClavePrivada,
		Firmante:          firmante.Hex(),
		To:                to.Hex(),
		Pieza:             FixturePieza,
		Deadline:          FixtureDeadline,
		Digest:            "0x" + hex.EncodeToString(digest[:]),
		Firma:             firma.Hex(),
	}
}
