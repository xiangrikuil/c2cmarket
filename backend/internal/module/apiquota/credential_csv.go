package apiquota

import (
	"encoding/csv"
	"io"
	"net/http"
	"strings"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/apiorder"
)

const (
	maxCredentialCSVRows  = 5000
	maxCredentialCSVBytes = 5 * 1024 * 1024
)

var credentialCSVHeaders = map[string][]string{
	apiorder.DeliveryKindAPIKeyEndpoint: {"api_base_url", "api_key", "instructions"},
	apiorder.DeliveryKindLoginAccount:   {"panel_login_url", "username", "password", "instructions"},
}

func ParseCredentialCSV(source io.Reader, deliveryKind string) ([]CredentialImportRow, *domain.AppError) {
	deliveryKind = strings.TrimSpace(deliveryKind)
	expectedHeader, ok := credentialCSVHeaders[deliveryKind]
	if !ok {
		return nil, credentialCSVFieldError(0, domain.CodeValidationFailed, "deliveryKind", "交付凭据模板类型无效。")
	}
	if source == nil {
		return nil, credentialCSVFieldError(0, domain.CodeValidationFailed, "file", "必须选择 CSV 文件。")
	}

	limited := &io.LimitedReader{R: source, N: maxCredentialCSVBytes + 1}
	reader := csv.NewReader(limited)
	reader.FieldsPerRecord = len(expectedHeader)
	reader.ReuseRecord = true
	header, err := reader.Read()
	if err != nil {
		return nil, credentialCSVFieldError(0, domain.CodeValidationFailed, "file", "CSV 文件缺少有效表头。")
	}
	header[0] = strings.TrimPrefix(header[0], "\ufeff")
	if !equalCredentialCSVHeader(header, expectedHeader) {
		return nil, credentialCSVFieldError(1, domain.CodeValidationFailed, "file", "CSV 表头与所选模板不一致。")
	}

	rows := make([]CredentialImportRow, 0, 64)
	seen := make(map[string]struct{}, 64)
	for {
		record, readErr := reader.Read()
		if readErr == io.EOF {
			break
		}
		rowNumber := len(rows) + 2
		if readErr != nil {
			return nil, credentialCSVFieldError(rowNumber, domain.CodeValidationFailed, "file", "CSV 行的列数或引号格式无效。")
		}
		if len(rows) >= maxCredentialCSVRows {
			return nil, credentialCSVFieldError(rowNumber, domain.CodeValidationFailed, "file", "单次最多导入 5000 行凭据。")
		}
		input := credentialInputFromCSVRecord(deliveryKind, record)
		normalized, appErr := apiorder.NormalizeDeliveryCredentialForStore(input)
		if appErr != nil {
			return nil, credentialCSVFieldError(rowNumber, appErr.Code, "file", appErr.Detail)
		}
		row := CredentialImportRow{
			DeliveryKind:  normalized.DeliveryKind,
			APIBaseURL:    normalized.APIBaseURL,
			APIKey:        normalized.APIKey,
			PanelLoginURL: normalized.PanelLoginURL,
			Username:      normalized.Username,
			Password:      normalized.Password,
			Instructions:  normalized.Instructions,
		}
		duplicateKey := credentialImportDuplicateKey(row)
		if _, exists := seen[duplicateKey]; exists {
			return nil, credentialCSVFieldError(rowNumber, domain.CodeValidationFailed, "file", "CSV 文件包含重复凭据。")
		}
		seen[duplicateKey] = struct{}{}
		rows = append(rows, row)
	}
	if limited.N == 0 {
		return nil, credentialCSVFieldError(0, domain.CodeValidationFailed, "file", "CSV 文件不能超过 5 MiB。")
	}
	if len(rows) == 0 {
		return nil, credentialCSVFieldError(0, domain.CodeValidationFailed, "file", "CSV 文件至少需要一行凭据。")
	}
	return rows, nil
}

func credentialInputFromCSVRecord(deliveryKind string, record []string) apiorder.DeliveryCredentialInput {
	if deliveryKind == apiorder.DeliveryKindAPIKeyEndpoint {
		return apiorder.DeliveryCredentialInput{
			DeliveryKind: deliveryKind,
			APIBaseURL:   record[0],
			APIKey:       record[1],
			Instructions: record[2],
		}
	}
	return apiorder.DeliveryCredentialInput{
		DeliveryKind:  deliveryKind,
		PanelLoginURL: record[0],
		Username:      record[1],
		Password:      record[2],
		Instructions:  record[3],
	}
}

func credentialImportDuplicateKey(row CredentialImportRow) string {
	if row.DeliveryKind == apiorder.DeliveryKindAPIKeyEndpoint {
		return row.DeliveryKind + "\x00" + row.APIKey
	}
	return row.DeliveryKind + "\x00" + row.PanelLoginURL + "\x00" + row.Username + "\x00" + row.Password
}

func equalCredentialCSVHeader(actual, expected []string) bool {
	if len(actual) != len(expected) {
		return false
	}
	for index := range expected {
		if strings.TrimSpace(actual[index]) != expected[index] {
			return false
		}
	}
	return true
}

func credentialCSVFieldError(row int, code, field, detail string) *domain.AppError {
	if row > 0 {
		detail = "CSV 第 " + integerText(row) + " 行：" + detail
	}
	status := http.StatusUnprocessableEntity
	if code == domain.CodeSecretContentDetected {
		status = http.StatusUnprocessableEntity
	}
	return domain.NewFieldError(status, code, "Credential CSV invalid", detail, field, "invalid", detail)
}
