package profile

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/auth"

	"github.com/google/uuid"
)

type Service struct {
	mu              sync.Mutex
	now             func() time.Time
	repo            Repository
	profiles        map[string]UserProfile
	profilesByName  map[string]string
	merchantByOwner map[string]MerchantProfile
	merchantBySlug  map[string]string
	emailCodes      map[string]emailChallenge
	emailSender     EmailSender
}

type emailChallenge struct {
	UserID    string
	Email     string
	CodeHash  string
	ExpiresAt time.Time
	Consumed  bool
}

func NewService(repo Repository, now func() time.Time) *Service {
	return NewServiceWithEmailSender(repo, now, NewDevelopmentEmailSender())
}

func NewServiceWithEmailSender(repo Repository, now func() time.Time, emailSender EmailSender) *Service {
	if now == nil {
		now = time.Now
	}
	if emailSender == nil {
		emailSender = NewDevelopmentEmailSender()
	}
	return &Service{
		now:             now,
		repo:            repo,
		profiles:        make(map[string]UserProfile),
		profilesByName:  make(map[string]string),
		merchantByOwner: make(map[string]MerchantProfile),
		merchantBySlug:  make(map[string]string),
		emailCodes:      make(map[string]emailChallenge),
		emailSender:     emailSender,
	}
}

