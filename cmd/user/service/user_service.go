package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	common_pb "go_projects/praProject1/PB/pb/common_pb"
	user_pb "go_projects/praProject1/PB/pb/user_pb"
	user_database "go_projects/praProject1/cmd/user/database"
	"go_projects/praProject1/cmd/user/model"
	"go_projects/praProject1/config"
	"go_projects/praProject1/pkg/db"
	pkgjwt "go_projects/praProject1/pkg/jwt"
	"go_projects/praProject1/pkg/snowflake"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	otelcodes "go.opentelemetry.io/otel/codes"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const serviceName = "user-service"

type UserServiceServer struct {
	user_pb.UnimplementedUserServiceServer
}

// ─── WeChat Login ─────────────────────────────────────────────────────────────

type wxSessionResp struct {
	OpenID     string `json:"openid"`
	UnionID    string `json:"unionid"`
	SessionKey string `json:"session_key"`
	ErrCode    int    `json:"errcode"`
	ErrMsg     string `json:"errmsg"`
}

func wxCode2Session(code string) (*wxSessionResp, error) {
	cfg := config.Conf.Wechat
	params := url.Values{}
	params.Set("appid", cfg.AppID)
	params.Set("secret", cfg.AppSecret)
	params.Set("js_code", code)
	params.Set("grant_type", "authorization_code")

	apiURL := "https://api.weixin.qq.com/sns/jscode2session?" + params.Encode()
	resp, err := http.Get(apiURL) //nolint:gosec
	if err != nil {
		return nil, fmt.Errorf("wx api request: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var wx wxSessionResp
	if err = json.Unmarshal(body, &wx); err != nil {
		return nil, fmt.Errorf("wx api parse: %w", err)
	}
	if wx.ErrCode != 0 {
		return nil, fmt.Errorf("wx api error %d: %s", wx.ErrCode, wx.ErrMsg)
	}
	return &wx, nil
}

func (s *UserServiceServer) WxLogin(ctx context.Context, req *user_pb.WxLoginRequest) (*user_pb.WxLoginResponse, error) {
	ctx = extractTraceFromMeta(ctx)
	tracer := otel.Tracer(serviceName)
	ctx, span := tracer.Start(ctx, "UserService.WxLogin")
	defer span.End()

	span.SetAttributes(attribute.Bool("wx.login", true))

	// 1. 换取 openid
	wx, err := wxCode2Session(req.GetJsCode())
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, err.Error())
		return nil, err
	}
	span.SetAttributes(attribute.String("wx.openid", wx.OpenID))

	// 2. 查库 / 创建用户
	u, isNewUser, err := user_database.GetOrCreateByOpenID(wx.OpenID, wx.UnionID, "", "")
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, err.Error())
		return nil, err
	}

	// 3. 新用户生成默认昵称和头像
	if isNewUser {
		defaultNickname := GenerateDefaultNickname()
		defaultAvatar := GenerateDefaultAvatarURL(u.ID)

		if err = user_database.UpdateUserInfo(u.ID, defaultNickname, defaultAvatar); err != nil {
			span.RecordError(err)
			// 更新失败不影响登录流程，只记录错误
		} else {
			u.Nickname = defaultNickname
			u.AvatarURL = defaultAvatar
		}
	}

	// 3a. 封禁用户拦截（登录入口）
	if u.Status == model.StatusBanned {
		span.SetAttributes(attribute.Bool("user.banned", true))
		return nil, status.Error(codes.PermissionDenied, "账号已被封禁")
	}

	// 4. 更新 Redis 缓存
	_ = user_database.SetUserCache(ctx, u)

	// 5. 签发双 Token（Access + Refresh；Refresh 仅含 user_id）
	jwtCfg := config.Conf.Jwt
	accessToken, err := pkgjwt.GenerateAccessToken(u.ID, u.SchoolID, int8(u.Role), jwtCfg.AuthKey, jwtCfg.AccessExpireH)
	if err != nil {
		span.RecordError(err)
		return nil, err
	}
	refreshToken, err := pkgjwt.GenerateRefreshToken(u.ID, jwtCfg.AuthKey, jwtCfg.RefreshExpireH)
	if err != nil {
		span.RecordError(err)
		return nil, err
	}

	span.SetAttributes(attribute.Int64("user.id", int64(u.ID)))
	return &user_pb.WxLoginResponse{
		AccessToken:   accessToken,
		RefreshToken:  refreshToken,
		IsBoundCampus: u.SchoolID != 0,
		SchoolId:      u.SchoolID,
	}, nil
}

