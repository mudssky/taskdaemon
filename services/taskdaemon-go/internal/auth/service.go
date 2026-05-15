package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"time"

	"golang.org/x/crypto/bcrypt"

	"taskdaemon/internal/data"
	"taskdaemon/internal/data/ent"
	entadmin "taskdaemon/internal/data/ent/admin"
	entsession "taskdaemon/internal/data/ent/session"
)

const (
	defaultSessionTTL = 24 * time.Hour
	adminSingletonKey = "primary"
	tokenByteLength   = 32

	bcryptMinCostForTest = bcrypt.MinCost
)

var (
	// ErrAdminAlreadyInitialized 表示系统中已经存在单管理员账号。
	ErrAdminAlreadyInitialized = errors.New("admin already initialized")
	// ErrInvalidCredentials 表示用户名或密码不匹配。
	ErrInvalidCredentials = errors.New("invalid credentials")
	// ErrInvalidSession 表示 session 缺失、过期或无法关联有效管理员。
	ErrInvalidSession = errors.New("invalid session")
)

// Options 控制认证服务的安全参数和时间来源。
type Options struct {
	BcryptCost int
	SessionTTL time.Duration
	Now        func() time.Time
}

// AdminAccount 是初始化管理员后返回的安全账号摘要。
type AdminAccount struct {
	ID       int
	Username string
}

// LoginMetadata 保存一次登录请求的非敏感上下文。
type LoginMetadata struct {
	UserAgent string
	IP        string
}

// Principal 表示通过认证后的当前管理员主体。
type Principal struct {
	AdminID  int
	Username string
}

// LoginResult 保存登录成功后的主体、session token 和 CSRF token。
type LoginResult struct {
	Principal    Principal
	SessionToken string
	CSRFToken    string
}

// Service 提供单管理员初始化、登录与 session 认证能力。
type Service struct {
	store      *data.Store
	bcryptCost int
	sessionTTL time.Duration
	now        func() time.Time
}

// New 创建认证服务。
//
// 参数:
//   - store: 已完成 migration 的数据层实例。
//   - opts: bcrypt cost、session TTL 和时间来源配置。
//
// 返回值:
//   - *Service: 可处理管理员初始化和登录态认证的服务。
func New(store *data.Store, opts Options) *Service {
	if opts.BcryptCost == 0 {
		opts.BcryptCost = bcrypt.DefaultCost
	}
	if opts.SessionTTL == 0 {
		opts.SessionTTL = defaultSessionTTL
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	return &Service{
		store:      store,
		bcryptCost: opts.BcryptCost,
		sessionTTL: opts.SessionTTL,
		now:        opts.Now,
	}
}

// InitializeAdmin 创建系统唯一管理员账号。
//
// 参数:
//   - ctx: 控制数据库查询和写入生命周期的 context。
//   - username: 管理员用户名。
//   - password: 管理员明文密码，只用于生成 bcrypt 哈希，不会持久化。
//
// 返回值:
//   - AdminAccount: 初始化成功后的管理员摘要。
//   - error: 已存在管理员、密码哈希失败或数据库写入失败时返回错误。
func (service *Service) InitializeAdmin(ctx context.Context, username string, password string) (AdminAccount, error) {
	exists, err := service.store.Client().Admin.Query().Exist(ctx)
	if err != nil {
		return AdminAccount{}, fmt.Errorf("check admin initialization: %w", err)
	}
	if exists {
		return AdminAccount{}, ErrAdminAlreadyInitialized
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), service.bcryptCost)
	if err != nil {
		return AdminAccount{}, fmt.Errorf("hash admin password: %w", err)
	}
	admin, err := service.store.Client().Admin.Create().
		SetSingletonKey(adminSingletonKey).
		SetUsername(username).
		SetPasswordHash(string(passwordHash)).
		Save(ctx)
	if err != nil {
		if ent.IsConstraintError(err) {
			return AdminAccount{}, ErrAdminAlreadyInitialized
		}
		return AdminAccount{}, fmt.Errorf("create admin: %w", err)
	}

	return AdminAccount{ID: admin.ID, Username: admin.Username}, nil
}

