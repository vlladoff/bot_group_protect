package main

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/vlladoff/bot_group_protect/internal/config"
	"github.com/vlladoff/bot_group_protect/internal/telegram"
	"log"
)

func main() {
	config, err := config.LoadConfig(".")
	if err != nil {
		log.Fatal("cannot load config:", err)
	}

	var tg telegram.ProtectBot
	bot, err := tgbotapi.NewBotAPI(config.BotToken)
	if err != nil {
		log.Panic(err)
	}

	tg.Client = bot
	tg.Settings = config.BotSettings

	tg.StartBot()
}