// ─── Refresh Token ───────────────────────────────────────────────────────────

// RefreshToken 用 refresh_token 换取新 access_token。
//
// 流程：
//  1. 解析 refresh_token，得到 user_id
//  2. 从 DB 重新读取用户（school_id/role 可能有变更）
//  3. 重新签发 access_token
//
// 返回值说明：
//   - refresh_token 不过期：返回新 access_token + 最新 school_id/is_bound
//   - refresh_token 过期：gRPC codes.Unauthenticated → 网关 20004
//   - refresh_token 非法：gRPC codes.Unauthenticated → 网关 20005
//   - 用户不存在：gRPC codes.NotFound → 网关 50005
func (s *UserServiceServer) RefreshToken(ctx context.Context, req *user_pb.RefreshTokenRequest) (*user_pb.RefreshTokenResponse, error) {
	ctx = extractTraceFromMeta(ctx)
	tracer := otel.Tracer(serviceName)
	ctx, span := tracer.Start(ctx, "UserService.RefreshToken")
	defer span.End()

	rt := req.GetRefreshToken()
	if rt == "" {
		return nil, status.Error(codes.Unauthenticated, "missing refresh token")
	}
	claims, err := pkgjwt.ParseRefreshToken(rt, config.Conf.Jwt.AuthKey)
	if err != nil {
		if errors.Is(err, pkgjwt.ErrRefreshTokenExpired) {
			return nil, status.Error(codes.Unauthenticated, "refresh token expired")
		}
		return nil, status.Error(codes.Unauthenticated, "refresh token invalid")
	}

	span.SetAttributes(attribute.Int64("user.id", int64(claims.UserID)))

	// 重新查库获取最新学校/角色
	u, err := user_database.GetByID(claims.UserID)
	if err != nil {
		span.RecordError(err)
		return nil, status.Error(codes.NotFound, "user not found")
	}

	// 封禁用户拦截
	if u.Status == model.StatusBanned {
		span.SetAttributes(attribute.Bool("user.banned", true))
		return nil, status.Error(codes.PermissionDenied, "账号已被封禁")
	}

	jwtCfg := config.Conf.Jwt
	accessToken, err := pkgjwt.GenerateAccessToken(u.ID, u.SchoolID, int8(u.Role), jwtCfg.AuthKey, jwtCfg.AccessExpireH)
	if err != nil {
		span.RecordError(err)
		return nil, err
	}

	return &user_pb.RefreshTokenResponse{
		AccessToken:   accessToken,
		SchoolId:      u.SchoolID,
		IsBoundCampus: u.SchoolID != 0,
	}, nil
}

// ─── Bind Campus ─────────────────────────────────────────────────────────────

func (s *UserServiceServer) BindCampus(ctx context.Context, req *user_pb.BindCampusRequest) (*common_pb.BaseResponse, error) {
	ctx = extractTraceFromMeta(ctx)
	tracer := otel.Tracer(serviceName)
	ctx, span := tracer.Start(ctx, "UserService.BindCampus")
	defer span.End()

	userID := userIDFromCtx(ctx)
	if userID == 0 {
		return &common_pb.BaseResponse{Code: 401, Message: "未认证：缺少用户身份"}, nil
	}

	schoolName := req.GetSchoolName()

	// 根据学校名称搜索学校，取第一个匹配项
	schools, err := user_database.SearchSchools(schoolName, 1)
	if err != nil {
		span.RecordError(err)
		return &common_pb.BaseResponse{Code: 500, Message: "search school failed"}, nil
	}
	if len(schools) == 0 {
		return &common_pb.BaseResponse{Code: 404, Message: "school not found"}, nil
	}
	school := schools[0]

	span.SetAttributes(
		attribute.Int64("user.id", int64(userID)),
		attribute.Int64("school.id", int64(school.ID)),
		attribute.String("school.name", school.Name),
	)

	if err = user_database.BindSchool(userID, school.ID); err != nil {
		span.RecordError(err)
		return &common_pb.BaseResponse{Code: 500, Message: err.Error()}, nil
	}

	// 清缓存
	_ = user_database.DelUserCache(ctx, userID)

	return &common_pb.BaseResponse{Code: 0, Message: "ok"}, nil
}

// ─── Get Current User ────────────────────────────────────────────────────────

