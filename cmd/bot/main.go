package main

import (
	"flag"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/vlladoff/bot_group_protect/internal/config"
	"github.com/vlladoff/bot_group_protect/internal/telegram"
	"log"
)

func main() {
	configPath := flag.String("cfg", ".", "config path")
	flag.Parse()

	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatal("cannot load cfg:", err)
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
