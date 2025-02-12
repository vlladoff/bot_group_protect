package telegram

import (
	"encoding/json"
	"log"
	"math/rand"
	"os"
	"strconv"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/vlladoff/bot_group_protect/internal/config"
)

type (
	ProtectBot struct {
		Client                 *tgbotapi.BotAPI
		Settings               config.BotSettings
		WelcomeMessageIds      map[int]int64
		LastWelcomeMessageTime int64
		NewUsers               map[int64]*User
		EnabledChats           map[int64]bool
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
		Attempts         int
	}
)

func NewProtectBot(botToken string, settings config.BotSettings) (*ProtectBot, error) {
	client, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		return nil, err
	}

	pb := &ProtectBot{
		Client:            client,
		Settings:          settings,
		WelcomeMessageIds: make(map[int]int64),
		NewUsers:          make(map[int64]*User),
		EnabledChats:      make(map[int64]bool),
	}

	pb.loadEnabledChats()

	return pb, nil
}

func (pb *ProtectBot) saveEnabledChats(data map[int64]bool) {
	file, err := os.Create(pb.Settings.EnableChatsFilePath)
	if err != nil {
		log.Printf("Save error: %v", err)
		return
	}
	defer file.Close()

	if err := json.NewEncoder(file).Encode(data); err != nil {
		log.Printf("Encode error: %v", err)
	}
}

func (pb *ProtectBot) loadEnabledChats() {
	pb.Mu.Lock()
	defer pb.Mu.Unlock()

	file, err := os.Open(pb.Settings.EnableChatsFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		log.Printf("Error loading chats: %v", err)
		return
	}
	defer file.Close()

	if err := json.NewDecoder(file).Decode(&pb.EnabledChats); err != nil {
		log.Printf("Error decoding chats: %v", err)
	}
}

func (pb *ProtectBot) StartBot() {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := pb.Client.GetUpdatesChan(u)
	for update := range updates {
		pb.Update(update)
	}
}

func (pb *ProtectBot) Update(update tgbotapi.Update) {
	if update.Message != nil {
		if update.Message.IsCommand() {
			pb.handleCommand(update)
			return
		}

		if !pb.IsChatEnabled(update.Message.Chat.ID) {
			return
		}

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
			pb.handleNewMembers(update)
			return
		}

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
		pb.checkUserAnswer(update)
	}
}

func (pb *ProtectBot) handleCommand(update tgbotapi.Update) {
	command := update.Message.Command()
	chatID := update.Message.Chat.ID
	userID := update.Message.From.ID

	switch command {
	case "enable":
		if pb.isUserAdmin(chatID, userID) {
			pb.setChatEnabled(chatID, true)
			pb.sendMessage(chatID, "Бот активирован ✅", update.Message.MessageID)
		}
	case "disable":
		if pb.isUserAdmin(chatID, userID) {
			pb.setChatEnabled(chatID, false)
			pb.sendMessage(chatID, "Бот деактивирован 🛑", update.Message.MessageID)
		}
	}
}

func (pb *ProtectBot) setChatEnabled(chatID int64, enabled bool) {
	pb.Mu.Lock()
	pb.EnabledChats[chatID] = enabled

	dataCopy := make(map[int64]bool)
	for k, v := range pb.EnabledChats {
		dataCopy[k] = v
	}
	pb.Mu.Unlock()

	go func(data map[int64]bool) {
		pb.saveEnabledChats(data)
	}(dataCopy)
}

func (pb *ProtectBot) IsChatEnabled(chatID int64) bool {
	pb.Mu.Lock()
	defer pb.Mu.Unlock()
	return pb.EnabledChats[chatID]
}

func (pb *ProtectBot) sendMessage(chatID int64, text string, replyMessageID int) int {
	msg := tgbotapi.NewMessage(chatID, text)

	if replyMessageID != 0 {
		msg.ReplyToMessageID = replyMessageID
	}

	resp, _ := pb.Client.Send(msg)

	if resp.MessageID != 0 {
		return resp.MessageID
	}

	return 0
}

