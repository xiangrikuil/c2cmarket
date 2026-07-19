package profile

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"html/template"
	"io"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/http"
	"net/mail"
	"net/smtp"
	"net/url"
	"strconv"
	"strings"
	"time"

	"c2c-market/backend/internal/domain"
)

type EmailSender interface {
	SendVerificationCode(ctx context.Context, toEmail, code string, expiresAt time.Time) *domain.AppError
	SendRegistrationSuccess(ctx context.Context, toEmail, username, displayName string, registeredAt time.Time) *domain.AppError
	SendCarpoolApplicationCreated(ctx context.Context, toEmail, listingTitle, applicationID string, createdAt time.Time) *domain.AppError
	SendCarpoolApplicationAccepted(ctx context.Context, toEmail, listingTitle, applicationID string, joinDeadline *time.Time) *domain.AppError
	SendAPIOrderCreated(ctx context.Context, toEmail, serviceTitle, orderID, amount, currency string, paymentExpiresAt, createdAt time.Time) *domain.AppError
	ExposeDevCode() bool
}

type DevelopmentEmailSender struct{}

func NewDevelopmentEmailSender() DevelopmentEmailSender {
	return DevelopmentEmailSender{}
}

func (DevelopmentEmailSender) SendVerificationCode(context.Context, string, string, time.Time) *domain.AppError {
	return nil
}

func (DevelopmentEmailSender) SendRegistrationSuccess(context.Context, string, string, string, time.Time) *domain.AppError {
	return nil
}

func (DevelopmentEmailSender) SendCarpoolApplicationCreated(context.Context, string, string, string, time.Time) *domain.AppError {
	return nil
}

func (DevelopmentEmailSender) SendCarpoolApplicationAccepted(context.Context, string, string, string, *time.Time) *domain.AppError {
	return nil
}

func (DevelopmentEmailSender) SendAPIOrderCreated(context.Context, string, string, string, string, string, time.Time, time.Time) *domain.AppError {
	return nil
}

func (DevelopmentEmailSender) ExposeDevCode() bool {
	return true
}

type SMTPConfig struct {
	Host           string
	Port           int
	Username       string
	Password       string
	FromAddress    string
	FromName       string
	FrontendOrigin string
}

type smtpClient interface {
	Auth(smtp.Auth) error
	Mail(string) error
	Rcpt(string) error
	Data() (io.WriteCloser, error)
	Quit() error
	Close() error
}

type smtpDialer func(ctx context.Context, host, address string) (smtpClient, error)

type SMTPEmailSender struct {
	host           string
	port           int
	username       string
	password       string
	fromAddress    string
	fromName       string
	frontendOrigin string
	templates      *emailTemplates
	dial           smtpDialer
}

var beijingEmailLocation = time.FixedZone("UTC+8", 8*60*60)

func NewSMTPEmailSender(cfg SMTPConfig) (*SMTPEmailSender, error) {
	cfg.Host = strings.TrimSpace(cfg.Host)
	cfg.Username = strings.TrimSpace(cfg.Username)
	cfg.Password = strings.TrimSpace(cfg.Password)
	cfg.FromAddress = strings.TrimSpace(cfg.FromAddress)
	cfg.FromName = strings.TrimSpace(cfg.FromName)
	cfg.FrontendOrigin = strings.TrimRight(strings.TrimSpace(cfg.FrontendOrigin), "/")
	if cfg.Port == 0 {
		cfg.Port = 465
	}
	if cfg.FromAddress == "" {
		cfg.FromAddress = "noreply@example.com"
	}
	if cfg.FromName == "" {
		cfg.FromName = "C2CMarket"
	}
	if cfg.Host == "" || cfg.Username == "" || cfg.Password == "" {
		return nil, fmt.Errorf("SMTP 配置不完整")
	}
	if cfg.FrontendOrigin == "" {
		return nil, fmt.Errorf("FRONTEND_ORIGIN 配置不完整")
	}
	if cfg.Port != 465 {
		return nil, fmt.Errorf("SMTP_PORT 必须为 465")
	}
	if _, err := mail.ParseAddress(cfg.FromAddress); err != nil {
		return nil, fmt.Errorf("MAIL_FROM_ADDRESS 格式不正确")
	}
	templates, err := newEmailTemplates()
	if err != nil {
		return nil, err
	}
	return &SMTPEmailSender{
		host:           cfg.Host,
		port:           cfg.Port,
		username:       cfg.Username,
		password:       cfg.Password,
		fromAddress:    cfg.FromAddress,
		fromName:       cfg.FromName,
		frontendOrigin: cfg.FrontendOrigin,
		templates:      templates,
		dial:           dialImplicitTLSSMTP,
	}, nil
}

