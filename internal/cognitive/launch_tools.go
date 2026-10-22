package cognitive

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"ace/internal/core"
)

const webSearchUserAgent = "Mozilla/5.0 (X11; Linux x86_64; rv:128.0) Gecko/20100101 Firefox/128.0"

func newSafePublicHTTPClient() *http.Client {
	dialer := &net.Dialer{Timeout: 8 * time.Second}
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}
			ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
			if err != nil || len(ips) == 0 {
				return nil, fmt.Errorf("DNS resolution failed for %s: %w", host, err)
			}
			for _, ipAddr := range ips {
				ip := ipAddr.IP
				if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
					return nil, fmt.Errorf("SSRF blocked private/local IP %s for host %s", ip.String(), host)
				}
			}
			return dialer.DialContext(ctx, network, net.JoinHostPort(ips[0].IP.String(), port))
		},
	}
	return &http.Client{
		Timeout:   15 * time.Second,
		Transport: transport,
	}
}

type telegraphAccount struct {
	ShortName   string `json:"short_name"`
	AuthorName  string `json:"author_name"`
	AccessToken string `json:"access_token"`
	AuthURL     string `json:"auth_url"`
	CreatedAt   string `json:"created_at"`
}

type telegraphNode struct {
	Tag      string            `json:"tag"`
	Attrs    map[string]string `json:"attrs,omitempty"`
	Children []interface{}     `json:"children,omitempty"`
}

// PublishToTelegraph autonomously provisions a Telegraph account (saved in /workspace/.telegraph_account.json)
// if one does not exist yet, and publishes a public page at https://telegra.ph/<slug>.
func PublishToTelegraph(ctx context.Context, sbx core.Sandbox, title string, markdownBody string) (string, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		title = "ACE Autonomous Venture Update"
	}
	markdownBody = strings.TrimSpace(markdownBody)
	if markdownBody == "" {
		return "", errors.New("telegraph content cannot be empty")
	}

	client := newSafePublicHTTPClient()
	acct, err := ensureTelegraphAccount(ctx, client, sbx)
	if err != nil {
		return "", fmt.Errorf("telegraph account setup: %w", err)
	}

	nodes := markdownToTelegraphNodes(markdownBody)
	nodesJSON, err := json.Marshal(nodes)
	if err != nil {
		return "", err
	}

	form := url.Values{}
	form.Set("access_token", acct.AccessToken)
	form.Set("title", title)
	form.Set("author_name", acct.AuthorName)
	form.Set("content", string(nodesJSON))
	form.Set("return_content", "false")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.telegra.ph/createPage", strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		OK     bool   `json:"ok"`
		Error  string `json:"error"`
		Result struct {
			Path  string `json:"path"`
			URL   string `json:"url"`
			Title string `json:"title"`
		} `json:"result"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 64*1024)).Decode(&result); err != nil {
		return "", err
	}
	if !result.OK {
		return "", fmt.Errorf("telegraph createPage error: %s", result.Error)
	}

	return fmt.Sprintf("Published live public page: %s (author=%s)", result.Result.URL, acct.AuthorName), nil
}

func ensureTelegraphAccount(ctx context.Context, client *http.Client, sbx core.Sandbox) (telegraphAccount, error) {
	if raw, err := sbx.ReadFile(".telegraph_account.json"); err == nil && strings.TrimSpace(raw) != "" {
		var existing telegraphAccount
		if json.Unmarshal([]byte(raw), &existing) == nil && existing.AccessToken != "" {
			return existing, nil
		}
	}

	form := url.Values{}
	form.Set("short_name", "ace_venture")
	form.Set("author_name", "ACE Autonomous Labs")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.telegra.ph/createAccount", strings.NewReader(form.Encode()))
	if err != nil {
		return telegraphAccount{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		return telegraphAccount{}, err
	}
	defer resp.Body.Close()

	var res struct {
		OK     bool             `json:"ok"`
		Error  string           `json:"error"`
		Result telegraphAccount `json:"result"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 32*1024)).Decode(&res); err != nil {
		return telegraphAccount{}, err
	}
	if !res.OK || res.Result.AccessToken == "" {
		return telegraphAccount{}, fmt.Errorf("createAccount failed: %s", res.Error)
	}
	res.Result.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	if b, err := json.MarshalIndent(res.Result, "", "  "); err == nil {
		_ = sbx.WriteFile(".telegraph_account.json", string(b))
	}
	return res.Result, nil
}

