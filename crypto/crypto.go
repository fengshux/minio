package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
	"net"
	"os"
	"strings"
)

const (
	KeyLength         = 32 // AES-256
	NonceLength       = 12 // GCM nonce
	CredentialsSep    = "\x1f"
	EncryptedPrefix   = "enc:aes:"
	EncryptedPrefixLen = 8
)

// getMachineKey 获取机器特征密钥
func getMachineKey() []byte {
	// 1. 获取 MAC 地址
	mac := getMACAddress()

	// 2. 获取主机名
	hostname, _ := os.Hostname()

	// 3. 获取用户名
	user := os.Getenv("USER")
	if user == "" {
		user = os.Getenv("USERNAME")
	}

	// 组合并 hash
	combined := mac + "|" + hostname + "|" + user
	hash := sha256.Sum256([]byte(combined))
	return hash[:]
}

// getMACAddress 获取第一个非空 MAC 地址
func getMACAddress() string {
	interfaces, err := net.Interfaces()
	if err != nil {
		return "unknown"
	}

	for _, iface := range interfaces {
		// 跳过回环接口和未启用的接口
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}
		if len(iface.HardwareAddr) > 0 {
			return iface.HardwareAddr.String()
		}
	}

	return "unknown"
}

// Encrypt 使用机器特征密钥加密
func Encrypt(plaintext string) (string, error) {
	key := getMachineKey()

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, NonceLength)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nil, nonce, []byte(plaintext), nil)
	result := append(nonce, ciphertext...)

	return base64.StdEncoding.EncodeToString(result), nil
}

// Decrypt 使用机器特征密钥解密
func Decrypt(ciphertextBase64 string) (string, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(ciphertextBase64)
	if err != nil {
		return "", errors.New("无效的密文")
	}

	if len(ciphertext) < NonceLength {
		return "", errors.New("密文太短")
	}

	key := getMachineKey()

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := ciphertext[:NonceLength]
	ciphertext = ciphertext[NonceLength:]

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", errors.New("解密失败，可能在本机以外的机器创建")
	}

	return string(plaintext), nil
}

// IsEncrypted 检查值是否已加密
func IsEncrypted(value string) bool {
	return len(value) >= EncryptedPrefixLen && value[:EncryptedPrefixLen] == EncryptedPrefix
}

// EncryptCredentials 将 accessKey 与 secretKey 合并后整体加密。
// 明文格式：accessKey + CredentialsSep + secretKey
func EncryptCredentials(accessKey, secretKey string) (string, error) {
	if accessKey == "" || secretKey == "" {
		return "", errors.New("accessKey 和 secretKey 不能为空")
	}
	return Encrypt(accessKey + CredentialsSep + secretKey)
}

// DecryptCredentials 解密一个由 EncryptCredentials 加密的字符串，
// 拆分后返回 accessKey 与 secretKey。
func DecryptCredentials(ciphertextBase64 string) (string, string, error) {
	if !IsEncrypted(ciphertextBase64) {
		return "", "", errors.New("凭据未加密，格式无效")
	}
	plain, err := Decrypt(ciphertextBase64[EncryptedPrefixLen:])
	if err != nil {
		return "", "", err
	}
	parts := strings.SplitN(plain, CredentialsSep, 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", errors.New("解密后的凭据格式无效")
	}
	return parts[0], parts[1], nil
}
