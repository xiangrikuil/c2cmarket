package devpersona

import (
	"net/http"

	"c2c-market/backend/internal/domain"
	"c2c-market/backend/internal/module/auth"
)

type Persona string

const (
	Buyer  Persona = "buyer"
	Seller Persona = "seller"
	Admin  Persona = "admin"

	SharedPassword = "DevPersona#2026"
)

type Definition struct {
	Persona     Persona
	Username    string
	DisplayName string
	Email       string
	IsAdmin     bool
}

type Result struct {
	Persona Persona
	User    auth.User
	Session auth.Session
}

var definitions = map[Persona]Definition{
	Buyer: {
		Persona:     Buyer,
		Username:    "dev-buyer",
		DisplayName: "开发买家",
		Email:       "dev-buyer@example.test",
	},
	Seller: {
		Persona:     Seller,
		Username:    "dev-seller",
		DisplayName: "开发卖家",
		Email:       "dev-seller@example.test",
	},
	Admin: {
		Persona:     Admin,
		Username:    "dev-admin",
		DisplayName: "开发管理员",
		Email:       "dev-admin@example.test",
		IsAdmin:     true,
	},
}

func Parse(value string) (Definition, *domain.AppError) {
	persona := Persona(value)
	definition, ok := definitions[persona]
	if ok {
		return definition, nil
	}
	return Definition{}, domain.NewFieldError(
		http.StatusUnprocessableEntity,
		domain.CodeValidationFailed,
		"Development persona invalid",
		"开发身份只能选择 buyer、seller 或 admin。",
		"persona",
		"invalid",
		"开发身份只能选择 buyer、seller 或 admin。",
	)
}

func Values() []Persona {
	return []Persona{Buyer, Seller, Admin}
}