func markdownToTelegraphNodes(md string) []telegraphNode {
	var nodes []telegraphNode
	lines := strings.Split(md, "\n")
	var inCodeBlock bool
	var codeBuf strings.Builder

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			if inCodeBlock {
				nodes = append(nodes, telegraphNode{Tag: "pre", Children: []interface{}{strings.TrimSuffix(codeBuf.String(), "\n")}})
				codeBuf.Reset()
				inCodeBlock = false
			} else {
				inCodeBlock = true
			}
			continue
		}
		if inCodeBlock {
			codeBuf.WriteString(line + "\n")
			continue
		}
		if trimmed == "" {
			continue
		}
		if trimmed == "---" || trimmed == "***" {
			nodes = append(nodes, telegraphNode{Tag: "hr"})
			continue
		}
		// Convert Markdown table rows (| a | b |) into clean structured bullet lines (skipping separator rows |---|)
		if strings.HasPrefix(trimmed, "|") && strings.HasSuffix(trimmed, "|") {
			inner := strings.Trim(trimmed, "| ")
			if strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(inner, "-", ""), "|", ""), " ", "") == "" {
				continue
			}
			cells := strings.Split(inner, "|")
			var cleaned []string
			for _, c := range cells {
				c = strings.TrimSpace(strings.ReplaceAll(c, "**", ""))
				if c != "" {
					cleaned = append(cleaned, c)
				}
			}
			if len(cleaned) > 0 {
				nodes = append(nodes, telegraphNode{
					Tag:      "p",
					Children: parseInlineTelegraph("▸ " + strings.Join(cleaned, " — ")),
				})
			}
			continue
		}
		switch {
		case strings.HasPrefix(trimmed, "# "):
			nodes = append(nodes, telegraphNode{Tag: "h3", Children: parseInlineTelegraph(strings.TrimPrefix(trimmed, "# "))})
		case strings.HasPrefix(trimmed, "## "):
			nodes = append(nodes, telegraphNode{Tag: "h4", Children: parseInlineTelegraph(strings.TrimPrefix(trimmed, "## "))})
		case strings.HasPrefix(trimmed, "### "):
			nodes = append(nodes, telegraphNode{Tag: "h4", Children: parseInlineTelegraph(strings.TrimPrefix(trimmed, "### "))})
		case strings.HasPrefix(trimmed, "> "):
			nodes = append(nodes, telegraphNode{Tag: "blockquote", Children: parseInlineTelegraph(strings.TrimPrefix(trimmed, "> "))})
		case strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* "):
			nodes = append(nodes, telegraphNode{Tag: "p", Children: parseInlineTelegraph("• " + trimmed[2:])})
		default:
			nodes = append(nodes, telegraphNode{Tag: "p", Children: parseInlineTelegraph(trimmed)})
		}
	}
	if len(nodes) == 0 {
		nodes = append(nodes, telegraphNode{Tag: "p", Children: []interface{}{md}})
	}
	return nodes
}