func (s *UserServiceServer) GetCurrentUser(ctx context.Context, req *user_pb.GetCurrentUserRequest) (*user_pb.UserInfo, error) {
	ctx = extractTraceFromMeta(ctx)
	tracer := otel.Tracer(serviceName)
	ctx, span := tracer.Start(ctx, "UserService.GetCurrentUser")
	defer span.End()

	targetID := userIDFromCtx(ctx)
	// 请求显式传入 user_id 时覆盖（便于查询其他用户的基本资料）
	if reqUserId := req.GetUserId(); reqUserId != 0 {
		targetID = reqUserId
	}
	span.SetAttributes(attribute.Int64("user.id", int64(targetID)))

	// 先查 Redis
	u, _ := user_database.GetUserCache(ctx, targetID)
	if u == nil {
		var err error
		u, err = user_database.GetByID(targetID)
		if err != nil {
			span.RecordError(err)
			return nil, err
		}
		_ = user_database.SetUserCache(ctx, u)
	}

	// 获取学校名
	schoolName := ""
	if u.SchoolID != 0 {
		if s, _ := getSchoolWithCache(ctx, u.SchoolID); s != nil {
			schoolName = s.Name
		}
	}

	return &user_pb.UserInfo{
		UserId:     u.ID,
		Nickname:   u.Nickname,
		AvatarUrl:  u.AvatarURL,
		SchoolId:   u.SchoolID, // 修复：移除不必要的类型转换，SchoolId 已是 uint64
		SchoolName: schoolName,
		Role:       u.Role.String(),
		CreatedAt:  timestamppb.New(u.CreatedAt),
	}, nil
}

// ─── Update User Info ────────────────────────────────────────────────────────

// UpdateUserInfo 修改当前登录用户的昵称和/或头像。
// 调用方身份由 JWT 注入的 "user-id" 元数据确定，禁止修改他人资料。
func (s *UserServiceServer) UpdateUserInfo(ctx context.Context, req *user_pb.UpdateUserInfoRequest) (*common_pb.BaseResponse, error) {
	ctx = extractTraceFromMeta(ctx)
	tracer := otel.Tracer(serviceName)
	ctx, span := tracer.Start(ctx, "UserService.UpdateUserInfo")
	defer span.End()

	userID := userIDFromCtx(ctx)
	if userID == 0 {
		return &common_pb.BaseResponse{Code: 401, Message: "未认证：缺少用户身份"}, nil
	}

	nickname := req.GetNickname()
	avatarURL := req.GetAvatarUrl()

	// 至少需要更新一个字段
	if nickname == "" && avatarURL == "" {
		return &common_pb.BaseResponse{Code: 400, Message: "至少需要更新昵称或头像"}, nil
	}

	// 获取当前用户信息
	u, err := user_database.GetByID(userID)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, err.Error())
		return &common_pb.BaseResponse{Code: 404, Message: "用户不存在"}, nil
	}

	// 验证昵称
	if nickname != "" && !isValidNickname(nickname) {
		return &common_pb.BaseResponse{Code: 400, Message: "昵称长度必须在2-20个字符之间"}, nil
	}

	// 验证头像URL
	if avatarURL != "" && !isValidAvatarURL(avatarURL) {
		return &common_pb.BaseResponse{Code: 400, Message: "头像URL格式无效"}, nil
	}

	// 如果没有提供新值，保持原值
	if nickname == "" {
		nickname = u.Nickname
	}
	if avatarURL == "" {
		avatarURL = u.AvatarURL
	}

	span.SetAttributes(
		attribute.Int64("user.id", int64(userID)),
		attribute.String("user.nickname", nickname),
	)

	// 更新数据库
	if err = user_database.UpdateUserInfo(userID, nickname, avatarURL); err != nil {
		span.RecordError(err)
		span.SetStatus(otelcodes.Error, err.Error())
		return &common_pb.BaseResponse{Code: 500, Message: "更新失败"}, nil
	}

	// 清除缓存（下次读取会重新加载）
	_ = user_database.DelUserCache(ctx, userID)

	return &common_pb.BaseResponse{Code: 0, Message: "更新成功"}, nil
}

// ─── ListSchools ────────────────────────────────────────────────────────────