func (s *SMTPEmailSender) SendVerificationCode(ctx context.Context, toEmail, code string, expiresAt time.Time) *domain.AppError {
	if s == nil {
		return emailUnavailableError()
	}
	htmlBody, err := s.templates.renderVerification(verificationTemplateData{
		Code: strings.TrimSpace(code),
	})
	if err != nil {
		return emailUnavailableError()
	}
	return s.send(ctx, emailMessage{
		To:       toEmail,
		Subject:  "C2CMarket 邮箱验证码",
		TextBody: verificationTextBody(code, expiresAt),
		HTMLBody: htmlBody,
	})
}

func (s *SMTPEmailSender) SendRegistrationSuccess(ctx context.Context, toEmail, username, displayName string, registeredAt time.Time) *domain.AppError {
	if s == nil {
		return emailUnavailableError()
	}
	name := strings.TrimSpace(displayName)
	if name == "" {
		name = strings.TrimSpace(username)
	}
	htmlBody, err := s.templates.renderRegistration(registrationTemplateData{
		DisplayName:  name,
		Username:     strings.TrimSpace(username),
		RegisteredAt: formatEmailTime(registeredAt),
		ProfileURL:   s.frontendOrigin + "/my",
	})
	if err != nil {
		return emailUnavailableError()
	}
	return s.send(ctx, emailMessage{
		To:       toEmail,
		Subject:  "C2CMarket 注册成功",
		TextBody: registrationTextBody(username, displayName, registeredAt, s.frontendOrigin+"/my"),
		HTMLBody: htmlBody,
	})
}

func (s *SMTPEmailSender) SendCarpoolApplicationCreated(ctx context.Context, toEmail, listingTitle, applicationID string, createdAt time.Time) *domain.AppError {
	if s == nil {
		return emailUnavailableError()
	}
	title := strings.TrimSpace(listingTitle)
	if title == "" {
		title = "你的车源"
	}
	applicationID = strings.TrimSpace(applicationID)
	detailURL := s.frontendOrigin + "/merchant/carpool-applications/" + url.PathEscape(applicationID)
	htmlBody, err := s.templates.renderCarpoolApplication(carpoolApplicationTemplateData{
		ListingTitle:  title,
		ApplicationID: emailReferenceID(applicationID, "CA"),
		CreatedAt:     formatEmailTime(createdAt),
		DetailURL:     detailURL,
	})
	if err != nil {
		return emailUnavailableError()
	}
	return s.send(ctx, emailMessage{
		To:       toEmail,
		Subject:  "C2CMarket 收到新的上车申请",
		TextBody: carpoolApplicationTextBody(title, emailReferenceID(applicationID, "CA"), createdAt, detailURL),
		HTMLBody: htmlBody,
	})
}

func (s *SMTPEmailSender) SendCarpoolApplicationAccepted(ctx context.Context, toEmail, listingTitle, applicationID string, joinDeadline *time.Time) *domain.AppError {
	if s == nil {
		return emailUnavailableError()
	}
	title := strings.TrimSpace(listingTitle)
	if title == "" {
		title = "你的上车申请"
	}
	applicationID = strings.TrimSpace(applicationID)
	deadline := formatOptionalEmailTime(joinDeadline)
	detailURL := s.frontendOrigin + "/my/rides/" + url.PathEscape(applicationID)
	htmlBody, err := s.templates.renderCarpoolAcceptance(carpoolAcceptanceTemplateData{
		ListingTitle:  title,
		ApplicationID: emailReferenceID(applicationID, "CA"),
		JoinDeadline:  deadline,
		DetailURL:     detailURL,
	})
	if err != nil {
		return emailUnavailableError()
	}
	return s.send(ctx, emailMessage{
		To:       toEmail,
		Subject:  "C2CMarket 上车申请已被接受",
		TextBody: carpoolAcceptanceTextBody(title, emailReferenceID(applicationID, "CA"), deadline, detailURL),
		HTMLBody: htmlBody,
	})
}

