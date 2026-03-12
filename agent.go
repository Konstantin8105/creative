package creative

import (
	"bytes"
	"fmt"
	"log"
)

type Prompt string

type Agent struct {
	Name  string // short name of agent
	Role  Prompt
	Other []Agent
}

func (a Agent) Run(input, output string, colleguase []Agent, mails string) (results []Mail) {
	log.Printf("Run agent: %s", a.Name)
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "Описание твоей имени\n")
	fmt.Fprintf(&buf, "%s\n", string(a.Name))
	fmt.Fprintf(&buf, "Окончание описания твоей имени\n")
	fmt.Fprintf(&buf, "Описание твоей роли\n")
	fmt.Fprintf(&buf, "%s\n", string(a.Role))
	fmt.Fprintf(&buf, "Окончание описания твоей роли\n")
	for _, c := range colleguase {
		if c.Name == a.Name {
			continue
		}
		fmt.Fprintf(&buf, "Описание роли твоего коллеги по имени: `%s`\n", c.Name)
		fmt.Fprintf(&buf, "%s\n", string(c.Role))
		fmt.Fprintf(&buf, "Окончание описания роли `%s`\n", c.Name)
	}
	fmt.Fprintf(&buf, "Общая задача, которую решается с точки зрения твоей роли\n")
	fmt.Fprintf(&buf, "%s\n", input)
	fmt.Fprintf(&buf, "Окончание описания общей задачи\n")
	if output != "" {
		fmt.Fprintf(&buf, "Достигнутые договоренности\n")
		fmt.Fprintf(&buf, "%s\n", output)
		fmt.Fprintf(&buf, "Окончание описания достигнутых договоренности\n")
	}
	fmt.Fprintf(&buf, "Твой почтовый ящик\n")
	fmt.Fprintf(&buf, "%s\n", mails)
	fmt.Fprintf(&buf, "Окончание твоего почтового ящика\n")
	fmt.Fprintf(&buf, "%s\n", MainBoxPrompt) // написание писем

	// запуск агента
	if AI == nil {
		panic(fmt.Errorf("empty AI"))
	}
	request := buf.String()
	log.Printf("agent request: %s", request)
	responce, err := AI.Run(request)
	if err != nil {
		log.Printf("Error of running %s: `%v`", a.Name, err)
		return
	}
	results, err = ParseMails(responce)
	if err != nil {
		log.Printf("Error of parse mails %s: `%v`", a.Name, err)
		return
	}
	for i := range results {
		results[i].From = a.Name
		log.Printf("result email: %s", results[i])
	}
	return
}

