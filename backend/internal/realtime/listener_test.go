package realtime

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestNewPostgresListenerValidatesRequiredDependencies(t *testing.T) {
	hub := NewHub()
	defer hub.Close()
	logger := log.New(io.Discard, "", 0)

	if _, err := NewPostgresListener("", hub, logger); !errors.Is(err, ErrDatabaseURLRequired) {
		t.Fatalf("NewPostgresListener() error = %v, want ErrDatabaseURLRequired", err)
	}
	if _, err := NewPostgresListener("postgres://example", nil, logger); !errors.Is(err, ErrHubRequired) {
		t.Fatalf("NewPostgresListener() error = %v, want ErrHubRequired", err)
	}
	if _, err := NewPostgresListener("postgres://example", hub, nil); !errors.Is(err, ErrLoggerRequired) {
		t.Fatalf("NewPostgresListener() error = %v, want ErrLoggerRequired", err)
	}
}

func TestListenOncePublishesBroadWakeAfterEverySuccessfulListen(t *testing.T) {
	hub := NewHub()
	defer hub.Close()
	subscription := mustSubscribe(t, hub, "user-a", false)
	defer subscription.Close()

	waitErr := errors.New("connection interrupted")
	var statements []string
	closeCount := 0
	connection := &stubListenConnection{
		execFn: func(_ context.Context, statement string, _ ...any) (pgconn.CommandTag, error) {
			statements = append(statements, statement)
			return pgconn.NewCommandTag("LISTEN"), nil
		},
		waitFn: func(context.Context) (*pgconn.Notification, error) {
			return nil, waitErr
		},
		closeFn: func(context.Context) error {
			closeCount++
			return nil
		},
	}
	listener := newPostgresListener(
		"postgres://example",
		hub,
		log.New(io.Discard, "", 0),
		func(context.Context, string) (listenConnection, error) { return connection, nil },
		waitWithTimer,
		time.Now,
	)

	for attempt := 0; attempt < 2; attempt++ {
		_, err := listener.listenOnce(context.Background())
		if !errors.Is(err, waitErr) {
			t.Fatalf("listenOnce() error = %v, want %v", err, waitErr)
		}
		expectWakeup(t, subscription.Events())
	}
	if len(statements) != 2 || statements[0] != listenStatement || statements[1] != listenStatement {
		t.Fatalf("LISTEN statements = %#v, want two %q statements", statements, listenStatement)
	}
	if closeCount != 2 {
		t.Fatalf("connection close count = %d, want 2", closeCount)
	}
}

func TestListenerRetryDelayBacksOffToThirtySeconds(t *testing.T) {
	hub := NewHub()
	defer hub.Close()
	connectErr := errors.New("database unavailable")
	var delays []time.Duration
	listener := newPostgresListener(
		"postgres://example",
		hub,
		log.New(io.Discard, "", 0),
		func(context.Context, string) (listenConnection, error) { return nil, connectErr },
		func(_ context.Context, delay time.Duration) bool {
			delays = append(delays, delay)
			return len(delays) < 6
		},
		time.Now,
	)

	listener.run(context.Background())
	want := []time.Duration{time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second, 16 * time.Second, 30 * time.Second}
	if len(delays) != len(want) {
		t.Fatalf("retry delays = %#v, want %#v", delays, want)
	}
	for index := range want {
		if delays[index] != want[index] {
			t.Fatalf("retry delay %d = %s, want %s", index, delays[index], want[index])
		}
	}
}

func TestListenOnceBoundsConnectionAttempt(t *testing.T) {
	hub := NewHub()
	defer hub.Close()
	connectErr := errors.New("database unavailable")
	listener := newPostgresListener(
		"postgres://example",
		hub,
		log.New(io.Discard, "", 0),
		func(ctx context.Context, _ string) (listenConnection, error) {
			deadline, ok := ctx.Deadline()
			if !ok {
				t.Fatal("connection attempt context has no deadline")
			}
			remaining := time.Until(deadline)
			if remaining < 9*time.Second || remaining > connectionAttemptTimeout {
				t.Fatalf("connection attempt deadline remaining = %s", remaining)
			}
			return nil, connectErr
		},
		waitWithTimer,
		time.Now,
	)

	_, err := listener.listenOnce(context.Background())
	if !errors.Is(err, connectErr) {
		t.Fatalf("listenOnce() error = %v, want %v", err, connectErr)
	}
}

func TestListenerDoesNotLogConnectionErrorDetails(t *testing.T) {
	hub := NewHub()
	defer hub.Close()
	var logs bytes.Buffer
	listener := newPostgresListener(
		"postgres://example",
		hub,
		log.New(&logs, "", 0),
		func(context.Context, string) (listenConnection, error) {
			return nil, errors.New("connect postgres://user:database-secret@example failed")
		},
		func(context.Context, time.Duration) bool { return false },
		time.Now,
	)

	listener.run(context.Background())
	if bytes.Contains(logs.Bytes(), []byte("database-secret")) {
		t.Fatalf("listener log leaked connection details: %q", logs.String())
	}
	if logs.Len() == 0 {
		t.Fatal("expected a generic reconnect log")
	}
}

