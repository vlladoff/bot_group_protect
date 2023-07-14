package telegram

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/vlladoff/bot_group_protect/internal/config"
	"log"
	"math/rand"
	"strconv"
	"time"
)

type (
	ProtectBot struct {
		Client   *tgbotapi.BotAPI
		Settings config.BotSettings
		NewUsers *map[int64]*User
	}
	User struct {
		NeedToAnswer     string
		MessagesToDelete []int
		ChatId           int64
		UserName         string
		UserNickName     string
		UserId           int64
		CancelBan        *bool
	}
)

func (pb *ProtectBot) StartBot() {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	newUsers := make(map[int64]*User)
	pb.NewUsers = &newUsers

	updates := pb.Client.GetUpdatesChan(u)
	for update := range updates {
		if update.Message != nil {
			//new member joined
			if update.Message.NewChatMembers != nil {
				for _, member := range update.Message.NewChatMembers {
					if member.ID == update.Message.From.ID {
						newUser := pb.StartChallenge(update)
						(*pb.NewUsers)[member.ID] = newUser
					}
				}
			}

			//delete banned user message
			if update.Message.LeftChatMember != nil && update.Message.From.UserName == pb.Settings.HimselfUserName {
				go pb.Client.Request(tgbotapi.NewDeleteMessage(update.Message.Chat.ID, update.Message.MessageID))
			}
		}

		//check new member answer
		if update.CallbackQuery != nil {
			if user, ok := (*pb.NewUsers)[update.CallbackQuery.From.ID]; ok {
				if update.CallbackQuery.Data == user.NeedToAnswer {
					copyUser := user
					pb.EndChallenge(user)
					pb.ClearUserMessages(user, false)
					pb.SendSuccessMessage(copyUser.ChatId, copyUser.MessagesToDelete[0])
				} else {
					pb.WaitAndBan(0, user)
				}
			}
		}
	}
}

var emojiMap = map[string]string{
	"смайл":       "🙂",
	"рукопожатие": "🤝",
	"глаза":       "👀",
	"зонтик":      "☂️",
	"очки":        "👓",
	"перчатки":    "🧤",
	"кепка":       "🧢",
	"кольцо":      "💍",
	"носки":       "🧦",
	"мышь":        "🐭",
	"единорог":    "🦄",
	"попугай":     "🦜",
	"фламинго":    "🦩",
	"заяц":        "🐇",
	"слон":        "🐘",
	"бабочка":     "🦋",
	"улитка":      "🐌",
	"муха":        "🪰",
	"дельфин":     "🐬",
	"крокодил":    "🐊",
	"кактус":      "🌵",
	"ель":         "🌲",
	"клевер":      "☘️",
	"цветок":      "🌸",
	"месяц":       "🌙",
	"звезда":      "⭐️",
	"облако":      "☁️",
	"огонь":       "🔥",
	"радуга":      "🌈",
	"снежинка":    "❄️",
	"клубника":    "🍓",
	"банан":       "🍌",
	"яблоко":      "🍏",
	"авокадо":     "🥑",
	"баклажан":    "🍆",
	"мяч":         "⚽️",
	"бумеранг":    "🪃",
	"гитара":      "🎸",
	"велосипед":   "🚲",
	"ракета":      "🚀",
	"палатка":     "⛺️",
	"топор":       "🪓",
	"шарик":       "🎈",
	"стул":        "🪑",
	"скрепка":     "📎",
	"ножницы":     "✂️",
	"карандаш":    "✏️",
	"лупа":        "🔍",
	"сигарета":    "🚬",
	"ключ":        "🔑",
	"сердечко":    "❤️",
}

func (pb *ProtectBot) StartChallenge(update tgbotapi.Update) *User {
	pb.DisallowUserSendMessages(update.Message.Chat.ID, update.Message.From.ID)

	emojiKey, keyboard := GenerateKeyboard()

	msg := tgbotapi.NewMessage(update.Message.Chat.ID, pb.Settings.WelcomeMessage+"***\""+emojiKey+"\"***")
	msg.ParseMode = "markdown"
	msg.ReplyToMessageID = update.Message.MessageID
	msg.ReplyMarkup = keyboard
	resp, _ := pb.Client.Send(msg)

	cancelBan := false
	newUser := User{
		NeedToAnswer: emojiMap[emojiKey],
		ChatId:       update.Message.Chat.ID,
		UserId:       update.Message.From.ID,
		UserName:     update.Message.From.FirstName + " " + update.Message.From.LastName,
		UserNickName: update.Message.From.UserName,
		CancelBan:    &cancelBan,
	}

	//joined message
	newUser.MessagesToDelete = append(newUser.MessagesToDelete, update.Message.MessageID)
	newUser.MessagesToDelete = append(newUser.MessagesToDelete, resp.MessageID)

	go pb.WaitAndBan(pb.Settings.ChallengeTime, &newUser)

	return &newUser
}

