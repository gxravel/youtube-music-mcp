package youtube

import (
	"context"
	"fmt"

	youtube_v3 "google.golang.org/api/youtube/v3"
)

// Subscription represents a YouTube channel subscription
type Subscription struct {
	ChannelID   string
	Title       string
	Description string
}

// GetSubscriptions retrieves ALL of the user's channel subscriptions with no pagination cap.
func (c *Client) GetSubscriptions(ctx context.Context) ([]Subscription, error) {
	var subscriptions []Subscription
	subscriptionsCall := c.service.Subscriptions.
		List([]string{"snippet"}).
		Mine(true).
		MaxResults(50)

	err := subscriptionsCall.Pages(ctx, func(response *youtube_v3.SubscriptionListResponse) error {
		// Check context cancellation
		if err := ctx.Err(); err != nil {
			return err
		}

		// Extract subscriptions from this page
		for _, item := range response.Items {
			subscriptions = append(subscriptions, Subscription{
				ChannelID:   item.Snippet.ResourceId.ChannelId,
				Title:       item.Snippet.Title,
				Description: item.Snippet.Description,
			})
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to retrieve subscriptions: %w", err)
	}

	return subscriptions, nil
}
