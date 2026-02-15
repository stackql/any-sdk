package providerinvoker

import (
	"context"
)

// Request carries an invoker-specific payload. Step 1 keeps this opaque so we can
// stabilise call-shapes in StackQL before lift/shift into any-sdk.
type Request struct {
	Payload any
}

// Result is a protocol-agnostic execution result from StackQL's point of view.
type Result struct {
	Body     any
	Messages []string
}

type Invoker interface {
	Invoke(ctx context.Context, req Request) (Result, error)
}

type ActionInsertPayload interface {
	GetItemisationResult() ItemisationResult
	IsHousekeepingDone() bool
	GetTableName() string
	GetParamsUsed() map[string]interface{}
	GetReqEncoding() string
}

type ItemisationResult interface {
	GetItems() (interface{}, bool)
	GetSingltetonResponse() (map[string]interface{}, bool)
	IsOk() bool
	IsNilPayload() bool
}

type ActionInsertResult interface {
	GetError() (error, bool)
	IsHousekeepingDone() bool
}

type InsertPreparator interface {
	ActionInsertPreparation(payload ActionInsertPayload) ActionInsertResult
}

type MessageHandler interface {
	LogHTTPResponseMap(any)
}
