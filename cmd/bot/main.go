package main

import (
	"flag"
	"log"

	"github.com/vlladoff/bot_group_protect/internal/config"
	"github.com/vlladoff/bot_group_protect/internal/telegram"
)

func main() {
	configPath := flag.String("cfg", ".", "config path")
	flag.Parse()

	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatal("cannot load cfg:", err)
	}

	tg, _ := telegram.NewProtectBot(cfg.BotToken, cfg.BotSettings)
	tg.StartBot()
}
