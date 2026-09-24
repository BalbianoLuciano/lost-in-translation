package chain

import (
	"encoding/binary"
	"errors"
	"math/big"
	"testing"

	"github.com/decred/dcrd/dcrec/secp256k1/v4/ecdsa"
)

// TestDireccionDeClaveVectorConocido: la clave de prueba más difundida que hay
// (la cuenta 0 de anvil) y la dirección que le corresponde. Si la derivación se
// equivoca —por ejemplo, dejando adentro el 0x04 del prefijo SEC— esto lo dice.
func TestDireccionDeClaveVectorConocido(t *testing.T) {
	priv, err := ClaveDesdeHex(FixtureClavePrivada)
	if err != nil {
		t.Fatal(err)
	}

	got := DireccionDeClave(priv.PubKey())
	if got.Hex() != FixtureFirmante {
		t.Fatalf("dirección = %s, esperaba %s", got, FixtureFirmante)
	}
}

func TestClaveDesdeHexRechazaLoQueNoEsClave(t *testing.T) {
	casos := map[string]string{
		"vacía":                "",
		"corta":                "0xac09",
		"no hexadecimal":       "0xzz0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80",
		"cero":                 "0x" + "00",
		"cero de 32 bytes":     "0x0000000000000000000000000000000000000000000000000000000000000000",
		"igual al orden":       "0xfffffffffffffffffffffffffffffffebaaedce6af48a03bbfd25e8cd0364141",
		"por encima del orden": "0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
	}

	for nombre, s := range casos {
		t.Run(nombre, func(t *testing.T) {
			if _, err := ClaveDesdeHex(s); err == nil {
				t.Fatalf("%q debería haber fallado", s)
			}
		})
	}
}

// TestFirmarYRecuperar es la ida y vuelta: lo que firma el servidor lo recupera
// el mismo `ecrecover` que corre adentro del contrato.
func TestFirmarYRecuperar(t *testing.T) {
	priv, err := ClaveDesdeHex(FixtureClavePrivada)
	if err != nil {
		t.Fatal(err)
	}
	esperada := DireccionDeClave(priv.PubKey())

	// Varios digests, porque el byte de recuperación depende del punto R que
	// salga, y con uno solo se prueba la mitad de los casos.
	for i := 0; i < 64; i++ {
		var n [8]byte
		binary.BigEndian.PutUint64(n[:], uint64(i))
		digest := Keccak256(n[:])

		f, err := Firmar(priv, digest)
		if err != nil {
			t.Fatalf("firmar %d: %v", i, err)
		}

		if f[64] != 27 && f[64] != 28 {
			t.Fatalf("firma %d: V = %d, tiene que ser 27 o 28", i, f[64])
		}

		got, err := Recuperar(digest, f)
		if err != nil {
			t.Fatalf("recuperar %d: %v", i, err)
		}
		if got != esperada {
			t.Fatalf("firma %d: recuperó %s en vez de %s", i, got, esperada)
		}
	}
}

// TestFirmarEsDeterminista: sin esto, el fixture del test cruzado no podría ser
// un golden file, porque cada corrida daría bytes distintos.
func TestFirmarEsDeterminista(t *testing.T) {
	priv, err := ClaveDesdeHex(FixtureClavePrivada)
	if err != nil {
		t.Fatal(err)
	}
	digest := Digest(dominioDePrueba(t), voucherDePrueba(t))

	primera, err := Firmar(priv, digest)
	if err != nil {
		t.Fatal(err)
	}
	segunda, err := Firmar(priv, digest)
	if err != nil {
		t.Fatal(err)
	}

	if primera != segunda {
		t.Fatalf("dos firmas del mismo digest dieron distinto:\n%s\n%s", primera.Hex(), segunda.Hex())
	}
}

// TestFirmarSiempreDevuelveSBaja. El contrato usa `ECDSA.tryRecover` de
// OpenZeppelin, que rechaza de plano cualquier S por encima de n/2. Si la
// mitad de las firmas salieran con S alta, la mitad de los mints fallaría sin
// explicación.
func TestFirmarSiempreDevuelveSBaja(t *testing.T) {
	priv, err := ClaveDesdeHex(FixtureClavePrivada)
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 128; i++ {
		var n [8]byte
		binary.BigEndian.PutUint64(n[:], uint64(i))
		digest := Keccak256(n[:])

		f, err := Firmar(priv, digest)
		if err != nil {
			t.Fatal(err)
		}

		s := new(big.Int).SetBytes(f[32:64])
		if s.Cmp(mitadDelOrden) > 0 {
			t.Fatalf("firma %d salió con la S alta: %s", i, f.Hex())
		}
	}
}

