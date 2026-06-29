package domain

import "net/http"

func NewAPIPurchaseIntentHasOrderError() *AppError {
	return NewError(
		http.StatusConflict,
		CodeAPIPurchaseIntentHasOrder,
		"API purchase intent has order",
		"该购买意向已生成订单，不能再按普通购买意向取消或关闭，请前往订单页继续处理。",
	)
}
