package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/z-wentao/voiceflow/pkg/models"
)

// RedisJobStore Redis 任务存储
// 面试亮点：使用 Redis 实现持久化存储，支持分布式部署
type RedisJobStore struct {
	client *redis.Client
	ttl    time.Duration // 数据过期时间
	ctx    context.Context
}

// NewRedisJobStore 创建 Redis 任务存储
func NewRedisJobStore(addr, password string, db int, ttl time.Duration) (*RedisJobStore, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,     // Redis 地址，如 "localhost:6379"
		Password: password, // 密码，无密码留空
		DB:       db,       // 数据库编号，默认 0
	})

	// 测试连接
	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("连接 Redis 失败: %w", err)
	}

	return &RedisJobStore{
		client: client,
		ttl:    ttl,
		ctx:    ctx,
	}, nil
}

// getKey 生成 Redis key
// 格式: "voiceflow:job:{jobID}"
func (rs *RedisJobStore) getKey(jobID string) string {
	return fmt.Sprintf("voiceflow:job:%s", jobID)
}

// Save 保存任务到 Redis
func (rs *RedisJobStore) Save(job *models.TranscriptionJob) error {
	// 1. 序列化为 JSON
	data, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("序列化任务失败: %w", err)
	}

	// 2. 保存到 Redis，设置过期时间
	key := rs.getKey(job.JobID)
	if err := rs.client.Set(rs.ctx, key, data, rs.ttl).Err(); err != nil {
		return fmt.Errorf("保存到 Redis 失败: %w", err)
	}

	// 3. 将 JobID 加入索引集合（用于 List 操作）
	// 使用 Sorted Set，score 为创建时间戳
	indexKey := "voiceflow:jobs:index"
	score := float64(job.CreatedAt.Unix())
	if err := rs.client.ZAdd(rs.ctx, indexKey, redis.Z{
		Score:  score,
		Member: job.JobID,
	}).Err(); err != nil {
		return fmt.Errorf("添加到索引失败: %w", err)
	}

	return nil
}

// Get 从 Redis 获取任务
func (rs *RedisJobStore) Get(jobID string) (*models.TranscriptionJob, error) {
	key := rs.getKey(jobID)

	// 1. 从 Redis 获取数据
	data, err := rs.client.Get(rs.ctx, key).Bytes()
	if err == redis.Nil {
		return nil, fmt.Errorf("任务不存在: %s", jobID)
	}
	if err != nil {
		return nil, fmt.Errorf("从 Redis 获取失败: %w", err)
	}

	// 2. 反序列化
	var job models.TranscriptionJob
	if err := json.Unmarshal(data, &job); err != nil {
		return nil, fmt.Errorf("反序列化任务失败: %w", err)
	}

	return &job, nil
}

// Update 更新任务
func (rs *RedisJobStore) Update(jobID string, updateFn func(*models.TranscriptionJob)) error {
	// 1. 获取现有任务
	job, err := rs.Get(jobID)
	if err != nil {
		return err
	}

	// 2. 执行更新函数
	updateFn(job)

	// 3. 保存回 Redis
	return rs.Save(job)
}

// List 列出所有任务
func (rs *RedisJobStore) List() ([]*models.TranscriptionJob, error) {
	indexKey := "voiceflow:jobs:index"

	// 1. 从索引获取所有 JobID（按时间倒序）
	jobIDs, err := rs.client.ZRevRange(rs.ctx, indexKey, 0, -1).Result()
	if err != nil {
		return nil, fmt.Errorf("获取任务索引失败: %w", err)
	}

	// 2. 批量获取任务详情
	jobs := make([]*models.TranscriptionJob, 0, len(jobIDs))
	for _, jobID := range jobIDs {
		job, err := rs.Get(jobID)
		if err != nil {
			// 任务可能已过期，跳过
			// 同时从索引中删除
			rs.client.ZRem(rs.ctx, indexKey, jobID)
			continue
		}
		jobs = append(jobs, job)
	}

	return jobs, nil
}

// Delete 删除任务
func (rs *RedisJobStore) Delete(jobID string) error {
	key := rs.getKey(jobID)
	indexKey := "voiceflow:jobs:index"

	// 1. 删除任务数据
	deleted, err := rs.client.Del(rs.ctx, key).Result()
	if err != nil {
		return fmt.Errorf("删除任务失败: %w", err)
	}

	if deleted == 0 {
		return fmt.Errorf("任务不存在: %s", jobID)
	}

	// 2. 从索引中删除
	rs.client.ZRem(rs.ctx, indexKey, jobID)

	return nil
}

// Close 关闭 Redis 连接
func (rs *RedisJobStore) Close() error {
	return rs.client.Close()
}

// CleanExpiredJobs 清理过期的任务索引（可选的维护方法）
// 这个方法可以定期调用，清理索引中已过期的任务
func (rs *RedisJobStore) CleanExpiredJobs() error {
	indexKey := "voiceflow:jobs:index"

	// 获取所有 JobID
	jobIDs, err := rs.client.ZRange(rs.ctx, indexKey, 0, -1).Result()
	if err != nil {
		return err
	}

	// 检查每个任务是否存在
	for _, jobID := range jobIDs {
		key := rs.getKey(jobID)
		exists, err := rs.client.Exists(rs.ctx, key).Result()
		if err != nil {
			continue
		}

		// 如果任务不存在，从索引中删除
		if exists == 0 {
			rs.client.ZRem(rs.ctx, indexKey, jobID)
		}
	}

	return nil
}