func GenerateKeyboard() (string, tgbotapi.InlineKeyboardMarkup) {
	emojiKey := PickRandEmojiKey()
	emojiKeyFake := PickRandEmojiKey()
	emojiKeyFake2 := PickRandEmojiKey()

	var buttons []tgbotapi.InlineKeyboardButton
	buttons = append(buttons, tgbotapi.NewInlineKeyboardButtonData(emojiMap[emojiKey], emojiMap[emojiKey]))
	buttons = append(buttons, tgbotapi.NewInlineKeyboardButtonData(emojiMap[emojiKeyFake], emojiMap[emojiKeyFake]))
	buttons = append(buttons, tgbotapi.NewInlineKeyboardButtonData(emojiMap[emojiKeyFake2], emojiMap[emojiKeyFake2]))

	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(buttons), func(i, j int) { buttons[i], buttons[j] = buttons[j], buttons[i] })

	return emojiKey, tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(buttons...))
}

func PickRandEmojiKey() string {
	k := rand.Intn(len(emojiMap))

	for key := range emojiMap {
		if k == 0 {
			return key
		}
		k--
	}

	return "мяч"
}

func (pb *ProtectBot) EndChallenge(user *User) {
	cancelBan := true
	user.CancelBan = &cancelBan

	pb.AllowUserSendMessages(user.ChatId, user.UserId)
}

func (pb *ProtectBot) WaitAndBan(waitTime int32, user *User) {
	if waitTime != 0 {
		time.Sleep(time.Second * time.Duration(waitTime))
	}

	defer pb.DeleteUser(user)
	defer pb.SendUserStatusToAdmin(user)

	if *user.CancelBan {
		return
	}

	if ok := pb.BanUser(user.ChatId, user.UserId); ok {
		log.Printf("User: %v was banned in chat: %v for: %v minutes", user.UserId, user.ChatId, pb.Settings.BanTime)
	}

	pb.ClearUserMessages(user, true)
}

func (pb *ProtectBot) BanUser(chatId, memberId int64) bool {
	banChatMemberConfig := tgbotapi.BanChatMemberConfig{
		ChatMemberConfig: tgbotapi.ChatMemberConfig{
			ChatID: chatId,
			UserID: memberId,
		},
		RevokeMessages: true,
	}

	if pb.Settings.BanTime != 0 {
		banChatMemberConfig.UntilDate = time.Now().Add(time.Minute * time.Duration(pb.Settings.BanTime)).Unix()
	}

	_, err := pb.Client.Request(banChatMemberConfig)
	if err != nil {
		return false
	}

	return true
}

func (pb *ProtectBot) ClearUserMessages(user *User, banned bool) {
	//skip joined message
	if !banned {
		user.MessagesToDelete = user.MessagesToDelete[1:]
	}

	for _, msgId := range user.MessagesToDelete {
		go pb.Client.Request(tgbotapi.NewDeleteMessage(user.ChatId, msgId))
	}
}

func (pb *ProtectBot) DeleteUser(user *User) {
	if _, ok := (*pb.NewUsers)[user.UserId]; ok {
		delete(*pb.NewUsers, user.UserId)
	}
}

func (pb *ProtectBot) DisallowUserSendMessages(chatId, memberId int64) {
	restrictConfig := tgbotapi.RestrictChatMemberConfig{
		ChatMemberConfig: tgbotapi.ChatMemberConfig{
			ChatID: chatId,
			UserID: memberId,
		},
		Permissions: &tgbotapi.ChatPermissions{
			CanSendMessages:      false,
			CanSendMediaMessages: false,
			CanSendPolls:         false,
			CanSendOtherMessages: false,
		},
	}

	go pb.Client.Request(restrictConfig)
}

func (pb *ProtectBot) AllowUserSendMessages(chatId, memberId int64) {
	restrictConfig := tgbotapi.RestrictChatMemberConfig{
		ChatMemberConfig: tgbotapi.ChatMemberConfig{
			ChatID: chatId,
			UserID: memberId,
		},
		Permissions: &tgbotapi.ChatPermissions{
			CanSendMessages:      true,
			CanSendMediaMessages: true,
			CanSendPolls:         true,
			CanSendOtherMessages: true,
		},
	}

	go pb.Client.Request(restrictConfig)
}

func (pb *ProtectBot) SendUserStatusToAdmin(user *User) {
	msg := "Пользователь " + user.UserName + " @" + user.UserNickName
	if *user.CancelBan {
		msg += " прошёл проверку спама в группе: " + strconv.Itoa(int(user.ChatId))
	} else {
		msg += " был забанен в группе: " + strconv.Itoa(int(user.ChatId))
	}

	pb.SendMessageToAdmin(msg)
}

func (pb *ProtectBot) SendMessageToAdmin(msg string) {
	if pb.Settings.AdminChatId != 0 {
		pb.Client.Send(tgbotapi.NewMessage(pb.Settings.AdminChatId, msg))
	}
}

func (pb *ProtectBot) SendSuccessMessage(chatId int64, replyMessageId int) {
	msg := tgbotapi.NewMessage(chatId, pb.Settings.SuccessMessage)
	msg.ReplyToMessageID = replyMessageId
	pb.Client.Send(tgbotapi.NewMessage(chatId, pb.Settings.SuccessMessage))
}
