package database

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/enterprise/android-remote-access/server/internal/models"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

// RedisCache provides Redis caching functionality
type RedisCache struct {
	client *redis.Client
}

// NewRedisCache creates a new Redis cache connection
func NewRedisCache(addr, password string, db int) (*RedisCache, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &RedisCache{client: client}, nil
}

// Close closes the Redis connection
func (r *RedisCache) Close() error {
	return r.client.Close()
}

// DeviceRegistry manages device connection state in Redis

const (
	deviceOnlineKey    = "device:online:%s"    // device UUID -> DeviceInfo JSON
	deviceLastSeenKey  = "device:lastseen:%s"   // device UUID -> timestamp
	deviceCommandQueue = "device:commands:%s"    // device UUID -> sorted set of pending commands
	onlineDevicesSet   = "devices:online"       // set of all online device UUIDs
	commandResponseKey = "command:response:%s"  // transaction ID -> response JSON
)

// SetDeviceOnline marks a device as online
func (r *RedisCache) SetDeviceOnline(ctx context.Context, device *models.Device) error {
	data, err := json.Marshal(device)
	if err != nil {
		return err
	}

	pipe := r.client.Pipeline()
	pipe.Set(ctx, fmt.Sprintf(deviceOnlineKey, device.ID), data, 24*time.Hour)
	pipe.Set(ctx, fmt.Sprintf(deviceLastSeenKey, device.ID), time.Now().Unix(), 24*time.Hour)
	pipe.SAdd(ctx, onlineDevicesSet, device.ID.String())
	pipe.Expire(ctx, onlineDevicesSet, 24*time.Hour)

	_, err = pipe.Exec(ctx)
	return err
}

// SetDeviceOffline marks a device as offline
func (r *RedisCache) SetDeviceOffline(ctx context.Context, deviceID uuid.UUID) error {
	pipe := r.client.Pipeline()
	pipe.Del(ctx, fmt.Sprintf(deviceOnlineKey, deviceID))
	pipe.Del(ctx, fmt.Sprintf(deviceLastSeenKey, deviceID))
	pipe.SRem(ctx, onlineDevicesSet, deviceID.String())
	_, err := pipe.Exec(ctx)
	return err
}

// GetOnlineDevice retrieves online device info
func (r *RedisCache) GetOnlineDevice(ctx context.Context, deviceID uuid.UUID) (*models.Device, error) {
	data, err := r.client.Get(ctx, fmt.Sprintf(deviceOnlineKey, deviceID)).Bytes()
	if err != nil {
		return nil, err
	}

	var device models.Device
	if err := json.Unmarshal(data, &device); err != nil {
		return nil, err
	}
	return &device, nil
}

