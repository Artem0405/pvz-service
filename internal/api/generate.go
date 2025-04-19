package api

//go:generate go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen --config=../../oapi-codegen.cfg.yaml ../../api/openapi/swagger.yaml

// Этот файл служит только для хранения директивы go:generate.
// Он может быть пустым, кроме объявления пакета и директивы.
// Компилятор Go игнорирует файлы с именем, заканчивающимся на _test.go,
// но файлы с именем generate.go обрабатываются как обычные файлы пакета.