// TestNormalizarSAlta arma a mano la firma maleable —la que Firmar nunca va a
// producir— y comprueba las dos mitades de la propiedad: que normalizar la
// devuelve a la forma buena, y que la versión de S alta seguía siendo
// matemáticamente válida (recupera la misma dirección) aunque el contrato la
// rechace igual.
func TestNormalizarSAlta(t *testing.T) {
	priv, err := ClaveDesdeHex(FixtureClavePrivada)
	if err != nil {
		t.Fatal(err)
	}
	digest := Digest(dominioDePrueba(t), voucherDePrueba(t))

	buena, err := Firmar(priv, digest)
	if err != nil {
		t.Fatal(err)
	}

	// La gemela maleable: misma R, S dada vuelta, V con la paridad cambiada.
	maleable := buena
	s := new(big.Int).SetBytes(buena[32:64])
	s.Sub(ordenSecp256k1, s)
	s.FillBytes(maleable[32:64])
	// Escrito a mano y no con darVueltaV: si el caso de prueba usara la misma
	// función que está probando, un error en las dos puntas se cancelaría.
	if buena[64] == 27 {
		maleable[64] = 28
	} else {
		maleable[64] = 27
	}

	if maleable == buena {
		t.Fatal("la firma maleable quedó igual a la buena: el armado del caso está mal")
	}

	// Sigue siendo una firma válida de la misma clave: eso es la maleabilidad.
	// Para comprobarlo hay que saltear la validación de S baja de Recuperar.
	if got := recuperarSinValidarS(t, digest, maleable); got != DireccionDeClave(priv.PubKey()) {
		t.Fatalf("la firma con S alta recuperó %s, que no es el firmante", got)
	}

	// Pero el contrato la rechaza, así que nosotros también.
	if _, err := Recuperar(digest, maleable); !errors.Is(err, ErrFirmaMaleable) {
		t.Fatalf("Recuperar debería rechazar la S alta, devolvió %v", err)
	}

	// Y normalizarla la devuelve exactamente a la firma buena.
	if normalizar(maleable) != buena {
		t.Fatalf("normalizar no volvió a la firma buena:\n%s\n%s",
			normalizar(maleable).Hex(), buena.Hex())
	}

	// Normalizar algo ya normalizado no lo toca.
	if normalizar(buena) != buena {
		t.Fatal("normalizar cambió una firma que ya tenía la S baja")
	}
}

// recuperarSinValidarS es el ecrecover crudo, sin la regla de S baja que aplica
// Recuperar. Sólo existe para poder mostrar que la firma maleable es una firma
// válida de verdad y no un montón de bytes rotos: la matemática la acepta, y
// quien la rechaza es la convención de Ethereum.
func recuperarSinValidarS(t *testing.T, digest [32]byte, f Firma) Direccion {
	t.Helper()

	compacta := make([]byte, LargoFirma)
	compacta[0] = f[64]
	copy(compacta[1:], f[0:64])

	pub, _, err := ecdsa.RecoverCompact(compacta, digest[:])
	if err != nil {
		t.Fatalf("la firma con S alta no recuperó nada: %v", err)
	}
	return DireccionDeClave(pub)
}

func TestRecuperarRechazaLoMalformado(t *testing.T) {
	priv, err := ClaveDesdeHex(FixtureClavePrivada)
	if err != nil {
		t.Fatal(err)
	}
	digest := Digest(dominioDePrueba(t), voucherDePrueba(t))

	buena, err := Firmar(priv, digest)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("V fuera de 27/28", func(t *testing.T) {
		// El 0/1 es el formato interno del EVM; Ethereum lo usa en 27/28 desde
		// siempre y OpenZeppelin no acepta otra cosa.
		mala := buena
		mala[64] = 0
		if _, err := Recuperar(digest, mala); !errors.Is(err, ErrFirmaInvalida) {
			t.Fatalf("esperaba ErrFirmaInvalida, dio %v", err)
		}
	})

	t.Run("otro digest recupera a otro", func(t *testing.T) {
		otro := Keccak256([]byte("otra cosa"))
		got, err := Recuperar(otro, buena)
		// Puede recuperar algo o no recuperar nada; lo que no puede es dar el
		// firmante. Es exactamente lo que hace el contrato con FirmaInvalida.
		if err == nil && got == DireccionDeClave(priv.PubKey()) {
			t.Fatal("la firma de un digest sirvió para otro")
		}
	})
}

func TestParseFirmaIdaYVuelta(t *testing.T) {
	priv, err := ClaveDesdeHex(FixtureClavePrivada)
	if err != nil {
		t.Fatal(err)
	}
	f, err := Firmar(priv, Digest(dominioDePrueba(t), voucherDePrueba(t)))
	if err != nil {
		t.Fatal(err)
	}

	vuelta, err := ParseFirma(f.Hex())
	if err != nil {
		t.Fatal(err)
	}
	if vuelta != f {
		t.Fatalf("la firma no sobrevivió el viaje por texto: %s vs %s", vuelta.Hex(), f.Hex())
	}

	for _, malo := range []string{"", "0x00", f.Hex() + "00", "0x" + "zz"} {
		if _, err := ParseFirma(malo); err == nil {
			t.Fatalf("%q debería haber fallado", malo)
		}
	}
}