func TestHealthyListenerSessionResetsRetryDelay(t *testing.T) {
	hub := NewHub()
	defer hub.Close()
	connectErr := errors.New("database unavailable")
	waitErr := errors.New("connection interrupted")
	connectCalls := 0
	waitCalls := 0
	connection := &stubListenConnection{
		execFn: func(context.Context, string, ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("LISTEN"), nil
		},
		waitFn: func(context.Context) (*pgconn.Notification, error) {
			waitCalls++
			if waitCalls == 1 {
				return &pgconn.Notification{Channel: ChannelName, Payload: `{"v":1,"audience":"all"}`}, nil
			}
			return nil, waitErr
		},
		closeFn: func(context.Context) error { return nil },
	}
	var delays []time.Duration
	listener := newPostgresListener(
		"postgres://example",
		hub,
		log.New(io.Discard, "", 0),
		func(context.Context, string) (listenConnection, error) {
			connectCalls++
			if connectCalls == 1 {
				return nil, connectErr
			}
			return connection, nil
		},
		func(_ context.Context, delay time.Duration) bool {
			delays = append(delays, delay)
			return len(delays) < 2
		},
		time.Now,
	)

	listener.run(context.Background())
	want := []time.Duration{time.Second, time.Second}
	if len(delays) != len(want) || delays[0] != want[0] || delays[1] != want[1] {
		t.Fatalf("retry delays = %#v, want %#v", delays, want)
	}
}

func TestListenerIgnoresInvalidPayloadWithoutDisconnecting(t *testing.T) {
	hub := NewHub()
	defer hub.Close()
	waitErr := errors.New("connection interrupted")
	waitCalls := 0
	var logs bytes.Buffer
	connection := &stubListenConnection{
		execFn: func(context.Context, string, ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("LISTEN"), nil
		},
		waitFn: func(context.Context) (*pgconn.Notification, error) {
			waitCalls++
			if waitCalls == 1 {
				return &pgconn.Notification{Channel: ChannelName, Payload: `{"v":1,"audience":"all","businessData":"forbidden"}`}, nil
			}
			return nil, waitErr
		},
		closeFn: func(context.Context) error { return nil },
	}
	listener := newPostgresListener(
		"postgres://example",
		hub,
		log.New(&logs, "", 0),
		func(context.Context, string) (listenConnection, error) { return connection, nil },
		waitWithTimer,
		time.Now,
	)

	_, err := listener.listenOnce(context.Background())
	if !errors.Is(err, waitErr) {
		t.Fatalf("listenOnce() error = %v, want %v", err, waitErr)
	}
	if waitCalls != 2 {
		t.Fatalf("WaitForNotification() calls = %d, want 2", waitCalls)
	}
	if logs.Len() == 0 {
		t.Fatal("expected invalid routing payload log")
	}
	if bytes.Contains(logs.Bytes(), []byte("forbidden")) {
		t.Fatalf("log leaked notification payload: %q", logs.String())
	}
}

func TestListenerCloseCancelsConnectionAndWaitsForRelease(t *testing.T) {
	hub := NewHub()
	defer hub.Close()
	waitStarted := make(chan struct{})
	closeCalled := make(chan struct{})
	var waitStartOnce sync.Once
	var closeOnce sync.Once
	connection := &stubListenConnection{
		execFn: func(context.Context, string, ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag("LISTEN"), nil
		},
		waitFn: func(ctx context.Context) (*pgconn.Notification, error) {
			waitStartOnce.Do(func() { close(waitStarted) })
			<-ctx.Done()
			return nil, ctx.Err()
		},
		closeFn: func(context.Context) error {
			closeOnce.Do(func() { close(closeCalled) })
			return nil
		},
	}
	listener := newPostgresListener(
		"postgres://example",
		hub,
		log.New(io.Discard, "", 0),
		func(context.Context, string) (listenConnection, error) { return connection, nil },
		waitWithTimer,
		time.Now,
	)

	if err := listener.Start(context.Background()); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if err := listener.Start(context.Background()); !errors.Is(err, ErrListenerStarted) {
		t.Fatalf("second Start() error = %v, want ErrListenerStarted", err)
	}
	select {
	case <-waitStarted:
	case <-time.After(time.Second):
		t.Fatal("listener did not start waiting for notifications")
	}

	listener.Close()
	listener.Close()
	waitDone := make(chan struct{})
	go func() {
		listener.Wait()
		close(waitDone)
	}()
	select {
	case <-waitDone:
	case <-time.After(time.Second):
		t.Fatal("Wait() did not return after Close()")
	}
	select {
	case <-closeCalled:
	default:
		t.Fatal("dedicated connection was not closed")
	}
	if err := listener.Start(context.Background()); !errors.Is(err, ErrListenerClosed) {
		t.Fatalf("Start() after Close() error = %v, want ErrListenerClosed", err)
	}
}

func TestListenerRejectsNilStartContextAndCloseBeforeStart(t *testing.T) {
	hub := NewHub()
	defer hub.Close()
	listener := newPostgresListener(
		"postgres://example",
		hub,
		log.New(io.Discard, "", 0),
		func(context.Context, string) (listenConnection, error) { return nil, errors.New("unused") },
		waitWithTimer,
		time.Now,
	)

	if err := listener.Start(nil); !errors.Is(err, ErrContextRequired) {
		t.Fatalf("Start(nil) error = %v, want ErrContextRequired", err)
	}
	listener.Close()
	listener.Wait()
	if err := listener.Start(context.Background()); !errors.Is(err, ErrListenerClosed) {
		t.Fatalf("Start() after Close() error = %v, want ErrListenerClosed", err)
	}
}

type stubListenConnection struct {
	execFn  func(context.Context, string, ...any) (pgconn.CommandTag, error)
	waitFn  func(context.Context) (*pgconn.Notification, error)
	closeFn func(context.Context) error
}

func (connection *stubListenConnection) Exec(ctx context.Context, statement string, arguments ...any) (pgconn.CommandTag, error) {
	return connection.execFn(ctx, statement, arguments...)
}

func (connection *stubListenConnection) WaitForNotification(ctx context.Context) (*pgconn.Notification, error) {
	return connection.waitFn(ctx)
}

func (connection *stubListenConnection) Close(ctx context.Context) error {
	return connection.closeFn(ctx)
}
