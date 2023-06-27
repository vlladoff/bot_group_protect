package main

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/vlladoff/bot_group_protect/internal/config"
	"github.com/vlladoff/bot_group_protect/internal/telegram"
	"log"
)

func main() {
	cfg, err := config.LoadConfig(".")
	if err != nil {
		cfg, err = config.LoadFromEnv()
		if err != nil {
			log.Fatal("cannot load cfg:", err)
		}
	}

	var tg telegram.ProtectBot
	bot, err := tgbotapi.NewBotAPI(cfg.BotToken)
	if err != nil {
		log.Panic(err)
	}

	tg.Client = bot
	tg.Settings = cfg.BotSettings

	tg.StartBot()
}
