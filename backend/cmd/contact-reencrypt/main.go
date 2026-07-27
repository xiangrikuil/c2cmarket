package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"c2c-market/backend/internal/config"
	"c2c-market/backend/internal/store/postgres"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:], os.Stdout); err != nil {
		log.Fatalf("联系人密文批处理失败: %v", err)
	}
}

func run(ctx context.Context, args []string, output io.Writer) error {
	flags := flag.NewFlagSet("contact-reencrypt", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	kind := flags.String("kind", postgres.ContactReencryptKindContactMethods, "encrypted data kind")
	cursor := flags.String("cursor", "", "last processed UUID")
	batchSize := flags.Int("batch-size", 100, "rows per batch")
	apply := flags.Bool("apply", false, "persist the re-encryption batch")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments")
	}

	cfg, err := config.LoadContactReencrypt()
	if err != nil {
		return fmt.Errorf("配置无效: %w", err)
	}
	connectCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	store, err := postgres.ConnectWithContactCryptoAndOptions(
		connectCtx,
		cfg.DatabaseURL,
		postgres.ContactCryptoConfig{
			EncryptionKey:         cfg.ContactEncryptionKey,
			FingerprintKey:        cfg.ContactFingerprintKey,
			EncryptionKeyVersion:  cfg.ContactKeyVersion,
			FingerprintKeyVersion: cfg.ContactKeyVersion,
			EncryptionKeys:        cfg.ContactEncryptionKeys,
			FingerprintKeys:       cfg.ContactFingerprintKeys,
		},
		cfg.Database,
	)
	if err != nil {
		return fmt.Errorf("连接 PostgreSQL 失败: %w", err)
	}
	defer store.Close()

	result, err := store.ReencryptContactCipherBatch(ctx, postgres.ContactReencryptOptions{
		Kind:      *kind,
		Cursor:    *cursor,
		BatchSize: *batchSize,
		DryRun:    !*apply,
	})
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(output)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(result); err != nil {
		return fmt.Errorf("写入批处理结果失败: %w", err)
	}
	return nil
}
