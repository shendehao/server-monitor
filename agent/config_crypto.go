package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/pbkdf2"
)

const (
	configMagic    = "ENC1:"
	pbkdf2Iter     = 100000
	aesKeyLen      = 32                                                             // AES-256
	legacyConfSalt = "\x73\x6d\x2d\x61\x67\x65\x6e\x74\x2d\x6b\x65\x79\x2d\x76\x31" // 旧版盐（兼容）
)

// deriveConfigKey 从机器标识派生 AES-256 密钥
// 盐值由 machineID 的 SHA-512 派生，二进制中无任何硬编码字符串
// PBKDF2-SHA256, 100000 次迭代
func deriveConfigKey() ([]byte, error) {
	mid, err := getMachineID()
	if err != nil {
		return nil, fmt.Errorf("get machine id: %v", err)
	}
	// 盐值 = SHA-512(machineID 反转)[:16]，与密钥派生路径不同
	reversed := reverseBytes([]byte(mid))
	saltHash := sha512.Sum512(reversed)
	salt := saltHash[:16]
	return pbkdf2.Key([]byte(mid), salt, pbkdf2Iter, aesKeyLen, sha256.New), nil
}

func reverseBytes(b []byte) []byte {
	out := make([]byte, len(b))
	for i, v := range b {
		out[len(b)-1-i] = v
	}
	return out
}

// encryptConfigData 使用 AES-256-GCM 加密配置内容
func encryptConfigData(plaintext []byte) ([]byte, error) {
	key, err := deriveConfigKey()
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	encoded := base64.StdEncoding.EncodeToString(ciphertext)
	return []byte(configMagic + encoded), nil
}

// deriveLegacyConfigKey 旧版密钥派生（兼容已加密的配置文件）
func deriveLegacyConfigKey() ([]byte, error) {
	mid, err := getMachineID()
	if err != nil {
		return nil, err
	}
	return pbkdf2.Key([]byte(mid), []byte(legacyConfSalt), pbkdf2Iter, aesKeyLen, sha256.New), nil
}

// decryptWithKey 用指定密钥解密
func decryptWithKey(raw, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonceSize := gcm.NonceSize()
	if len(raw) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}
	return gcm.Open(nil, raw[:nonceSize], raw[nonceSize:], nil)
}

// decryptConfigData 使用 AES-256-GCM 解密配置内容
// 先尝试新密钥，失败后 fallback 旧密钥（兼容升级）
func decryptConfigData(data []byte) ([]byte, error) {
	str := string(data)
	if !strings.HasPrefix(str, configMagic) {
		return nil, fmt.Errorf("not encrypted")
	}
	raw, err := base64.StdEncoding.DecodeString(str[len(configMagic):])
	if err != nil {
		return nil, fmt.Errorf("base64 decode: %v", err)
	}

	// 尝试新密钥
	newKey, err := deriveConfigKey()
	if err == nil {
		if pt, err := decryptWithKey(raw, newKey); err == nil {
			return pt, nil
		}
	}

	// fallback: 旧密钥
	oldKey, err := deriveLegacyConfigKey()
	if err != nil {
		return nil, err
	}
	plaintext, err := decryptWithKey(raw, oldKey)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w (wrong machine?)", err)
	}
	log.Printf("[配置] 旧密钥解密成功，将迁移到新密钥")
	return plaintext, nil
}

// cachedSignKey 缓存加载时的 SIGN_KEY，保存时回写
var cachedSignKey string

// isEncryptedConfig 检查配置数据是否已加密
func isEncryptedConfig(data []byte) bool {
	return strings.HasPrefix(string(data), configMagic)
}

// loadEncryptedConfigFile 加载配置文件（自动检测加密/明文，明文自动迁移为加密）
func loadEncryptedConfigFile() map[string]string {
	cfg := make(map[string]string)
	selfPath, err := os.Executable()
	if err != nil {
		return cfg
	}
	selfPath, _ = filepath.EvalSymlinks(selfPath)
	confPath := filepath.Join(filepath.Dir(selfPath), "agent.conf")
	data, err := os.ReadFile(confPath)
	if err != nil {
		return cfg
	}

	var content string
	if isEncryptedConfig(data) {
		// 已加密：解密
		plain, err := decryptConfigData(data)
		if err != nil {
			log.Printf("配置文件解密失败: %v", err)
			return cfg
		}
		content = string(plain)
		log.Printf("已加载加密配置文件: %s", confPath)
	} else {
		// 明文：读取后自动加密迁移
		content = string(data)
		content = strings.TrimPrefix(content, "\xEF\xBB\xBF") // UTF-8 BOM
		log.Printf("检测到明文配置，自动加密迁移: %s", confPath)
		if encrypted, err := encryptConfigData([]byte(content)); err == nil {
			os.WriteFile(confPath, encrypted, 0600)
		}
	}

	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			k := strings.TrimSpace(parts[0])
			v := strings.TrimSpace(parts[1])
			cfg[k] = v
			if k == "SIGN_KEY" {
				cachedSignKey = v
			}
		}
	}
	return cfg
}

// saveEncryptedConfigFile 保存加密配置文件
func saveEncryptedConfigFile(serverURL, token, interval string) {
	selfPath, err := os.Executable()
	if err != nil {
		return
	}
	selfPath, _ = filepath.EvalSymlinks(selfPath)
	confPath := filepath.Join(filepath.Dir(selfPath), "agent.conf")
	// 保留现有 SIGN_KEY（从已加载的配置中读取）
	signKey := cachedSignKey
	content := fmt.Sprintf("SERVER_URL=%s\nAGENT_TOKEN=%s\nINTERVAL=%s\n", serverURL, token, interval)
	if signKey != "" {
		content += fmt.Sprintf("SIGN_KEY=%s\n", signKey)
	}
	encrypted, err := encryptConfigData([]byte(content))
	if err != nil {
		log.Printf("配置加密失败，降级明文保存: %v", err)
		os.WriteFile(confPath, []byte(content), 0600)
		return
	}
	os.WriteFile(confPath, encrypted, 0600)
}
