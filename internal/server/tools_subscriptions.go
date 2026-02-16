package server

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Input/output types for subscriptions tool

type getSubscriptionsInput struct {
	MaxResults int64 `json:"maxResults" jsonschema:"description=Maximum number of subscriptions to return (default 25),minimum=1,maximum=500"`
}

type subscriptionInfo struct {
	ChannelID   string `json:"channelId" jsonschema:"description=YouTube channel ID"`
	Title       string `json:"title" jsonschema:"description=Channel title"`
	Description string `json:"description" jsonschema:"description=Channel description"`
}

type subscriptionsOutput struct {
	Subscriptions []subscriptionInfo `json:"subscriptions"`
	Count         int                `json:"count" jsonschema:"description=Number of subscriptions returned"`
}

// registerSubscriptionTools registers all subscription-related MCP tools
func (s *Server) registerSubscriptionTools() {
	// Tool: get_subscriptions
	mcp.AddTool(s.mcpServer, &mcp.Tool{
		Name:        "get_subscriptions",
		Description: "Retrieve the user's channel subscriptions from YouTube. These represent artists and channels the user follows. Quota cost: ~1 unit per 50 subscriptions.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, input getSubscriptionsInput) (*mcp.CallToolResult, subscriptionsOutput, error) {
		// Call YouTube client
		subscriptions, err := s.ytClient.GetSubscriptions(ctx, input.MaxResults)
		if err != nil {
			return nil, subscriptionsOutput{}, fmt.Errorf("failed to get subscriptions: %w", err)
		}

		// Convert to output format
		subscriptionInfos := make([]subscriptionInfo, len(subscriptions))
		for i, sub := range subscriptions {
			subscriptionInfos[i] = subscriptionInfo{
				ChannelID:   sub.ChannelID,
				Title:       sub.Title,
				Description: sub.Description,
			}
		}

		output := subscriptionsOutput{
			Subscriptions: subscriptionInfos,
			Count:         len(subscriptionInfos),
		}

		// Return result with summary
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Retrieved %d channel subscriptions", len(subscriptions))},
			},
		}, output, nil
	})
}