func (s *SMTPEmailSender) SendAPIOrderCreated(ctx context.Context, toEmail, serviceTitle, orderID, amount, currency string, paymentExpiresAt, createdAt time.Time) *domain.AppError {
	if s == nil {
		return emailUnavailableError()
	}
	title := strings.TrimSpace(serviceTitle)
	if title == "" {
		title = "你的 API 服务"
	}
	orderID = strings.TrimSpace(orderID)
	detailURL := s.frontendOrigin + "/merchant/api-orders/" + url.PathEscape(orderID)
	htmlBody, err := s.templates.renderAPIOrder(apiOrderTemplateData{
		ServiceTitle:     title,
		OrderID:          emailReferenceID(orderID, "AO"),
		Amount:           formatEmailAmount(amount, currency),
		PaymentExpiresAt: formatEmailTime(paymentExpiresAt),
		CreatedAt:        formatEmailTime(createdAt),
		DetailURL:        detailURL,
	})
	if err != nil {
		return emailUnavailableError()
	}
	return s.send(ctx, emailMessage{
		To:       toEmail,
		Subject:  "你有一笔新的 API 订单｜C2CMarket",
		TextBody: apiOrderTextBody(title, emailReferenceID(orderID, "AO"), formatEmailAmount(amount, currency), paymentExpiresAt, createdAt, detailURL),
		HTMLBody: htmlBody,
	})
}

func (s *SMTPEmailSender) ExposeDevCode() bool {
	return false
}

func (s *SMTPEmailSender) send(ctx context.Context, message emailMessage) *domain.AppError {
	to, err := mail.ParseAddress(strings.TrimSpace(message.To))
	if err != nil {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Email invalid", "邮箱格式不正确。", "email", "invalid", "邮箱格式不正确。")
	}
	from := mail.Address{Name: s.fromName, Address: s.fromAddress}
	body, err := buildMIMEMessage(from, *to, message)
	if err != nil {
		return emailUnavailableError()
	}
	client, err := s.dial(ctx, s.host, net.JoinHostPort(s.host, strconv.Itoa(s.port)))
	if err != nil {
		return emailSendFailedError()
	}
	defer client.Close()
	if err := client.Auth(smtp.PlainAuth("", s.username, s.password, s.host)); err != nil {
		return emailSendFailedError()
	}
	if err := client.Mail(s.fromAddress); err != nil {
		return emailSendFailedError()
	}
	if err := client.Rcpt(to.Address); err != nil {
		return emailSendFailedError()
	}
	writer, err := client.Data()
	if err != nil {
		return emailSendFailedError()
	}
	if _, err := writer.Write(body); err != nil {
		_ = writer.Close()
		return emailSendFailedError()
	}
	if err := writer.Close(); err != nil {
		return emailSendFailedError()
	}
	if err := client.Quit(); err != nil {
		return emailSendFailedError()
	}
	return nil
}

type standardSMTPClient struct {
	*smtp.Client
}

func dialImplicitTLSSMTP(ctx context.Context, host, address string) (smtpClient, error) {
	dialer := net.Dialer{Timeout: 5 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return nil, err
	}
	tlsConn := tls.Client(conn, &tls.Config{
		ServerName: host,
		MinVersion: tls.VersionTLS12,
	})
	if err := tlsConn.HandshakeContext(ctx); err != nil {
		_ = conn.Close()
		return nil, err
	}
	client, err := smtp.NewClient(tlsConn, host)
	if err != nil {
		_ = tlsConn.Close()
		return nil, err
	}
	return standardSMTPClient{Client: client}, nil
}

type emailMessage struct {
	To       string
	Subject  string
	TextBody string
	HTMLBody string
}

func buildMIMEMessage(from, to mail.Address, message emailMessage) ([]byte, error) {
	boundary := "c2cmarket-" + randomHex(12)
	var buf bytes.Buffer
	writeHeader(&buf, "From", from.String())
	writeHeader(&buf, "To", to.String())
	writeHeader(&buf, "Subject", mime.QEncoding.Encode("utf-8", strings.TrimSpace(message.Subject)))
	writeHeader(&buf, "MIME-Version", "1.0")
	writeHeader(&buf, "Content-Type", `multipart/alternative; boundary="`+boundary+`"`)
	buf.WriteString("\r\n")
	writeMIMEPart(&buf, boundary, "text/plain", message.TextBody)
	writeMIMEPart(&buf, boundary, "text/html", message.HTMLBody)
	buf.WriteString("--" + boundary + "--\r\n")
	return buf.Bytes(), nil
}

