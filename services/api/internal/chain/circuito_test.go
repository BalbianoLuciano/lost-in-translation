package chain_test

// El ensayo general: Go firma, una billetera manda la transacción y la cadena
// acuña.
//
// Hasta acá, Go y Solidity se ponían de acuerdo a través de un fixture
// commiteado: el test cruzado prueba que el contrato **acepta** una firma
// hecha en Go, pero nadie mandó nunca una transacción. Entre "las dos partes se
// entienden" y "el circuito funciona" quedan cosas que sólo aparecen contra un
// nodo: el formato con el que la firma viaja en el calldata, el gas, el recibo,
// y que el número de pieza que firma el servidor sea el que el contrato tiene
// cargado.
//
// Corre contra anvil, que es gratis y local. Se levanta todo con
// `scripts/circuito.sh`; sin sus variables, este archivo no hace nada.
//
// Lo que el servidor hace de verdad —armar el voucher, firmarlo, leer el
// recibo— se hace con el paquete `chain`. Lo que el servidor NO hace —mandar la
// transacción, que la manda la persona con su billetera (SDD §4)— se hace con
// `cast`, que acá hace de billetera.

import (
	"context"
	"encoding/json"
	"math/big"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/chain"
)

type entorno struct {
	rpc        string
	contrato   chain.Direccion
	chainID    uint64
	firmante   string // la clave con la que firma el servidor
	reclamante string // la clave de quien manda la transacción y recibe el token
}

func leerEntorno(t *testing.T) entorno {
	t.Helper()
	rpc := os.Getenv("CIRCUITO_RPC")
	if rpc == "" {
		t.Skip("CIRCUITO_RPC no definida: se levanta con scripts/circuito.sh")
	}
	contrato, err := chain.ParseDireccion(os.Getenv("CIRCUITO_CONTRATO"))
	if err != nil {
		t.Fatalf("CIRCUITO_CONTRATO: %v", err)
	}
	id, err := strconv.ParseUint(strings.TrimSpace(os.Getenv("CIRCUITO_CHAIN_ID")), 10, 64)
	if err != nil {
		t.Fatalf("CIRCUITO_CHAIN_ID: %v", err)
	}
	return entorno{
		rpc:        rpc,
		contrato:   contrato,
		chainID:    id,
		firmante:   os.Getenv("CIRCUITO_CLAVE_FIRMANTE"),
		reclamante: os.Getenv("CIRCUITO_CLAVE_RECLAMANTE"),
	}
}

// cast corre un comando de Foundry y devuelve su salida.
func castear(t *testing.T, args ...string) string {
	t.Helper()
	cmd := exec.Command("cast", args...)
	salida, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("cast %s falló: %v\n%s", strings.Join(args, " "), err, salida)
	}
	return strings.TrimSpace(string(salida))
}

// castearEsperandoFalla es lo mismo, pero para cuando la transacción tiene que
// revertir. Devuelve la salida para poder mirar el motivo.
func castearEsperandoFalla(t *testing.T, args ...string) string {
	t.Helper()
	salida, err := exec.Command("cast", args...).CombinedOutput()
	if err == nil {
		t.Fatalf("cast %s tenía que fallar y no falló:\n%s", strings.Join(args, " "), salida)
	}
	return string(salida)
}

// tokenIDEsperado es la misma cuenta que hace el contrato: la dirección corrida
// dieciséis bits y la pieza abajo.
func tokenIDEsperado(titular chain.Direccion, pieza uint16) *big.Int {
	id := new(big.Int).SetBytes(titular[:])
	id.Lsh(id, 16)
	return id.Or(id, big.NewInt(int64(pieza)))
}

