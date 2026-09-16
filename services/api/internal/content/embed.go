package content

import _ "embed"

// Generado por `cd tools && uv run lit-tools build`. No editar a mano.
//
//go:embed bundle.json
var bundleJSON []byte

// Load devuelve el catálogo embebido en el binario.
func Load() (*Catalog, error) {
	return Parse(bundleJSON)
}
