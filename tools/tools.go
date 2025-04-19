//go:build tools
// +build tools

package tools

import (
	// Импортируем существующий библиотечный пакет из модуля, чтобы отследить зависимость
	_ "github.com/oapi-codegen/oapi-codegen/v2/pkg/codegen"
	_ "gopkg.in/yaml.v2"
)
