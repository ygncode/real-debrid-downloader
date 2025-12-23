package realdebrid

import (
	"context"
	"fmt"
	"net/url"

	"github.com/ygncode/real-debrid-downloader/internal/models"
)

// UnrestrictLink converts a Real-Debrid hosted link to a direct download link
func (c *Client) UnrestrictLink(ctx context.Context, link string) (*models.UnrestrictedLink, error) {
	data := url.Values{}
	data.Set("link", link)

	var result models.UnrestrictedLink
	if err := c.post(ctx, "/unrestrict/link", data, &result); err != nil {
		return nil, fmt.Errorf("failed to unrestrict link: %w", err)
	}

	return &result, nil
}
