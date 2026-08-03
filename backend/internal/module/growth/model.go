package growth

import "time"

const (
	BusinessTimezone  = "Asia/Shanghai"
	DefaultWindowDays = 30
)

var SupportedWindowDays = []int{7, 30, 90}

type Summary struct {
	NewUsersToday                int      `json:"newUsersToday"`
	NewUsers7d                   int      `json:"newUsers7d"`
	NewUsers30d                  int      `json:"newUsers30d"`
	NewUsersInWindow             int      `json:"newUsersInWindow"`
	CumulativeEffectiveUsers     int      `json:"cumulativeEffectiveUsers"`
	ActivatedUsers               int      `json:"activatedUsers"`
	ActivationRate               *float64 `json:"activationRate"`
	MedianActivationHours        *float64 `json:"medianActivationHours"`
	DAU                          int      `json:"dau"`
	WAU                          int      `json:"wau"`
	MAU                          int      `json:"mau"`
	D1RetentionRate              *float64 `json:"d1RetentionRate"`
	D7RetentionRate              *float64 `json:"d7RetentionRate"`
	CompletedCarpoolTransactions int      `json:"completedCarpoolTransactions"`
	CompletedAPITransactions     int      `json:"completedApiTransactions"`
}

type RegistrationTrendPoint struct {
	Date            string `json:"date"`
	NewUsers        int    `json:"newUsers"`
	CumulativeUsers int    `json:"cumulativeUsers"`
}

type ActivityTrendPoint struct {
	Date        string `json:"date"`
	ActiveUsers int    `json:"activeUsers"`
}

type AttributionGroup struct {
	SourceType    string  `json:"sourceType"`
	Source        string  `json:"source"`
	Medium        string  `json:"medium,omitempty"`
	Campaign      string  `json:"campaign,omitempty"`
	Registrations int     `json:"registrations"`
	Share         float64 `json:"share"`
}

type Activation struct {
	CohortUsers          int      `json:"cohortUsers"`
	BuyerActivatedUsers  int      `json:"buyerActivatedUsers"`
	BuyerActivationRate  *float64 `json:"buyerActivationRate"`
	SellerActivatedUsers int      `json:"sellerActivatedUsers"`
	SellerActivationRate *float64 `json:"sellerActivationRate"`
	ActivatedUsers       int      `json:"activatedUsers"`
	ActivationRate       *float64 `json:"activationRate"`
}

type RetentionCohort struct {
	CohortDate      string   `json:"cohortDate"`
	RegisteredUsers int      `json:"registeredUsers"`
	D1RetainedUsers *int     `json:"d1RetainedUsers"`
	D1Rate          *float64 `json:"d1Rate"`
	D7RetainedUsers *int     `json:"d7RetainedUsers"`
	D7Rate          *float64 `json:"d7Rate"`
}

type Overview struct {
	GeneratedAt       time.Time                `json:"generatedAt"`
	Timezone          string                   `json:"timezone"`
	WindowDays        int                      `json:"windowDays"`
	Summary           Summary                  `json:"summary"`
	RegistrationTrend []RegistrationTrendPoint `json:"registrationTrend"`
	ActivityTrend     []ActivityTrendPoint     `json:"activityTrend"`
	Attribution       []AttributionGroup       `json:"attribution"`
	Activation        Activation               `json:"activation"`
	RetentionCohorts  []RetentionCohort        `json:"retentionCohorts"`
}
