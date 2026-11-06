package whatsapp

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"chatbot-pdf-video/backend/config"
	"chatbot-pdf-video/backend/gemini"

	"github.com/skip2/go-qrcode"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"
	_ "modernc.org/sqlite"
)

type ChatMessage struct {
	ID        string    `json:"id"`
	From      string    `json:"from"`
	FromName  string    `json:"fromName"`
	Text      string    `json:"text"`
	IsBot     bool      `json:"isBot"`
	Timestamp time.Time `json:"timestamp"`
	ModelUsed string    `json:"modelUsed,omitempty"`
	KeyUsed   string    `json:"keyUsed,omitempty"`
}

type WhatsAppManager struct {
	client       *whatsmeow.Client
	container    *sqlstore.Container
	gemini       *gemini.GeminiService
	mu           sync.RWMutex
	status       string // "disconnected", "connecting", "qr_ready", "connected"
	qrCodeBase64 string
	messages     []ChatMessage
	onStatusCh   chan string
	subscribers  map[chan string]bool
	subsMu       sync.Mutex
}

func NewWhatsAppManager(geminiSvc *gemini.GeminiService) (*WhatsAppManager, error) {
	dbLog := waLog.Stdout("Database", "ERROR", true)
	// modernc.org/sqlite driver name is "sqlite"
	container, err := sqlstore.New(context.Background(), "sqlite", "file:whatsapp.db?_pragma=foreign_keys(1)", dbLog)
	if err != nil {
		return nil, fmt.Errorf("error iniciando sqlstore para WhatsApp: %w", err)
	}

	mgr := &WhatsAppManager{
		container:   container,
		gemini:      geminiSvc,
		status:      "disconnected",
		messages:    make([]ChatMessage, 0),
		onStatusCh:  make(chan string, 10),
		subscribers: make(map[chan string]bool),
	}

	return mgr, nil
}

func (m *WhatsAppManager) Start() error {
	deviceStore, err := m.container.GetFirstDevice(context.Background())
	if err != nil {
		return fmt.Errorf("error obteniendo dispositivo WhatsApp: %w", err)
	}

	clientLog := waLog.Stdout("Client", "INFO", true)
	client := whatsmeow.NewClient(deviceStore, clientLog)
	m.client = client

	client.AddEventHandler(m.eventHandler)

	if client.Store.ID == nil {
		// Device is new, generate QR
		go m.handleQRLogin()
	} else {
		// Existing session, reconnect
		m.setStatus("connecting", "")
		err = client.Connect()
		if err != nil {
			m.setStatus("disconnected", "")
			return fmt.Errorf("error conectando sesión existente: %w", err)
		}
		m.setStatus("connected", "")
	}

	return nil
}

func (m *WhatsAppManager) handleQRLogin() {
	qrChan, err := m.client.GetQRChannel(context.Background())
	if err != nil {
		log.Printf("[WhatsApp] Error obteniendo canal QR: %v", err)
		m.setStatus("disconnected", "")
		return
	}

	err = m.client.Connect()
	if err != nil {
		log.Printf("[WhatsApp] Error conectando cliente para QR: %v", err)
		m.setStatus("disconnected", "")
		return
	}

	for evt := range qrChan {
		if evt.Event == "code" {
			qrPng, err := qrcode.Encode(evt.Code, qrcode.Medium, 256)
			if err == nil {
				b64 := base64.StdEncoding.EncodeToString(qrPng)
				m.setStatus("qr_ready", "data:image/png;base64,"+b64)
			}
		} else if evt.Event == "success" {
			m.setStatus("connected", "")
			log.Println("[WhatsApp] ¡Sesión vinculada con éxito mediante QR!")
		} else if evt.Event == "timeout" {
			m.setStatus("disconnected", "")
			log.Println("[WhatsApp] QR expiró")
		}
	}
}

func (m *WhatsAppManager) Disconnect() error {
	if m.client != nil {
		m.client.Disconnect()
	}
	m.setStatus("disconnected", "")
	return nil
}

func (m *WhatsAppManager) Logout() error {
	if m.client != nil {
		err := m.client.Logout(context.Background())
		m.setStatus("disconnected", "")
		return err
	}
	return nil
}

func (m *WhatsAppManager) GetStatus() (string, string) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.status, m.qrCodeBase64
}

func (m *WhatsAppManager) setStatus(st, qr string) {
	m.mu.Lock()
	m.status = st
	m.qrCodeBase64 = qr
	m.mu.Unlock()

	m.notifySubscribers(st)
}

func (m *WhatsAppManager) Subscribe() chan string {
	m.subsMu.Lock()
	defer m.subsMu.Unlock()
	ch := make(chan string, 5)
	m.subscribers[ch] = true
	return ch
}

func (m *WhatsAppManager) Unsubscribe(ch chan string) {
	m.subsMu.Lock()
	defer m.subsMu.Unlock()
	delete(m.subscribers, ch)
	close(ch)
}