func writeHeader(buf *bytes.Buffer, key, value string) {
	buf.WriteString(key)
	buf.WriteString(": ")
	buf.WriteString(strings.ReplaceAll(value, "\n", ""))
	buf.WriteString("\r\n")
}

func writeMIMEPart(buf *bytes.Buffer, boundary, contentType, body string) {
	buf.WriteString("--" + boundary + "\r\n")
	buf.WriteString("Content-Type: " + contentType + "; charset=UTF-8\r\n")
	buf.WriteString("Content-Transfer-Encoding: quoted-printable\r\n\r\n")
	writer := quotedprintable.NewWriter(buf)
	_, _ = writer.Write([]byte(body))
	_ = writer.Close()
	buf.WriteString("\r\n")
}

func randomHex(size int) string {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "static-boundary"
	}
	return hex.EncodeToString(buf)
}

type emailTemplates struct {
	verification       *template.Template
	registration       *template.Template
	carpoolApplication *template.Template
	carpoolAcceptance  *template.Template
	apiOrder           *template.Template
}

func newEmailTemplates() (*emailTemplates, error) {
	verification, err := template.New("verification").Parse(verificationHTMLTemplate)
	if err != nil {
		return nil, err
	}
	registration, err := template.New("registration").Parse(registrationHTMLTemplate)
	if err != nil {
		return nil, err
	}
	carpoolApplication, err := template.New("carpool_application").Parse(carpoolApplicationHTMLTemplate)
	if err != nil {
		return nil, err
	}
	carpoolAcceptance, err := template.New("carpool_acceptance").Parse(carpoolAcceptanceHTMLTemplate)
	if err != nil {
		return nil, err
	}
	apiOrder, err := template.New("api_order").Parse(apiOrderHTMLTemplate)
	if err != nil {
		return nil, err
	}
	return &emailTemplates{
		verification:       verification,
		registration:       registration,
		carpoolApplication: carpoolApplication,
		carpoolAcceptance:  carpoolAcceptance,
		apiOrder:           apiOrder,
	}, nil
}

type verificationTemplateData struct {
	Code string
}

type registrationTemplateData struct {
	DisplayName  string
	Username     string
	RegisteredAt string
	ProfileURL   string
}

type carpoolApplicationTemplateData struct {
	ListingTitle  string
	ApplicationID string
	CreatedAt     string
	DetailURL     string
}

type carpoolAcceptanceTemplateData struct {
	ListingTitle  string
	ApplicationID string
	JoinDeadline  string
	DetailURL     string
}

type apiOrderTemplateData struct {
	ServiceTitle     string
	OrderID          string
	Amount           string
	PaymentExpiresAt string
	CreatedAt        string
	DetailURL        string
}

func (t *emailTemplates) renderVerification(data verificationTemplateData) (string, error) {
	var buf bytes.Buffer
	err := t.verification.Execute(&buf, data)
	return buf.String(), err
}

func (t *emailTemplates) renderRegistration(data registrationTemplateData) (string, error) {
	var buf bytes.Buffer
	err := t.registration.Execute(&buf, data)
	return buf.String(), err
}

func (t *emailTemplates) renderCarpoolApplication(data carpoolApplicationTemplateData) (string, error) {
	var buf bytes.Buffer
	err := t.carpoolApplication.Execute(&buf, data)
	return buf.String(), err
}

func (t *emailTemplates) renderCarpoolAcceptance(data carpoolAcceptanceTemplateData) (string, error) {
	var buf bytes.Buffer
	err := t.carpoolAcceptance.Execute(&buf, data)
	return buf.String(), err
}

func (t *emailTemplates) renderAPIOrder(data apiOrderTemplateData) (string, error) {
	var buf bytes.Buffer
	err := t.apiOrder.Execute(&buf, data)
	return buf.String(), err
}

