package main

import (
	"context"
	"flag"
	"log"

	"github.com/vlladoff/bot_group_protect/internal/config"
	gemini "github.com/vlladoff/bot_group_protect/internal/llm"
	"github.com/vlladoff/bot_group_protect/internal/telegram"
)

func main() {
	ctx := context.Background()

	configPath := flag.String("cfg", ".", "config path")
	flag.Parse()

	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatal("cannot load cfg:", err)
	}

	googleAiClient, err := gemini.NewGeminiClient(ctx, cfg.GeminiAPIKey)
	if err != nil {
		log.Fatal("Error creating google ai client: ", err)
	}

	tg, _ := telegram.NewProtectBot(cfg.BotToken, cfg.BotSettings, googleAiClient)
	tg.StartBot()
}
