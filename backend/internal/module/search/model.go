package search

import "time"

const (
	TypeOfficialPrice = "官方价格"
	TypeCarpool       = "车源"
	TypeDemand        = "求车"
	TypeAPIService    = "API 服务"
	TypeUser          = "用户"
	TypeMerchant      = "商户"
)

type Result struct {
	ID       string
	Type     string
	Title    string
	Subtitle string
	Badge    string
	To       string
	RankTime time.Time
}