// GetOnlineDevices returns all online device UUIDs
func (r *RedisCache) GetOnlineDevices(ctx context.Context) ([]uuid.UUID, error) {
	members, err := r.client.SMembers(ctx, onlineDevicesSet).Result()
	if err != nil {
		return nil, err
	}

	ids := make([]uuid.UUID, 0, len(members))
	for _, m := range members {
		id, err := uuid.Parse(m)
		if err != nil {
			continue
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// IsDeviceOnline checks if a device is online
func (r *RedisCache) IsDeviceOnline(ctx context.Context, deviceID uuid.UUID) (bool, error) {
	return r.client.SIsMember(ctx, onlineDevicesSet, deviceID.String()).Result()
}

// UpdateDeviceHeartbeat updates device last seen timestamp
func (r *RedisCache) UpdateDeviceHeartbeat(ctx context.Context, deviceID uuid.UUID, batteryLevel int) error {
	pipe := r.client.Pipeline()
	pipe.Set(ctx, fmt.Sprintf(deviceLastSeenKey, deviceID), time.Now().Unix(), 24*time.Hour)

	// Update battery level in device data
	data, err := r.client.Get(ctx, fmt.Sprintf(deviceOnlineKey, deviceID)).Bytes()
	if err == nil {
		var device models.Device
		if err := json.Unmarshal(data, &device); err == nil {
			device.BatteryLevel = batteryLevel
			device.LastCheckIn = time.Now()
			newData, _ := json.Marshal(&device)
			pipe.Set(ctx, fmt.Sprintf(deviceOnlineKey, deviceID), newData, 24*time.Hour)
		}
	}

	_, err = pipe.Exec(ctx)
	return err
}

// Command queue operations

// EnqueueCommand adds a command to device's pending queue
func (r *RedisCache) EnqueueCommand(ctx context.Context, deviceID uuid.UUID, cmd *models.PendingCommand) error {
	data, err := json.Marshal(cmd)
	if err != nil {
		return err
	}

	// Score is timestamp for ordering, plus priority boost
	score := float64(cmd.CreatedAt.Unix()) + float64(cmd.Priority)
	return r.client.ZAdd(ctx, fmt.Sprintf(deviceCommandQueue, deviceID), redis.Z{
		Score:  score,
		Member: string(data),
	}).Err()
}

// DequeueCommand retrieves and removes the next command for a device
func (r *RedisCache) DequeueCommand(ctx context.Context, deviceID uuid.UUID) (*models.PendingCommand, error) {
	results, err := r.client.ZRange(ctx, fmt.Sprintf(deviceCommandQueue, deviceID), 0, 0).Result()
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, nil
	}

	var cmd models.PendingCommand
	if err := json.Unmarshal([]byte(results[0]), &cmd); err != nil {
		return nil, err
	}

	r.client.ZRem(ctx, fmt.Sprintf(deviceCommandQueue, deviceID), results[0])
	return &cmd, nil
}

// GetQueuedCommandCount returns the number of pending commands for a device
func (r *RedisCache) GetQueuedCommandCount(ctx context.Context, deviceID uuid.UUID) (int64, error) {
	return r.client.ZCard(ctx, fmt.Sprintf(deviceCommandQueue, deviceID)).Result()
}

// StoreCommandResponse stores a command response temporarily
func (r *RedisCache) StoreCommandResponse(ctx context.Context, transactionID uuid.UUID, response *models.AgentResponse, ttl time.Duration) error {
	data, err := json.Marshal(response)
	if err != nil {
		return err
	}
	return r.client.Set(ctx, fmt.Sprintf(commandResponseKey, transactionID), data, ttl).Err()
}

// GetCommandResponse retrieves a stored command response
func (r *RedisCache) GetCommandResponse(ctx context.Context, transactionID uuid.UUID) (*models.AgentResponse, error) {
	data, err := r.client.Get(ctx, fmt.Sprintf(commandResponseKey, transactionID)).Bytes()
	if err != nil {
		return nil, err
	}

	var response models.AgentResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// DeleteCommandResponse removes a stored response
func (r *RedisCache) DeleteCommandResponse(ctx context.Context, transactionID uuid.UUID) error {
	return r.client.Del(ctx, fmt.Sprintf(commandResponseKey, transactionID)).Err()
}

// Rate limiting

const rateLimitKey = "ratelimit:%s:%s" // type:id

// CheckRateLimit checks and increments rate limit counter
// Returns true if request is allowed, false if rate limited
func (r *RedisCache) CheckRateLimit(ctx context.Context, limitType, identifier string, limit int, window time.Duration) (bool, error) {
	key := fmt.Sprintf(rateLimitKey, limitType, identifier)

	pipe := r.client.Pipeline()
	incr := pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, window)
	_, err := pipe.Exec(ctx)
	if err != nil {
		return false, err
	}

	count := incr.Val()
	return count <= int64(limit), nil
}

// Session caching

const sessionCacheKey = "session:%s" // session ID -> admin info JSON

// CacheSession caches session data
func (r *RedisCache) CacheSession(ctx context.Context, sessionID string, adminID uuid.UUID, ttl time.Duration) error {
	return r.client.Set(ctx, fmt.Sprintf(sessionCacheKey, sessionID), adminID.String(), ttl).Err()
}

// GetCachedSession retrieves cached session admin ID
func (r *RedisCache) GetCachedSession(ctx context.Context, sessionID string) (uuid.UUID, error) {
	result, err := r.client.Get(ctx, fmt.Sprintf(sessionCacheKey, sessionID)).Result()
	if err != nil {
		return uuid.Nil, err
	}
	return uuid.Parse(result)
}

// InvalidateSession removes cached session
func (r *RedisCache) InvalidateSession(ctx context.Context, sessionID string) error {
	return r.client.Del(ctx, fmt.Sprintf(sessionCacheKey, sessionID)).Err()
}
