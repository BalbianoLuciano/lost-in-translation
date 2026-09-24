package chain

import (
	"encoding/hex"
	"strconv"
	"testing"
)

// El vector es el de siempre: "Hello World" firmado con personal_sign. El hash
// está publicado en mil lados y lo devuelve igual `cast hash-message`, así que
// si esto pasa, el prefijo y el conteo de bytes están bien.
func TestHashPersonal(t *testing.T) {
	tests := []struct {
		name    string
		mensaje string
		want    string
	}{
		{
			name:    "el vector conocido",
			mensaje: "Hello World",
			want:    "a1de988600a42c4b4ab089b619297c17d53cffae5d5120d82d8a92d0bb3b78f2",
		},
		{
			name:    "el mensaje vacío también tiene hash",
			mensaje: "",
			want:    "5f35dce98ba4fba25530a026ed80b2cecdaa31091ba4958b99b52ea1d068adad",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hex.EncodeToString(mustHash(tt.mensaje))
			if got != tt.want {
				t.Fatalf("HashPersonal(%q) = %s\nwant %s", tt.mensaje, got, tt.want)
			}
		})
	}
}

func mustHash(s string) []byte {
	h := HashPersonal([]byte(s))
	return h[:]
}

// El largo del prefijo va en **bytes**, no en caracteres.
//
// No hay un vector publicado para esto, así que se prueba por la contraria: se
// arma a mano el hash que saldría contando runas y se exige que no sea el
// mismo. Con un statement que dice "año" —y el de este servidor tiene acentos—
// contar mal recupera una dirección cualquiera y el vínculo falla sin explicar
// nada.
func TestHashPersonalCuentaBytesYNoCaracteres(t *testing.T) {
	const mensaje = "año ñandú"
	if len(mensaje) == len([]rune(mensaje)) {
		t.Fatal("el mensaje de prueba tiene que tener más bytes que runas")
	}

	conRunas := Keccak256(
		[]byte(prefijoPersonal),
		[]byte(strconv.Itoa(len([]rune(mensaje)))),
		[]byte(mensaje),
	)
	if HashPersonal([]byte(mensaje)) == conRunas {
		t.Fatal("contar runas tendría que dar otro hash")
	}
}

// La vuelta completa: firmar con una clave conocida y recuperar su dirección.
// Es exactamente lo que hace Vincular con lo que manda la billetera.
func TestFirmarYRecuperarUnMensajePersonal(t *testing.T) {
	priv, err := ClaveDesdeHex(FixtureClavePrivada)
	if err != nil {
		t.Fatal(err)
	}
	mensaje := []byte("localhost:5173 wants you to sign in with your Ethereum account:")

	firma, err := Firmar(priv, HashPersonal(mensaje))
	if err != nil {
		t.Fatal(err)
	}
	dir, err := Recuperar(HashPersonal(mensaje), firma)
	if err != nil {
		t.Fatal(err)
	}
	if dir.Hex() != FixtureFirmante {
		t.Fatalf("recuperó %s, want %s", dir.Hex(), FixtureFirmante)
	}

	// Un byte distinto en el mensaje y la dirección que sale es otra: no hay
	// error, hay una dirección que no es de nadie. Es justamente por eso que
	// Vincular compara contra la que el texto declara.
	otra, err := Recuperar(HashPersonal(append(mensaje, '.')), firma)
	if err == nil && otra == dir {
		t.Fatal("cambiar el mensaje tenía que dar otra dirección")
	}
}

// Las billeteras que devuelven V en 0/1 firman bien: lo que está mal es el byte.
func TestParseFirmaDeBilleteraAcomodaLaV(t *testing.T) {
	priv, err := ClaveDesdeHex(FixtureClavePrivada)
	if err != nil {
		t.Fatal(err)
	}
	digest := HashPersonal([]byte("hola"))
	firma, err := Firmar(priv, digest)
	if err != nil {
		t.Fatal(err)
	}

	cruda := firma
	cruda[64] -= 27 // lo que devuelven algunas librerías
	arreglada, err := ParseFirmaDeBilletera(cruda.Hex())
	if err != nil {
		t.Fatal(err)
	}
	if arreglada != firma {
		t.Fatalf("no acomodó la V: %s vs %s", arreglada.Hex(), firma.Hex())
	}
	if _, err := Recuperar(digest, arreglada); err != nil {
		t.Fatalf("la firma acomodada tenía que recuperar: %v", err)
	}

	// Y sin acomodar, Recuperar la rechaza: por eso hace falta la función.
	if _, err := Recuperar(digest, cruda); err == nil {
		t.Fatal("una V en 0 o 1 no la acepta ecrecover")
	}
}
