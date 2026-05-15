package worker

import "time"

type DepositSuccessPayload struct {
	UserId    string     `json:"user_id"`
	Amount    int64      `json:"amount"`
	TxType    string     `json:"tx_type"`
	Timestamp *time.Time `json:"timestamp"`
}

type AccountRegisteredPayload struct {
	Email     string     `json:"email"`
	UserId    string     `json:"user_id"`
	Timestamp *time.Time `json:"timestamp"`
}