// InitializeAdminWithSession 创建系统唯一管理员账号并立即建立登录态。
//
// 参数:
//   - ctx: 控制数据库查询和写入生命周期的 context。
//   - username: 管理员用户名。
//   - password: 管理员明文密码，只用于生成 bcrypt 哈希，不会持久化。
//   - metadata: 首次初始化请求的 user agent 与 IP 摘要。
//
// 返回值:
//   - LoginResult: 初始化成功后的主体、原始 session token 和 CSRF token。
//   - error: 已存在管理员、密码哈希失败、随机 token 生成失败或数据库写入失败时返回错误。
func (service *Service) InitializeAdminWithSession(ctx context.Context, username string, password string, metadata LoginMetadata) (LoginResult, error) {
	tx, err := service.store.Client().Tx(ctx)
	if err != nil {
		return LoginResult{}, fmt.Errorf("begin admin initialization: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	exists, err := tx.Admin.Query().Exist(ctx)
	if err != nil {
		return LoginResult{}, fmt.Errorf("check admin initialization: %w", err)
	}
	if exists {
		return LoginResult{}, ErrAdminAlreadyInitialized
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), service.bcryptCost)
	if err != nil {
		return LoginResult{}, fmt.Errorf("hash admin password: %w", err)
	}
	admin, err := tx.Admin.Create().
		SetSingletonKey(adminSingletonKey).
		SetUsername(username).
		SetPasswordHash(string(passwordHash)).
		Save(ctx)
	if err != nil {
		if ent.IsConstraintError(err) {
			return LoginResult{}, ErrAdminAlreadyInitialized
		}
		return LoginResult{}, fmt.Errorf("create admin: %w", err)
	}

	result, err := service.createSessionWithClient(ctx, tx.Client(), *admin, metadata)
	if err != nil {
		return LoginResult{}, err
	}
	if err := tx.Commit(); err != nil {
		return LoginResult{}, fmt.Errorf("commit admin initialization: %w", err)
	}
	committed = true

	return result, nil
}

// IsAdminInitialized 判断系统是否已经创建管理员。
//
// 参数:
//   - ctx: 控制数据库查询生命周期的 context。
//
// 返回值:
//   - bool: true 表示已经存在管理员账号。
//   - error: 数据库查询失败时返回错误。
func (service *Service) IsAdminInitialized(ctx context.Context) (bool, error) {
	exists, err := service.store.Client().Admin.Query().Exist(ctx)
	if err != nil {
		return false, fmt.Errorf("check admin initialization: %w", err)
	}
	return exists, nil
}

// Login 校验管理员密码并创建新的 session。
//
// 参数:
//   - ctx: 控制数据库查询和写入生命周期的 context。
//   - username: 管理员用户名。
//   - password: 登录请求中的明文密码。
//   - metadata: 登录请求的 user agent 与 IP 摘要。
//
// 返回值:
//   - LoginResult: 登录成功后的主体、原始 session token 和 CSRF token。
//   - error: 凭证错误、随机 token 生成失败或数据库写入失败时返回错误。
func (service *Service) Login(ctx context.Context, username string, password string, metadata LoginMetadata) (LoginResult, error) {
	admin, err := service.store.Client().Admin.Query().
		Where(entadmin.UsernameEQ(username), entadmin.Active(true)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return LoginResult{}, ErrInvalidCredentials
		}
		return LoginResult{}, fmt.Errorf("find admin by username: %w", err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(password)); err != nil {
		return LoginResult{}, ErrInvalidCredentials
	}

	return service.createSession(ctx, *admin, metadata)
}

// Logout 删除指定 session token 对应的登录态。
//
// 参数:
//   - ctx: 控制数据库写入生命周期的 context。
//   - token: Cookie 中携带的原始 session token，空值会被视为已经退出。
//
// 返回值:
//   - error: 数据库删除失败时返回错误。
func (service *Service) Logout(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	if _, err := service.store.Client().Session.Delete().
		Where(entsession.TokenHashEQ(hashToken(token))).
		Exec(ctx); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

// createSession 为指定管理员创建 session 和 CSRF token。
//
// 参数:
//   - ctx: 控制数据库写入生命周期的 context。
//   - admin: 已通过身份验证的管理员实体。
//   - metadata: 登录请求的 user agent 与 IP 摘要。
//
// 返回值:
//   - LoginResult: 主体、原始 session token 和 CSRF token。
//   - error: 随机 token 生成失败或数据库写入失败时返回错误。
func (service *Service) createSession(ctx context.Context, admin ent.Admin, metadata LoginMetadata) (LoginResult, error) {
	return service.createSessionWithClient(ctx, service.store.Client(), admin, metadata)
}

// createSessionWithClient 使用指定 Ent client 创建 session 和 CSRF token。
//
// 参数:
//   - ctx: 控制数据库写入生命周期的 context。
//   - client: 用于写入 session 的 Ent client，可绑定普通连接或事务。
//   - admin: 已通过身份验证的管理员实体。
//   - metadata: 登录请求的 user agent 与 IP 摘要。
//
// 返回值:
//   - LoginResult: 主体、原始 session token 和 CSRF token。
//   - error: 随机 token 生成失败或数据库写入失败时返回错误。
func (service *Service) createSessionWithClient(ctx context.Context, client *ent.Client, admin ent.Admin, metadata LoginMetadata) (LoginResult, error) {
	sessionToken, err := generateToken()
	if err != nil {
		return LoginResult{}, fmt.Errorf("generate session token: %w", err)
	}
	csrfToken, err := generateToken()
	if err != nil {
		return LoginResult{}, fmt.Errorf("generate csrf token: %w", err)
	}

	_, err = client.Session.Create().
		SetTokenHash(hashToken(sessionToken)).
		SetCsrfTokenHash(hashToken(csrfToken)).
		SetExpiresAt(service.now().Add(service.sessionTTL)).
		SetUserAgent(metadata.UserAgent).
		SetIP(metadata.IP).
		SetAdminID(admin.ID).
		Save(ctx)
	if err != nil {
		return LoginResult{}, fmt.Errorf("create session: %w", err)
	}

	principal := Principal{AdminID: admin.ID, Username: admin.Username}
	return LoginResult{
		Principal:    principal,
		SessionToken: sessionToken,
		CSRFToken:    csrfToken,
	}, nil
}

// AuthenticateSession 根据原始 session token 认证当前管理员主体。
//
// 参数:
//   - ctx: 控制数据库查询生命周期的 context。
//   - token: Cookie 中携带的原始 session token。
//
// 返回值:
//   - Principal: 认证成功后的管理员主体。
//   - error: token 缺失、过期、无效或数据库查询失败时返回错误。
func (service *Service) AuthenticateSession(ctx context.Context, token string) (Principal, error) {
	if token == "" {
		return Principal{}, ErrInvalidSession
	}

	session, err := service.store.Client().Session.Query().
		Where(
			entsession.TokenHashEQ(hashToken(token)),
			entsession.ExpiresAtGT(service.now()),
		).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return Principal{}, ErrInvalidSession
		}
		return Principal{}, fmt.Errorf("find session: %w", err)
	}

	admin, err := session.QueryAdmin().
		Where(entadmin.Active(true)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return Principal{}, ErrInvalidSession
		}
		return Principal{}, fmt.Errorf("find session admin: %w", err)
	}

	return Principal{AdminID: admin.ID, Username: admin.Username}, nil
}

// generateToken 创建 URL 安全的随机 token。
//
// 参数:
//   - 无。
//
// 返回值:
//   - string: base64url 编码后的随机 token。
//   - error: 系统随机数读取失败时返回错误。
func generateToken() (string, error) {
	raw := make([]byte, tokenByteLength)
	if _, err := io.ReadFull(rand.Reader, raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

// hashToken 将原始 token 转换为可持久化的稳定哈希。
//
// 参数:
//   - token: 原始 session 或 CSRF token。
//
// 返回值:
//   - string: SHA-256 十六进制哈希。
func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