func verificationTextBody(code string, _ time.Time) string {
	return fmt.Sprintf("你正在进行 C2CMarket 邮箱验证，本次验证码为：\n\n%s\n\n验证码在 15 分钟内有效，请勿将验证码告知他人。C2CMarket 工作人员不会向你索要验证码。\n\n若非本人操作，请忽略本邮件。%s", strings.TrimSpace(code), systemEmailFooterText)
}

func registrationTextBody(username, displayName string, registeredAt time.Time, profileURL string) string {
	name := strings.TrimSpace(displayName)
	if name == "" {
		name = strings.TrimSpace(username)
	}
	if name == "" {
		name = "C2CMarket 用户"
	}
	usernameLine := ""
	if strings.TrimSpace(username) != "" {
		usernameLine = "\n账号：@" + strings.TrimSpace(username)
	}
	return fmt.Sprintf("你好，%s：\n\n你的 C2CMarket 账号已注册成功。%s\n注册时间：%s\n\n你现在可以前往个人中心，完善资料与常用联系方式。\n个人中心：%s\n\n若非本人操作，请及时检查账号状态。%s", name, usernameLine, formatEmailTime(registeredAt), strings.TrimSpace(profileURL), systemEmailFooterText)
}

func carpoolApplicationTextBody(listingTitle, applicationID string, createdAt time.Time, detailURL string) string {
	title := strings.TrimSpace(listingTitle)
	if title == "" {
		title = "你的车源"
	}
	return fmt.Sprintf("你的车源「%s」收到一条新的上车申请。\n申请编号：%s\n提交时间：%s\n\n请前往经营中心 → 上车申请，查看申请详情并及时处理。\n查看上车申请：%s\n\n上车申请仅表示买家希望加入，接受后才会进入确认上车阶段。%s", title, strings.TrimSpace(applicationID), formatEmailTime(createdAt), strings.TrimSpace(detailURL), systemEmailFooterText)
}

func carpoolAcceptanceTextBody(listingTitle, applicationID, joinDeadline, detailURL string) string {
	title := strings.TrimSpace(listingTitle)
	if title == "" {
		title = "你的上车申请"
	}
	deadlineLine := ""
	if strings.TrimSpace(joinDeadline) != "" {
		deadlineLine = "\n确认截止时间：" + strings.TrimSpace(joinDeadline)
	}
	return fmt.Sprintf("你的上车申请「%s」已被车主接受。\n申请编号：%s%s\n\n请查看车主联系方式，并在截止时间前完成“确认上车”。\n查看申请详情：%s%s", title, strings.TrimSpace(applicationID), deadlineLine, strings.TrimSpace(detailURL), systemEmailFooterText)
}

func apiOrderTextBody(serviceTitle, orderID, amount string, paymentExpiresAt, createdAt time.Time, detailURL string) string {
	title := strings.TrimSpace(serviceTitle)
	if title == "" {
		title = "你的 API 服务"
	}
	return fmt.Sprintf("你的 API 服务「%s」产生一笔新订单。\n订单状态：待买家付款\n订单编号：%s\n订单金额：%s\n创建时间：%s\n付款截止时间：%s\n\n请打开订单详情，等待买家付款，并在收款账户实际到账后确认收款。\n查看订单：%s\n\n温馨提示：订单已创建不代表款项已到账，请以你的收款账户实际到账为准。%s", title, strings.TrimSpace(orderID), strings.TrimSpace(amount), formatEmailTime(createdAt), formatEmailTime(paymentExpiresAt), strings.TrimSpace(detailURL), systemEmailFooterText)
}

func emailReferenceID(value, prefix string) string {
	var compact strings.Builder
	for _, char := range strings.TrimSpace(value) {
		if char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' {
			compact.WriteRune(char)
		}
	}
	suffix := compact.String()
	if len(suffix) > 6 {
		suffix = suffix[len(suffix)-6:]
	}
	if suffix == "" {
		suffix = strings.TrimSpace(value)
		if len(suffix) > 6 {
			suffix = suffix[len(suffix)-6:]
		}
	}
	suffix = strings.ToUpper(suffix)
	if strings.TrimSpace(prefix) == "" {
		return suffix
	}
	return strings.ToUpper(strings.TrimSpace(prefix)) + "-" + suffix
}

func formatEmailAmount(amount, currency string) string {
	amount = strings.TrimSpace(amount)
	currency = strings.ToUpper(strings.TrimSpace(currency))
	if currency == "CNY" || currency == "" {
		return "¥" + amount
	}
	return amount + " " + currency
}

