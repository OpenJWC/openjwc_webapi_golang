package sqlite

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/OpenJWC/openjwc_webapi_golang/internal/domain/access"
	"github.com/OpenJWC/openjwc_webapi_golang/internal/service/identity"
	"golang.org/x/crypto/bcrypt"
)

// Register 在事务内复核用户名和邮箱唯一性后保存待审核申请。
func (store *Store) Register(ctx context.Context, input identity.RegistrationInput, hash string) error {
	tx, err := store.writer.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var count int
	if err = tx.QueryRowContext(ctx, `SELECT (SELECT count(*) FROM users WHERE username=? OR email=?) + (SELECT count(*) FROM user_registrations WHERE username=? OR email=?)`, input.Username, input.Email, input.Username, input.Email).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return access.ErrConflict
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO user_registrations(username,email,password_hash,created_at,updated_at) VALUES(?,?,?,?,?)`, input.Username, input.Email, hash, now(), now())
	if err != nil {
		return err
	}
	return tx.Commit()
}

// Login 验证密码后在写事务中再次检查账号状态及设备配额。
func (store *Store) Login(ctx context.Context, input identity.LoginInput, device string) (identity.LoginResult, error) {
	var id int64
	var username, email, hash string
	var active bool
	err := store.reader.QueryRowContext(ctx, "SELECT id,username,email,password_hash,is_active FROM users WHERE username=? OR email=?", input.Account, input.Account).Scan(&id, &username, &email, &hash, &active)
	if errors.Is(err, sql.ErrNoRows) {
		return identity.LoginResult{}, access.ErrUnauthorized
	}
	if err != nil {
		return identity.LoginResult{}, err
	}
	if !active || bcrypt.CompareHashAndPassword([]byte(hash), []byte(input.Password)) != nil {
		return identity.LoginResult{}, access.ErrUnauthorized
	}
	token := newToken()
	tx, err := store.writer.BeginTx(ctx, nil)
	if err != nil {
		return identity.LoginResult{}, err
	}
	defer tx.Rollback()
	var quota, count, exists int
	if err = tx.QueryRowContext(ctx, "SELECT is_active,max_devices FROM users WHERE id=? AND password_hash=?", id, hash).Scan(&active, &quota); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return identity.LoginResult{}, access.ErrUnauthorized
		}
		return identity.LoginResult{}, err
	}
	if !active {
		return identity.LoginResult{}, access.ErrUnauthorized
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM user_devices WHERE user_id=? AND expires_at<=?", id, time.Now().UTC().Format(time.RFC3339)); err != nil {
		return identity.LoginResult{}, err
	}
	if err = tx.QueryRowContext(ctx, "SELECT count(*),coalesce(sum(device_uuid=?),0) FROM user_devices WHERE user_id=?", device, id).Scan(&count, &exists); err != nil {
		return identity.LoginResult{}, err
	}
	if exists == 0 && count >= quota {
		return identity.LoginResult{}, access.ErrConflict
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO user_devices(user_id,device_uuid,device_name,token_hash,expires_at,last_login) VALUES(?,?,?,?,?,?)
		ON CONFLICT(user_id,device_uuid) DO UPDATE SET device_name=excluded.device_name,token_hash=excluded.token_hash,expires_at=excluded.expires_at,last_login=excluded.last_login`, id, device, input.DeviceName, hashToken(token), time.Now().UTC().Add(30*24*time.Hour).Format(time.RFC3339), now())
	if err != nil {
		return identity.LoginResult{}, err
	}
	if err = tx.Commit(); err != nil {
		return identity.LoginResult{}, err
	}
	return identity.LoginResult{Token: token, Username: username, Email: email}, nil
}

// Authenticate 每次请求读取最新权限和设备绑定，停用与解绑立即生效。
func (store *Store) Authenticate(ctx context.Context, token string, device string) (access.Principal, error) {
	if token == "" || len(token) > 4096 || len(device) > 128 {
		return access.Principal{}, access.ErrUnauthorized
	}
	var id int64
	var username, bound string
	var browse, chat, submit bool
	err := store.reader.QueryRowContext(ctx, `SELECT u.id,u.username,u.can_browse,u.can_chat,u.can_submit,d.device_uuid
		FROM user_devices d JOIN users u ON u.id=d.user_id WHERE d.token_hash=? AND d.expires_at>? AND u.is_active=1`, hashToken(token), time.Now().UTC().Format(time.RFC3339)).Scan(&id, &username, &browse, &chat, &submit, &bound)
	if errors.Is(err, sql.ErrNoRows) {
		return store.authenticateKey(ctx, token, device)
	}
	if err != nil {
		return access.Principal{}, fmt.Errorf("读取客户端身份: %w", err)
	}
	if device != "" && device != bound {
		return access.Principal{}, access.ErrUnauthorized
	}
	return access.RestorePrincipal(id, username, browse, chat, submit)
}

// newToken 生成高熵不透明凭据，客户端无需解析内部编码。
func newToken() string { return base64.RawURLEncoding.EncodeToString(randomBytes()) }

// randomBytes 返回由系统安全随机源生成的凭据材料。
func randomBytes() []byte { result := make([]byte, 32); rand.Read(result); return result }

// hashToken 对高熵凭据计算存储摘要，不用于用户密码。
func hashToken(token string) string {
	digest := sha256.Sum256([]byte(token))
	return hex.EncodeToString(digest[:])
}