func parseInlineTelegraph(text string) []interface{} {
	// Strip raw backticks and bold markers into clean Telegraph strong/code nodes
	var out []interface{}
	rem := text
	for len(rem) > 0 {
		boldIdx := strings.Index(rem, "**")
		codeIdx := strings.Index(rem, "`")

		if boldIdx >= 0 && (codeIdx < 0 || boldIdx < codeIdx) {
			endIdx := strings.Index(rem[boldIdx+2:], "**")
			if endIdx >= 0 {
				if boldIdx > 0 {
					out = append(out, rem[:boldIdx])
				}
				boldText := rem[boldIdx+2 : boldIdx+2+endIdx]
				out = append(out, telegraphNode{Tag: "strong", Children: []interface{}{boldText}})
				rem = rem[boldIdx+4+endIdx:]
				continue
			}
		}
		if codeIdx >= 0 {
			endIdx := strings.Index(rem[codeIdx+1:], "`")
			if endIdx >= 0 {
				if codeIdx > 0 {
					out = append(out, rem[:codeIdx])
				}
				codeText := rem[codeIdx+1 : codeIdx+1+endIdx]
				out = append(out, telegraphNode{Tag: "code", Children: []interface{}{codeText}})
				rem = rem[codeIdx+2+endIdx:]
				continue
			}
		}
		out = append(out, rem)
		break
	}
	return out
}

type mailTmAccount struct {
	ID        string `json:"id"`
	Address   string `json:"address"`
	Password  string `json:"password"`
	Token     string `json:"token"`
	CreatedAt string `json:"created_at"`
}

