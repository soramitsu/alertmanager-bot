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

func (s *ChatStore) RemoveUserFromProject(c telebot.Chat, projectName string) error {
	key := fmt.Sprintf("%s/%s", telegramProjectsDirectory, projectName)
	return RemoveChat(s, c, key)
}

func (s *ChatStore) RemoveUserFromEnvironment(c telebot.Chat, environmentName string) error {
	key := fmt.Sprintf("%s/%s", telegramEnvironmentsDirectory, environmentName)
	return RemoveChat(s, c, key)
}

func RemoveChat(s *ChatStore, c telebot.Chat, key string) error {
	kvPairs, err := s.kv.Get(key)
	if err != nil {
		if err == store.ErrKeyNotFound {
			return nil
		} else {
			return err
		}
	}

	var chatsInBytes [][]byte
	if err := json.Unmarshal(kvPairs.Value, &chatsInBytes); err != nil {
		return err
	}

	var chatsToSave [][]byte

	for _, chatBytes := range chatsInBytes {
		var chat telebot.Chat
		if err := json.Unmarshal(chatBytes, &chat); err != nil {
			return nil
		}
		if chat.ID != c.ID {
			marshalledChat, err := json.Marshal(chat)
			if err != nil {
				return nil
			}
			chatsToSave = append(chatsToSave, marshalledChat)
		}
	}

	marshalledChats, err := json.Marshal(chatsToSave)
	if err != nil {
		return err
	}

	return s.kv.Put(key, marshalledChats, nil)

}

func (s *ChatStore) GetUsersForProject(projectName string) ([]telebot.Chat, error) {
	key := fmt.Sprintf("%s/%s", telegramProjectsDirectory, projectName)
	return GetChatsForKey(s, key)
}

func (s *ChatStore) GetUsersForEnvironment(environmentName string) ([]telebot.Chat, error) {
	key := fmt.Sprintf("%s/%s", telegramEnvironmentsDirectory, environmentName)
	return GetChatsForKey(s, key)
}

func GetChatsForKey(s *ChatStore, key string) ([]telebot.Chat, error) {
	kvPairs, err := s.kv.Get(key)
	if err != nil {
		if err == store.ErrKeyNotFound {
			return []telebot.Chat{}, nil
		} else {
			return nil, err
		}
	}

	var chatsInBytes [][]byte
	if err := json.Unmarshal(kvPairs.Value, &chatsInBytes); err != nil {
		return nil, err
	}

	var chatsForKey []telebot.Chat

	for _, chatBytes := range chatsInBytes {
		var chat telebot.Chat
		if err := json.Unmarshal(chatBytes, &chat); err != nil {
			return nil, err
		}
		chatsForKey = append(chatsForKey, chat)
	}
	return chatsForKey, nil
}

func (s *ChatStore) AddUserToProject(c telebot.Chat, projectName string) error {
	key := fmt.Sprintf("%s/%s", telegramProjectsDirectory, projectName)
	return AddUserByKey(s, c, key)
}

func (s *ChatStore) AddUserToEnvironment(c telebot.Chat, environmentName string) error {
	key := fmt.Sprintf("%s/%s", telegramEnvironmentsDirectory, environmentName)
	return AddUserByKey(s, c, key)

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