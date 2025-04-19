package domain

import "errors"

// Определяем кастомные ошибки для домена аутентификации
var (
	ErrAuthValidation            = errors.New("ошибка валидации данных аутентификации") // Общая ошибка валидации
	ErrAuthInvalidCredentials    = errors.New("неверный email или пароль")              // Неверные учетные данные
	ErrAuthTokenExpired          = errors.New("токен истек")                            // Токен просрочен
	ErrAuthTokenMalformed        = errors.New("некорректный формат токена")             // Неверный формат токена
	ErrAuthTokenInvalidSignature = errors.New("неверная подпись токена")                // Ошибка проверки подписи
	ErrAuthTokenInvalid          = errors.New("невалидный токен")                       // Общая ошибка невалидного токена
	// Можно добавить другие специфичные ошибки домена, если нужно
)

// Вы также можете добавить сюда ошибки репозитория, если хотите
// или оставить их объявление в пакете repository.
// Например:
// var (
// 	ErrUserNotFound        = errors.New("пользователь не найден")
//  ErrUserDuplicateEmail = errors.New("пользователь с таким email уже существует")
// )
// Но лучше, чтобы ошибки, специфичные для репозитория, оставались там.