func (pb *ProtectBot) checkUserAnswer(update tgbotapi.Update) {
	pb.Mu.Lock()
	user, exists := pb.NewUsers[update.Message.From.ID]
	pb.Mu.Unlock()

	if !exists {
		return
	}

	user.MessagesToDelete = append(user.MessagesToDelete, update.Message.MessageID)

	if update.Message.Text == user.NeedToAnswer {
		copyUser := *user
		pb.EndChallenge(user)
		pb.ClearUserMessages(user, false)
		pb.SendSuccessMessage(copyUser.ChatId, copyUser.MessagesToDelete[0])
	} else {
		user.Attempts--
		if user.Attempts > 0 {
			msgId := pb.sendMessage(user.ChatId, "Неверный код. У вас осталась одна попытка.", update.Message.MessageID)
			pb.DeleteMessageById(user.ChatId, update.Message.MessageID)
			user.MessagesToDelete = append(user.MessagesToDelete, msgId)
		} else {
			pb.WaitAndBan(0, user)
		}
	}
}

func (pb *ProtectBot) handleNewMembers(update tgbotapi.Update) {
	if pb.isUserAdmin(update.Message.Chat.ID, update.Message.From.ID) {
		return
	}

	for _, member := range update.Message.NewChatMembers {
		newUser := pb.StartChallenge(update)
		pb.Mu.Lock()
		pb.NewUsers[member.ID] = newUser
		pb.Mu.Unlock()
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

	pb.UploadWelcomeGif()
	gifMsg := tgbotapi.NewAnimation(update.Message.Chat.ID, tgbotapi.FileID(pb.Settings.WelcomeGifId))
	gifResp, _ := pb.Client.Send(gifMsg)

	cancelBan := false
	newUser := User{
		NeedToAnswer: verifyCode,
		ChatId:       update.Message.Chat.ID,
		UserId:       update.Message.From.ID,
		UserName:     update.Message.From.FirstName + " " + update.Message.From.LastName,
		UserNickName: update.Message.From.UserName,
		CancelBan:    &cancelBan,
		Attempts:     2,
	}

	// joined message
	newUser.MessagesToDelete = append(newUser.MessagesToDelete, update.Message.MessageID)
	newUser.MessagesToDelete = append(newUser.MessagesToDelete, resp.MessageID)
	newUser.MessagesToDelete = append(newUser.MessagesToDelete, gifResp.MessageID)

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

func (pb *ProtectBot) DeleteMessageById(chatId int64, msgId int) {
	pb.Client.Request(tgbotapi.NewDeleteMessage(chatId, msgId))
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

func (pb *ProtectBot) isUserAdmin(chatID int64, userID int64) bool {
	member, err := pb.Client.GetChatMember(tgbotapi.GetChatMemberConfig{
		ChatConfigWithUser: tgbotapi.ChatConfigWithUser{
			ChatID: chatID,
			UserID: userID,
		},
	})

	return err == nil && (member.IsAdministrator() || member.IsCreator() || pb.Settings.AdminId == userID)
}

func (pb *ProtectBot) CleanBotMessages() {
	if len(pb.WelcomeMessageIds) > 1 {
		for messageId, chatId := range pb.WelcomeMessageIds {
			go pb.Client.Request(tgbotapi.NewDeleteMessage(chatId, messageId))
		}
	}
}

func (pb *ProtectBot) UploadWelcomeGif() {
	if pb.Settings.WelcomeGifId == "" {
		if pb.Settings.WelcomeGifPath != "" {
			gifId, _ := pb.UploadGif(pb.Settings.WelcomeGifPath)
			if gifId != "" {
				pb.Settings.WelcomeGifId = gifId
			}
		}
	}
}

func (pb *ProtectBot) UploadGif(filePath string) (string, error) {
	file := tgbotapi.FilePath(filePath)
	gifConfig := tgbotapi.NewAnimation(pb.Settings.AdminChatId, file)
	resp, err := pb.Client.Send(gifConfig)
	if err != nil {
		return "", err
	}

	pb.SendMessageToAdmin("Id welcome файла gif: " + resp.Animation.FileID + "\nДобавьте его в конфигурационный файл ")

	return resp.Animation.FileID, nil
}