func (m *WhatsAppManager) notifySubscribers(status string) {
	m.subsMu.Lock()
	defer m.subsMu.Unlock()
	for ch := range m.subscribers {
		select {
		case ch <- status:
		default:
		}
	}
}

func (m *WhatsAppManager) GetMessages() []ChatMessage {
	m.mu.RLock()
	defer m.mu.RUnlock()
	msgs := make([]ChatMessage, len(m.messages))
	copy(msgs, m.messages)
	return msgs
}

func (m *WhatsAppManager) AddMessage(msg ChatMessage) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, msg)
	if len(m.messages) > 100 {
		m.messages = m.messages[len(m.messages)-100:]
	}
}

func (m *WhatsAppManager) eventHandler(evt interface{}) {
	switch v := evt.(type) {
	case *events.Connected:
		m.setStatus("connected", "")
		log.Println("[WhatsApp] Cliente conectado")

	case *events.LoggedOut:
		m.setStatus("disconnected", "")
		log.Println("[WhatsApp] Cliente desvinculado / sesión cerrada")

	case *events.Message:
		m.handleIncomingMessage(v)
	}
}

func (m *WhatsAppManager) handleIncomingMessage(evt *events.Message) {
	// Ignore messages from self, broadcast channels, or group chats
	if evt.Info.IsFromMe || evt.Info.IsGroup || evt.Info.Chat.Server == types.GroupServer || evt.Info.Chat.Server == "broadcast" {
		return
	}

	text := ""
	if evt.Message.GetConversation() != "" {
		text = evt.Message.GetConversation()
	} else if evt.Message.GetExtendedTextMessage() != nil {
		text = evt.Message.GetExtendedTextMessage().GetText()
	}

	text = strings.TrimSpace(text)
	if text == "" {
		return
	}

	senderName := evt.Info.PushName
	if senderName == "" {
		senderName = evt.Info.Sender.User
	}

	incomingMsg := ChatMessage{
		ID:        evt.Info.ID,
		From:      evt.Info.Sender.ToNonAD().String(),
		FromName:  senderName,
		Text:      text,
		IsBot:     false,
		Timestamp: time.Now(),
	}
	m.AddMessage(incomingMsg)

	cfg := config.GetConfig()
	if !cfg.AutoReply {
		return
	}

	// Call Gemini with failover & dynamic cheapest Flash model selection
	go func() {
		reply, modelUsed, keyUsed, err := m.gemini.GenerateReply(text, nil)
		if err != nil {
			log.Printf("[WhatsApp Gemini Error] %v", err)
			return
		}

		// Send reply via whatsmeow
		targetJID := evt.Info.Chat
		sendMsg := &waE2E.Message{
			Conversation: proto.String(reply),
		}

		_, err = m.client.SendMessage(context.Background(), targetJID, sendMsg)
		if err != nil {
			log.Printf("[WhatsApp Send Error] %v", err)
			return
		}

		botMsg := ChatMessage{
			ID:        fmt.Sprintf("bot_%d", time.Now().UnixNano()),
			From:      "WhatsApp Bot (Gemini)",
			FromName:  "Gemini Flash Bot",
			Text:      reply,
			IsBot:     true,
			Timestamp: time.Now(),
			ModelUsed: modelUsed,
			KeyUsed:   keyUsed,
		}
		m.AddMessage(botMsg)
	}()
}

// SendManualMessage sends a direct WhatsApp message from dashboard
func (m *WhatsAppManager) SendManualMessage(recipientJID, text string) error {
	if m.client == nil || !m.client.IsConnected() {
		return fmt.Errorf("cliente de WhatsApp no conectado")
	}

	raw := strings.TrimSpace(recipientJID)
	// Remove '+', spaces, dashes, parentheses
	var digits strings.Builder
	for _, r := range raw {
		if r >= '0' && r <= '9' {
			digits.WriteRune(r)
		}
	}
	cleanDigits := digits.String()
	if cleanDigits == "" {
		return fmt.Errorf("el número telefónico está vacío o no contiene dígitos")
	}

	var jid types.JID
	if strings.Contains(raw, "@") {
		var err error
		jid, err = types.ParseJID(raw)
		if err != nil {
			return fmt.Errorf("JID inválido: %w", err)
		}
	} else {
		// Individual WhatsApp chat JID (e.g. 51977638887@s.whatsapp.net)
		jid = types.NewJID(cleanDigits, types.DefaultUserServer)
	}

	sendMsg := &waE2E.Message{
		Conversation: proto.String(text),
	}

	_, err := m.client.SendMessage(context.Background(), jid, sendMsg)
	if err == nil {
		m.AddMessage(ChatMessage{
			ID:        fmt.Sprintf("manual_%d", time.Now().UnixNano()),
			From:      "Dashboard",
			FromName:  "Operador",
			Text:      text,
			IsBot:     false,
			Timestamp: time.Now(),
		})
	}
	return err
}
