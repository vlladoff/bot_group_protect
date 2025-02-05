package config

import "github.com/spf13/viper"

type (
	Settings struct {
		BotToken    string `mapstructure:"TG_BOT_GROUP_PROTECT_TOKEN"`
		BotSettings `mapstructure:",squash"`
	}
	BotSettings struct {
		HimselfUserName   string `mapstructure:"TG_BOT_HIMSELF_USER_NAME"`
		WelcomeMessage    string `mapstructure:"TG_BOT_GROUP_PROTECT_WELCOME_MESSAGE"`
		GroupDescription  string `mapstructure:"TG_BOT_GROUP_PROTECT_GROUP_DESCRIPTION"`
		SuccessMessage    string `mapstructure:"TG_BOT_GROUP_PROTECT_SUCCESS_MESSAGE"`
		ChallengeTime     int32  `mapstructure:"TG_BOT_GROUP_PROTECT_CHALLENGE_TIME"`
		CleanMessagesTime int64  `mapstructure:"TG_BOT_GROUP_PROTECT_CLEAN_MESSAGES_TIME"`
		BanTime           int64  `mapstructure:"TG_BOT_GROUP_PROTECT_BAN_TIME"` // 0 - forever
		AdminId           int64  `mapstructure:"TG_BOT_GROUP_PROTECT_ADMIN_ID"`
		AdminChatId       int64  `mapstructure:"TG_BOT_GROUP_PROTECT_ADMIN_CHAT_ID"`
		WelcomeGifId      string `mapstructure:"TG_BOT_GROUP_PROTECT_WELCOME_GIF_ID"`
		WelcomeGifPath    string `mapstructure:"TG_BOT_GROUP_PROTECT_WELCOME_GIF_PATH"`
	}
)

func LoadConfig(path string) (config Settings, err error) {
	viper.AddConfigPath(path)
	viper.SetConfigName("tg_bot_group_protect")
	viper.SetConfigType("env")

	viper.AutomaticEnv()
	err = viper.ReadInConfig()
	if err != nil {
		return
	}

	err = viper.Unmarshal(&config)

	return
}