func (s *Service) MyProfile(ctx context.Context, user auth.User) (UserProfile, *domain.AppError) {
	if s.repo != nil {
		return s.repo.GetUserProfile(ctx, user.ID, s.now())
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ensureProfileLocked(user), nil
}

func (s *Service) UpdateMyProfile(ctx context.Context, user auth.User, input UpdateUserProfileInput) (UserProfile, *domain.AppError) {
	input.UserID = user.ID
	if appErr := validateProfileInput(input); appErr != nil {
		return UserProfile{}, appErr
	}
	if s.repo != nil {
		return s.repo.UpdateUserProfile(ctx, input, s.now())
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	profile := s.ensureProfileLocked(user)
	username := normalizeUsername(input.Username)
	if username == "" {
		username = profile.Username
	}
	if existingID := s.profilesByName[username]; existingID != "" && existingID != user.ID {
		return UserProfile{}, domain.NewFieldError(http.StatusConflict, domain.CodeValidationFailed, "Username unavailable", "站内用户名已被占用。", "username", "unavailable", "站内用户名已被占用。")
	}
	delete(s.profilesByName, profile.Username)
	profile.Username = username
	profile.DisplayName = strings.TrimSpace(input.DisplayName)
	profile.Bio = strings.TrimSpace(input.Bio)
	profile.RegionCode = strings.TrimSpace(input.RegionCode)
	profile.Timezone = strings.TrimSpace(input.Timezone)
	profile.AvatarMode = normalizeAvatarMode(input.AvatarMode)
	profile.CustomAvatarURL = ""
	if profile.AvatarMode == "custom_url" {
		profile.CustomAvatarURL = strings.TrimSpace(input.AvatarURL)
		profile.AvatarURL = profile.CustomAvatarURL
	} else {
		profile.AvatarURL = profile.LinuxDoAvatarURL
	}
	profile.Privacy = normalizePrivacy(input.Privacy)
	profile.UpdatedAt = s.now()
	profile.Version++
	s.profiles[user.ID] = profile
	s.profilesByName[profile.Username] = user.ID
	return profile, nil
}

func (s *Service) StartEmailVerification(ctx context.Context, user auth.User, input EmailVerificationStartInput) (EmailVerificationChallenge, *domain.AppError) {
	input.UserID = user.ID
	input.Email = normalizeEmail(input.Email)
	if err := validateEmail(input.Email); err != nil {
		return EmailVerificationChallenge{}, err
	}
	now := s.now()
	code := newEmailVerificationCode()
	expiresAt := now.Add(15 * time.Minute)
	codeHash := emailCodeHash(user.ID, input.Email, code)
	if s.repo != nil {
		if appErr := s.repo.CreateEmailVerificationCode(ctx, input, codeHash, expiresAt, now); appErr != nil {
			return EmailVerificationChallenge{}, appErr
		}
		if appErr := s.emailSender.SendVerificationCode(ctx, input.Email, code, expiresAt); appErr != nil {
			return EmailVerificationChallenge{}, appErr
		}
		return s.emailChallengeResponse(input.Email, expiresAt, code), nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	for _, existing := range s.profiles {
		if existing.ID != user.ID && existing.Email == input.Email && existing.EmailVerifiedAt != nil {
			return EmailVerificationChallenge{}, domain.NewFieldError(http.StatusConflict, domain.CodeValidationFailed, "Email unavailable", "该邮箱已绑定其他账号。", "email", "unavailable", "该邮箱已绑定其他账号。")
		}
	}
	s.emailCodes[emailChallengeKey(user.ID, input.Email)] = emailChallenge{
		UserID:    user.ID,
		Email:     input.Email,
		CodeHash:  codeHash,
		ExpiresAt: expiresAt,
	}
	if appErr := s.emailSender.SendVerificationCode(ctx, input.Email, code, expiresAt); appErr != nil {
		return EmailVerificationChallenge{}, appErr
	}
	return s.emailChallengeResponse(input.Email, expiresAt, code), nil
}

func (s *Service) ConfirmEmailVerification(ctx context.Context, user auth.User, input EmailVerificationConfirmInput) (UserProfile, *domain.AppError) {
	input.UserID = user.ID
	input.Email = normalizeEmail(input.Email)
	input.Code = strings.TrimSpace(input.Code)
	if err := validateEmail(input.Email); err != nil {
		return UserProfile{}, err
	}
	if !emailCodePattern.MatchString(input.Code) {
		return UserProfile{}, domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Code invalid", "验证码格式不正确。", "code", "invalid", "验证码格式不正确。")
	}
	codeHash := emailCodeHash(user.ID, input.Email, input.Code)
	if s.repo != nil {
		return s.repo.ConfirmEmailVerificationCode(ctx, input, codeHash, s.now())
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	challenge, ok := s.emailCodes[emailChallengeKey(user.ID, input.Email)]
	if !ok || challenge.Consumed || challenge.CodeHash != codeHash || !s.now().Before(challenge.ExpiresAt) {
		return UserProfile{}, domain.NewError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Code invalid", "验证码无效或已过期。")
	}
	for _, existing := range s.profiles {
		if existing.ID != user.ID && existing.Email == input.Email && existing.EmailVerifiedAt != nil {
			return UserProfile{}, domain.NewFieldError(http.StatusConflict, domain.CodeValidationFailed, "Email unavailable", "该邮箱已绑定其他账号。", "email", "unavailable", "该邮箱已绑定其他账号。")
		}
	}
	profile := s.ensureProfileLocked(user)
	now := s.now()
	profile.Email = input.Email
	profile.EmailVerifiedAt = &now
	profile.UpdatedAt = now
	profile.Version++
	challenge.Consumed = true
	s.emailCodes[emailChallengeKey(user.ID, input.Email)] = challenge
	s.profiles[user.ID] = profile
	return profile, nil
}

func (s *Service) PublicUserProfile(ctx context.Context, username string) (PublicUserProfile, *domain.AppError) {
	if s.repo != nil {
		return s.repo.GetPublicUserProfile(ctx, username, s.now())
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	id := s.profilesByName[normalizeUsername(username)]
	if id == "" {
		return PublicUserProfile{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Profile not found", "公开主页不存在。")
	}
	return toPublicUserProfile(s.profiles[id]), nil
}

func (s *Service) MyMerchantProfile(ctx context.Context, user auth.User) (MerchantProfile, *domain.AppError) {
	if s.repo != nil {
		return s.repo.GetMerchantProfile(ctx, user.ID, s.now())
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	merchant, ok := s.merchantByOwner[user.ID]
	if !ok {
		return MerchantProfile{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Merchant profile not found", "商户资料不存在。")
	}
	return merchant, nil
}

func (s *Service) UpsertMyMerchantProfile(ctx context.Context, user auth.User, input UpsertMerchantProfileInput) (MerchantProfile, *domain.AppError) {
	input.OwnerUserID = user.ID
	if appErr := validateMerchantInput(input); appErr != nil {
		return MerchantProfile{}, appErr
	}
	if s.repo != nil {
		return s.repo.UpsertMerchantProfile(ctx, input, s.now())
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.now()
	slug := normalizeSlug(input.Slug)
	if slug == "" {
		slug = normalizeSlug(input.DisplayName)
	}
	if slug == "" {
		slug = normalizeSlug(user.Username)
	}
	if ownerID := s.merchantBySlug[slug]; ownerID != "" && ownerID != user.ID {
		return MerchantProfile{}, domain.NewFieldError(http.StatusConflict, domain.CodeValidationFailed, "Merchant slug unavailable", "店铺别名已被占用。", "slug", "unavailable", "店铺别名已被占用。")
	}
	merchant, exists := s.merchantByOwner[user.ID]
	if exists {
		delete(s.merchantBySlug, merchant.Slug)
		merchant.Slug = slug
		merchant.DisplayName = strings.TrimSpace(input.DisplayName)
		merchant.AvatarURL = strings.TrimSpace(input.AvatarURL)
		merchant.UpdatedAt = now
		merchant.Version++
	} else {
		merchant = MerchantProfile{
			ID:          uuid.NewString(),
			OwnerUserID: user.ID,
			Slug:        slug,
			DisplayName: strings.TrimSpace(input.DisplayName),
			AvatarURL:   strings.TrimSpace(input.AvatarURL),
			Status:      "active",
			CreatedAt:   now,
			UpdatedAt:   now,
			Version:     1,
		}
	}
	s.merchantByOwner[user.ID] = merchant
	s.merchantBySlug[merchant.Slug] = user.ID
	return merchant, nil
}

func (s *Service) PublicMerchantProfile(ctx context.Context, slug string) (PublicMerchantProfile, *domain.AppError) {
	if s.repo != nil {
		return s.repo.GetPublicMerchantProfile(ctx, slug, s.now())
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ownerID := s.merchantBySlug[normalizeSlug(slug)]
	if ownerID == "" {
		return PublicMerchantProfile{}, domain.NewError(http.StatusNotFound, domain.CodeObjectNotFound, "Merchant profile not found", "商户公开主页不存在。")
	}
	merchant := s.merchantByOwner[ownerID]
	return PublicMerchantProfile{
		ID:                merchant.ID,
		Slug:              merchant.Slug,
		DisplayName:       merchant.DisplayName,
		AvatarURL:         merchant.AvatarURL,
		AvatarText:        avatarText(merchant.DisplayName),
		Identity:          "API 商户",
		TrustLevel:        3,
		LinuxDoBound:      true,
		OriginalPostBound: false,
		JoinedAt:          merchant.CreatedAt,
		LastActiveAt:      nil,
	}, nil
}

func (s *Service) emailChallengeResponse(email string, expiresAt time.Time, code string) EmailVerificationChallenge {
	challenge := EmailVerificationChallenge{Email: email, ExpiresAt: expiresAt}
	if s.emailSender != nil && s.emailSender.ExposeDevCode() {
		challenge.DevCode = code
	}
	return challenge
}

func (s *Service) ensureProfileLocked(user auth.User) UserProfile {
	if profile, ok := s.profiles[user.ID]; ok {
		return profile
	}
	now := s.now()
	profile := UserProfile{
		ID:                user.ID,
		Username:          user.Username,
		DisplayName:       user.DisplayName,
		AccountStatus:     user.Status,
		IsAdmin:           user.IsAdmin,
		AvatarMode:        "linuxdo",
		Privacy:           defaultPrivacy(),
		CreatedAt:         now,
		UpdatedAt:         now,
		LastActiveAt:      &now,
		Version:           1,
		LinuxDoBound:      true,
		LinuxDoUsername:   user.Username,
		LinuxDoAvatarURL:  "",
		UsernameCanChange: true,
	}
	trust := 3
	profile.LinuxDoTrustLevel = &trust
	if user.LinuxDoBinding != nil && user.LinuxDoBinding.Bound {
		profile.LinuxDoBound = true
		profile.LinuxDoUserID = user.LinuxDoBinding.LinuxDoUserID
		profile.LinuxDoUsername = user.LinuxDoBinding.LinuxDoUsername
		profile.LinuxDoAvatarURL = user.LinuxDoBinding.AvatarURL
		profile.AvatarURL = user.LinuxDoBinding.AvatarURL
		profile.LinuxDoLastSyncedAt = &user.LinuxDoBinding.LastSyncedAt
		trust = user.LinuxDoBinding.TrustLevel
		profile.LinuxDoTrustLevel = &trust
	}
	s.profiles[user.ID] = profile
	s.profilesByName[profile.Username] = user.ID
	return profile
}

func validateProfileInput(input UpdateUserProfileInput) *domain.AppError {
	if strings.TrimSpace(input.DisplayName) == "" {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Display name required", "显示名称不能为空。", "displayName", "required", "显示名称不能为空。")
	}
	if utf8.RuneCountInString(strings.TrimSpace(input.DisplayName)) > 32 {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Display name too long", "显示名称最多 32 字。", "displayName", "too_long", "显示名称最多 32 字。")
	}
	username := normalizeUsername(input.Username)
	if username != "" && !usernamePattern.MatchString(username) {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Username invalid", "站内用户名只允许 3-24 位字母、数字、下划线和短横线。", "username", "invalid", "站内用户名格式不正确。")
	}
	if utf8.RuneCountInString(strings.TrimSpace(input.Bio)) > 160 {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Bio too long", "个人简介最多 160 字。", "bio", "too_long", "个人简介最多 160 字。")
	}
	switch normalizeAvatarMode(input.AvatarMode) {
	case "linuxdo":
	case "custom_url":
		if err := validateCustomAvatarURL(input.AvatarURL); err != nil {
			return err
		}
	default:
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Avatar mode invalid", "头像来源只能选择 linuxdo 或 custom_url。", "avatarMode", "invalid", "头像来源不正确。")
	}
	return nil
}

func validateMerchantInput(input UpsertMerchantProfileInput) *domain.AppError {
	if strings.TrimSpace(input.DisplayName) == "" {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Merchant display name required", "必须填写店铺展示名。", "displayName", "required", "必须填写店铺展示名。")
	}
	if utf8.RuneCountInString(strings.TrimSpace(input.DisplayName)) > 32 {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Merchant display name too long", "店铺展示名最多 32 字。", "displayName", "too_long", "店铺展示名最多 32 字。")
	}
	slug := normalizeSlug(input.Slug)
	if slug != "" && !usernamePattern.MatchString(slug) {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Merchant slug invalid", "店铺别名只允许 3-24 位字母、数字、下划线和短横线。", "slug", "invalid", "店铺别名格式不正确。")
	}
	return nil
}

func toPublicUserProfile(profile UserProfile) PublicUserProfile {
	createdAt := &profile.CreatedAt
	if !profile.Privacy.ShowCreatedAt {
		createdAt = nil
	}
	lastActiveAt := profile.LastActiveAt
	if !profile.Privacy.ShowLastActiveAt {
		lastActiveAt = nil
	}
	badges := []string{}
	if profile.LinuxDoBound {
		badges = append(badges, "linuxdo_bound")
	}
	if profile.IsAdmin {
		badges = append(badges, "admin")
	}
	return PublicUserProfile{
		ID:              profile.ID,
		Username:        profile.Username,
		DisplayName:     profile.DisplayName,
		Bio:             profile.Bio,
		AvatarURL:       profile.AvatarURL,
		AvatarText:      avatarText(profile.DisplayName),
		LinuxDoBound:    profile.LinuxDoBound,
		LinuxDoUsername: profile.LinuxDoUsername,
		TrustLevel:      profile.LinuxDoTrustLevel,
		AccountStatus:   profile.AccountStatus,
		CreatedAt:       createdAt,
		LastActiveAt:    lastActiveAt,
		Privacy:         profile.Privacy,
		Stats: PublicStats{
			BuyerResponsibilityCancellationCount:  0,
			SellerResponsibilityCancellationCount: 0,
			UnresolvedDisputeCount:                0,
		},
		Badges: badges,
	}
}

func normalizePrivacy(value PrivacySettings) PrivacySettings {
	return value
}

func defaultPrivacy() PrivacySettings {
	return PrivacySettings{
		ShowCreatedAt:               true,
		ShowLastActiveAt:            true,
		ShowCompletedCarpoolCount:   true,
		ShowCompletedAPIIntentCount: true,
		ShowResponseMedian:          true,
		ShowResolvedDisputeSummary:  true,
		AllowPublicProfileReport:    true,
	}
}

func normalizeAvatarMode(value string) string {
	switch strings.TrimSpace(value) {
	case "linuxdo":
		return "linuxdo"
	case "custom_url":
		return "custom_url"
	default:
		return ""
	}
}

func validateCustomAvatarURL(value string) *domain.AppError {
	raw := strings.TrimSpace(value)
	if raw == "" {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Avatar URL required", "自定义头像必须填写 HTTPS 图片 URL。", "avatarUrl", "required", "必须填写 HTTPS 图片 URL。")
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Avatar URL invalid", "自定义头像必须是 HTTPS 图片 URL。", "avatarUrl", "invalid", "必须是 HTTPS 图片 URL。")
	}
	lowerPath := strings.ToLower(parsed.Path)
	lowerQuery := strings.ToLower(parsed.RawQuery)
	if !(strings.HasSuffix(lowerPath, ".jpg") || strings.HasSuffix(lowerPath, ".jpeg") || strings.HasSuffix(lowerPath, ".png") || strings.HasSuffix(lowerPath, ".webp") || strings.Contains(lowerQuery, "format=jpg") || strings.Contains(lowerQuery, "format=jpeg") || strings.Contains(lowerQuery, "format=png") || strings.Contains(lowerQuery, "format=webp")) {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Avatar URL invalid", "自定义头像 URL 必须指向 JPG、PNG 或 WebP 图片。", "avatarUrl", "invalid_image", "必须指向 JPG、PNG 或 WebP 图片。")
	}
	return nil
}

func normalizeEmail(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func validateEmail(value string) *domain.AppError {
	if !emailPattern.MatchString(value) {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Email invalid", "邮箱格式不正确。", "email", "invalid", "邮箱格式不正确。")
	}
	if len(value) > 254 {
		return domain.NewFieldError(http.StatusUnprocessableEntity, domain.CodeValidationFailed, "Email too long", "邮箱长度不能超过 254 个字符。", "email", "too_long", "邮箱过长。")
	}
	return nil
}

func newEmailVerificationCode() string {
	var buf [4]byte
	if _, err := rand.Read(buf[:]); err != nil {
		panic(err)
	}
	number := int(buf[0])<<24 | int(buf[1])<<16 | int(buf[2])<<8 | int(buf[3])
	if number < 0 {
		number = -number
	}
	return leftPadCode(number%1000000, 6)
}

func leftPadCode(value int, width int) string {
	text := strconv.Itoa(value)
	for len(text) < width {
		text = "0" + text
	}
	return text
}

func emailCodeHash(userID, email, code string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(userID) + ":" + normalizeEmail(email) + ":" + strings.TrimSpace(code)))
	return hex.EncodeToString(sum[:])
}

func emailChallengeKey(userID, email string) string {
	return strings.TrimSpace(userID) + ":" + normalizeEmail(email)
}

func normalizeUsername(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, " ", "-")
	return value
}

func normalizeSlug(value string) string {
	return normalizeUsername(value)
}

func avatarText(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "U"
	}
	r, _ := utf8.DecodeRuneInString(value)
	return strings.ToUpper(string(r))
}

var usernamePattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{3,24}$`)
var emailPattern = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)
var emailCodePattern = regexp.MustCompile(`^[0-9]{6}$`)
