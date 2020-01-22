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
	chat := telebot.Chat{ID:234223424, Username: "test"}
	project := "some_project"
	err := bot.chats.AddUserToProject(chat, project)
	assert.Nil(t, err)
}

func TestAddUserToExistingProject(t *testing.T) {
	chat := telebot.Chat{ID:234223424, Username: "test"}
	project := "some_project"
	err := bot.chats.AddUserToProject(chat, project)
	assert.Nil(t, err)

	chat = telebot.Chat{ID:234224, Username: "someone"}
	err = bot.chats.AddUserToProject(chat, project)
	assert.Nil(t, err)
}

func TestGetUserForExistingProject(t *testing.T) {
	projectName := "some_project"
	chat1 := telebot.Chat{ID: 232, Username: "test"}
	err := bot.chats.AddUserToProject(chat1, projectName)
	assert.Nil(t, err)

	chat2 := telebot.Chat{ID:12, Username: "tess"}
	err = bot.chats.AddUserToProject(chat2, projectName)
	assert.Nil(t, err)

	chats, err := bot.chats.GetUsersForProject(projectName)
	assert.Nil(t, err)
	assert.True(t, len(chats) == 2)
	assert.True(t, chats[0].ID == chat1.ID)
	assert.True(t, chats[1].ID == chat2.ID)
}

func TestGetUsersForUnknownProject(t *testing.T) {
	projectName := "awesome_project"
	chats, err := bot.chats.GetUsersForProject(projectName)
	assert.Nil(t, err)
	assert.True(t, len(chats) == 0)
}

func TestRemoveUserFromProject(t *testing.T) {
	projectName := "awesomness_project"
	chat1 := telebot.Chat{ID:12412, Username:"super_user"}
	chat2 := telebot.Chat{ID:654, Username:"useeeer"}

	err := bot.chats.AddUserToProject(chat1, projectName)
	assert.Nil(t, err)

	err = bot.chats.AddUserToProject(chat2, projectName)
	assert.Nil(t, err)

	chats, err := bot.chats.GetUsersForProject(projectName)
	assert.Nil(t, err)
	assert.True(t, len(chats) == 2)

	err = bot.chats.RemoveUserFromProject(chat2, projectName)
	assert.Nil(t, err)

	chats, err = bot.chats.GetUsersForProject(projectName)
	assert.Nil(t, err)
	assert.True(t, len(chats) == 1)
	assert.True(t, chats[0].ID == chat1.ID)

}