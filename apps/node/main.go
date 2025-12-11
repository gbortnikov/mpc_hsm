package main

import (
	"log"
	"os"

	"github.com/mpc_hsm/node/app"
)

func main() {
	// Разбор флагов командной строки
	flags := app.ParseFlags()

	// Загрузка конфигурации
	cfg, err := app.LoadConfig(flags)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Настройка логирования в начале, чтобы работали отладочные логи
	app.SetupLogging(cfg.Logging)
	app.LogConfigInfo(cfg, flags.ConfigFile)

	// Создание и инициализация приложения.
	application := app.New(cfg)
	if err := application.Initialize(); err != nil {
		log.Fatalf("Failed to initialize application: %v", err)
	}

	// Запуск приложения и обработка завершения
	if err := application.Run(); err != nil {
		log.Fatalf("Application error: %v", err)
		os.Exit(1)
	}
}