// ListSchools 搜索/列出学校。keyword 非空时模糊搜索，否则返回全部。
func (s *UserServiceServer) ListSchools(ctx context.Context, req *user_pb.ListSchoolsRequest) (*user_pb.ListSchoolsResponse, error) {
	ctx = extractTraceFromMeta(ctx)
	tracer := otel.Tracer(serviceName)
	ctx, span := tracer.Start(ctx, "UserService.ListSchools")
	defer span.End()

	pageSize := int(req.GetPageSize())
	if pageSize <= 0 || pageSize > 50 {
		pageSize = 20
	}

	var schools []model.School
	var err error

	if keyword := strings.TrimSpace(req.GetKeyword()); keyword != "" {
		schools, err = user_database.SearchSchools(keyword, pageSize)
	} else {
		schools, err = user_database.ListSchools()
	}
	if err != nil {
		span.RecordError(err)
		return nil, err
	}

	span.SetAttributes(attribute.Int("schools.count", len(schools)))

	pbSchools := make([]*user_pb.SchoolInfo, 0, len(schools))
	for i := range schools {
		pbSchools = append(pbSchools, &user_pb.SchoolInfo{
			SchoolId: schools[i].ID,
			Name:     schools[i].Name,
			Province: schools[i].City,
		})
	}

	return &user_pb.ListSchoolsResponse{
		Schools:  pbSchools,
		HasMore:  false,
		NextCursor: "",
	}, nil
}

// ─── v2.0: 管理员接口 ─────────────────────────────────────────────────────

// BanUser 封禁用户。
// admin 只能封禁本校学生；super_admin 可跨校封禁任意用户。
func (s *UserServiceServer) BanUser(ctx context.Context, req *user_pb.BanUserRequest) (*common_pb.BaseResponse, error) {
	ctx = extractTraceFromMeta(ctx)
	tracer := otel.Tracer(serviceName)
	ctx, span := tracer.Start(ctx, "UserService.BanUser")
	defer span.End()

	operatorRole := userRoleFromCtx(ctx)
	operatorSchool := userSchoolFromCtx(ctx)
	targetID := req.GetUserId()
	reason := req.GetReason()

	span.SetAttributes(
		attribute.Int64("ban.target", targetID),
		attribute.Int64("ban.operator_role", int64(operatorRole)),
		attribute.String("ban.reason", reason),
	)

	// 1. 权限校验
	if operatorRole < int8(model.RoleAdmin) {
		return nil, status.Error(codes.PermissionDenied, "仅管理员可封禁用户")
	}

	// 2. 获取目标用户
	target, err := user_database.GetByID(targetID)
	if err != nil {
		return nil, status.Error(codes.NotFound, "用户不存在")
	}

	// 3. admin 只能封禁本校学生，不能封禁其他 admin/super_admin
	if operatorRole == int8(model.RoleAdmin) {
		if target.SchoolID != int64(operatorSchool) {
			return nil, status.Error(codes.PermissionDenied, "仅可操作本校用户")
		}
		if target.Role != model.RoleStudent {
			return nil, status.Error(codes.PermissionDenied, "无权封禁管理员")
		}
	}

	// 4. 修改状态
	if err = user_database.SetUserStatus(targetID, model.StatusBanned); err != nil {
		span.RecordError(err)
		return nil, err
	}

	// 5. 清除缓存
	_ = user_database.DelUserCache(ctx, targetID)
	_ = recordAuditLog(ctx, targetID, operatorSchool, model.AuditActionBanUser, reason)

	return &common_pb.BaseResponse{Code: 0, Message: "已封禁"}, nil
}

// UnbanUser 解封用户。权限规则同 BanUser。
func (s *UserServiceServer) UnbanUser(ctx context.Context, req *user_pb.UnbanUserRequest) (*common_pb.BaseResponse, error) {
	ctx = extractTraceFromMeta(ctx)
	tracer := otel.Tracer(serviceName)
	ctx, span := tracer.Start(ctx, "UserService.UnbanUser")
	defer span.End()

	operatorRole := userRoleFromCtx(ctx)
	operatorSchool := userSchoolFromCtx(ctx)
	targetID := req.GetUserId()

	// 1. 权限校验
	if operatorRole < int8(model.RoleAdmin) {
		return nil, status.Error(codes.PermissionDenied, "仅管理员可解封用户")
	}

	// 2. 获取目标用户
	target, err := user_database.GetByID(targetID)
	if err != nil {
		return nil, status.Error(codes.NotFound, "用户不存在")
	}

	// 3. admin 仅可操作本校
	if operatorRole == int8(model.RoleAdmin) {
		if target.SchoolID != int64(operatorSchool) {
			return nil, status.Error(codes.PermissionDenied, "仅可操作本校用户")
		}
		if target.Role != model.RoleStudent {
			return nil, status.Error(codes.PermissionDenied, "无权操作管理员")
		}
	}

	// 4. 修改状态
	if err = user_database.SetUserStatus(targetID, model.StatusNormal); err != nil {
		span.RecordError(err)
		return nil, err
	}

	// 5. 清除缓存
	_ = user_database.DelUserCache(ctx, targetID)
	_ = recordAuditLog(ctx, targetID, operatorSchool, model.AuditActionUnbanUser, "")

	return &common_pb.BaseResponse{Code: 0, Message: "已解封"}, nil
}