func formatEmailTime(value time.Time) string {
	return value.In(beijingEmailLocation).Format("2006-01-02 15:04:05") + "（北京时间）"
}

func formatOptionalEmailTime(value *time.Time) string {
	if value == nil || value.IsZero() {
		return ""
	}
	return formatEmailTime(*value)
}

func emailUnavailableError() *domain.AppError {
	return domain.NewError(http.StatusBadGateway, domain.CodeInternalError, "Email sender unavailable", "邮件服务未配置。")
}

func emailSendFailedError() *domain.AppError {
	return domain.NewError(http.StatusBadGateway, domain.CodeInternalError, "Email send failed", "邮件发送失败，请稍后重试。")
}

const systemEmailFooterText = "\n\n此邮件由系统自动发送，请勿直接回复。"

const systemEmailFooterHTML = `<p style="margin-top:24px;color:#64748b;font-size:12px">此邮件由系统自动发送，请勿直接回复。</p>`

const verificationHTMLTemplate = `<p>你正在进行 C2CMarket 邮箱验证，本次验证码为：</p><p style="font-size:24px;font-weight:700;letter-spacing:4px">{{.Code}}</p><p>验证码在 <strong>15 分钟内有效</strong>，请勿将验证码告知他人。C2CMarket 工作人员不会向你索要验证码。</p><p>若非本人操作，请忽略本邮件。</p>` + systemEmailFooterHTML

const registrationHTMLTemplate = `<p>你好，{{if .DisplayName}}{{.DisplayName}}{{else}}C2CMarket 用户{{end}}：</p><p>你的 C2CMarket 账号已注册成功。</p>{{if .Username}}<p>账号：@{{.Username}}</p>{{end}}<p>注册时间：{{.RegisteredAt}}</p><p>你现在可以前往个人中心，完善资料与常用联系方式。</p><p><a href="{{.ProfileURL}}" style="display:inline-block;padding:10px 18px;border-radius:6px;background:#2563eb;color:#ffffff;text-decoration:none;">前往个人中心</a></p><p>若非本人操作，请及时检查账号状态。</p>` + systemEmailFooterHTML

const carpoolApplicationHTMLTemplate = `<p>你的车源「{{.ListingTitle}}」收到一条新的上车申请。</p><p>申请编号：{{.ApplicationID}}</p><p>提交时间：{{.CreatedAt}}</p><p>请前往经营中心 → 上车申请，查看申请详情并及时处理。</p><p><a href="{{.DetailURL}}" style="display:inline-block;padding:10px 18px;border-radius:6px;background:#2563eb;color:#ffffff;text-decoration:none;">查看上车申请</a></p><p>上车申请仅表示买家希望加入，接受后才会进入确认上车阶段。</p>` + systemEmailFooterHTML

const carpoolAcceptanceHTMLTemplate = `<p>你的上车申请「{{.ListingTitle}}」已被车主接受。</p><p>申请编号：{{.ApplicationID}}</p>{{if .JoinDeadline}}<p>确认截止时间：{{.JoinDeadline}}</p>{{end}}<p>请查看车主联系方式，并在截止时间前完成“确认上车”。</p><p><a href="{{.DetailURL}}" style="display:inline-block;padding:10px 18px;border-radius:6px;background:#2563eb;color:#ffffff;text-decoration:none;">查看申请详情</a></p>` + systemEmailFooterHTML

const apiOrderHTMLTemplate = `<p>你的 API 服务「{{.ServiceTitle}}」产生一笔新订单。</p><p>订单状态：<strong>待买家付款</strong></p><p>订单编号：{{.OrderID}}</p><p>订单金额：{{.Amount}}</p><p>创建时间：{{.CreatedAt}}</p><p>付款截止时间：{{.PaymentExpiresAt}}</p><p>请打开订单详情，等待买家付款，并在收款账户实际到账后确认收款。</p><p><a href="{{.DetailURL}}" style="display:inline-block;padding:10px 18px;border-radius:6px;background:#2563eb;color:#ffffff;text-decoration:none;">查看 API 订单</a></p><p><strong>温馨提示：</strong>订单已创建不代表款项已到账，请以你的收款账户实际到账为准。</p>` + systemEmailFooterHTML
