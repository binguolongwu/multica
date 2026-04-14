package events

import (
	"log/slog"
	"sync"
)

// Event 表示由处理器或服务发布的领域事件
type Event struct {
	Type        string // 事件类型，例如 "issue:created", "inbox:new"
	WorkspaceID string // 路由到正确的 Hub 房间
	ActorType   string // 触发者类型："member"（成员）、"agent"（代理）、"system"（系统）
	ActorID     string
	Payload     any // 可 JSON 序列化的数据，格式与当前 WebSocket 载荷相同
}

// Handler 是处理事件的函数类型
type Handler func(Event)

// Bus 是进程内同步的发布/订阅事件总线
type Bus struct {
	mu             sync.RWMutex
	listeners      map[string][]Handler
	globalHandlers []Handler
}

// New 创建新的事件总线
func New() *Bus {
	return &Bus{
		listeners: make(map[string][]Handler),
	}
}

// Subscribe 为指定事件类型注册处理器
// 处理器按注册顺序同步调用
func (b *Bus) Subscribe(eventType string, h Handler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.listeners[eventType] = append(b.listeners[eventType], h)
}

// SubscribeAll 注册接收所有事件（无论类型）的全局处理器
// 全局处理器在特定类型处理器之后调用
func (b *Bus) SubscribeAll(h Handler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.globalHandlers = append(b.globalHandlers, h)
}

// Publish 将事件分派给该事件类型的所有注册处理器
// 先执行特定类型处理器，然后执行全局（SubscribeAll）处理器
// 每个处理器同步调用。单个处理器中的 panic 会被恢复，确保一个失败的处理器不会阻止其他处理器执行
func (b *Bus) Publish(e Event) {
	b.mu.RLock()
	handlers := b.listeners[e.Type]
	globals := b.globalHandlers
	b.mu.RUnlock()

	for _, h := range handlers {
		func() {
			defer func() {
				if r := recover(); r != nil {
					slog.Error("panic in event listener", "event_type", e.Type, "recovered", r)
				}
			}()
			h(e)
		}()
	}

	for _, h := range globals {
		func() {
			defer func() {
				if r := recover(); r != nil {
					slog.Error("panic in global event listener", "event_type", e.Type, "recovered", r)
				}
			}()
			h(e)
		}()
	}
}
