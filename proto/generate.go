// proto/generate.go
package proto

// Обратите внимание на использование / вместо \ в пути для флага -I
// Также путь заключен в кавычки на случай пробелов (хотя в данном пути их нет)
//go:generate protoc -I=. -I "C:/Users/artem/AppData/Local/Microsoft/WinGet/Packages/Google.Protobuf_Microsoft.Winget.Source_8wekyb3d8bbwe/include" --go_out=../pkg/ --go_opt=paths=source_relative --go-grpc_out=../pkg/ --go-grpc_opt=paths=source_relative pvz/v1/pvz.proto

// Этот файл для go:generate директив
