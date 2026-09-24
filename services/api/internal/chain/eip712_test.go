package chain

import (
	"encoding/hex"
	"testing"
)

// dominioDePrueba y voucherDePrueba son exactamente los del fixture cruzado.
// Están acá para que los valores esperados de abajo sean los mismos que Foundry
// va a recalcular del otro lado.
func dominioDePrueba(t *testing.T) Dominio {
	t.Helper()

	contrato, err := ParseDireccion(FixtureVerifyingContract)
	if err != nil {
		t.Fatal(err)
	}
	return DominioDistinciones(FixtureChainID, contrato)
}

func voucherDePrueba(t *testing.T) Voucher {
	t.Helper()

	to, err := ParseDireccion(FixtureTo)
	if err != nil {
		t.Fatal(err)
	}
	return Voucher{To: to, Pieza: FixturePieza, Deadline: FixtureDeadline}
}

// TestSeparadorDominio compara contra un valor calculado afuera, con `cast`, y
// no contra esta misma implementación.
//
// El valor tapa el error más común de un EIP-712 escrito a mano: meter el
// nombre y la versión como texto crudo en lugar de su keccak. Si alguien lo
// hace, este test lo agarra sin necesidad de levantar Foundry.
func TestSeparadorDominio(t *testing.T) {
	const esperado = "94c5f0a4f93c5e1274e1c0e13a489c33227213109feea528cdfb359bd0473e00"

	got := dominioDePrueba(t).Separador()
	if hex.EncodeToString(got[:]) != esperado {
		t.Fatalf("separador de dominio = %x, esperaba %s", got, esperado)
	}
}

func TestHashStruct(t *testing.T) {
	const esperado = "e5500a1ead3bcd217e416c06bd29e9a20d71f6a9f0ed860cee4573d640e89899"

	got := voucherDePrueba(t).HashStruct()
	if hex.EncodeToString(got[:]) != esperado {
		t.Fatalf("hashStruct = %x, esperaba %s", got, esperado)
	}
}

func TestDigest(t *testing.T) {
	got := Digest(dominioDePrueba(t), voucherDePrueba(t))
	if "0x"+hex.EncodeToString(got[:]) != FixtureDigest {
		t.Fatalf("digest = %x, esperaba %s", got, FixtureDigest)
	}
}

// TestElDominioAtaLaFirmaASuContexto es la propiedad por la que existe EIP-712:
// la misma firma no puede servir en otra cadena ni en otro contrato.
func TestElDominioAtaLaFirmaASuContexto(t *testing.T) {
	base := dominioDePrueba(t)
	v := voucherDePrueba(t)
	digest := Digest(base, v)

	otroContrato, err := ParseDireccion("0x000000000000000000000000000000000000dead")
	if err != nil {
		t.Fatal(err)
	}

	variantes := map[string]Dominio{
		"otra cadena": {
			Nombre: base.Nombre, Version: base.Version,
			ChainID: base.ChainID + 1, VerifyingContract: base.VerifyingContract,
		},
		"otro contrato": {
			Nombre: base.Nombre, Version: base.Version,
			ChainID: base.ChainID, VerifyingContract: otroContrato,
		},
		"otro nombre": {
			Nombre: "Otra App", Version: base.Version,
			ChainID: base.ChainID, VerifyingContract: base.VerifyingContract,
		},
		"otra versión": {
			Nombre: base.Nombre, Version: "2",
			ChainID: base.ChainID, VerifyingContract: base.VerifyingContract,
		},
	}

	for nombre, d := range variantes {
		if Digest(d, v) == digest {
			t.Fatalf("%s: el digest no cambió, el dominio no está atando nada", nombre)
		}
	}
}

// TestCadaCampoDelVoucherCambiaElDigest: si un campo no entrara en el hash,
// alguien podría reusar una firma cambiándolo. Esto lo prueba campo por campo.
func TestCadaCampoDelVoucherCambiaElDigest(t *testing.T) {
	d := dominioDePrueba(t)
	base := voucherDePrueba(t)
	digest := Digest(d, base)

	otro, err := ParseDireccion("0x000000000000000000000000000000000000c0de")
	if err != nil {
		t.Fatal(err)
	}

	variantes := map[string]Voucher{
		"otro destinatario": {To: otro, Pieza: base.Pieza, Deadline: base.Deadline},
		"otra pieza":        {To: base.To, Pieza: base.Pieza + 1, Deadline: base.Deadline},
		"otro deadline":     {To: base.To, Pieza: base.Pieza, Deadline: base.Deadline + 1},
	}

	for nombre, v := range variantes {
		if Digest(d, v) == digest {
			t.Fatalf("%s: el digest no cambió, ese campo no está entrando al hash", nombre)
		}
	}
}
