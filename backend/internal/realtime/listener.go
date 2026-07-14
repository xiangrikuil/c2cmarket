package realtime

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const (
	initialRetryDelay        = time.Second
	maximumRetryDelay        = 30 * time.Second
	healthyConnectionPeriod  = 30 * time.Second
	connectionAttemptTimeout = 10 * time.Second
	connectionCloseTimeout   = 5 * time.Second
	listenStatement          = "listen " + ChannelName
)

var (
	ErrDatabaseURLRequired = errors.New("realtime database URL is required")
	ErrHubRequired         = errors.New("realtime hub is required")
	ErrLoggerRequired      = errors.New("realtime logger is required")
	ErrListenerStarted     = errors.New("realtime postgres listener is already started")
	ErrListenerClosed      = errors.New("realtime postgres listener is closed")
	ErrContextRequired     = errors.New("realtime listener context is required")
)

type listenConnection interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	WaitForNotification(context.Context) (*pgconn.Notification, error)
	Close(context.Context) error
}

type connectPostgres func(context.Context, string) (listenConnection, error)
type waitForRetry func(context.Context, time.Duration) bool

// PostgresListener 独占一个用于 LISTEN/NOTIFY 的 PostgreSQL 会话。
// 它不会占用查询连接池中的连接，并且只处理路由元数据。
type PostgresListener struct {
	databaseURL string
	hub         *Hub
	logger      *log.Logger
	connect     connectPostgres
	waitRetry   waitForRetry
	now         func() time.Time

	lifecycleMu sync.Mutex
	started     bool
	closed      bool
	cancel      context.CancelFunc
	done        chan struct{}
}

func NewPostgresListener(databaseURL string, hub *Hub, logger *log.Logger) (*PostgresListener, error) {
	if strings.TrimSpace(databaseURL) == "" {
		return nil, ErrDatabaseURLRequired
	}
	if hub == nil {
		return nil, ErrHubRequired
	}
	if logger == nil {
		return nil, ErrLoggerRequired
	}

	return newPostgresListener(databaseURL, hub, logger, connectDedicatedPostgres, waitWithTimer, time.Now), nil
}

func newPostgresListener(
	databaseURL string,
	hub *Hub,
	logger *log.Logger,
	connect connectPostgres,
	waitRetry waitForRetry,
	now func() time.Time,
) *PostgresListener {
	return &PostgresListener{
		databaseURL: databaseURL,
		hub:         hub,
		logger:      logger,
		connect:     connect,
		waitRetry:   waitRetry,
		now:         now,
		done:        make(chan struct{}),
	}
}

// Start 启动且仅启动一次自动重连监听循环。
func (l *PostgresListener) Start(parent context.Context) error {
	if parent == nil {
		return ErrContextRequired
	}

	l.lifecycleMu.Lock()
	defer l.lifecycleMu.Unlock()
	if l.closed {
		return ErrListenerClosed
	}
	if l.started {
		return ErrListenerStarted
	}

	ctx, cancel := context.WithCancel(parent)
	l.cancel = cancel
	l.started = true
	go l.run(ctx)
	return nil
}

// Close 取消活动的 LISTEN 会话。关闭数据存储前应调用 Wait。
func (l *PostgresListener) Close() {
	if l == nil {
		return
	}

	l.lifecycleMu.Lock()
	if l.closed {
		l.lifecycleMu.Unlock()
		return
	}
	l.closed = true
	cancel := l.cancel
	l.lifecycleMu.Unlock()

	if cancel != nil {
		cancel()
	}
}

// Wait 阻塞到已启动的监听器释放其独占连接。
func (l *PostgresListener) Wait() {
	if l == nil {
		return
	}

	l.lifecycleMu.Lock()
	started := l.started
	done := l.done
	l.lifecycleMu.Unlock()
	if started {
		<-done
	}
}

func (l *PostgresListener) run(ctx context.Context) {
	defer close(l.done)
	retryDelay := initialRetryDelay

	for {
		healthy, err := l.listenOnce(ctx)
		if ctx.Err() != nil {
			return
		}
		if err == nil {
			return
		}
		if healthy {
			retryDelay = initialRetryDelay
		}

		// 连接错误可能携带数据库连接信息，运行日志只记录重试状态。
		l.logger.Printf("realtime postgres listener interrupted; retrying in %s", retryDelay)
		if !l.waitRetry(ctx, retryDelay) {
			return
		}
		retryDelay = nextRetryDelay(retryDelay)
	}
}

func (l *PostgresListener) listenOnce(ctx context.Context) (bool, error) {
	connectCtx, cancelConnect := context.WithTimeout(ctx, connectionAttemptTimeout)
	conn, err := l.connect(connectCtx, l.databaseURL)
	cancelConnect()
	if err != nil {
		return false, fmt.Errorf("connect realtime postgres listener: %w", err)
	}
	if conn == nil {
		return false, errors.New("connect realtime postgres listener: connector returned nil connection")
	}
	defer l.closeConnection(conn)

	if _, err := conn.Exec(ctx, listenStatement); err != nil {
		return false, fmt.Errorf("listen on realtime postgres channel: %w", err)
	}

	// LISTEN/NOTIFY 没有重放日志，因此每次 LISTEN 成功后都唤醒当前浏览器做权威数据校准。
	l.hub.PublishAll()
	listenedAt := l.now()
	healthy := false

	for {
		notification, err := conn.WaitForNotification(ctx)
		if err != nil {
			if l.now().Sub(listenedAt) >= healthyConnectionPeriod {
				healthy = true
			}
			return healthy, fmt.Errorf("wait for realtime postgres notification: %w", err)
		}
		healthy = true
		if notification == nil || notification.Channel != ChannelName {
			continue
		}

		invalidation, err := parseNotificationPayload(notification.Payload)
		if err != nil {
			l.logger.Printf("realtime postgres listener ignored invalid routing payload: %v", err)
			continue
		}
		publishInvalidation(l.hub, invalidation)
	}
}

func (l *PostgresListener) closeConnection(conn listenConnection) {
	ctx, cancel := context.WithTimeout(context.Background(), connectionCloseTimeout)
	defer cancel()
	if err := conn.Close(ctx); err != nil {
		l.logger.Printf("realtime postgres listener connection close failed")
	}
}

func connectDedicatedPostgres(ctx context.Context, databaseURL string) (listenConnection, error) {
	return pgx.Connect(ctx, databaseURL)
}

func waitWithTimer(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func nextRetryDelay(current time.Duration) time.Duration {
	if current >= maximumRetryDelay {
		return maximumRetryDelay
	}
	next := current * 2
	if next > maximumRetryDelay {
		return maximumRetryDelay
	}
	return next
}
