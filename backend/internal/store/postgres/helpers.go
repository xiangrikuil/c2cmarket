package postgres

import (
	"c2c-market/backend/internal/domain"
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"math/big"
	"net/http"
	"strings"
)

type queryer interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

type scanner interface {
	Scan(...any) error
}
type rowQueryer interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func queryRows(ctx context.Context, q queryer, sql string, args ...any) (pgx.Rows, error) {
	if rowQ, ok := q.(rowQueryer); ok {
		return rowQ.Query(ctx, sql, args...)
	}
	return nil, errors.New("query rows unsupported")
}
func storeValidateOptionalNonSecretText(field, value string) *domain.AppError {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	if len(strings.TrimSpace(value)) > 4000 {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Text too long", "文本内容过长。", field, "too_long", "文本内容过长。")
	}
	if strings.ContainsAny(value, "\x00") {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Text invalid", "文本内容包含非法字符。", field, "control_character", "文本内容包含非法字符。")
	}
	if storeLooksLikeSecret(value) {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeSecretContentDetected, "Secret content detected", "不能在平台填写、粘贴或上传任何凭据。", field, "secret_content", "不能包含 API Key、密码、Token、Session 或 Cookie。")
	}
	return nil
}

func storeLooksLikeSecret(value string) bool {
	return domain.LooksLikeSecretContent(value)
}

func storeParsePositiveDecimal(value string) (*big.Rat, bool) {
	rat, ok := new(big.Rat).SetString(strings.TrimSpace(value))
	if !ok || rat.Sign() <= 0 {
		return nil, false
	}
	return rat, true
}

func storeDecimalStringMust(value string, places int) string {
	rat, ok := storeParsePositiveDecimal(value)
	if !ok {
		return strings.TrimSpace(value)
	}
	return storeDecimalString(rat, places)
}

func storeDecimalStringOptional(value string, places int) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	return storeDecimalStringMust(value, places)
}

func storeDecimalString(value *big.Rat, places int) string {
	scale := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(places)), nil)
	scaled := new(big.Rat).Mul(value, new(big.Rat).SetInt(scale))
	rounded := storeRoundRatHalfUp(scaled)
	intPart := new(big.Int).Quo(rounded, scale)
	frac := new(big.Int).Mod(rounded, scale)
	fracText := frac.String()
	for len(fracText) < places {
		fracText = "0" + fracText
	}
	return intPart.String() + "." + fracText
}

func storeRoundRatHalfUp(value *big.Rat) *big.Int {
	num := new(big.Int).Set(value.Num())
	den := new(big.Int).Set(value.Denom())
	quotient, remainder := new(big.Int).QuoRem(num, den, new(big.Int))
	twice := new(big.Int).Mul(remainder, big.NewInt(2))
	if twice.Cmp(den) >= 0 {
		quotient.Add(quotient, big.NewInt(1))
	}
	return quotient
}

func nullText(value string) any {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return value
}

func nullNumeric(value string) any {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return value
}

func nullJSON(value string) any {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return json.RawMessage(value)
}

func nullUUID(value string) any {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return value
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func isUniqueViolationOnConstraint(err error, constraintName string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == constraintName
}

func internalStoreError() *domain.AppError {
	return domain.NewError(http.StatusInternalServerError, domain.CodeInternalError, "Internal error", "持久化操作失败。")
}

func rollback(ctx context.Context, tx pgx.Tx) {
	if tx != nil {
		_ = tx.Rollback(ctx)
	}
}
