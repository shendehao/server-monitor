package config

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"log"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"
)

// TLSConfig TLS 配置
type TLSConfig struct {
	Enabled  bool   // 是否启用 TLS
	CertFile string // 证书文件路径（PEM）
	KeyFile  string // 私钥文件路径（PEM）
	AutoTLS  bool   // 自动生成自签名证书
}

// LoadTLS 加载 TLS 配置，优先环境变量，否则自动生成
func LoadTLS(dataDir string) *TLSConfig {
	tc := &TLSConfig{}

	// 环境变量指定证书
	certFile := os.Getenv("TLS_CERT")
	keyFile := os.Getenv("TLS_KEY")
	if certFile != "" && keyFile != "" {
		tc.Enabled = true
		tc.CertFile = certFile
		tc.KeyFile = keyFile
		log.Printf("TLS: 使用自定义证书 cert=%s key=%s", certFile, keyFile)
		return tc
	}

	// AUTO_TLS 环境变量
	if os.Getenv("AUTO_TLS") == "true" || os.Getenv("AUTO_TLS") == "1" {
		tc.AutoTLS = true
	}

	if !tc.AutoTLS {
		// 检查 data 目录下是否已有证书（之前生成的）
		certPath := filepath.Join(dataDir, "server.crt")
		keyPath := filepath.Join(dataDir, "server.key")
		if fileExists(certPath) && fileExists(keyPath) {
			tc.Enabled = true
			tc.CertFile = certPath
			tc.KeyFile = keyPath
			log.Printf("TLS: 使用已有证书 %s", certPath)
			return tc
		}
		// 没有证书且未开启 AUTO_TLS，不启用 TLS
		return tc
	}

	// 自动生成自签名证书
	certPath := filepath.Join(dataDir, "server.crt")
	keyPath := filepath.Join(dataDir, "server.key")

	if fileExists(certPath) && fileExists(keyPath) {
		tc.Enabled = true
		tc.CertFile = certPath
		tc.KeyFile = keyPath
		log.Printf("TLS: 使用已有自签名证书 %s", certPath)
		return tc
	}

	if err := generateSelfSignedCert(certPath, keyPath); err != nil {
		log.Printf("TLS: 自签名证书生成失败: %v，降级为 HTTP", err)
		return tc
	}

	tc.Enabled = true
	tc.CertFile = certPath
	tc.KeyFile = keyPath
	log.Printf("TLS: 已生成自签名证书 %s", certPath)
	return tc
}

// generateSelfSignedCert 生成 ECDSA P-256 自签名证书，有效期 10 年
func generateSelfSignedCert(certPath, keyPath string) error {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("generate key: %v", err)
	}

	serialNumber, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Server Monitor"},
			CommonName:   "Server Monitor CA",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(10 * 365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IPAddresses:           []net.IP{net.IPv4(0, 0, 0, 0), net.IPv6loopback, net.IPv4(127, 0, 0, 1)},
		DNSNames:              []string{"localhost", "*"},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return fmt.Errorf("create cert: %v", err)
	}

	// 写证书
	os.MkdirAll(filepath.Dir(certPath), 0755)
	certOut, err := os.Create(certPath)
	if err != nil {
		return fmt.Errorf("write cert: %v", err)
	}
	pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	certOut.Close()

	// 写私钥
	keyDER, err := x509.MarshalECPrivateKey(privateKey)
	if err != nil {
		return fmt.Errorf("marshal key: %v", err)
	}
	keyOut, err := os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("write key: %v", err)
	}
	pem.Encode(keyOut, &pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	keyOut.Close()

	return nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
