package telegram

import (
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/vlladoff/bot_group_protect/internal/config"
	"log"
	"math/rand"
	"strconv"
	"sync"
	"time"
)

type (
	ProtectBot struct {
		Client                 *tgbotapi.BotAPI
		Settings               config.BotSettings
		WelcomeMessageIds      map[int]int64
		LastWelcomeMessageTime int64
		NewUsers               map[int64]*User
		Mu                     sync.Mutex
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
	pb.NewUsers = make(map[int64]*User)
	pb.WelcomeMessageIds = make(map[int]int64)

	updates := pb.Client.GetUpdatesChan(u)
	for update := range updates {
		pb.Update(update)
	}
}

func (pb *ProtectBot) Update(update tgbotapi.Update) {
	if update.Message != nil {
		// bot health check
		if update.Message.Text == "PING" {
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, "PONG")
			msg.ReplyToMessageID = update.Message.MessageID

			pb.Client.Send(msg)
		}

		// clean bot messages
		currentTime := time.Now().UnixNano()
		if currentTime-pb.LastWelcomeMessageTime < pb.Settings.CleanMessagesTime*time.Hour.Nanoseconds() {
			pb.CleanBotMessages()
		}

		// new member joined
		if update.Message.NewChatMembers != nil {
			for _, member := range update.Message.NewChatMembers {
				if member.ID == update.Message.From.ID {
					newUser := pb.StartChallenge(update)
					pb.Mu.Lock()
					pb.NewUsers[member.ID] = newUser
					pb.Mu.Unlock()

					return
				}
			}
		}

		// delete banned user message
		if update.Message.LeftChatMember != nil && update.Message.From.UserName == pb.Settings.HimselfUserName {
			go pb.Client.Request(tgbotapi.NewDeleteMessage(update.Message.Chat.ID, update.Message.MessageID))

			return
		}

		// check new member answer
		var userNeedToCheck *User
		var checkAnswer bool
		pb.Mu.Lock()
		if checkUser, userExist := pb.NewUsers[update.Message.From.ID]; userExist {
			userNeedToCheck = checkUser
			checkAnswer = userExist
		}
		pb.Mu.Unlock()
		if checkAnswer {
			userNeedToCheck.MessagesToDelete = append(userNeedToCheck.MessagesToDelete, update.Message.MessageID)
			if update.Message.Text == userNeedToCheck.NeedToAnswer {
				copyUser := *userNeedToCheck
				pb.EndChallenge(userNeedToCheck)
				pb.ClearUserMessages(userNeedToCheck, false)
				pb.SendSuccessMessage(copyUser.ChatId, copyUser.MessagesToDelete[0])
			} else {
				pb.WaitAndBan(0, userNeedToCheck)
			}
		}
	}
}

func (pb *ProtectBot) StartChallenge(update tgbotapi.Update) *User {
	//pb.DisallowUserSendMessages(update.Message.Chat.ID, update.Message.From.ID)

	verifyCode := getRandomCode(4)

	pb.ChangeGroupDescription(update.Message.Chat.ID, pb.Settings.GroupDescription+"\n\nПроверочный код:\n"+verifyCode+"")

	msg := tgbotapi.NewMessage(update.Message.Chat.ID, pb.Settings.WelcomeMessage)
	msg.ParseMode = "markdown"
	msg.ReplyToMessageID = update.Message.MessageID
	resp, _ := pb.Client.Send(msg)

	cancelBan := false
	newUser := User{
		NeedToAnswer: verifyCode,
		ChatId:       update.Message.Chat.ID,
		UserId:       update.Message.From.ID,
		UserName:     update.Message.From.FirstName + " " + update.Message.From.LastName,
		UserNickName: update.Message.From.UserName,
		CancelBan:    &cancelBan,
	}

	// joined message
	newUser.MessagesToDelete = append(newUser.MessagesToDelete, update.Message.MessageID)
	newUser.MessagesToDelete = append(newUser.MessagesToDelete, resp.MessageID)

	go pb.WaitAndBan(pb.Settings.ChallengeTime, &newUser)

	return &newUser
}

func getRandomCode(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"

	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}

	return string(b)
}

func (pb *ProtectBot) EndChallenge(user *User) {
	cancelBan := true
	user.CancelBan = &cancelBan

	//pb.AllowUserSendMessages(user.ChatId, user.UserId)
}

func (pb *ProtectBot) WaitAndBan(waitTime int32, user *User) {
	if waitTime != 0 {
		time.Sleep(time.Second * time.Duration(waitTime))
	}

	defer pb.ClearUserMap(user)
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
	if !banned {
		user.MessagesToDelete = user.MessagesToDelete[1:]
	}

	for _, msgId := range user.MessagesToDelete {
		go pb.Client.Request(tgbotapi.NewDeleteMessage(user.ChatId, msgId))
	}
}

func (pb *ProtectBot) ClearUserMap(user *User) {
	pb.Mu.Lock()
	defer pb.Mu.Unlock()
	if _, ok := pb.NewUsers[user.UserId]; ok {
		delete(pb.NewUsers, user.UserId)
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

func (pb *ProtectBot) ChangeGroupDescription(chatId int64, text string) {
	descriptionConfig := tgbotapi.SetChatDescriptionConfig{
		ChatID:      chatId,
		Description: text,
	}

	pb.Client.Request(descriptionConfig)
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
	sentMessage, err := pb.Client.Send(msg)
	if err != nil {
		return
	}

	pb.CleanBotMessages()
	pb.WelcomeMessageIds[sentMessage.MessageID] = chatId
	currentTime := time.Now().UnixNano()
	pb.LastWelcomeMessageTime = currentTime
}

func (pb *ProtectBot) CleanBotMessages() {
	if len(pb.WelcomeMessageIds) > 1 {
		for messageId, chatId := range pb.WelcomeMessageIds {
			go pb.Client.Request(tgbotapi.NewDeleteMessage(chatId, messageId))
		}
	}
}
