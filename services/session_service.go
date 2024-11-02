package services

import (
	"context"
	"time"

	"github.com/go-redis/redis/v8"
)

type SessionService struct {
    RedisClient *redis.Client
}

func NewSessionService(redisClient *redis.Client) *SessionService {
    return &SessionService{RedisClient: redisClient}
}

// 创建会话
func (s *SessionService) CreateSession(userID string) (string, error) {
    sessionID := generateSessionID() // 生成唯一的sessionID
    err := s.RedisClient.Set(context.Background(), sessionID, userID, 24*time.Hour).Err()
    return sessionID, err
}

// 验证会话
func (s *SessionService) ValidateSession(sessionID string) (string, error) {
    userID, err := s.RedisClient.Get(context.Background(), sessionID).Result()
    return userID, err
}

// 生成唯一的sessionID
func generateSessionID() string {
    // 这里可以使用更复杂的算法生成sessionID
    return "unique-session-id" // 示例
}
