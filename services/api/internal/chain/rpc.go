package chain

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"time"
)

// Un cliente JSON-RPC del tamaño de lo que se usa, que hoy es un método:
// `eth_getTransactionReceipt`.
//
// La tentación acá es la misma que en el resto del paquete —traer el cliente de
// go-ethereum— y la respuesta también: JSON-RPC sobre HTTP es un POST con un
// objeto de cuatro campos. Lo caro de un nodo no es hablarle, es entender qué
// contesta, y eso no lo resuelve ninguna librería.
//
// El servidor nunca **manda** transacciones: la manda la persona con su
// billetera (SDD §4). Así que de todo el RPC de Ethereum hace falta sólo la
// parte de lectura, y la clave que vive en el entorno es de firma y no tiene
// fondos. Un backend sin fondos tiene una superficie de ataque mucho más chica.

// ErrSinRecibo: el nodo contestó bien y dijo que de esa transacción no sabe
// nada todavía. No es una falla; es una transacción que no se minó.
//
// Va separado de los errores de red a propósito: los dos dejan el minteo en
// `pending`, pero uno se reintenta porque el nodo está caído y el otro porque
// el bloque no salió. Confundirlos es lo que hace que un log no sirva.
var ErrSinRecibo = errors.New("chain: el nodo todavía no tiene el recibo de esa transacción")

// ErrRPC envuelve lo que contesta el nodo cuando contesta un error.
var ErrRPC = errors.New("chain: el nodo devolvió un error")

// RPC es un nodo al que se le pueden hacer preguntas de lectura.
type RPC struct {
	url string
	hc  *http.Client
}

// tiempoRPC: el nodo es de otro, y ningún camino de la app puede quedar
// esperando a la red (SDD §2). Cinco segundos y se sigue con lo que hay.
const tiempoRPC = 5 * time.Second

func NuevoRPC(url string) *RPC {
	return &RPC{url: url, hc: &http.Client{Timeout: tiempoRPC}}
}

// NuevoRPCCon deja cambiar el cliente HTTP, que es lo que hace el test para
// hablarle a un httptest.Server sin levantar un nodo.
func NuevoRPCCon(url string, hc *http.Client) *RPC {
	return &RPC{url: url, hc: hc}
}

// Recibo es lo poco que interesa de un `eth_getTransactionReceipt`.
type Recibo struct {
	// Exitoso: el status del recibo. Ojo con la trampa: un recibo existe
	// también para una transacción que revirtió. "Se minó" y "salió bien" son
	// dos preguntas distintas, y tratarlas como una es lo que hace que una app
	// dé por hecho un minteo que la EVM rechazó.
	Exitoso bool
	Bloque  uint64
	Logs    []LogRecibo
}

// LogRecibo es un evento emitido durante la transacción.
type LogRecibo struct {
	Address string
	Topics  []string
	Data    string
}

type peticionRPC struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Method  string `json:"method"`
	Params  []any  `json:"params"`
}

type respuestaRPC struct {
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

type reciboCrudo struct {
	Status      string `json:"status"`
	BlockNumber string `json:"blockNumber"`
	Logs        []struct {
		Address string   `json:"address"`
		Topics  []string `json:"topics"`
		Data    string   `json:"data"`
	} `json:"logs"`
}

// Recibo pide el recibo de una transacción.
//
// Devuelve ErrSinRecibo si el nodo contesta `null`, que es lo que contesta
// mientras la transacción está en el mempool o si nunca existió. Desde afuera
// no se pueden distinguir, y está bien que no se puedan: en los dos casos lo
// único que se puede hacer es volver a preguntar más tarde.
func (r *RPC) Recibo(ctx context.Context, txHash string) (Recibo, error) {
	var out Recibo

	crudo, err := r.llamar(ctx, "eth_getTransactionReceipt", txHash)
	if err != nil {
		return out, err
	}
	// `null` es una respuesta legítima, no un error de formato.
	if len(crudo) == 0 || string(crudo) == "null" {
		return out, ErrSinRecibo
	}

	var rec reciboCrudo
	if err := json.Unmarshal(crudo, &rec); err != nil {
		return out, fmt.Errorf("chain: el recibo no se entiende: %w", err)
	}

	out.Exitoso = strings.EqualFold(rec.Status, "0x1")
	out.Bloque, _ = ParseCantidad(rec.BlockNumber)
	for _, l := range rec.Logs {
		out.Logs = append(out.Logs, LogRecibo{Address: l.Address, Topics: l.Topics, Data: l.Data})
	}
	return out, nil
}

func (r *RPC) llamar(ctx context.Context, metodo string, params ...any) (json.RawMessage, error) {
	if params == nil {
		params = []any{}
	}
	cuerpo, err := json.Marshal(peticionRPC{JSONRPC: "2.0", ID: 1, Method: metodo, Params: params})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.url, bytes.NewReader(cuerpo))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := r.hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("chain: no se pudo hablar con el nodo: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: HTTP %d", ErrRPC, res.StatusCode)
	}

	var rpcRes respuestaRPC
	if err := json.NewDecoder(res.Body).Decode(&rpcRes); err != nil {
		return nil, fmt.Errorf("chain: el nodo no devolvió JSON-RPC: %w", err)
	}
	if rpcRes.Error != nil {
		return nil, fmt.Errorf("%w: %d %s", ErrRPC, rpcRes.Error.Code, rpcRes.Error.Message)
	}
	return rpcRes.Result, nil
}

// ── Lo que viaja en hexadecimal ───────────────────────────────────────────

// ParseCantidad lee una "quantity" de JSON-RPC: hexadecimal con "0x", sin ceros
// de relleno adelante. Es el formato de todos los números del protocolo.
func ParseCantidad(s string) (uint64, error) {
	n, ok := new(big.Int).SetString(strings.TrimPrefix(s, "0x"), 16)
	if !ok {
		return 0, fmt.Errorf("chain: %q no es una cantidad hexadecimal", s)
	}
	return n.Uint64(), nil
}

// ParseUint256 lee una palabra de 32 bytes como entero decimal.
//
// Devuelve un *big.Int porque un id de ERC-721 no entra en ningún entero de Go:
// acá son 160 bits de dirección corridos 16 lugares. Por eso la columna
// `token_id` es numeric y no bigint.
func ParseUint256(s string) (*big.Int, error) {
	n, ok := new(big.Int).SetString(strings.TrimPrefix(s, "0x"), 16)
	if !ok {
		return nil, fmt.Errorf("chain: %q no es un uint256 hexadecimal", s)
	}
	return n, nil
}

// LargoHashTx son los 32 bytes de un hash de transacción.
const LargoHashTx = 32

// ErrHashInvalido: eso no tiene forma de hash de transacción.
var ErrHashInvalido = errors.New("chain: no es un hash de transacción de 32 bytes")

// ParseHashTx valida un hash de transacción y lo devuelve en minúsculas.
//
// Se valida acá y no en el handler porque lo que sigue es una llamada a un nodo
// ajeno: mandarle cualquier cosa que llegue por HTTP es regalarle a un tercero
// la posibilidad de decidir qué le preguntamos.
func ParseHashTx(s string) (string, error) {
	crudo := strings.TrimPrefix(strings.TrimPrefix(s, "0x"), "0X")
	if len(crudo) != LargoHashTx*2 {
		return "", fmt.Errorf("%w: %q", ErrHashInvalido, s)
	}
	if _, err := hex.DecodeString(crudo); err != nil {
		return "", fmt.Errorf("%w: %q", ErrHashInvalido, s)
	}
	return "0x" + strings.ToLower(crudo), nil
}
