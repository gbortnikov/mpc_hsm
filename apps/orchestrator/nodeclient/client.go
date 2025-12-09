package nodeclient

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"sync"
	"time"

	pb "github.com/mpc_hsm/node/proto"
	"github.com/mpc_hsm/orchestrator/config"
	"github.com/mpc_hsm/orchestrator/logger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

// NodeClient представляет клиент для подключения к MPC ноде
type NodeClient struct {
	conn      *grpc.ClientConn
	client    pb.MPCNodeServiceClient
	address   string
	nodeID    string
	partyID   string
	mu        sync.RWMutex
	tlsConfig *config.TLSConfig

	signingPublicKey string // base64-encoded Ed25519 public key
}

// NewNodeClient создаёт новый клиент для подключения к ноде
func NewNodeClient(address string, tlsConfig *config.TLSConfig) *NodeClient {
	return &NodeClient{
		address:   address,
		tlsConfig: tlsConfig,
	}
}

// loadTLSCredentials загружает TLS credentials для mTLS
func (nc *NodeClient) loadTLSCredentials() (credentials.TransportCredentials, error) {
	// Загружаем CA сертификат
	caCert, err := os.ReadFile(nc.tlsConfig.CAFile)
	if err != nil {
		return nil, fmt.Errorf("не удалось прочитать CA сертификат %s: %w", nc.tlsConfig.CAFile, err)
	}

	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("не удалось добавить CA сертификат в pool")
	}

	// Загружаем клиентский сертификат
	clientCert, err := tls.LoadX509KeyPair(nc.tlsConfig.CertFile, nc.tlsConfig.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("не удалось загрузить клиентский сертификат: %w", err)
	}

	// Настраиваем TLS конфигурацию
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{clientCert},
		RootCAs:      caCertPool,
		MinVersion:   tls.VersionTLS13,
	}

	// Если указано ServerName, используем его для верификации
	if nc.tlsConfig.ServerName != "" {
		tlsConfig.ServerName = nc.tlsConfig.ServerName
	}

	return credentials.NewTLS(tlsConfig), nil
}

