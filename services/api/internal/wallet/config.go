package wallet

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/decred/dcrd/dcrec/secp256k1/v4"

	"github.com/BalbianoLuciano/lost-in-translation/services/api/internal/chain"
)

// Config es todo lo que hace falta para que la cadena exista. Sin esto, el
// servicio se construye igual y contesta 503 a todo: la app no se entera de que
// la cadena podría existir, igual que hace hoy sin la clave de Groq.
type Config struct {
	// RPCURL es el nodo al que se le preguntan los recibos. Sólo lectura.
	RPCURL string
	// ChainID: 8453 es Base, 84532 Base Sepolia, 31337 anvil.
	ChainID uint64
	// Contrato es Distinciones.sol, ya desplegado.
	Contrato chain.Direccion
	// Firmante es la clave que firma vouchers. No tiene fondos y no puede
	// gastar: si se filtra, se falsifican distinciones —malo— pero no se roba
	// un peso, y el contrato tiene setFirmante para rotarla sin redeploy.
	Firmante *secp256k1.PrivateKey
	// Dominio y URI son los que el mensaje SIWE tiene que declarar: el sitio
	// desde el que se firma. Salen del primer origen de CORS, que ya es "dónde
	// vive la web" y no hay razón para configurarlo dos veces.
	Dominio string
	URI     string
}

// ErrConfigIncompleta: hay parte de la configuración de cadena y no toda. Es
// peor que no tener nada, porque parece que anda.
var ErrConfigIncompleta = errors.New("wallet: la configuración de cadena está a medias")

// Crudo son las cuatro variables de entorno tal como llegan, sin interpretar,
// más los orígenes de CORS, de donde sale el dominio del mensaje SIWE.
type Crudo struct {
	RPCURL   string
	ChainID  uint64
	Contrato string
	ClaveHex string
	Origenes []string
}

// Definido dice si alguien intentó configurar la cadena.
func (c Crudo) Definido() bool {
	return c.RPCURL != "" || c.ChainID != 0 || c.Contrato != "" || c.ClaveHex != ""
}

// Interpretar convierte las variables de entorno en una Config usable.
//
// Devuelve (nil, nil) cuando no hay nada configurado: ése es el caso normal en
// desarrollo y no es un error. Devuelve error cuando hay algo pero está mal o
// incompleto, y eso sí tiene que romper el arranque: una cadena a medias es un
// endpoint que contesta 500 en vez de 503, y nadie sabe por qué.
func Interpretar(c Crudo) (*Config, error) {
	if !c.Definido() {
		return nil, nil
	}

	var faltan []string
	if c.RPCURL == "" {
		faltan = append(faltan, "CHAIN_RPC_URL")
	}
	if c.ChainID == 0 {
		faltan = append(faltan, "CHAIN_ID")
	}
	if c.Contrato == "" {
		faltan = append(faltan, "CHAIN_CONTRACT")
	}
	if c.ClaveHex == "" {
		faltan = append(faltan, "CHAIN_SIGNER_KEY")
	}
	if len(faltan) > 0 {
		return nil, fmt.Errorf("%w: faltan %s", ErrConfigIncompleta, strings.Join(faltan, ", "))
	}

	contrato, err := chain.ParseDireccion(c.Contrato)
	if err != nil {
		return nil, fmt.Errorf("CHAIN_CONTRACT: %w", err)
	}
	// El mensaje del error no lleva la clave ni un pedazo: es el secreto más
	// serio del proyecto y los logs de arranque se leen en cualquier lado.
	clave, err := chain.ClaveDesdeHex(c.ClaveHex)
	if err != nil {
		return nil, errors.New("CHAIN_SIGNER_KEY no es una clave privada de 32 bytes en hexadecimal")
	}

	dominio, uri, err := sitio(c.Origenes)
	if err != nil {
		return nil, err
	}

	return &Config{
		RPCURL:   c.RPCURL,
		ChainID:  c.ChainID,
		Contrato: contrato,
		Firmante: clave,
		Dominio:  dominio,
		URI:      uri,
	}, nil
}

// sitio saca el dominio y la URI del primer origen de CORS.
func sitio(origenes []string) (string, string, error) {
	if len(origenes) == 0 {
		return "", "", fmt.Errorf("%w: sin CORS_ORIGINS no hay dominio que poner en el mensaje SIWE", ErrConfigIncompleta)
	}
	u, err := url.Parse(origenes[0])
	if err != nil || u.Host == "" {
		return "", "", fmt.Errorf("%w: %q no es un origen del que salga un dominio", ErrConfigIncompleta, origenes[0])
	}
	return u.Host, strings.TrimSuffix(u.String(), "/"), nil
}

// Firmante es la dirección pública de la clave de firma. Se muestra para poder
// compararla con la que el contrato tiene configurada: si no coinciden, todo
// mint va a revertir con FirmaInvalida y esto es lo único que lo explica.
func (c *Config) FirmanteHex() string {
	return chain.DireccionDeClave(c.Firmante.PubKey()).Hex()
}
