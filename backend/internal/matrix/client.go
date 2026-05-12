package matrix

import (
	"context"
	"fmt"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/id"
)

// Client wraps a mautrix.Client. If accessToken is empty the client is disabled
// and all operations become no-ops.
type Client struct {
	*mautrix.Client
	enabled bool
}

// New creates a Matrix client from homeserver URL, bot user ID, and access token.
// When any of the three is empty the client is disabled.
func New(homeserverURL, userID, accessToken string) (*Client, error) {
	if homeserverURL == "" || userID == "" || accessToken == "" {
		return &Client{enabled: false}, nil
	}
	mc, err := mautrix.NewClient(homeserverURL, id.UserID(userID), accessToken)
	if err != nil {
		return nil, fmt.Errorf("matrix: create client: %w", err)
	}

	// Sanity-check the token by hitting /whoami.
	whoami, err := mc.Whoami(context.Background())
	if err != nil {
		return nil, fmt.Errorf("matrix: whoami failed (check homeserver/token): %w", err)
	}
	if whoami.UserID.String() != userID {
		return nil, fmt.Errorf("matrix: token belongs to %s, expected %s", whoami.UserID, userID)
	}

	return &Client{Client: mc, enabled: true}, nil
}

// Enabled returns true when the client has valid credentials configured.
func (c *Client) Enabled() bool {
	return c != nil && c.enabled && c.Client != nil
}

// UserID returns the bot's MXID (or an empty ID if disabled).
func (c *Client) UserID() id.UserID {
	if !c.Enabled() {
		return ""
	}
	return c.Client.UserID
}
