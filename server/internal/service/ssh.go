package service

import (
	"fmt"
	"time"

	"golang.org/x/crypto/ssh"
)

type SSHClient struct {
	host      string
	port      int
	username  string
	authType  string // password / key
	authValue string
	timeout   time.Duration
}

func NewSSHClient(host string, port int, username, authType, authValue string, timeout time.Duration) *SSHClient {
	return &SSHClient{
		host:      host,
		port:      port,
		username:  username,
		authType:  authType,
		authValue: authValue,
		timeout:   timeout,
	}
}

func (s *SSHClient) Run(cmd string) (string, error) {
	config := &ssh.ClientConfig{
		User:            s.username,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         s.timeout,
	}

	switch s.authType {
	case "key":
		signer, err := ssh.ParsePrivateKey([]byte(s.authValue))
		if err != nil {
			return "", fmt.Errorf("解析私钥失败: %w", err)
		}
		config.Auth = []ssh.AuthMethod{ssh.PublicKeys(signer)}
	default:
		config.Auth = []ssh.AuthMethod{ssh.Password(s.authValue)}
	}

	addr := fmt.Sprintf("%s:%d", s.host, s.port)
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return "", fmt.Errorf("SSH连接失败 %s: %w", addr, err)
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("创建会话失败: %w", err)
	}
	defer session.Close()

	output, err := session.CombinedOutput(cmd)
	if err != nil {
		return string(output), fmt.Errorf("命令执行失败: %w, output: %s", err, string(output))
	}

	return string(output), nil
}