func TestElCircuitoCompletoContraUnNodo(t *testing.T) {
	e := leerEntorno(t)
	ctx := context.Background()

	clave, err := chain.ClaveDesdeHex(e.firmante)
	if err != nil {
		t.Fatal(err)
	}
	reclamanteDir := strings.TrimSpace(castear(t, "wallet", "address", "--private-key", e.reclamante))
	titular, err := chain.ParseDireccion(reclamanteDir)
	if err != nil {
		t.Fatal(err)
	}

	// Antes que nada: que el firmante que el contrato espera sea el nuestro. Si
	// no coinciden, todo lo demás falla con "firma inválida" y la causa real
	// queda escondida detrás del síntoma.
	enElContrato := castear(t, "call", e.contrato.Hex(), "firmante()(address)", "--rpc-url", e.rpc)
	nuestro := chain.DireccionDeClave(clave.PubKey())
	if !strings.EqualFold(strings.TrimSpace(enElContrato), nuestro.Hex()) {
		t.Fatalf("el contrato espera que firme %s y la clave configurada es %s",
			enElContrato, nuestro.Hex())
	}

	const pieza = uint16(7)
	deadline := uint64(time.Now().Add(10 * time.Minute).Unix())

	// Esto es exactamente lo que hace el servidor al pedir un voucher.
	voucher := chain.Voucher{To: titular, Pieza: pieza, Deadline: deadline}
	digest := chain.Digest(chain.DominioDistinciones(e.chainID, e.contrato), voucher)
	firma, err := chain.Firmar(clave, digest)
	if err != nil {
		t.Fatal(err)
	}

	// Y esto es lo que hace la persona con su billetera. El servidor nunca manda
	// transacciones: no tiene fondos y no los necesita.
	salida := castear(t, "send", e.contrato.Hex(),
		"mint(address,uint16,uint64,bytes)",
		titular.Hex(), itoa(uint64(pieza)), itoa(deadline), firma.Hex(),
		"--private-key", e.reclamante, "--rpc-url", e.rpc, "--json")

	var recibo struct {
		Hash   string `json:"transactionHash"`
		Estado string `json:"status"`
	}
	if err := json.Unmarshal([]byte(salida), &recibo); err != nil {
		t.Fatalf("no se pudo leer el recibo de cast: %v\n%s", err, salida)
	}

	// Y acá vuelve el servidor: leer el recibo es lo único que hace contra la
	// red, y lo hace con este cliente.
	r, err := chain.NuevoRPC(e.rpc).Recibo(ctx, recibo.Hash)
	if err != nil {
		t.Fatalf("el servidor no pudo leer el recibo de su propia transacción: %v", err)
	}
	if !r.Exitoso {
		t.Fatalf("la transacción se minó pero falló: %+v", r)
	}

	// El token existe, es de quien lo reclamó, y su id es el que el servidor
	// puede calcular sin preguntarle a nadie.
	quiero := tokenIDEsperado(titular, pieza)
	duenio := castear(t, "call", e.contrato.Hex(), "ownerOf(uint256)(address)",
		quiero.String(), "--rpc-url", e.rpc)
	if !strings.EqualFold(strings.TrimSpace(duenio), titular.Hex()) {
		t.Fatalf("el token %s es de %s y tenía que ser de %s", quiero, duenio, titular.Hex())
	}

	// Y está trabado, que es toda la gracia.
	if trabado := castear(t, "call", e.contrato.Hex(), "locked(uint256)(bool)",
		quiero.String(), "--rpc-url", e.rpc); !strings.Contains(trabado, "true") {
		t.Fatalf("locked() = %s, want true", trabado)
	}

	// El dibujo lo hace el contrato: alcanza con ver que devuelva algo con forma
	// de metadatos. Cómo se ve, lo prueban los tests de Solidity.
	uri := castear(t, "call", e.contrato.Hex(), "tokenURI(uint256)(string)",
		quiero.String(), "--rpc-url", e.rpc)
	if !strings.Contains(uri, "data:application/json;base64,") {
		t.Fatalf("tokenURI no devolvió metadatos embebidos: %.120s", uri)
	}
}

// La otra mitad: que la cadena rechace lo que tiene que rechazar. Un circuito
// que sólo se probó con el caso feliz no se probó.
func TestLaCadenaRechazaUnVoucherQueNoFirmoElServidor(t *testing.T) {
	e := leerEntorno(t)

	// Una clave cualquiera que no es la del servidor.
	intruso, err := chain.ClaveDesdeHex(
		"0x59c6995e998f97a5a0044966f0945389dc9e86dae88c7a8412f4603b6b78690d")
	if err != nil {
		t.Fatal(err)
	}
	reclamanteDir := strings.TrimSpace(castear(t, "wallet", "address", "--private-key", e.reclamante))
	titular, err := chain.ParseDireccion(reclamanteDir)
	if err != nil {
		t.Fatal(err)
	}

	const pieza = uint16(11)
	deadline := uint64(time.Now().Add(10 * time.Minute).Unix())
	digest := chain.Digest(chain.DominioDistinciones(e.chainID, e.contrato),
		chain.Voucher{To: titular, Pieza: pieza, Deadline: deadline})
	firma, err := chain.Firmar(intruso, digest)
	if err != nil {
		t.Fatal(err)
	}

	salida := castearEsperandoFalla(t, "send", e.contrato.Hex(),
		"mint(address,uint16,uint64,bytes)",
		titular.Hex(), itoa(uint64(pieza)), itoa(deadline), firma.Hex(),
		"--private-key", e.reclamante, "--rpc-url", e.rpc)

	if !strings.Contains(salida, "FirmaInvalida") {
		t.Fatalf("revirtió, pero no por firma inválida:\n%s", salida)
	}
}

func itoa(n uint64) string { return strconv.FormatUint(n, 10) }
