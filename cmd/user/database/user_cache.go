package user_database

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go_projects/praProject1/cmd/user/model"
	"go_projects/praProject1/pkg/rdb"
)

const (
	userCacheTTL   = 30 * time.Minute
	schoolCacheTTL = 24 * time.Hour
)

func userKey(id int64) string   { return fmt.Sprintf("user:id:%d", id) }
func schoolKey(id int64) string { return fmt.Sprintf("school:id:%d", id) }

// SetUserCache writes a User into Redis.
func SetUserCache(ctx context.Context, u *model.User) error {
	b, err := json.Marshal(u)
	if err != nil {
		return err
	}
	return rdb.RDB.Set(ctx, userKey(u.ID), b, userCacheTTL).Err()
}

// GetUserCache reads a User from Redis; returns nil, nil on cache miss.
func GetUserCache(ctx context.Context, id int64) (*model.User, error) {
	val, err := rdb.RDB.Get(ctx, userKey(id)).Bytes()
	if err != nil {
		return nil, nil // cache miss is not an error
	}
	var u model.User
	if err = json.Unmarshal(val, &u); err != nil {
		return nil, err
	}
	return &u, nil
}

// DelUserCache removes a user from Redis (call after writes).
func DelUserCache(ctx context.Context, id int64) error {
	return rdb.RDB.Del(ctx, userKey(id)).Err()
}

// SetSchoolCache writes a School into Redis.
func SetSchoolCache(ctx context.Context, s *model.School) error {
	b, err := json.Marshal(s)
	if err != nil {
		return err
	}
	return rdb.RDB.Set(ctx, schoolKey(s.ID), b, schoolCacheTTL).Err()
}

// GetSchoolCache reads a School from Redis; returns nil, nil on miss.
func GetSchoolCache(ctx context.Context, id int64) (*model.School, error) {
	val, err := rdb.RDB.Get(ctx, schoolKey(id)).Bytes()
	if err != nil {
		return nil, nil
	}
	var s model.School
	if err = json.Unmarshal(val, &s); err != nil {
		return nil, err
	}
	return &s, nil
}