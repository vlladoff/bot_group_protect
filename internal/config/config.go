package config

import "github.com/spf13/viper"

type (
	Settings struct {
		BotToken    string `mapstructure:"TG_BOT_GROUP_PROTECT_TOKEN"`
		BotSettings `mapstructure:",squash"`
	}
	BotSettings struct {
		WelcomeMessage string `mapstructure:"TG_BOT_GROUP_PROTECT_WELCOME_MESSAGE"`
		SuccessMessage string `mapstructure:"TG_BOT_GROUP_PROTECT_SUCCESS_MESSAGE"`
		ChallengeTime  int32  `mapstructure:"TG_BOT_GROUP_PROTECT_CHALLENGE_TIME"`
		BanTime        int64  `mapstructure:"TG_BOT_GROUP_PROTECT_BAN_TIME"` // 0 - forever
		AdminChatId    int64  `mapstructure:"TG_BOT_GROUP_PROTECT_ADMIN_CHAT_ID"`
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
