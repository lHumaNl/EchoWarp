// Package i18n provides internationalization support for EchoWarp.
package i18n

import "embed"

//go:embed locales/*.yaml
var localesFS embed.FS
