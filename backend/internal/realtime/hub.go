package realtime

import (
	"errors"
	"strings"
	"sync"
)

var (
	ErrHubClosed      = errors.New("realtime hub is closed")
	ErrUserIDRequired = errors.New("realtime subscription user id is required")
)

// Hub 向已认证会话分发低基数失效唤醒信号。
// 每个订阅通道容量为一，因为一次唤醒就会触发完整的权威 REST 数据校准。
type Hub struct {
	mu          sync.Mutex
	subscribers map[*subscriber]struct{}
	closed      bool
}

type subscriber struct {
	userID string
	admin  bool
	wakeup chan struct{}
}

// Subscription 表示一个已认证浏览器的合并唤醒流。
type Subscription struct {
	events    <-chan struct{}
	cancel    func()
	closeOnce sync.Once
}

func NewHub() *Hub {
	return &Hub{subscribers: make(map[*subscriber]struct{})}
}

// Subscribe 注册一个用户会话。管理员会话同时接收自身用户范围和管理员范围的唤醒。
func (h *Hub) Subscribe(userID string, admin bool) (*Subscription, error) {
	if !validRoutingUserID(userID) {
		return nil, ErrUserIDRequired
	}

	entry := &subscriber{
		userID: userID,
		admin:  admin,
		wakeup: make(chan struct{}, 1),
	}

	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		return nil, ErrHubClosed
	}
	h.subscribers[entry] = struct{}{}
	h.mu.Unlock()

	return &Subscription{
		events: entry.wakeup,
		cancel: func() {
			h.unsubscribe(entry)
		},
	}, nil
}

// Events 返回仅承载合并失效唤醒信号的通道。
func (s *Subscription) Events() <-chan struct{} {
	if s == nil {
		return nil
	}
	return s.events
}

// Close 注销订阅并关闭事件通道。
func (s *Subscription) Close() {
	if s == nil {
		return
	}
	s.closeOnce.Do(s.cancel)
}

// PublishUser 唤醒属于指定用户的会话。
func (h *Hub) PublishUser(userID string) {
	if !validRoutingUserID(userID) {
		return
	}
	h.publish(func(entry *subscriber) bool {
		return entry.userID == userID
	})
}

func validRoutingUserID(userID string) bool {
	return userID != "" && strings.TrimSpace(userID) == userID
}

// PublishAdmin 只唤醒已认证的管理员会话。
func (h *Hub) PublishAdmin() {
	h.publish(func(entry *subscriber) bool {
		return entry.admin
	})
}

// PublishAll 唤醒所有已认证会话。
func (h *Hub) PublishAll() {
	h.publish(func(*subscriber) bool {
		return true
	})
}

// Close 停止接收新订阅并关闭全部现有订阅。
func (h *Hub) Close() {
	if h == nil {
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return
	}
	h.closed = true
	for entry := range h.subscribers {
		delete(h.subscribers, entry)
		close(entry.wakeup)
	}
}

func (h *Hub) publish(matches func(*subscriber) bool) {
	if h == nil {
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return
	}
	for entry := range h.subscribers {
		if !matches(entry) {
			continue
		}
		select {
		case entry.wakeup <- struct{}{}:
		default:
			// 已有待消费唤醒即可保证一次完整 REST 数据校准。
		}
	}
}

func (h *Hub) unsubscribe(entry *subscriber) {
	if h == nil || entry == nil {
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.subscribers[entry]; !ok {
		return
	}
	delete(h.subscribers, entry)
	close(entry.wakeup)
}
