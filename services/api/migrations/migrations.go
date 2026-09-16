// Package migrations embebe las migraciones SQL para correrlas al arrancar la API.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
