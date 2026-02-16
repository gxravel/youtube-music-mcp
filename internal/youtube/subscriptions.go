package youtube

import (
	"context"
	"errors"
	"fmt"

	youtube_v3 "google.golang.org/api/youtube/v3"
)

// Subscription represents a YouTube channel subscription
type Subscription struct {
	ChannelID   string
	Title       string
	Description string
}

// GetSubscriptions retrieves the user's channel subscriptions
func (c *Client) GetSubscriptions(ctx context.Context, maxResults int64) ([]Subscription, error) {
	// Default to 25 if not specified
	if maxResults <= 0 {
		maxResults = 25
	}

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

			// Stop if we've reached the requested count
			if int64(len(subscriptions)) >= maxResults {
				return errStopPagination
			}
		}

		return nil
	})

	// Handle pagination stop
	if err != nil && !errors.Is(err, errStopPagination) {
		return nil, fmt.Errorf("failed to retrieve subscriptions: %w", err)
	}

	// Truncate to maxResults
	if int64(len(subscriptions)) > maxResults {
		subscriptions = subscriptions[:maxResults]
	}

	return subscriptions, nil
}
