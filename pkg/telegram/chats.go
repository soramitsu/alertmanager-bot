package telegram

import (
	"encoding/json"
	"fmt"

	"github.com/docker/libkv/store"
	"github.com/tucnak/telebot"
)

const telegramChatsDirectory = "telegram/chats"

// ChatStore writes the users to a libkv store backend
type ChatStore struct {
	kv store.Store
}

// NewChatStore stores telegram chats in the provided kv backend
func NewChatStore(kv store.Store) (*ChatStore, error) {
	return &ChatStore{kv: kv}, nil
}

// List all chats saved in the kv backend
func (s *ChatStore) List() ([]ChatInfo, error) {
	kvPairs, err := s.kv.List(telegramChatsDirectory)
	if err != nil {
		return nil, err
	}

	var chatInfos []ChatInfo

	for _, kv := range kvPairs {
		var chatInfo ChatInfo
		if err := json.Unmarshal(kv.Value, &chatInfo); err != nil {
			return nil, err
		}
		chatInfos = append(chatInfos, chatInfo)
	}
	return chatInfos, nil
}

func (s *ChatStore) AddChat(c telebot.Chat, allEnvs []string, allPrs []string) error {
	newChat := ChatInfo{Chat: c,  AlertEnvironments: allEnvs, AlertProjects: allPrs,
		MutedEnvironments: []string{}, MutedProjects: []string{}}
	info, err := json.Marshal(newChat)
	if err != nil {
		return err
	}
	key := fmt.Sprintf("%s/%d", telegramChatsDirectory, c.ID)
	return s.kv.Put(key, info, nil)
}

func (s *ChatStore) GetChatInfo(c telebot.Chat) (ChatInfo, error) {
	key := fmt.Sprintf("%s/%d", telegramChatsDirectory, c.ID)
	kvPairs, err := s.kv.Get(key)
	if err != nil {
		return ChatInfo{}, err
	}

	var chatInfo ChatInfo
	if err = json.Unmarshal(kvPairs.Value, &chatInfo); err != nil {
		return ChatInfo{}, err
	}
	return chatInfo, nil
}

func (s *ChatStore) RemoveChat(c telebot.Chat) error {
	key := fmt.Sprintf("%s/%d", telegramChatsDirectory, c.ID)
	return s.kv.Delete(key)
}

func (s *ChatStore) MuteEnvironments(c telebot.Chat, envsToMute []string, allEnvs []string) error {
	key := fmt.Sprintf("%s/%d", telegramChatsDirectory, c.ID)
	kvPairs, err := s.kv.Get(key)
	if err != nil {
		return err
	}

	var chatInfo ChatInfo
	if err = json.Unmarshal(kvPairs.Value, &chatInfo); err != nil {
		return err
	}
	chatInfo.MuteEnvironments(envsToMute, allEnvs)
	updated, err := json.Marshal(chatInfo)
	if err != nil {
		return err
	}
	return s.kv.Put(key, updated, nil)
}

func (s *ChatStore) MuteProjects(c telebot.Chat, prsToMute []string, allPrs []string) error {
	key := fmt.Sprintf("%s/%d", telegramChatsDirectory, c.ID)
	kvPairs, err := s.kv.Get(key)
	if err != nil {
		return err
	}

	var chatInfo *ChatInfo
	if err = json.Unmarshal(kvPairs.Value, &chatInfo); err != nil {
		return err
	}
	chatInfo.MuteProjects(prsToMute, allPrs)
	updated, err := json.Marshal(chatInfo)
	if err != nil {
		return err
	}
	return s.kv.Put(key, updated, nil)
}

func (s *ChatStore) UnmuteEnvironment(c telebot.Chat, envToUnmute string, allEnvs []string) error {
	key := fmt.Sprintf("%s/%d", telegramChatsDirectory, c.ID)
	kvPairs, err := s.kv.Get(key)
	if err != nil {
		return err
	}

	var chatInfo ChatInfo
	if err = json.Unmarshal(kvPairs.Value, &chatInfo); err != nil {
		return err
	}
	chatInfo.UnmuteEnvironment(envToUnmute, allEnvs)
	updated, err := json.Marshal(chatInfo)
	if err != nil {
		return err
	}
	return s.kv.Put(key, updated, nil)
}

func (s *ChatStore) UnmuteProject(c telebot.Chat, prToUnmute string, allPrs []string) error {
	key := fmt.Sprintf("%s/%d", telegramChatsDirectory, c.ID)
	kvPairs, err := s.kv.Get(key)
	if err != nil {
		return err
	}

	var chatInfo ChatInfo
	if err = json.Unmarshal(kvPairs.Value, &chatInfo); err != nil {
		return err
	}
	chatInfo.UnmuteProject(prToUnmute, allPrs)
	updated, err := json.Marshal(chatInfo)
	if err != nil {
		return err
	}
	return s.kv.Put(key, updated, nil)
}

func (s *ChatStore) MutedEnvironments(c telebot.Chat) ([]string, error) {
	key := fmt.Sprintf("%s/%d", telegramChatsDirectory, c.ID)
	kvPairs, err := s.kv.Get(key)
	if err != nil {
		return nil, err
	}

	var chatInfo ChatInfo
	if err = json.Unmarshal(kvPairs.Value, &chatInfo); err != nil {
		return nil, err
	}
	return chatInfo.MutedEnvironments, nil
}

func (s *ChatStore) MutedProjects(c telebot.Chat) ([]string, error) {
	key := fmt.Sprintf("%s/%d", telegramChatsDirectory, c.ID)
	kvPairs, err := s.kv.Get(key)
	if err != nil {
		return nil, err
	}

	var chatInfo ChatInfo
	if err = json.Unmarshal(kvPairs.Value, &chatInfo); err != nil {
		return nil, err
	}
	return chatInfo.MutedProjects, nil
}