// ─── helpers ──────────────────────────────────────────────────────────────────

// extractTraceFromMeta extracts W3C TraceContext from gRPC incoming metadata.
func extractTraceFromMeta(ctx context.Context) context.Context {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ctx
	}
	carrier := make(map[string]string)
	for k, vals := range md {
		if len(vals) > 0 {
			carrier[k] = vals[0]
		}
	}
	return otel.GetTextMapPropagator().Extract(ctx, propagationMapCarrier(carrier))
}

type propagationMapCarrier map[string]string

func (c propagationMapCarrier) Get(key string) string { return c[key] }
func (c propagationMapCarrier) Set(key, value string) { c[key] = value }
func (c propagationMapCarrier) Keys() []string {
	keys := make([]string, 0, len(c))
	for k := range c {
		keys = append(keys, k)
	}
	return keys
}

// userIDFromCtx reads the user ID injected by the gateway via gRPC metadata.
// Gateway sets "user-id" (stringified int64) in outgoing metadata after JWT validation.
// 返回 0 表示无法获取用户身份（未认证）。
func userIDFromCtx(ctx context.Context) int64 {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return 0
	}

	vals := md.Get("user-id")
	if len(vals) == 0 {
		return 0
	}

	id, err := strconv.ParseInt(vals[0], 10, 64)
	if err != nil {
		return 0
	}

	return id
}

// userRoleFromCtx 从 gRPC metadata 读取操作人的角色（由 Gateway 注入）。
// 返回 0 表示无法获取（当作最低权限处理）。
func userRoleFromCtx(ctx context.Context) int8 {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return 0
	}
	vals := md.Get("user-role")
	if len(vals) == 0 {
		return 0
	}
	role, err := strconv.ParseInt(vals[0], 10, 8)
	if err != nil {
		return 0
	}
	return int8(role)
}

// userSchoolFromCtx 从 gRPC metadata 读取操作人的学校（由 Gateway 注入）。
// 返回 0 表示未绑定学校。
func userSchoolFromCtx(ctx context.Context) int64 {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return 0
	}
	vals := md.Get("school-id")
	if len(vals) == 0 {
		return 0
	}
	schoolID, err := strconv.ParseInt(vals[0], 10, 64)
	if err != nil {
		return 0
	}
	return schoolID
}

// recordAuditLog 记录管理员操作审计日志。写入失败不阻塞主流程。
func recordAuditLog(ctx context.Context, targetID, operatorSchool int64, action model.AuditAction, detail string) error {
	// 审计日志写入延用 user_database 的 DB 连接
	d, err := db.GetUserDB()
	if err != nil {
		log.Printf("[user-service] 审计日志写入失败(DB未初始化): %v", err)
		return err
	}
	logEntry := &model.AdminAuditLog{
		ID:         snowflake.GenerateID(),
		OperatorID: userIDFromCtx(ctx),
		TargetID:   targetID,
		Action:     action,
		Detail:     detail,
	}
	return d.Create(logEntry).Error
}

// getSchoolWithCache fetches a school, using Redis as L1 cache.
func getSchoolWithCache(ctx context.Context, id int64) (*model.School, error) {
	if s, _ := user_database.GetSchoolCache(ctx, id); s != nil {
		return s, nil
	}
	s, err := user_database.GetSchoolByID(id)
	if err != nil {
		return nil, err
	}
	_ = user_database.SetSchoolCache(ctx, s)
	return s, nil
}

// Ensure interface compat at compile time.
var _ user_pb.UserServiceServer = (*UserServiceServer)(nil)

// Unused import guard.
var _ = time.Now
