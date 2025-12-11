package security

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"

	"google.golang.org/grpc/credentials"
)

// TLSConfig конфигурация TLS для mTLS
type TLSConfig struct {
	CertFile   string // Путь к сертификату ноды
	KeyFile    string // Путь к приватному ключу
	CAFile     string // Путь к CA сертификату
	ServerName string // Имя сервера для верификации (опционально)
	SkipVerify bool   // Пропустить верификацию (только для тестов!)
}

// NewServerTLSConfig создаёт TLS конфиг для gRPC сервера с mTLS
func NewServerTLSConfig(cfg *TLSConfig) (*tls.Config, error) {
	// Загрузка сертификата и ключа сервера
	cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load server certificate: %w", err)
	}

	// Загрузка CA для верификации клиентов
	caCert, err := os.ReadFile(cfg.CAFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read CA certificate: %w", err)
	}

	caPool := x509.NewCertPool()
	if !caPool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("failed to parse CA certificate")
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientAuth:   tls.RequireAndVerifyClientCert, // mTLS: требуем сертификат клиента
		ClientCAs:    caPool,
		MinVersion:   tls.VersionTLS13, // Только TLS 1.3
	}, nil
}

// NewServerCredentials создаёт gRPC credentials для сервера
func NewServerCredentials(cfg *TLSConfig) (credentials.TransportCredentials, error) {
	tlsConfig, err := NewServerTLSConfig(cfg)
	if err != nil {
		return nil, err
	}
	return credentials.NewTLS(tlsConfig), nil
}

// ValidateTLSConfig проверяет корректность конфигурации TLS
func ValidateTLSConfig(cfg *TLSConfig) error {
	if cfg.CertFile == "" {
		return fmt.Errorf("certificate file path is required")
	}
	if cfg.KeyFile == "" {
		return fmt.Errorf("private key file path is required")
	}
	if cfg.CAFile == "" {
		return fmt.Errorf("CA certificate file path is required")
	}

	// Проверка существования файлов
	if _, err := os.Stat(cfg.CertFile); os.IsNotExist(err) {
		return fmt.Errorf("certificate file does not exist: %s", cfg.CertFile)
	}
	if _, err := os.Stat(cfg.KeyFile); os.IsNotExist(err) {
		return fmt.Errorf("private key file does not exist: %s", cfg.KeyFile)
	}
	if _, err := os.Stat(cfg.CAFile); os.IsNotExist(err) {
		return fmt.Errorf("CA certificate file does not exist: %s", cfg.CAFile)
	}

	return nil
}
