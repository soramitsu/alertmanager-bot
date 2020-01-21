package telegram

import (
	"encoding/json"
	"fmt"

	"github.com/docker/libkv/store"
	"github.com/tucnak/telebot"
)

const telegramChatsDirectory = "telegram/chats"
const telegramProjectsDirectory = "telegram/chats/projects"
const telegramEnvironmentsDirectory = "telegram/chats/environments"

// ChatStore writes the users to a libkv store backend
type ChatStore struct {
	kv store.Store
}

// NewChatStore stores telegram chats in the provided kv backend
func NewChatStore(kv store.Store) (*ChatStore, error) {
	return &ChatStore{kv: kv}, nil
}

// List all chats saved in the kv backend
func (s *ChatStore) List() ([]telebot.Chat, error) {
	kvPairs, err := s.kv.List(telegramChatsDirectory)
	if err != nil {
		return nil, err
	}

	var chats []telebot.Chat
	for _, kv := range kvPairs {
		var c telebot.Chat
		if err := json.Unmarshal(kv.Value, &c); err != nil {
			return nil, err
		}
		chats = append(chats, c)
	}

	return chats, nil
}

// Add a telegram chat to the kv backend
func (s *ChatStore) Add(c telebot.Chat) error {
	b, err := json.Marshal(c)
	if err != nil {
		return err
	}

	key := fmt.Sprintf("%s/%d", telegramChatsDirectory, c.ID)

	return s.kv.Put(key, b, nil)
}

// Remove a telegram chat from the kv backend
func (s *ChatStore) Remove(c telebot.Chat) error {
	key := fmt.Sprintf("%s/%d", telegramChatsDirectory, c.ID)
	return s.kv.Delete(key)
}

func (s *ChatStore) AddUserToProject(c telebot.Chat, projectName string) error {
	key := fmt.Sprintf("%s/%s", telegramProjectsDirectory, projectName)
	return AddUserByKey(s, c, key)
}

func (s *ChatStore) AddUserToEnvironment(c telebot.Chat, environmentName string) error {
	key := fmt.Sprintf("%s/%s", telegramEnvironmentsDirectory, environmentName)
	return AddUserByKey(s, c, key)

}

func userExists(c telebot.Chat, chats[][]byte) (bool, error) {
	var userChats []telebot.Chat
	for _, chatInBytes := range chats {
		var chat telebot.Chat
		if err := json.Unmarshal(chatInBytes, &chat); err != nil {
			return false, err
		}
		userChats = append(userChats, chat)
	}
	return chatContains(userChats, c), nil
}

func chatContains(chats []telebot.Chat, chat telebot.Chat) bool {
	for _, c := range chats {
		if c.ID == chat.ID {
			return true
		}
	}
	return false
}

func AddUserByKey(s *ChatStore, c telebot.Chat, key string) error {
	chatToSave, err := json.Marshal(c)
	if err != nil {
		return err
	}
	kvPairs, err := s.kv.Get(key)
	if err != nil {
		if err == store.ErrKeyNotFound {
			chats := [][]byte{chatToSave}
			chatsToSave, err := json.Marshal(chats)
			if err != nil {
				return err
			}
			return s.kv.Put(key, chatsToSave, nil)
		} else {
			return err
		}
	}
	var chatsInBytes [][]byte
	if err := json.Unmarshal(kvPairs.Value, &chatsInBytes); err != nil {
		return err
	}

	if res, _ := userExists(c, chatsInBytes); res  {
		return nil
	}

	chatsInBytes = append(chatsInBytes, chatToSave)

	marshallChats, err := json.Marshal(chatsInBytes)
	if err != nil {
		return nil
	}
	return s.kv.Put(key, marshallChats, nil)
}