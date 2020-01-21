package telegram

import (
	"github.com/docker/libkv/store"
	"github.com/docker/libkv/store/boltdb"
	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/stretchr/testify/assert"
	"github.com/tucnak/telebot"
	"os"
	"testing"
)

var bot *Bot

func TestMain(m *testing.M) {
	logger := log.NewLogfmtLogger(log.NewSyncWriter(os.Stderr))
	var kvStore store.Store
	{
		var err error
		kvStore, err  = boltdb.New([]string{"/tmp/bot.db"}, &store.Config{Bucket: "alertmanager"})
		if err != nil {
			level.Error(logger).Log("msg", "failed to create bolt store backend", "err", err)
		}
	}
	defer kvStore.Close()

	chats, err := NewChatStore(kvStore)
	if err != nil {
		level.Error(logger).Log("msg", "failed to create chat store", "err", err)
		os.Exit(1)
	}

	bot = &Bot{chats:chats}

	if err != nil {
		level.Error(logger).Log("msg", "failed to create bot", "err", err)
		os.Exit(2)
	}
	code := m.Run()
	os.Exit(code)
}

func TestAddUserToNewProject(t *testing.T) {
	chat := telebot.Chat{ID:234223424, Username: "kgusman"}
	project := "iroha"
	err := bot.chats.AddUserToProject(chat, project)
	assert.Nil(t, err)
}

func TestAddUserToExistingProject(t *testing.T) {
	chat := telebot.Chat{ID:234223424, Username: "kgusman"}
	project := "iroha"
	err := bot.chats.AddUserToProject(chat, project)
	assert.Nil(t, err)

	chat = telebot.Chat{ID:234224, Username: "someone"}
	err = bot.chats.AddUserToProject(chat, project)
	assert.Nil(t, err)
}

func TestAddUserToEnvironment(t *testing.T) {
	chat := telebot.Chat{ID:234223424, Username: "kgusman"}
	environment := "stage1"
	err := bot.chats.AddUserToEnvironment(chat, environment)
	assert.Nil(t, err)
}

func TestAddUserToExistingEnvironment(t *testing.T) {
	chat := telebot.Chat{ID:234223424, Username: "kgusman"}
	environment := "stage1"
	err := bot.chats.AddUserToEnvironment(chat, environment)
	assert.Nil(t, err)

	chat = telebot.Chat{ID:23243424, Username: "someone"}
	err = bot.chats.AddUserToEnvironment(chat, environment)
	assert.Nil(t, err)
}