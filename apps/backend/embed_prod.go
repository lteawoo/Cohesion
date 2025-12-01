//go:build production

package main

import "embed"

//go:embed dist/web
var WebDist embed.FS
