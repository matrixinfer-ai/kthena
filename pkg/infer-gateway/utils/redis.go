package utils

import (
	"context"

	"github.com/redis/go-redis/v9"
	"k8s.io/klog/v2"
)

func GetRedisClient() *redis.Client {
	redisHost := LoadEnv("REDIS_HOST", "localhost")
	redisPort := LoadEnv("REDIS_PORT", "6379")
	redisPassword := LoadEnv("REDIS_PASSWORD", "")
	client := redis.NewClient(&redis.Options{
		Addr:     redisHost + ":" + redisPort,
		Password: redisPassword,
		DB:       0,
	})
	pong, err := client.Ping(context.Background()).Result()
	if err != nil {
		klog.Fatalf("Error connecting to Redis: %v", err)
	}
	klog.Infof("Connected to Redis: %s", pong)
	return client
}