// ExecuteMailTm autonomously creates a real disposable email inbox via api.mail.tm (saved in /workspace/.mail_tm_account.json)
// and reads incoming messages/verification links.
func ExecuteMailTm(ctx context.Context, sbx core.Sandbox, subcmd string) (string, error) {
	client := newSafePublicHTTPClient()
	acct, err := ensureMailTmAccount(ctx, client, sbx)
	if err != nil {
		return "", err
	}

	subcmd = strings.ToLower(strings.TrimSpace(subcmd))
	if subcmd == "address" || subcmd == "create" || subcmd == "account" {
		return fmt.Sprintf("Active Autonomous Email Inbox: %s (saved in /workspace/.mail_tm_account.json)", acct.Address), nil
	}

	// Fetch inbox messages
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.mail.tm/messages", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+acct.Token)
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	var list struct {
		HydraMember []struct {
			ID      string `json:"id"`
			Subject string `json:"subject"`
			Intro   string `json:"intro"`
			From    struct {
				Address string `json:"address"`
			} `json:"from"`
			CreatedAt string `json:"createdAt"`
		} `json:"hydra:member"`
	}
	_ = json.Unmarshal(raw, &list)

	if len(list.HydraMember) == 0 {
		return fmt.Sprintf("Inbox %s is active (0 messages currently).", acct.Address), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Inbox %s (%d messages):\n", acct.Address, len(list.HydraMember)))
	for i, m := range list.HydraMember {
		if i >= 5 {
			break
		}
		sb.WriteString(fmt.Sprintf("%d. [%s] From: %s | Subject: %s\n   Preview: %s\n", i+1, m.ID, m.From.Address, m.Subject, m.Intro))
	}
	return sb.String(), nil
}

func ensureMailTmAccount(ctx context.Context, client *http.Client, sbx core.Sandbox) (mailTmAccount, error) {
	if raw, err := sbx.ReadFile(".mail_tm_account.json"); err == nil && strings.TrimSpace(raw) != "" {
		var existing mailTmAccount
		if json.Unmarshal([]byte(raw), &existing) == nil && existing.Address != "" && existing.Token != "" {
			return existing, nil
		}
	}

	// 1. Get available domain
	domReq, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.mail.tm/domains", nil)
	if err != nil {
		return mailTmAccount{}, err
	}
	domResp, err := client.Do(domReq)
	if err != nil {
		return mailTmAccount{}, err
	}
	defer domResp.Body.Close()

	var domData struct {
		HydraMember []struct {
			Domain   string `json:"domain"`
			IsActive bool   `json:"isActive"`
		} `json:"hydra:member"`
	}
	if err := json.NewDecoder(io.LimitReader(domResp.Body, 32*1024)).Decode(&domData); err != nil {
		return mailTmAccount{}, err
	}
	domain := ""
	for _, d := range domData.HydraMember {
		if d.IsActive && d.Domain != "" {
			domain = d.Domain
			break
		}
	}
	if domain == "" && len(domData.HydraMember) > 0 {
		domain = domData.HydraMember[0].Domain
	}
	if domain == "" {
		return mailTmAccount{}, errors.New("no active mail.tm domain available")
	}

	randBytes := make([]byte, 4)
	_, _ = rand.Read(randBytes)
	suffix := hex.EncodeToString(randBytes)
	address := fmt.Sprintf("ace-labs-%s@%s", suffix, domain)
	password := "AceVenture!" + suffix

	// 2. Create account
	bodyBytes, _ := json.Marshal(map[string]string{
		"address":  address,
		"password": password,
	})
	accReq, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.mail.tm/accounts", bytes.NewReader(bodyBytes))
	if err != nil {
		return mailTmAccount{}, err
	}
	accReq.Header.Set("Content-Type", "application/json")
	accResp, err := client.Do(accReq)
	if err != nil {
		return mailTmAccount{}, err
	}
	defer accResp.Body.Close()
	var created struct {
		ID string `json:"id"`
	}
	_ = json.NewDecoder(io.LimitReader(accResp.Body, 32*1024)).Decode(&created)

	// 3. Obtain JWT Token
	tokReq, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.mail.tm/token", bytes.NewReader(bodyBytes))
	if err != nil {
		return mailTmAccount{}, err
	}
	tokReq.Header.Set("Content-Type", "application/json")
	tokResp, err := client.Do(tokReq)
	if err != nil {
		return mailTmAccount{}, err
	}
	defer tokResp.Body.Close()
	var tokData struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(io.LimitReader(tokResp.Body, 32*1024)).Decode(&tokData); err != nil || tokData.Token == "" {
		return mailTmAccount{}, errors.New("failed to obtain mail.tm JWT token")
	}

	acct := mailTmAccount{
		ID:        created.ID,
		Address:   address,
		Password:  password,
		Token:     tokData.Token,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	if b, err := json.MarshalIndent(acct, "", "  "); err == nil {
		_ = sbx.WriteFile(".mail_tm_account.json", string(b))
	}
	return acct, nil
}

// SafeHTTPRequest performs an SSRF-isolated outbound HTTP request (GET, POST, PUT) to public APIs.
// target format: "POST https://api.example.com/v1/endpoint" (or just URL for POST/GET)
// content: JSON body or form body
func SafeHTTPRequest(ctx context.Context, target string, content string) (string, error) {
	target = strings.TrimSpace(target)
	method := http.MethodPost
	rawURL := target
	parts := strings.Fields(target)
	if len(parts) >= 2 {
		m := strings.ToUpper(parts[0])
		if m == "GET" || m == "POST" || m == "PUT" || m == "DELETE" || m == "PATCH" {
			method = m
			rawURL = parts[1]
		}
	} else if strings.TrimSpace(content) == "" {
		method = http.MethodGet
	}

	parsed, err := url.Parse(rawURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", errors.New("http_request requires a valid public http:// or https:// URL")
	}
	host := strings.ToLower(parsed.Hostname())
	if host == "localhost" || strings.HasSuffix(host, ".local") || strings.HasSuffix(host, ".internal") {
		return "", errors.New("SSRF guard: local/internal hostnames are blocked")
	}

	var bodyReader io.Reader
	if strings.TrimSpace(content) != "" && method != http.MethodGet {
		bodyReader = strings.NewReader(content)
	}

	req, err := http.NewRequestWithContext(ctx, method, rawURL, bodyReader)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", webSearchUserAgent)
	if bodyReader != nil {
		trimmed := strings.TrimSpace(content)
		if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
			req.Header.Set("Content-Type", "application/json")
		} else {
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		}
	}

	client := newSafePublicHTTPClient()
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(io.LimitReader(resp.Body, 32*1024))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("HTTP %d (%s %s):\n%s", resp.StatusCode, method, rawURL, truncateText(string(respBytes), 2400)), nil
}