// Connect устанавливает соединение с нодой
func (nc *NodeClient) Connect(ctx context.Context) error {
	nc.mu.Lock()
	defer nc.mu.Unlock()

	var dialOpts []grpc.DialOption

	// Настраиваем TLS или insecure соединение
	if nc.tlsConfig != nil && nc.tlsConfig.Enabled {
		logger.Info("Подключение к ноде с mTLS", map[string]interface{}{
			"address": nc.address,
		})

		creds, err := nc.loadTLSCredentials()
		if err != nil {
			return fmt.Errorf("не удалось загрузить TLS credentials: %w", err)
		}

		dialOpts = append(dialOpts,
			grpc.WithTransportCredentials(creds),
		)
	} else {
		logger.Warn("Подключение к ноде БЕЗ TLS (INSECURE)", map[string]interface{}{
			"address": nc.address,
		})

		dialOpts = append(dialOpts,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
	}

	conn, err := grpc.NewClient(nc.address, dialOpts...)
	if err != nil {
		return fmt.Errorf("не удалось подключиться к ноде %s: %w", nc.address, err)
	}

	nc.conn = conn
	nc.client = pb.NewMPCNodeServiceClient(conn)

	// Получаем информацию о ноде
	info, err := nc.client.GetNodeInfo(ctx, &pb.GetNodeInfoRequest{})
	if err != nil {
		conn.Close()
		return fmt.Errorf("не удалось получить информацию о ноде: %w", err)
	}

	nc.nodeID = info.NodeId
	nc.partyID = info.PartyId
	nc.signingPublicKey = info.SigningPublicKey

	logger.Info("Успешное подключение к ноде", map[string]interface{}{
		"address":  nc.address,
		"party_id": nc.partyID,
		"tls":      nc.tlsConfig != nil && nc.tlsConfig.Enabled,
	})

	return nil
}

// Close закрывает соединение
func (nc *NodeClient) Close() error {
	nc.mu.Lock()
	defer nc.mu.Unlock()

	if nc.conn != nil {
		return nc.conn.Close()
	}
	return nil
}

// GetNodeID возвращает ID ноды
func (nc *NodeClient) GetNodeID() string {
	nc.mu.RLock()
	defer nc.mu.RUnlock()
	return nc.nodeID
}

// GetPartyID возвращает Party ID ноды
func (nc *NodeClient) GetPartyID() string {
	nc.mu.RLock()
	defer nc.mu.RUnlock()
	return nc.partyID
}

// GetAddress возвращает адрес ноды
func (nc *NodeClient) GetAddress() string {
	return nc.address
}

// GetSigningPublicKeyBase64 возвращает публичный ключ в base64
func (nc *NodeClient) GetSigningPublicKeyBase64() string {
	nc.mu.RLock()
	defer nc.mu.RUnlock()
	return nc.signingPublicKey
}

// HealthCheck проверяет состояние ноды
func (nc *NodeClient) HealthCheck(ctx context.Context) (bool, error) {
	nc.mu.RLock()
	client := nc.client
	nc.mu.RUnlock()

	if client == nil {
		return false, fmt.Errorf("not connected")
	}

	resp, err := client.HealthCheck(ctx, &pb.HealthCheckRequest{})
	if err != nil {
		return false, err
	}

	return resp.Healthy, nil
}

// GetNodeInfo возвращает информацию о ноде
func (nc *NodeClient) GetNodeInfo(ctx context.Context) (*pb.GetNodeInfoResponse, error) {
	nc.mu.RLock()
	client := nc.client
	nc.mu.RUnlock()

	if client == nil {
		return nil, fmt.Errorf("not connected")
	}

	return client.GetNodeInfo(ctx, &pb.GetNodeInfoRequest{})
}

// InitKeygen инициализирует сессию генерации ключей на ноде
func (nc *NodeClient) InitKeygen(ctx context.Context, sessionID string, parties []*pb.PartyInfo, threshold int32, curve string) error {
	nc.mu.RLock()
	client := nc.client
	nc.mu.RUnlock()

	if client == nil {
		return fmt.Errorf("not connected")
	}

	resp, err := client.InitKeygen(ctx, &pb.InitKeygenRequest{
		SessionId: sessionID,
		Parties:   parties,
		Threshold: threshold,
		Curve:     curve,
	})
	if err != nil {
		return err
	}

	if !resp.Success {
		return fmt.Errorf("init keygen failed: %s", resp.ErrorMessage)
	}

	return nil
}

// ProcessKeygenMessage отправляет сообщение keygen на ноду
func (nc *NodeClient) ProcessKeygenMessage(ctx context.Context, msg *pb.KeygenMessage) (*pb.KeygenMessageResponse, error) {
	nc.mu.RLock()
	client := nc.client
	nc.mu.RUnlock()

	if client == nil {
		return nil, fmt.Errorf("not connected")
	}

	return client.ProcessKeygenMessage(ctx, msg)
}

// GetKeygenResult получает результат генерации ключей
func (nc *NodeClient) GetKeygenResult(ctx context.Context, sessionID string) (*pb.GetKeygenResultResponse, error) {
	nc.mu.RLock()
	client := nc.client
	nc.mu.RUnlock()

	if client == nil {
		return nil, fmt.Errorf("not connected")
	}

	return client.GetKeygenResult(ctx, &pb.GetKeygenResultRequest{SessionId: sessionID})
}

// InitSigning инициализирует сессию подписания на ноде
func (nc *NodeClient) InitSigning(ctx context.Context, sessionID, keyID string, message []byte, parties []*pb.PartyInfo) error {
	nc.mu.RLock()
	client := nc.client
	nc.mu.RUnlock()

	if client == nil {
		return fmt.Errorf("not connected")
	}

	resp, err := client.InitSigning(ctx, &pb.InitSigningRequest{
		SessionId:     sessionID,
		KeyId:         keyID,
		MessageToSign: message,
		Parties:       parties,
	})
	if err != nil {
		return err
	}

	if !resp.Success {
		return fmt.Errorf("init signing failed: %s", resp.ErrorMessage)
	}

	return nil
}

// ProcessSigningMessage отправляет сообщение подписания на ноду
func (nc *NodeClient) ProcessSigningMessage(ctx context.Context, msg *pb.SigningMessage) (*pb.SigningMessageResponse, error) {
	nc.mu.RLock()
	client := nc.client
	nc.mu.RUnlock()

	if client == nil {
		return nil, fmt.Errorf("not connected")
	}

	return client.ProcessSigningMessage(ctx, msg)
}

// GetSigningResult получает результат подписания
func (nc *NodeClient) GetSigningResult(ctx context.Context, sessionID string) (*pb.GetSigningResultResponse, error) {
	nc.mu.RLock()
	client := nc.client
	nc.mu.RUnlock()

	if client == nil {
		return nil, fmt.Errorf("not connected")
	}

	return client.GetSigningResult(ctx, &pb.GetSigningResultRequest{SessionId: sessionID})
}

// NodeManager управляет подключениями ко всем нодам
type NodeManager struct {
	nodes     map[string]*NodeClient // partyID -> client
	mu        sync.RWMutex
	tlsConfig *config.TLSConfig
}

// NewNodeManager создаёт новый менеджер нод
func NewNodeManager(tlsConfig *config.TLSConfig) *NodeManager {
	return &NodeManager{
		nodes:     make(map[string]*NodeClient),
		tlsConfig: tlsConfig,
	}
}

// AddNode добавляет и подключает ноду
func (nm *NodeManager) AddNode(ctx context.Context, address string) (*NodeClient, error) {
	client := NewNodeClient(address, nm.tlsConfig)

	connectCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := client.Connect(connectCtx); err != nil {
		return nil, err
	}

	nm.mu.Lock()
	nm.nodes[client.GetPartyID()] = client
	nm.mu.Unlock()

	return client, nil
}

// RemoveNode удаляет и отключает ноду
func (nm *NodeManager) RemoveNode(partyID string) error {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	client, exists := nm.nodes[partyID]
	if !exists {
		return fmt.Errorf("node %s not found", partyID)
	}

	delete(nm.nodes, partyID)
	return client.Close()
}

// GetNode возвращает клиент ноды по party ID
func (nm *NodeManager) GetNode(partyID string) (*NodeClient, bool) {
	nm.mu.RLock()
	defer nm.mu.RUnlock()

	client, exists := nm.nodes[partyID]
	return client, exists
}

// GetAllNodes возвращает всех подключённых нод
func (nm *NodeManager) GetAllNodes() []*NodeClient {
	nm.mu.RLock()
	defer nm.mu.RUnlock()

	nodes := make([]*NodeClient, 0, len(nm.nodes))
	for _, client := range nm.nodes {
		nodes = append(nodes, client)
	}
	return nodes
}

// GetNodesByPartyIDs возвращает ноды по списку party ID
func (nm *NodeManager) GetNodesByPartyIDs(partyIDs []string) ([]*NodeClient, error) {
	nm.mu.RLock()
	defer nm.mu.RUnlock()

	nodes := make([]*NodeClient, 0, len(partyIDs))
	for _, partyID := range partyIDs {
		client, exists := nm.nodes[partyID]
		if !exists {
			return nil, fmt.Errorf("node %s not found", partyID)
		}
		nodes = append(nodes, client)
	}
	return nodes, nil
}

// CloseAll закрывает все соединения
func (nm *NodeManager) CloseAll() {
	nm.mu.Lock()
	defer nm.mu.Unlock()

	for _, client := range nm.nodes {
		client.Close()
	}
	nm.nodes = make(map[string]*NodeClient)
}
