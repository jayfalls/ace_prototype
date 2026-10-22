package cognitive

import (
	"context"
	"testing"
	"time"

	"ace/internal/sandbox"
	"github.com/stretchr/testify/require"
)

func TestAutonomousTelegraphAndMailTmAndSSRF(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	jail, err := sandbox.NewJail(t.TempDir())
	require.NoError(t, err)

	// 1. SSRF guard on SafeHTTPRequest
	_, err = SafeHTTPRequest(ctx, "POST http://127.0.0.1:8080/api/state", `{"test":1}`)
	require.Error(t, err)

	// 2. Autonomous Telegraph Account Creation + Public Page Publishing
	pubRes, err := PublishToTelegraph(ctx, jail, "ACE Autonomous Launch Test", "# LLM Cost & Regression Guard\n- Autonomous B2B Micro-SaaS\n- Built inside isolated bwrap sandbox")
	if err == nil {
		require.Contains(t, pubRes, "https://telegra.ph/")
		acctJSON, err := jail.ReadFile(".telegraph_account.json")
		require.NoError(t, err)
		require.Contains(t, acctJSON, "access_token")
	}

	// 3. Autonomous Mail.tm Email Inbox Provisioning
	mailRes, err := ExecuteMailTm(ctx, jail, "create")
	if err == nil {
		require.Contains(t, mailRes, "@")
		inboxRes, err := ExecuteMailTm(ctx, jail, "inbox")
		require.NoError(t, err)
		require.Contains(t, inboxRes, "Inbox ")
	}
}
