package chain

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// nodoDeMentira contesta lo que se le diga y guarda lo que le preguntaron.
func nodoDeMentira(t *testing.T, cuerpo string, status int) (*RPC, *peticionRPC) {
	t.Helper()
	var visto peticionRPC
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		crudo, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(crudo, &visto)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, cuerpo)
	}))
	t.Cleanup(srv.Close)
	return NuevoRPCCon(srv.URL, srv.Client()), &visto
}

const hashDePrueba = "0x1111111111111111111111111111111111111111111111111111111111111111"

func TestReciboExitoso(t *testing.T) {
	const respuesta = `{"jsonrpc":"2.0","id":1,"result":{
		"status":"0x1","blockNumber":"0x1a2b",
		"logs":[{"address":"0x7fa04a5a7fd31c6215c1d980514db5e173d09e69",
		         "topics":["0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef",
		                   "0x0000000000000000000000000000000000000000000000000000000000000000",
		                   "0x00000000000000000000000070997970c51812dc3a010c7d01b50e0d17dc79c8",
		                   "0x0000000000000000000000000000000000000000000000000000000000000011"],
		         "data":"0x"}]}}`

	rpc, visto := nodoDeMentira(t, respuesta, http.StatusOK)
	rec, err := rpc.Recibo(context.Background(), hashDePrueba)
	if err != nil {
		t.Fatal(err)
	}
	if !rec.Exitoso || rec.Bloque != 0x1a2b || len(rec.Logs) != 1 {
		t.Fatalf("recibo = %+v", rec)
	}
	if visto.Method != "eth_getTransactionReceipt" || len(visto.Params) != 1 || visto.Params[0] != hashDePrueba {
		t.Fatalf("le preguntó otra cosa: %+v", visto)
	}
}

// La trampa del recibo: existe también cuando la transacción revirtió. "Se
// minó" y "salió bien" son dos preguntas distintas.
func TestReciboQueRevirtio(t *testing.T) {
	rpc, _ := nodoDeMentira(t, `{"jsonrpc":"2.0","id":1,"result":{"status":"0x0","blockNumber":"0x5","logs":[]}}`, http.StatusOK)
	rec, err := rpc.Recibo(context.Background(), hashDePrueba)
	if err != nil {
		t.Fatal(err)
	}
	if rec.Exitoso {
		t.Fatal("status 0x0 es una transacción que revirtió")
	}
}

func TestReciboQueTodaviaNoEsta(t *testing.T) {
	rpc, _ := nodoDeMentira(t, `{"jsonrpc":"2.0","id":1,"result":null}`, http.StatusOK)
	if _, err := rpc.Recibo(context.Background(), hashDePrueba); !errors.Is(err, ErrSinRecibo) {
		t.Fatalf("err = %v, want ErrSinRecibo", err)
	}
}

// Los dos errores que dejan el minteo en pending, y que están separados porque
// se reintentan por razones distintas.
func TestReciboConNodoRoto(t *testing.T) {
	tests := []struct {
		name   string
		cuerpo string
		status int
		want   error
	}{
		{"el nodo devuelve un error de JSON-RPC",
			`{"jsonrpc":"2.0","id":1,"error":{"code":-32000,"message":"rate limited"}}`, http.StatusOK, ErrRPC},
		{"el nodo devuelve 500", `nada`, http.StatusInternalServerError, ErrRPC},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rpc, _ := nodoDeMentira(t, tt.cuerpo, tt.status)
			if _, err := rpc.Recibo(context.Background(), hashDePrueba); !errors.Is(err, tt.want) {
				t.Fatalf("err = %v, want %v", err, tt.want)
			}
		})
	}

	// Y el nodo que directamente no está.
	rpc := NuevoRPCCon("http://127.0.0.1:1", http.DefaultClient)
	if _, err := rpc.Recibo(context.Background(), hashDePrueba); err == nil || errors.Is(err, ErrSinRecibo) {
		t.Fatalf("un nodo caído no es un recibo ausente: %v", err)
	}
}

func TestParseHashTx(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
		ok   bool
	}{
		{"con 0x y en mayúsculas", "0x" + "AB" + "11111111111111111111111111111111111111111111111111111111111111", "0xab11111111111111111111111111111111111111111111111111111111111111", true},
		{"sin 0x", "1111111111111111111111111111111111111111111111111111111111111111", hashDePrueba, true},
		{"corto", "0x1111", "", false},
		{"no es hexadecimal", "0x" + "zz11111111111111111111111111111111111111111111111111111111111111", "", false},
		{"vacío", "", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseHashTx(tt.in)
			if tt.ok != (err == nil) {
				t.Fatalf("err = %v, ok esperado %v", err, tt.ok)
			}
			if tt.ok && got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
			if !tt.ok && !errors.Is(err, ErrHashInvalido) {
				t.Fatalf("err = %v, want ErrHashInvalido", err)
			}
		})
	}
}

// Un id de ERC-721 es un uint256 y no entra en ningún entero de Go: por eso la
// columna es numeric y por eso ParseUint256 devuelve un *big.Int.
func TestParseUint256NoSeDesborda(t *testing.T) {
	// (0x70997970c51812dc3a010c7d01b50e0d17dc79c8 << 16) | 17
	const palabra = "0x00000000000070997970c51812dc3a010c7d01b50e0d17dc79c80011"
	n, err := ParseUint256(palabra)
	if err != nil {
		t.Fatal(err)
	}
	const want = "42128477998799320712182006334230556568206271482167313"
	if n.String() != want {
		t.Fatalf("ParseUint256 = %s\nwant %s", n, want)
	}
	if n.IsUint64() {
		t.Fatal("si entrara en un uint64 este test no probaría nada")
	}
}
