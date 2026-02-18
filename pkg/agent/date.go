package agent

import (
	"time"

	"google.golang.org/adk/tool"
)

type GetDateArgs struct{}

type GetDateResp struct {
	Today string `json:"current_date"`
}

func getDate(_ tool.Context, a GetDateArgs) (GetDateResp, error) {
	return GetDateResp{
		Today: time.Now().Format(time.RFC3339),
	}, nil
}
