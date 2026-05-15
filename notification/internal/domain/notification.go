package domain

type SaveProcessedMessageParams struct {
	EventId       string
	AggregateId   string
	AggregateType string
	EventType     string
}
