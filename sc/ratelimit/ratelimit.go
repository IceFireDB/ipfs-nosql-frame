/*
 * @Author: gitsrc
 * @Date: 2020-08-19 18:55:12
 * @LastEditors: gitsrc
 * @LastEditTime: 2020-10-15 09:21:49
 * @FilePath: /log-redis-cluster-proxy/sc/ratelimit/ratelimit.go
 */

package ratelimit

import (
	"context"

	"github.com/go-redis/redis/v8"
)

// Implements RedisClient for redis.ClusterClient
type ClusterClient struct {
	*redis.ClusterClient
}

func (c *ClusterClient) RateDel(ctx context.Context, key string) error {
	return c.Del(ctx, key).Err()
}

func (c *ClusterClient) RateEvalSha(ctx context.Context, sha1 string, keys []string, args ...interface{}) (interface{}, error) {
	return c.EvalSha(ctx, sha1, keys, args...).Result()
}

func (c *ClusterClient) RateScriptLoad(ctx context.Context, script string) (string, error) {
	var sha1 string
	res, err := c.ScriptLoad(ctx, script).Result()
	if err == nil {
		sha1 = res
	}
	return sha1, err
}