/*
import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"
)

// ==================== ТИПЫ ДАННЫХ ====================

// AgentRole определяет роль агента
type AgentRole string

// Message – структура письма
type Message struct {
	ID        string
	TaskID    string
	From      string
	To        []string
	Cc        []string
	Body      string
	Timestamp time.Time
	InReplyTo string
}

// AgentInfo содержит описание агента
type AgentInfo struct {
	Role        AgentRole
	Name        string
	Description string
	Address     string
}

// AgentContext хранит состояние агента для текущей задачи
type AgentContext struct {
	TaskID       string
	TaskText     string
	CreatedAt    time.Time
	Data         map[string]interface{}
	MessageCount int
}

// AgentConfig содержит конфигурацию агента
type AgentConfig struct {
	Role        AgentRole
	Name        string
	Description string
	Address     string
	LogLevel    string // "debug", "info", "error"
}

// MessageHandler определяет функцию обработки сообщения
type MessageHandler func(agent *BaseAgent, msg *Message, ctx *AgentContext) error

// ==================== РЕЕСТР АГЕНТОВ ====================

// AgentRegistry хранит информацию обо всех агентах
type AgentRegistry struct {
	agents map[string]AgentInfo // по адресу
	byRole map[AgentRole]string // по роли -> адрес
}

// NewAgentRegistry создаёт новый реестр
func NewAgentRegistry() *AgentRegistry {
	return &AgentRegistry{
		agents: make(map[string]AgentInfo),
		byRole: make(map[AgentRole]string),
	}
}

// Register регистрирует агента
func (r *AgentRegistry) Register(info AgentInfo) {
	r.agents[info.Address] = info
	if info.Role != "" {
		r.byRole[info.Role] = info.Address
	}
}

// GetByAddress возвращает информацию об агенте по адресу
func (r *AgentRegistry) GetByAddress(address string) (AgentInfo, bool) {
	info, ok := r.agents[address]
	return info, ok
}

// GetByRole возвращает адрес агента по роли
func (r *AgentRegistry) GetByRole(role AgentRole) (string, bool) {
	addr, ok := r.byRole[role]
	return addr, ok
}

// GetAllDescriptions возвращает описание всех агентов для промптов
func (r *AgentRegistry) GetAllDescriptions() string {
	var sb strings.Builder
	sb.WriteString("Доступные агенты:\n")
	for addr, info := range r.agents {
		fmt.Fprintf(&sb, "- %s (%s): %s\n", info.Name, addr, info.Description)
	}
	return sb.String()
}

// ==================== БАЗОВЫЙ АГЕНТ ====================

// BaseAgent содержит общую логику для всех агентов
type BaseAgent struct {
	// Конфигурация
	config AgentConfig

	// Зависимости
	registry *AgentRegistry

	// Состояние
	currentCtx *AgentContext

	// Логирование
	logger *log.Logger
}

// NewBaseAgent создаёт нового базового агента
func NewBaseAgent(config AgentConfig, registry *AgentRegistry) *BaseAgent {
	logPrefix := fmt.Sprintf("[%s] ", config.Address)
	logger := log.New(log.Writer(), logPrefix, log.LstdFlags)

	return &BaseAgent{
		config:   config,
		registry: registry,
		logger:   logger,
	}
}

// ==================== БАЗОВЫЙ ПРОМПТ АГЕНТА ====================

// BuildBasePrompt формирует универсальный промпт для агента
func (a *BaseAgent) BuildBasePrompt(additionalContext string) string {
	// Получаем информацию о текущей задаче
	taskText := ""
	taskID := ""
	if a.currentCtx != nil {
		taskText = a.currentCtx.TaskText
		taskID = a.currentCtx.TaskID
	}

	// Получаем информацию о других агентах
	agentsDesc := a.registry.GetAllDescriptions()

	// Получаем информацию о самом агенте
	selfInfo, _ := a.registry.GetByAddress(a.config.Address)

	// Получаем доступные хранилища
	completedTasks := a.getCompletedTasksSummary()

	// Базовый промпт
	prompt := fmt.Sprintf(`# РОЛЬ АГЕНТА
Ты — %s (%s). %s

Твой адрес: %s

# ИНФОРМАЦИЯ О ДРУГИХ АГЕНТАХ
%s

# ТЕКУЩАЯ ЗАДАЧА
ID задачи: %s
Описание задачи: %s

# ДОСТУПНЫЕ ХРАНИЛИЩА
- Task Board: текущее задание (только чтение)
- Completed Board: архив завершённых задач (только чтение)
%s

# ДОПОЛНИТЕЛЬНЫЙ КОНТЕКСТ
%s

# ПРАВИЛА РАБОТЫ
1. Ты общаешься с другими агентами через письма
2. Всегда указывай task_id в письмах
3. Отвечай только на письма, адресованные тебе
4. При обнаружении новой задачи (task_id изменился) сбрасывай внутренний контекст
5. Используй Completed Board для справки, чтобы не повторять уже отвергнутые идеи

# ФОРМАТ ОТВЕТА
Твой ответ должен быть в формате JSON с полем "body" для отправки письма,
или с полем "action" для внутренних действий.`,
		selfInfo.Name, selfInfo.Role, selfInfo.Description,
		a.config.Address,
		agentsDesc,
		taskID, taskText,
		a.getCompletedTasksSummary(),
		additionalContext)

	return prompt
}

// getCompletedTasksSummary возвращает краткую сводку завершённых задач
func (a *BaseAgent) getCompletedTasksSummary() string {
	// Здесь должен быть доступ к CompletedBoard
	// Для простоты возвращаем заглушку
	return "Завершённые задачи: (доступны для чтения)"
}

// ==================== УПРАВЛЕНИЕ КОНТЕКСТОМ ====================

// SetTask устанавливает текущую задачу и сбрасывает контекст при необходимости
func (a *BaseAgent) SetTask(taskID, taskText string) {
	// Если задача сменилась, сбрасываем контекст
	if a.currentCtx == nil || a.currentCtx.TaskID != taskID {
		a.logger.Printf("New task detected: %s, resetting context", taskID)
		a.currentCtx = &AgentContext{
			TaskID:    taskID,
			TaskText:  taskText,
			CreatedAt: time.Now(),
			Data:      make(map[string]interface{}),
		}
	} else {
		// Обновляем текст задачи (на случай изменений)
		a.currentCtx.TaskText = taskText
	}
}

// GetContext возвращает текущий контекст
func (a *BaseAgent) GetContext() *AgentContext {
	return a.currentCtx
}

// ResetContext принудительно сбрасывает контекст
func (a *BaseAgent) ResetContext() {
	a.logger.Println("Resetting context")
	a.currentCtx = nil
}

// ==================== РАБОТА С СООБЩЕНИЯМИ ====================

// ProcessMessage обрабатывает входящее сообщение
func (a *BaseAgent) ProcessMessage(msg *Message, handler MessageHandler) error {
	a.logger.Printf("Processing message from %s", msg.From)

	// Проверяем, что задача существует
	taskText, taskID := taskBoard.GetTask()
	if taskID == "" {
		a.logger.Println("No active task, ignoring message")
		return nil
	}

	// Устанавливаем контекст задачи
	a.SetTask(taskID, taskText)

	// Проверяем, что сообщение относится к текущей задаче
	if msg.TaskID != taskID {
		a.logger.Printf("Message task ID %s doesn't match current task %s, ignoring", msg.TaskID, taskID)
		return nil
	}

	// Увеличиваем счётчик сообщений в контексте
	if a.currentCtx != nil {
		a.currentCtx.MessageCount++
	}

	// Вызываем обработчик
	if handler != nil {
		if err := handler(a, msg, a.currentCtx); err != nil {
			a.logger.Printf("Handler error: %v", err)
			return err
		}
	}

	// Помечаем письмо как прочитанное
	messageStore.MarkAsRead(msg.ID, a.config.Address)

	return nil
}
Buffer

// SendMessage отправляет сообщение другому агенту
func (a *BaseAgent) SendMessage(to []string, cc []string, taskID, body string) {
	msg := &Message{
		TaskID:    taskID,
		From:      a.config.Address,
		To:        to,
		Cc:        cc,
		Body:      body,
		Timestamp: time.Now(),
	}
	messageStore.AddMessage(msg)
	a.logger.Printf("Sent message to %v (cc: %v)", to, cc)
}

// ==================== ЛОГИРОВАНИЕ ====================

// LogInfo логирует информационное сообщение
func (a *BaseAgent) LogInfo(format string, args ...interface{}) {
	a.logger.Printf("[INFO] "+format, args...)
}

// LogError логирует ошибку
func (a *BaseAgent) LogError(format string, args ...interface{}) {
	a.logger.Printf("[ERROR] "+format, args...)
}

// LogDebug логирует отладочное сообщение (только если уровень debug)
func (a *BaseAgent) LogDebug(format string, args ...interface{}) {
	if a.config.LogLevel == "debug" {
		a.logger.Printf("[DEBUG] "+format, args...)
	}
}

// ==================== ВСПОМОГАТЕЛЬНЫЕ МЕТОДЫ ====================

// GetAgentInfo возвращает информацию о другом агенте по адресу
func (a *BaseAgent) GetAgentInfo(address string) (AgentInfo, bool) {
	return a.registry.GetByAddress(address)
}

// GetAgentByRole возвращает адрес агента по роли
func (a *BaseAgent) GetAgentByRole(role AgentRole) (string, bool) {
	return a.registry.GetByRole(role)
}

// FormatPromptWithContext формирует промпт с дополнительным контекстом
func (a *BaseAgent) FormatPromptWithContext(agentSpecificContext string) string {
	return a.BuildBasePrompt(agentSpecificContext)
}
*/
