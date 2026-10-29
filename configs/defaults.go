// Package configs embeds the shipped application defaults. Deployments can
// override them with files in a configuration directory and environment values.
package configs

import _ "embed"

//go:embed app.yaml
var Defaults []byte
