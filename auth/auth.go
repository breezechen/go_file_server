package auth

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// PermissionMode 权限模式
type PermissionMode int

const (
	// PermissionNone 不需要认证
	PermissionNone PermissionMode = iota
	// PermissionRequired 需要认证
	PermissionRequired
)

// AuthConfig 认证配置
type AuthConfig struct {
	// 用户名
	Username string
	// 密码
	Password string
	// 读权限模式（GET请求、浏览）
	ReadPermission PermissionMode
	// 写权限模式（POST/PUT/DELETE请求、上传、删除等）
	WritePermission PermissionMode
	// Realm for basic auth
	Realm string
}

// NewAuthConfig 创建默认认证配置
func NewAuthConfig(username, password string) *AuthConfig {
	return &AuthConfig{
		Username:        username,
		Password:        password,
		ReadPermission:  PermissionNone,    // 默认读不需要认证
		WritePermission: PermissionRequired, // 默认写需要认证
		Realm:          "Restricted",
	}
}

// SetReadPermission 设置读权限
func (a *AuthConfig) SetReadPermission(requireAuth bool) {
	if requireAuth {
		a.ReadPermission = PermissionRequired
	} else {
		a.ReadPermission = PermissionNone
	}
}

// SetWritePermission 设置写权限
func (a *AuthConfig) SetWritePermission(requireAuth bool) {
	if requireAuth {
		a.WritePermission = PermissionRequired
	} else {
		a.WritePermission = PermissionNone
	}
}

// IsAuthRequired 检查是否需要认证
func (a *AuthConfig) IsAuthRequired(method string, path string) bool {
	// 如果没有设置用户名密码，不需要认证
	if a.Username == "" || a.Password == "" {
		return false
	}

	// 根据HTTP方法判断是读还是写操作
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		// 读操作
		return a.ReadPermission == PermissionRequired
	case http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch:
		// 写操作
		return a.WritePermission == PermissionRequired
	default:
		// WebDAV特殊方法
		if isWebDAVWriteMethod(method) {
			return a.WritePermission == PermissionRequired
		}
		if isWebDAVReadMethod(method) {
			return a.ReadPermission == PermissionRequired
		}
		// 默认需要认证
		return true
	}
}

// isWebDAVReadMethod 判断是否为WebDAV读方法
func isWebDAVReadMethod(method string) bool {
	readMethods := []string{"PROPFIND"}
	for _, m := range readMethods {
		if strings.EqualFold(method, m) {
			return true
		}
	}
	return false
}

// isWebDAVWriteMethod 判断是否为WebDAV写方法
func isWebDAVWriteMethod(method string) bool {
	writeMethods := []string{"MKCOL", "COPY", "MOVE", "LOCK", "UNLOCK", "PROPPATCH"}
	for _, m := range writeMethods {
		if strings.EqualFold(method, m) {
			return true
		}
	}
	return false
}

// ValidateCredentials 验证凭据
func (a *AuthConfig) ValidateCredentials(username, password string) bool {
	if a.Username == "" || a.Password == "" {
		return true // 未配置认证
	}
	
	usernameMatch := subtle.ConstantTimeCompare([]byte(username), []byte(a.Username)) == 1
	passwordMatch := subtle.ConstantTimeCompare([]byte(password), []byte(a.Password)) == 1
	
	return usernameMatch && passwordMatch
}

// GinMiddleware 为Gin创建认证中间件
func (a *AuthConfig) GinMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 检查是否需要认证
		if !a.IsAuthRequired(c.Request.Method, c.Request.URL.Path) {
			c.Next()
			return
		}

		// 获取Basic Auth凭据
		username, password, hasAuth := c.Request.BasicAuth()
		
		if !hasAuth || !a.ValidateCredentials(username, password) {
			// 要求认证
			c.Header("WWW-Authenticate", `Basic realm="`+a.Realm+`"`)
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		c.Next()
	}
}

// HTTPMiddleware 为标准HTTP Handler创建认证中间件
func (a *AuthConfig) HTTPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 检查是否需要认证
		if !a.IsAuthRequired(r.Method, r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		// 获取Basic Auth凭据
		username, password, ok := r.BasicAuth()
		
		if !ok || !a.ValidateCredentials(username, password) {
			// 要求认证
			w.Header().Set("WWW-Authenticate", `Basic realm="`+a.Realm+`"`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// CheckAPIAuth 检查API请求的认证（用于AJAX请求）
func (a *AuthConfig) CheckAPIAuth(c *gin.Context) bool {
	// API请求通常是写操作
	if a.WritePermission != PermissionRequired {
		return true
	}

	// 检查Basic Auth
	username, password, hasAuth := c.Request.BasicAuth()
	if hasAuth && a.ValidateCredentials(username, password) {
		return true
	}

	// 也可以支持通过Header传递认证信息（用于AJAX）
	headerUser := c.GetHeader("X-Auth-User")
	headerPass := c.GetHeader("X-Auth-Pass")
	if headerUser != "" && headerPass != "" {
		return a.ValidateCredentials(headerUser, headerPass)
	}

	return false
}

// RequireAPIAuth 要求API认证的中间件（用于特定的API路由）
func (a *AuthConfig) RequireAPIAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !a.CheckAPIAuth(c) {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Authentication required",
			})
			c.Abort()
			return
		}
		c.Next()
	}
}