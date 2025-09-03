/*
Copyright MatrixInfer-AI Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package utils

import (
	"context"
	"sync"

	"github.com/redis/go-redis/v9"
	"k8s.io/klog/v2"
)

var (
	redisClient *redis.Client
	redisOnce   sync.Once
)

func GetRedisClient() *redis.Client {
	redisOnce.Do(func() {
		redisHost := LoadEnv("REDIS_HOST", "redis-server")
		redisPort := LoadEnv("REDIS_PORT", "6379")
		redisPassword := LoadEnv("REDIS_PASSWORD", "")
		redisClient = redis.NewClient(&redis.Options{
			Addr:     redisHost + ":" + redisPort,
			Password: redisPassword,
			DB:       0,
		})
		pong, err := redisClient.Ping(context.Background()).Result()
		if err != nil {
			klog.Fatalf("Error connecting to Redis: %v", err)
		}
		klog.Infof("Connected to Redis: %s", pong)
	})
	return redisClient
}
